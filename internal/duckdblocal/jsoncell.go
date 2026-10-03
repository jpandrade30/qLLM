package duckdblocal

import (
	"encoding/json"

	"qLLM/internal/protocol"
)

// encodeJSONCell turns a Go object/list into a JSON string for DuckDB JSON columns.
func encodeJSONCell(v any) any {
	if v == nil {
		return nil
	}
	switch v.(type) {
	case string:
		return v
	case []byte:
		return string(v.([]byte))
	case map[string]any, []any:
		b, err := json.Marshal(v)
		if err != nil {
			return v
		}
		return string(b)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return v
		}
		var probe any
		if err := json.Unmarshal(b, &probe); err != nil {
			return v
		}
		switch probe.(type) {
		case map[string]any, []any:
			return string(b)
		default:
			return v
		}
	}
}

func parseJSONCell(v any) any {
	if v == nil {
		return nil
	}
	switch t := v.(type) {
	case []byte:
		return parseJSONBytes(t)
	case string:
		return parseJSONBytes([]byte(t))
	default:
		return v
	}
}

func parseJSONBytes(b []byte) any {
	if len(b) == 0 {
		return nil
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return string(b)
	}
	return out
}

func normalizeSQLJSON(t protocol.LogicalType, v any) any {
	if t != protocol.TypeJSON {
		return v
	}
	return parseJSONCell(v)
}
