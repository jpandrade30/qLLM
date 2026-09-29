package access

import (
	"crypto/subtle"
	"fmt"
	"os"
	"regexp"
	"strings"

	"qLLM/internal/catalogidx"
	"qLLM/internal/protocol"
	"qLLM/internal/validate"
)

var envKey = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)

type App struct {
	Name   string
	Key    string
	Tables map[string]struct{}
}

type Registry struct {
	Apps []*App
}

func Resolve(file *protocol.AccessFile, idx *catalogidx.Index) (*Registry, *protocol.ProtocolError) {
	if file == nil {
		return nil, nil
	}
	if err := validate.Access(file); err != nil {
		return nil, err
	}
	reg := &Registry{}
	names := map[string]struct{}{}
	for i, raw := range file.Apps {
		if _, dup := names[raw.Name]; dup {
			return nil, protocol.NewError(protocol.ErrConfigError, "duplicate app name: "+raw.Name, map[string]any{"app": raw.Name})
		}
		names[raw.Name] = struct{}{}
		key, err := expandKey(raw.Key, i)
		if err != nil {
			return nil, err
		}
		tables := map[string]struct{}{}
		for _, t := range raw.Tables {
			ent, perr := idx.ResolveEntity(t)
			if perr != nil {
				return nil, protocol.NewError(protocol.ErrConfigError,
					fmt.Sprintf("app %s tables entry %q is not a catalog entity", raw.Name, t),
					map[string]any{"app": raw.Name, "table": t})
			}
			tables[ent.Name] = struct{}{}
		}
		reg.Apps = append(reg.Apps, &App{Name: raw.Name, Key: key, Tables: tables})
	}
	return reg, nil
}

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

func (r *Registry) LookupBearer(token string) *App {
	if r == nil || token == "" {
		return nil
	}
	got := []byte(token)
	var found *App
	for _, a := range r.Apps {
		want := []byte(a.Key)
		if len(got) != len(want) {
			_ = subtle.ConstantTimeCompare(want, want)
			continue
		}
		if subtle.ConstantTimeCompare(got, want) == 1 {
			found = a
		}
	}
	return found
}

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

func (a *App) Allows(entityName string) bool {
	if a == nil {
		return true
	}
	_, ok := a.Tables[entityName]
	return ok
}

func (a *App) TableSet() map[string]struct{} {
	if a == nil {
		return nil
	}
	return a.Tables
}
