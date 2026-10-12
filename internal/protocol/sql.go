package protocol

import (
	"encoding/json"
	"fmt"
	"strconv"
)

const (
	SQLDialect1      = "1"
	SQLDialect2      = "2"
	SQLDialectLatest = SQLDialect2

	ConstraintModeValidate = "validate"
	ConstraintModeInject   = "inject"
)

type SQLRequest struct {
	Version        string         `json:"version,omitempty" yaml:"version,omitempty"`
	SQL            string         `json:"sql" yaml:"sql"`
	Constraints    map[string]any `json:"constraints,omitempty" yaml:"constraints,omitempty"`
	ConstraintMode string         `json:"constraintMode,omitempty" yaml:"constraintMode,omitempty"`
}

// ResolveSQLVersion resolves names or paths.
func ResolveSQLVersion(version string) (string, *ProtocolError) {
	if version == "" {
		return SQLDialectLatest, nil
	}
	switch version {
	case SQLDialect1, SQLDialect2:
		return version, nil
	default:
		return "", NewError(ErrUnsupportedVersion, "unsupported SQL dialect version: "+version, map[string]any{
			"version": version,
			"latest":  SQLDialectLatest,
		})
	}
}

// ResolveConstraintMode returns validate/inject when constraints are set; empty mode when none.
func ResolveConstraintMode(mode string, constraints map[string]any) (string, *ProtocolError) {
	if len(constraints) == 0 {
		if mode != "" {
			return "", NewError(ErrInvalidSQL, "constraintMode requires constraints", map[string]any{"constraintMode": mode})
		}
		return "", nil
	}
	if mode == "" {
		return ConstraintModeValidate, nil
	}
	switch mode {
	case ConstraintModeValidate, ConstraintModeInject:
		return mode, nil
	default:
		return "", NewError(ErrInvalidSQL, "unsupported constraintMode: "+mode, map[string]any{
			"constraintMode": mode,
		})
	}
}

// ConstraintValueString normalizes a JSON scalar constraint value.
func ConstraintValueString(v any) (string, *ProtocolError) {
	switch t := v.(type) {
	case nil:
		return "", NewError(ErrInvalidSQL, "constraint value must be a scalar", nil)
	case string:
		return t, nil
	case bool:
		if t {
			return "true", nil
		}
		return "false", nil
	case float64:
		// JSON numbers decode as float64
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10), nil
		}
		return strconv.FormatFloat(t, 'f', -1, 64), nil
	case int:
		return strconv.Itoa(t), nil
	case int64:
		return strconv.FormatInt(t, 10), nil
	case json.Number:
		return t.String(), nil
	default:
		return "", NewError(ErrInvalidSQL, "constraint value must be string, number, or boolean", map[string]any{
			"type": fmt.Sprintf("%T", v),
		})
	}
}

type AccessFile struct {
	ScopeMode string      `json:"scopeMode,omitempty" yaml:"scopeMode,omitempty"`
	Apps      []AccessApp `json:"apps" yaml:"apps"`
}

type AccessApp struct {
	Name           string            `json:"name" yaml:"name"`
	Key            string            `json:"key,omitempty" yaml:"key,omitempty"`
	KeySecret      string            `json:"keySecret,omitempty" yaml:"keySecret,omitempty"`
	Tables         []string          `json:"tables" yaml:"tables"`
	UnscopedTables []string          `json:"unscopedTables,omitempty" yaml:"unscopedTables,omitempty"`
	Scope          map[string]string `json:"scope,omitempty" yaml:"scope,omitempty"`
	ScopeMode      string            `json:"scopeMode,omitempty" yaml:"scopeMode,omitempty"`
}
