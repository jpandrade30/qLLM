package cataloggen

import (
	"encoding/json"
	"fmt"
	"strings"

	"qLLM/internal/protocol"

	"gopkg.in/yaml.v3"
)

// OpenAPIResult is catalog entities plus REST options.resources for a source.
type OpenAPIResult struct {
	Entities  []protocol.Entity
	Resources map[string]any
}

// FromOpenAPI parses OpenAPI 3 JSON or YAML and emits rest_resource entities + list/getById ops.
func FromOpenAPI(raw []byte, sourceID string) (*OpenAPIResult, error) {
	var doc map[string]any
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		if err2 := json.Unmarshal(raw, &doc); err2 != nil {
			return nil, protocol.NewError(protocol.ErrConfigError,
				fmt.Sprintf("openapi parse failed: %v", err), nil)
		}
	}
	paths := asStringMap(doc["paths"])
	if len(paths) == 0 {
		return nil, protocol.NewError(protocol.ErrConfigError, "openapi has no paths", nil)
	}
	components := asStringMap(doc["components"])
	schemas := asStringMap(nil)
	if components != nil {
		schemas = asStringMap(components["schemas"])
	}

	type acc struct {
		list    map[string]any
		getById map[string]any
		fields  []protocol.Field
	}
	byName := map[string]*acc{}
	order := []string{}

	pathKeys := make([]string, 0, len(paths))
	for p := range paths {
		pathKeys = append(pathKeys, p)
	}
	sortStrings(pathKeys)

	for _, path := range pathKeys {
		item := asStringMap(paths[path])
		if item == nil {
			continue
		}
		get := asStringMap(item["get"])
		if get == nil {
			continue
		}
		name := resourceName(path)
		if name == "" {
			continue
		}
		a, ok := byName[name]
		if !ok {
			a = &acc{}
			byName[name] = a
			order = append(order, name)
		}
		op := map[string]any{
			"method": "GET",
			"path":   path,
		}
		if qp := queryParamNames(get); len(qp) > 0 {
			op["queryParams"] = qp
		}
		if hasPathParam(path) {
			a.getById = op
		} else {
			a.list = op
			if f := fieldsFromResponse(get, schemas); len(f) > 0 {
				a.fields = f
			}
		}
	}

	res := &OpenAPIResult{Resources: map[string]any{}}
	for _, name := range order {
		a := byName[name]
		if a.list == nil {
			continue
		}
		resDef := map[string]any{"list": a.list}
		if a.getById != nil {
			resDef["getById"] = a.getById
		}
		res.Resources[name] = resDef
		fields := a.fields
		if len(fields) == 0 {
			fields = []protocol.Field{{Name: "id", Type: protocol.TypeString, Physical: "id"}}
		}
		res.Entities = append(res.Entities, protocol.Entity{
			Name:        name,
			Description: "Draft from OpenAPI GET; review fields/relations before serve.",
			Source:      sourceID,
			Binding:     protocol.Binding{Kind: "rest_resource", Resource: name},
			Fields:      fields,
		})
	}
	if len(res.Entities) == 0 {
		return nil, protocol.NewError(protocol.ErrConfigError,
			"no listable GET paths (without path params) found", nil)
	}
	return res, nil
}

// resourceName implements runtime behavior for this package.
func resourceName(path string) string {
	p := strings.Trim(path, "/")
	if p == "" {
		return ""
	}
	segs := strings.Split(p, "/")
	for i := len(segs) - 1; i >= 0; i-- {
		s := segs[i]
		if s == "" || strings.HasPrefix(s, "{") {
			continue
		}
		return sanitizeName(s)
	}
	return ""
}

// sanitizeName implements runtime behavior for this package.
func sanitizeName(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '_' {
			b.WriteRune(r)
		} else if r == '-' {
			b.WriteByte('_')
		}
	}
	out := b.String()
	if out == "" {
		return "resource"
	}
	return out
}

// hasPathParam implements runtime behavior for this package.
func hasPathParam(path string) bool {
	return strings.Contains(path, "{")
}

// asStringMap implements runtime behavior for this package.
func asStringMap(v any) map[string]any {
	switch m := v.(type) {
	case map[string]any:
		return m
	case map[any]any:
		out := make(map[string]any, len(m))
		for k, val := range m {
			out[fmt.Sprint(k)] = val
		}
		return out
	default:
		return nil
	}
}

// queryParamNames implements runtime behavior for this package.
func queryParamNames(get map[string]any) []string {
	params, _ := get["parameters"].([]any)
	var names []string
	for _, p := range params {
		m := asStringMap(p)
		if m == nil {
			continue
		}
		in, _ := m["in"].(string)
		name, _ := m["name"].(string)
		if in == "query" && name != "" {
			names = append(names, name)
		}
	}
	return names
}

// fieldsFromResponse implements runtime behavior for this package.
func fieldsFromResponse(get map[string]any, schemas map[string]any) []protocol.Field {
	resp := asStringMap(get["responses"])
	if resp == nil {
		return nil
	}
	var body map[string]any
	for _, code := range []string{"200", "201"} {
		if r := asStringMap(resp[code]); r != nil {
			body = r
			break
		}
	}
	if body == nil {
		return nil
	}
	content := asStringMap(body["content"])
	if content == nil {
		return nil
	}
	appJSON := asStringMap(content["application/json"])
	if appJSON == nil {
		return nil
	}
	schema := derefSchema(asStringMap(appJSON["schema"]), schemas)
	if schema == nil {
		return nil
	}
	if t, _ := schema["type"].(string); t == "array" {
		schema = derefSchema(asStringMap(schema["items"]), schemas)
	} else if props := asStringMap(schema["properties"]); props != nil {
		if data := derefSchema(asStringMap(props["data"]), schemas); data != nil {
			if t, _ := data["type"].(string); t == "array" {
				schema = derefSchema(asStringMap(data["items"]), schemas)
			}
		}
	}
	if schema == nil {
		return nil
	}
	props := asStringMap(schema["properties"])
	if len(props) == 0 {
		return nil
	}
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	// stable-ish: id first then rest unsorted is ok for draft; sort for tests
	sortStrings(keys)
	fields := make([]protocol.Field, 0, len(keys))
	for _, k := range keys {
		pm, _ := props[k].(map[string]any)
		fields = append(fields, protocol.Field{
			Name:     k,
			Type:     mapJSONSchemaType(pm),
			Physical: k,
		})
	}
	return fields
}

// derefSchema implements runtime behavior for this package.
func derefSchema(schema map[string]any, schemas map[string]any) map[string]any {
	if schema == nil {
		return nil
	}
	ref, _ := schema["$ref"].(string)
	if ref == "" {
		return schema
	}
	const p = "#/components/schemas/"
	if !strings.HasPrefix(ref, p) {
		return schema
	}
	name := strings.TrimPrefix(ref, p)
	if next := asStringMap(schemas[name]); next != nil {
		return next
	}
	return schema
}

// mapJSONSchemaType implements runtime behavior for this package.
func mapJSONSchemaType(m map[string]any) protocol.LogicalType {
	if m == nil {
		return protocol.TypeJSON
	}
	t, _ := m["type"].(string)
	switch t {
	case "integer", "number":
		return protocol.TypeNumber
	case "boolean":
		return protocol.TypeBoolean
	case "string":
		if f, _ := m["format"].(string); f == "date-time" || f == "date" {
			return protocol.TypeTimestamp
		}
		return protocol.TypeString
	default:
		return protocol.TypeJSON
	}
}

// sortStrings implements runtime behavior for this package.
func sortStrings(s []string) {
	for i := 0; i < len(s); i++ {
		for j := i + 1; j < len(s); j++ {
			if s[j] < s[i] {
				s[i], s[j] = s[j], s[i]
			}
		}
	}
}
