package access

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"qLLM/internal/catalogidx"
	"qLLM/internal/cryptox"
	"qLLM/internal/protocol"
	"qLLM/internal/validate"
)

var envKey = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)
var derivedAppName = regexp.MustCompile(`^[a-z][a-z0-9_-]*$`)

const (
	ScopeModeReject = "reject"
	ScopeModeInject = "inject"
)

type App struct {
	Name        string
	Key         string
	KeySecret   string
	Tables      map[string]struct{}
	Unscoped    map[string]struct{}
	ScopeField  string
	ScopeValues map[string]string
	ScopeMode   string
}

type Registry struct {
	Apps []*App
}

// HasScope reports whether this app carries a row-scope template or values.
func (a *App) HasScope() bool {
	if a == nil {
		return false
	}
	return a.ScopeField != "" || len(a.ScopeValues) > 0
}

// ScopeValue returns the credential value for a catalog scope key.
func (a *App) ScopeValue(field string) (string, bool) {
	if a == nil || field == "" {
		return "", false
	}
	if v, ok := a.ScopeValues[field]; ok && v != "" {
		return v, true
	}
	return "", false
}

func (a *App) clone() *App {
	if a == nil {
		return nil
	}
	out := *a
	if a.Tables != nil {
		out.Tables = make(map[string]struct{}, len(a.Tables))
		for k, v := range a.Tables {
			out.Tables[k] = v
		}
	}
	if a.Unscoped != nil {
		out.Unscoped = make(map[string]struct{}, len(a.Unscoped))
		for k, v := range a.Unscoped {
			out.Unscoped[k] = v
		}
	}
	if a.ScopeValues != nil {
		out.ScopeValues = make(map[string]string, len(a.ScopeValues))
		for k, v := range a.ScopeValues {
			out.ScopeValues[k] = v
		}
	}
	return &out
}

// Resolve resolves names or paths.
func Resolve(file *protocol.AccessFile, idx *catalogidx.Index) (*Registry, *protocol.ProtocolError) {
	if file == nil {
		return nil, nil
	}
	if err := validate.Access(file); err != nil {
		return nil, err
	}
	fileMode := file.ScopeMode
	if fileMode == "" {
		fileMode = ScopeModeReject
	}
	if fileMode != ScopeModeReject && fileMode != ScopeModeInject {
		return nil, protocol.NewError(protocol.ErrConfigError, "scopeMode must be reject or inject", nil)
	}
	reg := &Registry{}
	names := map[string]struct{}{}
	for i, raw := range file.Apps {
		if _, dup := names[raw.Name]; dup {
			return nil, protocol.NewError(protocol.ErrConfigError, "duplicate app name: "+raw.Name, map[string]any{"app": raw.Name})
		}
		names[raw.Name] = struct{}{}
		hasKey := strings.TrimSpace(raw.Key) != ""
		hasSecret := strings.TrimSpace(raw.KeySecret) != ""
		if hasKey == hasSecret {
			return nil, protocol.NewError(protocol.ErrConfigError,
				"app must set exactly one of key or keySecret", map[string]any{"app": raw.Name})
		}
		var key, secret string
		var err *protocol.ProtocolError
		if hasKey {
			key, err = expandKey(raw.Key, i)
			if err != nil {
				return nil, err
			}
		} else {
			secret, err = expandKey(raw.KeySecret, i)
			if err != nil {
				return nil, err
			}
			if !derivedAppName.MatchString(raw.Name) {
				return nil, protocol.NewError(protocol.ErrConfigError,
					"app name for keySecret must match [a-z][a-z0-9_-]*: "+raw.Name,
					map[string]any{"app": raw.Name})
			}
		}
		scopeField, scopeValues := parseAppScope(raw.Scope)
		if hasSecret && scopeField == "" {
			return nil, protocol.NewError(protocol.ErrConfigError,
				"keySecret requires scope.field", map[string]any{"app": raw.Name})
		}
		mode := raw.ScopeMode
		if mode == "" {
			mode = fileMode
		}
		if mode != ScopeModeReject && mode != ScopeModeInject {
			return nil, protocol.NewError(protocol.ErrConfigError, "scopeMode must be reject or inject", map[string]any{"app": raw.Name})
		}
		tables := map[string]struct{}{}
		for _, t := range raw.Tables {
			if t == "*" {
				for _, e := range idx.Catalog.Entities {
					tables[e.Name] = struct{}{}
				}
				continue
			}
			ent, perr := idx.ResolveEntity(t)
			if perr != nil {
				return nil, protocol.NewError(protocol.ErrConfigError,
					fmt.Sprintf("app %s tables entry %q is not a catalog entity", raw.Name, t),
					map[string]any{"app": raw.Name, "table": t})
			}
			tables[ent.Name] = struct{}{}
		}
		unscoped := map[string]struct{}{}
		for _, t := range raw.UnscopedTables {
			ent, perr := idx.ResolveEntity(t)
			if perr != nil {
				return nil, protocol.NewError(protocol.ErrConfigError,
					fmt.Sprintf("app %s unscopedTables entry %q is not a catalog entity", raw.Name, t),
					map[string]any{"app": raw.Name, "table": t})
			}
			if _, ok := tables[ent.Name]; !ok {
				return nil, protocol.NewError(protocol.ErrConfigError,
					fmt.Sprintf("app %s unscopedTables entry %q is not in tables", raw.Name, t),
					map[string]any{"app": raw.Name, "table": t})
			}
			unscoped[ent.Name] = struct{}{}
		}
		app := &App{
			Name: raw.Name, Key: key, KeySecret: secret, Tables: tables, Unscoped: unscoped,
			ScopeField: scopeField, ScopeValues: scopeValues, ScopeMode: mode,
		}
		if err := checkScopedTables(app, idx); err != nil {
			return nil, err
		}
		reg.Apps = append(reg.Apps, app)
	}
	return reg, nil
}

func parseAppScope(scope map[string]string) (field string, values map[string]string) {
	if len(scope) == 0 {
		return "", nil
	}
	values = map[string]string{}
	for k, v := range scope {
		if k == "field" {
			field = v
			continue
		}
		values[k] = v
	}
	return field, values
}

func checkScopedTables(app *App, idx *catalogidx.Index) *protocol.ProtocolError {
	if !app.HasScope() {
		return nil
	}
	for name := range app.Tables {
		ent, err := idx.ResolveEntity(name)
		if err != nil {
			return err
		}
		if ent.Scope != nil && ent.Scope.Field != "" {
			continue
		}
		if _, ok := app.Unscoped[name]; ok {
			continue
		}
		return protocol.NewError(protocol.ErrConfigError,
			fmt.Sprintf("app %s table %q has no entity scope and is not in unscopedTables", app.Name, name),
			map[string]any{"app": app.Name, "table": name})
	}
	return nil
}

// expandKey implements runtime behavior for this package.
func expandKey(raw string, idx int) (string, *protocol.ProtocolError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", protocol.NewError(protocol.ErrConfigError, "app key is empty", map[string]any{"index": idx})
	}
	if strings.Contains(raw, "${") {
		m := envKey.FindStringSubmatch(raw)
		if m == nil {
			return "", protocol.NewError(protocol.ErrConfigError,
				"key env placeholder must be exactly ${ENV_NAME}", map[string]any{"index": idx})
		}
		val := os.Getenv(m[1])
		if val == "" {
			return "", protocol.NewError(protocol.ErrConfigError,
				fmt.Sprintf("environment variable %s is empty (required by app key)", m[1]),
				map[string]any{"env": m[1]})
		}
		return val, nil
	}
	return raw, nil
}

// LookupBearer implements runtime behavior for this package.
func (r *Registry) LookupBearer(token string) *App {
	if r == nil || token == "" {
		return nil
	}
	var found *App
	for _, a := range r.Apps {
		if a.Key != "" && cryptox.HMACEqual(token, a.Key) {
			found = a
		}
	}
	if found != nil {
		return found.clone()
	}
	parsed, ok := parseDerivedKey(token)
	if !ok {
		return nil
	}
	tmpl := r.LookupName(parsed.App)
	if tmpl == nil || tmpl.KeySecret == "" {
		return nil
	}
	if !verifyDerived(tmpl.KeySecret, parsed) {
		return nil
	}
	out := tmpl.clone()
	if out.ScopeValues == nil {
		out.ScopeValues = map[string]string{}
	}
	out.ScopeValues[out.ScopeField] = parsed.Value
	return out
}

// LookupName implements runtime behavior for this package.
func (r *Registry) LookupName(name string) *App {
	if r == nil {
		return nil
	}
	for _, a := range r.Apps {
		if a.Name == name {
			return a
		}
	}
	return nil
}

// Allows computes an allowlist.
func (a *App) Allows(entityName string) bool {
	if a == nil {
		return true
	}
	_, ok := a.Tables[entityName]
	return ok
}

// TableSet implements runtime behavior for this package.
func (a *App) TableSet() map[string]struct{} {
	if a == nil {
		return nil
	}
	return a.Tables
}
