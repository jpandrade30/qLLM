package executor

import (
	"strings"

	"qLLM/internal/access"
	"qLLM/internal/protocol"
	"qLLM/internal/sqlparse"
)

// normalizeConstraints maps field → string value; empty map if none.
func normalizeConstraints(raw map[string]any) (map[string]string, *protocol.ProtocolError) {
	if len(raw) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(raw))
	for k, v := range raw {
		key := strings.TrimSpace(k)
		if key == "" {
			return nil, protocol.NewError(protocol.ErrInvalidSQL, "constraint key must be non-empty", nil)
		}
		s, err := protocol.ConstraintValueString(v)
		if err != nil {
			err.Details = map[string]any{"field": key}
			return nil, err
		}
		out[key] = s
	}
	return out, nil
}

func checkConstraintsKnown(constraints map[string]string, scans []sqlScan) *protocol.ProtocolError {
	for field := range constraints {
		if !fieldOnScans(field, scans) {
			return protocol.NewError(protocol.ErrInvalidSQL,
				"unknown constraint field: "+field,
				map[string]any{"field": field})
		}
	}
	return nil
}

func fieldOnScans(field string, scans []sqlScan) bool {
	want := strings.ToLower(field)
	for _, sc := range scans {
		if sc.entity == nil {
			continue
		}
		if sc.entity.Scope != nil {
			if strings.EqualFold(sc.entity.Scope.Field, field) || strings.EqualFold(sc.entity.Scope.FilterField(), field) {
				return true
			}
		}
		for _, f := range sc.entity.Fields {
			if strings.ToLower(f.Name) == want {
				return true
			}
		}
	}
	return false
}

func checkConstraintsVsD21(app *access.App, constraints map[string]string) *protocol.ProtocolError {
	if app == nil || !app.HasScope() || len(constraints) == 0 {
		return nil
	}
	for field, want := range constraints {
		if cred, ok := app.ScopeValue(field); ok && cred != want {
			return protocol.NewError(protocol.ErrForbiddenScope,
				"constraint conflicts with credential scope",
				map[string]any{"field": field, "constraint": want, "scope": cred})
		}
	}
	return nil
}

func validateSQLConstraints(sql string, constraints map[string]string) *protocol.ProtocolError {
	if len(constraints) == 0 {
		return nil
	}
	ex, err := sqlparse.ExtractEqualityFilters(sql)
	if err != nil {
		return err
	}
	for field, want := range constraints {
		vals := ex.ValuesForField(field)
		if len(vals) == 0 {
			// Absent from WHERE: allowed in validate (and inject still forces on fetch).
			// If WHERE is ambiguous, still allow when we cannot attribute eqs to this field.
			continue
		}
		if ex.Ambiguous {
			return protocol.NewError(protocol.ErrForbiddenScope,
				"constraint field appears under OR/NOT/IN (ambiguous WHERE)",
				map[string]any{"field": field})
		}
		for _, got := range vals {
			if got != want {
				return protocol.NewError(protocol.ErrForbiddenScope,
					"SQL equality conflicts with constraint",
					map[string]any{"field": field, "sqlValue": got, "constraint": want})
			}
		}
	}
	return nil
}

func mergeConstraintWhere(where map[string]any, ent *protocol.Entity, constraints map[string]string, mode string) map[string]any {
	if mode != protocol.ConstraintModeInject || len(constraints) == 0 || ent == nil {
		return where
	}
	for field, val := range constraints {
		col := constraintPushdownField(ent, field)
		if col == "" {
			continue
		}
		where = andEq(where, col, val)
	}
	return where
}

func constraintPushdownField(ent *protocol.Entity, field string) string {
	want := strings.ToLower(field)
	if ent.Scope != nil {
		if strings.EqualFold(ent.Scope.Field, field) {
			return ent.Scope.FilterField()
		}
	}
	for _, f := range ent.Fields {
		if strings.ToLower(f.Name) == want {
			return f.Name
		}
	}
	return ""
}
