package catalogidx

import "qLLM/internal/protocol"

// ACL is a table (+ optional field) allowlist for catalog projection (D16/D24).
// nil ACL = full catalog. Tables empty map = no entities.
// Fields nil/empty = no field restrictions. Missing entity key under Fields = all fields for that table.
type ACL struct {
	Tables map[string]struct{}
	Fields map[string]map[string]struct{}
}

// AllowsTable reports whether the entity is visible.
func (a *ACL) AllowsTable(name string) bool {
	if a == nil {
		return true
	}
	_, ok := a.Tables[name]
	return ok
}

// AllowsField reports whether entity.field is visible.
func (a *ACL) AllowsField(entity, field string) bool {
	if a == nil {
		return true
	}
	if !a.AllowsTable(entity) {
		return false
	}
	if len(a.Fields) == 0 {
		return true
	}
	set, restricted := a.Fields[entity]
	if !restricted {
		return true
	}
	_, ok := set[field]
	return ok
}

// projectEntity copies e keeping only allowed fields and relations whose join keys remain.
func (a *ACL) projectEntity(e protocol.Entity) protocol.Entity {
	if a == nil || len(a.Fields) == 0 {
		return e
	}
	set, restricted := a.Fields[e.Name]
	if !restricted {
		return e
	}
	cp := e
	cp.Fields = nil
	for _, f := range e.Fields {
		if _, ok := set[f.Name]; ok {
			cp.Fields = append(cp.Fields, f)
		}
	}
	cp.Relations = nil
	for _, r := range e.Relations {
		if !a.AllowsTable(r.To) {
			continue
		}
		ok := true
		for _, pair := range r.On {
			if len(pair) >= 1 && !a.AllowsField(e.Name, pair[0]) {
				ok = false
				break
			}
			if len(pair) >= 2 && !a.AllowsField(r.To, pair[1]) {
				ok = false
				break
			}
		}
		if ok {
			cp.Relations = append(cp.Relations, r)
		}
	}
	return cp
}
