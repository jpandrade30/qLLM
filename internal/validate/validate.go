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
)

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
}

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

func Preset(p *protocol.Preset) *protocol.ProtocolError {
	if err := validateSchema(presetSchema, p, protocol.ErrConfigError); err != nil {
		return err
	}
	if p.Limits.DefaultLimit > p.Limits.MaxLimit {
		return protocol.NewError(protocol.ErrConfigError, "defaultLimit cannot exceed maxLimit", nil)
	}
	return nil
}

func Catalog(c *protocol.Catalog) *protocol.ProtocolError {
	return validateSchema(catalogSchema, c, protocol.ErrConfigError)
}

func QuerySchema(q *protocol.QueryIR) *protocol.ProtocolError {
	return validateSchema(querySchema, q, protocol.ErrInvalidIR)
}

func Bundle(preset *protocol.Preset, catalog *protocol.Catalog) (*catalogidx.Index, *protocol.ProtocolError) {
	if err := Preset(preset); err != nil {
		return nil, err
	}
	if err := Catalog(catalog); err != nil {
		return nil, err
	}
	return catalogidx.New(preset, catalog)
}

type binding struct {
	name   string
	entity *protocol.Entity
}

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
