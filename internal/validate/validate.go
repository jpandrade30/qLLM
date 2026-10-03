package validate

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"embed"

	"qLLM/internal/catalogidx"
	"qLLM/internal/protocol"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed schemas/*.schema.json
var schemaFS embed.FS

var (
	presetSchema  *jsonschema.Schema
	catalogSchema *jsonschema.Schema
	querySchema   *jsonschema.Schema
	accessSchema  *jsonschema.Schema
	sqlReqSchema  *jsonschema.Schema
	envFileSchema *jsonschema.Schema
	queryRespSchema *jsonschema.Schema
)

// init registers package defaults.
func init() {
	c := jsonschema.NewCompiler()
	c.Draft = jsonschema.Draft2020
	mustAdd := func(name string) {
		b, err := schemaFS.ReadFile("schemas/" + name)
		if err != nil {
			panic(err)
		}
		if err := c.AddResource("https://qllm.dev/schemas/"+name, bytes.NewReader(b)); err != nil {
			panic(err)
		}
	}
	mustAdd("preset.schema.json")
	mustAdd("catalog.schema.json")
	mustAdd("query-ir.schema.json")
	mustAdd("access.schema.json")
	mustAdd("sql-request.schema.json")
	mustAdd("env-file.schema.json")
	mustAdd("query-response.schema.json")
	var err error
	presetSchema, err = c.Compile("https://qllm.dev/schemas/preset.schema.json")
	if err != nil {
		panic(err)
	}
	catalogSchema, err = c.Compile("https://qllm.dev/schemas/catalog.schema.json")
	if err != nil {
		panic(err)
	}
	querySchema, err = c.Compile("https://qllm.dev/schemas/query-ir.schema.json")
	if err != nil {
		panic(err)
	}
	accessSchema, err = c.Compile("https://qllm.dev/schemas/access.schema.json")
	if err != nil {
		panic(err)
	}
	sqlReqSchema, err = c.Compile("https://qllm.dev/schemas/sql-request.schema.json")
	if err != nil {
		panic(err)
	}
	envFileSchema, err = c.Compile("https://qllm.dev/schemas/env-file.schema.json")
	if err != nil {
		panic(err)
	}
	queryRespSchema, err = c.Compile("https://qllm.dev/schemas/query-response.schema.json")
	if err != nil {
		panic(err)
	}
}

// toJSONValue implements runtime behavior for this package.
func toJSONValue(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	if err := json.Unmarshal(b, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// validateSchema implements runtime behavior for this package.
func validateSchema(s *jsonschema.Schema, v any, code protocol.ErrorCode) *protocol.ProtocolError {
	doc, err := toJSONValue(v)
	if err != nil {
		return protocol.NewError(code, err.Error(), nil)
	}
	if err := s.Validate(doc); err != nil {
		return protocol.NewError(code, err.Error(), nil)
	}
	return nil
}

// Preset implements runtime behavior for this package.
func Preset(p *protocol.Preset) *protocol.ProtocolError {
	if err := validateSchema(presetSchema, p, protocol.ErrConfigError); err != nil {
		return err
	}
	if p.Limits.DefaultLimit > p.Limits.MaxLimit {
		return protocol.NewError(protocol.ErrConfigError, "defaultLimit cannot exceed maxLimit", nil)
	}
	return nil
}

// Catalog builds catalog data.
func Catalog(c *protocol.Catalog) *protocol.ProtocolError {
	return validateSchema(catalogSchema, c, protocol.ErrConfigError)
}

// QuerySchema fetches rows from a source.
func QuerySchema(q *protocol.QueryIR) *protocol.ProtocolError {
	return validateSchema(querySchema, q, protocol.ErrInvalidIR)
}

// Access implements runtime behavior for this package.
func Access(a *protocol.AccessFile) *protocol.ProtocolError {
	return validateSchema(accessSchema, a, protocol.ErrConfigError)
}

// SQLRequest implements runtime behavior for this package.
func SQLRequest(r *protocol.SQLRequest) *protocol.ProtocolError {
	return validateSchema(sqlReqSchema, r, protocol.ErrInvalidSQL)
}

// EnvFile implements runtime behavior for this package.
func EnvFile(e *protocol.EnvFile) *protocol.ProtocolError {
	return validateSchema(envFileSchema, e, protocol.ErrConfigError)
}

// QueryResponse checks a serialized response against the public schema (tests and contract).
func QueryResponse(r *protocol.QueryResponse) *protocol.ProtocolError {
	return validateSchema(queryRespSchema, r, protocol.ErrInternal)
}

// EnforceACL implements runtime behavior for this package.
func EnforceACL(idx *catalogidx.Index, q *protocol.QueryIR, allow map[string]struct{}) *protocol.ProtocolError {
	if allow == nil {
		return nil
	}
	check := func(ref string) *protocol.ProtocolError {
		ent, err := idx.ResolveEntity(ref)
		if err != nil {
			return err
		}
		if _, ok := allow[ent.Name]; !ok {
			return protocol.NewError(protocol.ErrForbidden, "entity not allowed for this app: "+ent.Name, map[string]any{"entity": ent.Name})
		}
		return nil
	}
	if err := check(q.From); err != nil {
		return err
	}
	for _, j := range q.Joins {
		if err := check(j.From); err != nil {
			return err
		}
	}
	return nil
}

// Bundle implements runtime behavior for this package.
func Bundle(preset *protocol.Preset, catalog *protocol.Catalog) (*catalogidx.Index, *protocol.ProtocolError) {
	if err := Preset(preset); err != nil {
		return nil, err
	}
	if err := Catalog(catalog); err != nil {
		return nil, err
	}
	idx, err := catalogidx.New(preset, catalog)
	if err != nil {
		return nil, err
	}
	if err := checkFromFilter(idx); err != nil {
		return nil, err
	}
	if err := checkEntityScope(idx); err != nil {
		return nil, err
	}
	return idx, nil
}

func checkEntityScope(idx *catalogidx.Index) *protocol.ProtocolError {
	for i := range idx.Catalog.Entities {
		e := &idx.Catalog.Entities[i]
		if e.Scope == nil || e.Scope.Field == "" {
			continue
		}
		col := e.Scope.FilterField()
		if _, ok := idx.Field(e, col); !ok {
			return protocol.NewError(protocol.ErrConfigError,
				"entity "+e.Name+" scope column is not a field: "+col,
				map[string]any{"entity": e.Name, "field": col})
		}
	}
	return nil
}

func checkFromFilter(idx *catalogidx.Index) *protocol.ProtocolError {
	for i := range idx.Catalog.Entities {
		e := &idx.Catalog.Entities[i]
		src, ok := idx.Source(e.Source)
		if !ok {
			continue
		}
		for _, f := range e.Fields {
			if !f.FromFilter {
				continue
			}
			if protocol.WireFamily(src.Type) != protocol.SourceREST {
				return protocol.NewError(protocol.ErrConfigError,
					"fromFilter is only allowed on REST entities: "+e.Name+"."+f.Name,
					map[string]any{"entity": e.Name, "field": f.Name})
			}
		}
	}
	return nil
}

type binding struct {
	name   string
	entity *protocol.Entity
}

// Query fetches rows from a source.
func Query(idx *catalogidx.Index, q *protocol.QueryIR) *protocol.ProtocolError {
	if err := lintQueryIRLLM(q); err != nil {
		return err
	}
	if err := QuerySchema(q); err != nil {
		return err
	}
	if q.From == "" {
		return protocol.NewError(protocol.ErrInvalidIR, "from is required", nil)
	}
	if len(q.Select) == 0 {
		return protocol.NewError(protocol.ErrInvalidIR, "select is required", nil)
	}

	bindings := map[string]binding{}

	add := func(entityRef, as string) *protocol.ProtocolError {
		ent, err := idx.ResolveEntity(entityRef)
		if err != nil {
			return err
		}
		name := as
		if name == "" {
			name = ent.Name
		}
		if _, exists := bindings[name]; exists {
			return protocol.NewError(protocol.ErrAmbiguousAlias,
				"duplicate binding alias: "+name, map[string]any{"alias": name})
		}
		bindings[name] = binding{name: name, entity: ent}
		return nil
	}

	if err := add(q.From, q.As); err != nil {
		return err
	}
	for _, j := range q.Joins {
		if err := add(j.From, j.As); err != nil {
			return err
		}
	}

	multi := len(bindings) > 1
	limit := idx.Preset.Limits.DefaultLimit
	if q.Limit != nil {
		limit = *q.Limit
	}
	if limit < 1 || limit > idx.Preset.Limits.MaxLimit {
		return protocol.NewError(protocol.ErrLimitExceeded,
			fmt.Sprintf("limit %d outside 1..%d", limit, idx.Preset.Limits.MaxLimit),
			map[string]any{"limit": limit, "maxLimit": idx.Preset.Limits.MaxLimit})
	}

	checkField := func(ref string) *protocol.ProtocolError {
		return resolveField(idx, bindings, multi, ref)
	}

	for _, item := range q.Select {
		switch v := item.(type) {
		case string:
			if err := checkField(v); err != nil {
				return err
			}
		default:
			m, ok := asMap(item)
			if !ok {
				return protocol.NewError(protocol.ErrInvalidIR, "invalid select item", nil)
			}
			if field, ok := m["field"].(string); ok && field != "" {
				if err := checkField(field); err != nil {
					return err
				}
			}
			as, _ := m["as"].(string)
			if as == "" {
				return protocol.NewError(protocol.ErrInvalidIR, "agg select requires as", nil)
			}
		}
	}

	if q.Where != nil {
		if err := walkWhere(idx, bindings, multi, q.Where); err != nil {
			return err
		}
	}
	for _, g := range q.GroupBy {
		if err := checkField(g); err != nil {
			return err
		}
	}
	for _, o := range q.OrderBy {
		if isOutputAlias(q, o.Field) {
			continue
		}
		if err := checkField(o.Field); err != nil {
			return err
		}
	}
	for _, j := range q.Joins {
		for _, on := range j.On {
			if err := checkField(on.Left); err != nil {
				return err
			}
			if err := checkField(on.Right); err != nil {
				return err
			}
		}
	}
	return nil
}

// isOutputAlias implements runtime behavior for this package.
func isOutputAlias(q *protocol.QueryIR, name string) bool {
	for _, item := range q.Select {
		m, ok := asMap(item)
		if !ok {
			continue
		}
		if as, _ := m["as"].(string); as == name {
			return true
		}
	}
	return false
}

// asMap implements runtime behavior for this package.
func asMap(v any) (map[string]any, bool) {
	if m, ok := v.(map[string]any); ok {
		return m, true
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, false
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, false
	}
	return m, true
}

// resolveField implements runtime behavior for this package.
func resolveField(idx *catalogidx.Index, bindings map[string]binding, multi bool, ref string) *protocol.ProtocolError {
	parts := strings.Split(ref, ".")
	switch len(parts) {
	case 1:
		if multi {
			return protocol.NewError(protocol.ErrAmbiguousField,
				"unqualified field with multiple entities: "+ref,
				map[string]any{"field": ref})
		}
		var only binding
		for _, b := range bindings {
			only = b
			break
		}
		if _, ok := idx.Field(only.entity, parts[0]); ok {
			return nil
		}
		return protocol.NewError(protocol.ErrUnknownField,
			"unknown field: "+ref, map[string]any{"field": ref})
	case 2:
		b, ok := bindings[parts[0]]
		if !ok {
			ent, err := idx.ResolveEntity(parts[0])
			if err != nil {
				return protocol.NewError(protocol.ErrUnknownField,
					"unknown binding in field: "+ref, map[string]any{"field": ref})
			}
			found := false
			for _, bb := range bindings {
				if bb.entity.Name == ent.Name {
					found = true
					b = bb
					break
				}
			}
			if !found {
				return protocol.NewError(protocol.ErrUnknownField,
					"entity not in query scope: "+parts[0], map[string]any{"field": ref})
			}
		}
		if _, ok := idx.Field(b.entity, parts[1]); !ok {
			return protocol.NewError(protocol.ErrUnknownField,
				"unknown field: "+ref, map[string]any{"field": ref})
		}
		return nil
	default:
		return protocol.NewError(protocol.ErrInvalidIR, "invalid field ref: "+ref, nil)
	}
}

// walkWhere implements runtime behavior for this package.
func walkWhere(idx *catalogidx.Index, bindings map[string]binding, multi bool, w map[string]any) *protocol.ProtocolError {
	if op, ok := w["op"].(string); ok {
		switch op {
		case "and", "or", "not":
			args, _ := w["args"].([]any)
			for _, a := range args {
				m, ok := asMap(a)
				if !ok {
					return protocol.NewError(protocol.ErrInvalidIR, "invalid where args", nil)
				}
				if err := walkWhere(idx, bindings, multi, m); err != nil {
					return err
				}
			}
			return nil
		}
	}
	if field, ok := w["field"].(string); ok {
		return resolveField(idx, bindings, multi, field)
	}
	return protocol.NewError(protocol.ErrInvalidIR, "invalid where clause", nil)
}
