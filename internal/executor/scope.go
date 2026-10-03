package executor

import (
	"fmt"
	"strings"

	"qLLM/internal/access"
	"qLLM/internal/connector/def"
	"qLLM/internal/planner"
	"qLLM/internal/protocol"
)

func applyScope(plan *planner.Plan, q *protocol.QueryIR, app *access.App) *protocol.ProtocolError {
	if app == nil || !app.HasScope() || plan == nil {
		return nil
	}
	multi := q != nil && len(q.Joins) > 0
	for i := range plan.Steps {
		if err := applyStepScope(&plan.Steps[i], q, app, multi); err != nil {
			return err
		}
	}
	return nil
}

func applyStepScope(step *def.PushdownStep, q *protocol.QueryIR, app *access.App, multi bool) *protocol.ProtocolError {
	ent := step.Entity
	if ent == nil || ent.Scope == nil || ent.Scope.Field == "" {
		return nil
	}
	col := ent.Scope.FilterField()
	val, ok := app.ScopeValue(ent.Scope.Field)
	if !ok {
		return protocol.NewError(protocol.ErrForbiddenScope,
			"this key has no scope value for "+ent.Scope.Field,
			map[string]any{"field": ent.Scope.Field, "entity": ent.Name})
	}
	bind := step.Binding
	if firstConflictingEq(step.Where, col, bind, val) != "" ||
		(q != nil && firstConflictingEq(q.Where, col, bind, val) != "") {
		if app.ScopeMode != access.ScopeModeInject {
			return forbiddenScope(col, val)
		}
	}
	step.Where = andEq(step.Where, col, val)
	if q != nil {
		field := col
		if multi {
			field = bind + "." + col
		}
		q.Where = andEq(q.Where, field, val)
	}
	return nil
}

func forbiddenScope(field, want string) *protocol.ProtocolError {
	return protocol.NewError(protocol.ErrForbiddenScope,
		fmt.Sprintf("this key is scoped to %s; remove or match that filter", field),
		map[string]any{"field": field, "want": want})
}

func andEq(w map[string]any, field, value string) map[string]any {
	pred := map[string]any{"op": "eq", "field": field, "value": value}
	if w == nil {
		return pred
	}
	if matchingOnlyEq(w, field, value) {
		return w
	}
	return map[string]any{"op": "and", "args": []any{w, pred}}
}

func matchingOnlyEq(w map[string]any, field, want string) bool {
	if w == nil {
		return false
	}
	op, _ := w["op"].(string)
	f, _ := w["field"].(string)
	if (op == "eq" || op == "") && logicalField(f) == logicalField(field) {
		return fmt.Sprint(w["value"]) == want
	}
	return false
}

func firstConflictingEq(w map[string]any, field, bind, want string) string {
	if w == nil {
		return ""
	}
	op, _ := w["op"].(string)
	if op == "and" || op == "or" || op == "not" {
		args, _ := w["args"].([]any)
		for _, a := range args {
			m, _ := a.(map[string]any)
			if c := firstConflictingEq(m, field, bind, want); c != "" {
				return c
			}
		}
		return ""
	}
	f, _ := w["field"].(string)
	if !sameScopeField(f, field, bind) {
		return ""
	}
	if op != "eq" && op != "" {
		return fmt.Sprint(w["value"])
	}
	got := fmt.Sprint(w["value"])
	if got != want {
		return got
	}
	return ""
}

func sameScopeField(ref, field, bind string) bool {
	if logicalField(ref) != logicalField(field) {
		return false
	}
	if i := strings.LastIndex(ref, "."); i >= 0 {
		return bind == "" || ref[:i] == bind
	}
	return true
}

func logicalField(ref string) string {
	if i := strings.LastIndex(ref, "."); i >= 0 {
		return ref[i+1:]
	}
	return ref
}
