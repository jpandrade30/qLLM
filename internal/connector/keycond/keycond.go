package keycond

import (
	"strings"

	"qLLM/internal/protocol"
)

// EqValues collects field→value for op=eq (and nested and). Other shapes return UNSUPPORTED.
func EqValues(where map[string]any) (map[string]any, *protocol.ProtocolError) {
	out := map[string]any{}
	if where == nil {
		return out, nil
	}
	if err := walk(where, out); err != nil {
		return nil, err
	}
	return out, nil
}

func walk(w map[string]any, out map[string]any) *protocol.ProtocolError {
	if op, ok := w["op"].(string); ok && (op == "and" || op == "or" || op == "not") {
		if op != "and" {
			return protocol.NewError(protocol.ErrUnsupported,
				"key-addressed sources only accept AND of equality predicates", nil)
		}
		args, _ := w["args"].([]any)
		for _, a := range args {
			m, ok := a.(map[string]any)
			if !ok {
				return protocol.NewError(protocol.ErrInvalidIR, "invalid where arg", nil)
			}
			if err := walk(m, out); err != nil {
				return err
			}
		}
		return nil
	}
	field, _ := w["field"].(string)
	op, _ := w["op"].(string)
	if field == "" || op == "" {
		return protocol.NewError(protocol.ErrInvalidIR, "invalid compare expr", nil)
	}
	if op != "eq" {
		return nil
	}
	if i := strings.LastIndex(field, "."); i >= 0 {
		field = field[i+1:]
	}
	out[field] = w["value"]
	return nil
}

func RequireEq(where map[string]any, logicalKeys []string) (map[string]any, *protocol.ProtocolError) {
	if len(logicalKeys) == 0 {
		return nil, protocol.NewError(protocol.ErrUnsupported,
			"entity binding.accessPath is required (partition/pk or ksqlKey)", nil)
	}
	eqs, err := EqValues(where)
	if err != nil {
		return nil, err
	}
	for _, k := range logicalKeys {
		if _, ok := eqs[k]; !ok {
			return nil, protocol.NewError(protocol.ErrUnsupported,
				"query must include equality on access path field "+k+" (no table scan)",
				map[string]any{"field": k})
		}
	}
	return eqs, nil
}
