package validate

import (
	"fmt"
	"strings"

	"qLLM/internal/protocol"
)

var sqlOpHints = map[string]string{
	"=":       "eq",
	"==":      "eq",
	"!=":      "neq",
	"<>":      "neq",
	">":       "gt",
	">=":      "gte",
	"<":       "lt",
	"<=":      "lte",
	"like":    "contains",
	"BETWEEN": "gte/lte",
	"between": "gte/lte",
}

// lintQueryIRLLM catches common LLM shape mistakes before JSON Schema noise.
func lintQueryIRLLM(q *protocol.QueryIR) *protocol.ProtocolError {
	if err := lintWhereLLM(q.Where); err != nil {
		return err
	}
	return lintSelectGroupByLLM(q)
}

// lintWhereLLM implements runtime behavior for this package.
func lintWhereLLM(w map[string]any) *protocol.ProtocolError {
	if w == nil {
		return nil
	}
	for _, key := range []string{"and", "or", "not"} {
		if _, has := w[key]; has {
			if _, hasOp := w["op"]; !hasOp {
				return protocol.NewError(protocol.ErrInvalidIR,
					fmt.Sprintf(`where must use {"op":"%s","args":[...]} — not {"%s":[...]}`, key, key),
					map[string]any{
						"got":      w,
						"expected": map[string]any{"op": key, "args": []any{"/* BoolExpr */"}},
					})
			}
		}
	}
	op, _ := w["op"].(string)
	if hint, ok := sqlOpHints[op]; ok {
		return protocol.NewError(protocol.ErrInvalidIR,
			fmt.Sprintf(`where.op %q is SQL-style; use CompareOp %q (eq|neq|gt|gte|lt|lte|in|nin|contains|is_null|not_null)`, op, hint),
			map[string]any{"got": op, "expected": hint})
	}
	switch op {
	case "and", "or", "not":
		args, _ := w["args"].([]any)
		for _, a := range args {
			m, ok := a.(map[string]any)
			if !ok {
				continue
			}
			if err := lintWhereLLM(m); err != nil {
				return err
			}
		}
	}
	return nil
}

// lintSelectGroupByLLM implements runtime behavior for this package.
func lintSelectGroupByLLM(q *protocol.QueryIR) *protocol.ProtocolError {
	hasBare := false
	hasAgg := false
	bares := []string{}
	for _, item := range q.Select {
		switch v := item.(type) {
		case string:
			hasBare = true
			bares = append(bares, v)
		default:
			m, ok := asMap(item)
			if !ok {
				continue
			}
			if agg, _ := m["agg"].(string); agg != "" {
				hasAgg = true
			}
		}
	}
	if hasBare && hasAgg && len(q.GroupBy) == 0 {
		return protocol.NewError(protocol.ErrInvalidIR,
			"select mixes bare fields and aggregates: every bare field must appear in groupBy",
			map[string]any{
				"bareFields": bares,
				"expected":   map[string]any{"groupBy": bares},
			})
	}
	if hasBare && hasAgg && len(q.GroupBy) > 0 {
		gb := map[string]struct{}{}
		for _, g := range q.GroupBy {
			gb[g] = struct{}{}
			// also allow unqualified match on last segment
			if i := strings.LastIndex(g, "."); i >= 0 {
				gb[g[i+1:]] = struct{}{}
			}
		}
		missing := []string{}
		for _, b := range bares {
			if _, ok := gb[b]; ok {
				continue
			}
			col := b
			if i := strings.LastIndex(b, "."); i >= 0 {
				col = b[i+1:]
			}
			if _, ok := gb[col]; ok {
				continue
			}
			missing = append(missing, b)
		}
		if len(missing) > 0 {
			return protocol.NewError(protocol.ErrInvalidIR,
				"every selected bare field must appear in groupBy",
				map[string]any{"missingFromGroupBy": missing, "groupBy": q.GroupBy})
		}
	}
	return nil
}
