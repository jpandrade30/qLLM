package graphql

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"qLLM/internal/config"
	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
	"qLLM/internal/result"
)

type Connector struct {
	id               string
	endpoint         string
	client           *http.Client
	auth             map[string]any
	ops              map[string]any
	caps             def.Caps
	maxResponseBytes int64
}

type OpenOpts struct {
	MaxResponseBodyBytes int64
}

type operation struct {
	document      string
	variables     []string
	itemsPath     string
	limitVariable string
}

// Open opens a GraphQL HTTP source (experimental).
func Open(src protocol.Source, opts OpenOpts) (*Connector, error) {
	base, err := config.EnvString(src.Connection, "baseUrlEnv")
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	auth, _ := src.Connection["auth"].(map[string]any)
	var ops map[string]any
	if src.Options != nil {
		ops, _ = src.Options["operations"].(map[string]any)
	}
	if len(ops) == 0 {
		return nil, protocol.NewError(protocol.ErrConfigError,
			"graphql source missing options.operations", map[string]any{"source": src.ID})
	}
	for name, raw := range ops {
		op, err := parseOperation(name, raw)
		if err != nil {
			return nil, err
		}
		if perr := ValidateDocument(op.document); perr != nil {
			return nil, protocol.NewError(protocol.ErrConfigError,
				fmt.Sprintf("graphql operation %q: %s", name, perr.Message),
				map[string]any{"source": src.ID, "operation": name, "keyword": perr.Details["keyword"], "operationType": perr.Details["operation"]})
		}
	}
	timeout := 10 * time.Second
	if src.Options != nil {
		if ms, ok := asInt(src.Options["timeoutMs"]); ok && ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}
	maxBytes := opts.MaxResponseBodyBytes
	if maxBytes <= 0 {
		maxBytes = protocol.DefaultMaxRestResponseBytes
	}
	return &Connector{
		id:               src.ID,
		endpoint:         strings.TrimRight(base, "/"),
		client:           &http.Client{Timeout: timeout},
		auth:             auth,
		ops:              ops,
		maxResponseBytes: maxBytes,
		caps:             def.Caps{Filter: true, Project: true, Limit: true},
	}, nil
}

func (c *Connector) ID() string                { return c.id }
func (c *Connector) Type() protocol.SourceType { return protocol.SourceGraphQL }
func (c *Connector) Capabilities() def.Caps    { return c.caps }
func (c *Connector) Close() error              { return nil }

func (c *Connector) Query(ctx context.Context, step def.PushdownStep) (*protocol.TabularResult, error) {
	for _, s := range step.Select {
		if s.Agg != "" {
			return nil, protocol.NewError(protocol.ErrUnsupported,
				"graphql connector does not push down aggregations", map[string]any{"source": c.id})
		}
	}
	name := step.Entity.Binding.Resource
	if name == "" {
		return nil, protocol.NewError(protocol.ErrConfigError,
			"graphql entity needs binding.resource", map[string]any{"source": c.id})
	}
	op, err := parseOperation(name, c.ops[name])
	if err != nil {
		return nil, err
	}
	if perr := ValidateDocument(op.document); perr != nil {
		return nil, perr
	}
	eqs, err := collectEqFilters(step.Where)
	if err != nil {
		return nil, err
	}
	vars := map[string]any{}
	if len(op.variables) > 0 {
		for _, v := range op.variables {
			val, ok := eqs[v]
			if !ok {
				return nil, protocol.NewError(protocol.ErrUnsupported,
					"graphql operation requires eq filter for variable "+v,
					map[string]any{"source": c.id, "variable": v})
			}
			vars[v] = val
		}
		for k := range eqs {
			if !contains(op.variables, k) {
				return nil, protocol.NewError(protocol.ErrUnsupported,
					"graphql pushdown only supports declared variables",
					map[string]any{"source": c.id, "field": k})
			}
		}
	} else {
		for k, v := range eqs {
			vars[k] = v
		}
	}
	if op.limitVariable != "" && step.Limit > 0 {
		vars[op.limitVariable] = step.Limit
	}
	body, err := c.doHTTP(ctx, op.document, vars)
	if err != nil {
		return nil, err
	}
	items, err := extractItems(body, op.itemsPath)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
	}
	columns := []protocol.Column{}
	for _, s := range step.Select {
		as := s.As
		if as == "" {
			as = s.Field
		}
		columns = append(columns, protocol.Column{Name: as, Type: def.FieldType(step.Entity, s.Field)})
	}
	rows := make([][]any, 0, len(items))
	for _, item := range items {
		row := make([]any, len(step.Select))
		for i, s := range step.Select {
			phys := def.PhysicalName(step.Entity, s.Field)
			row[i] = lookupPath(item, phys)
		}
		rows = append(rows, row)
	}
	if step.Limit > 0 && len(rows) > step.Limit {
		rows = rows[:step.Limit]
	}
	truncated := step.Limit > 0 && len(rows) >= step.Limit
	return result.New(columns, rows, truncated), nil
}

func parseOperation(name string, raw any) (operation, error) {
	m, ok := raw.(map[string]any)
	if !ok || m == nil {
		return operation{}, protocol.NewError(protocol.ErrConfigError,
			"unknown graphql operation: "+name, map[string]any{"operation": name})
	}
	doc, _ := m["document"].(string)
	if strings.TrimSpace(doc) == "" {
		return operation{}, protocol.NewError(protocol.ErrConfigError,
			"graphql operation missing document: "+name, map[string]any{"operation": name})
	}
	itemsPath, _ := m["itemsPath"].(string)
	if strings.TrimSpace(itemsPath) == "" {
		return operation{}, protocol.NewError(protocol.ErrConfigError,
			"graphql operation missing itemsPath: "+name, map[string]any{"operation": name})
	}
	limitVar, _ := m["limitVariable"].(string)
	var vars []string
	if arr, ok := m["variables"].([]any); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok && s != "" {
				vars = append(vars, s)
			}
		}
	}
	return operation{document: doc, variables: vars, itemsPath: itemsPath, limitVariable: limitVar}, nil
}

func (c *Connector) doHTTP(ctx context.Context, document string, vars map[string]any) ([]byte, error) {
	payload := map[string]any{"query": document}
	if len(vars) > 0 {
		payload["variables"] = vars
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(raw))
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if err := c.applyAuth(req); err != nil {
		return nil, err
	}
	res, err := c.client.Do(req)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
	}
	defer res.Body.Close()
	limited := io.LimitReader(res.Body, c.maxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
	}
	if int64(len(body)) > c.maxResponseBytes {
		return nil, protocol.NewError(protocol.ErrSourceError,
			"graphql response exceeds maxRestResponseBytes", map[string]any{"source": c.id})
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, protocol.NewError(protocol.ErrSourceError,
			fmt.Sprintf("graphql HTTP %d", res.StatusCode), map[string]any{"source": c.id, "status": res.StatusCode})
	}
	var envelope map[string]any
	if err := json.Unmarshal(body, &envelope); err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, "invalid graphql JSON", map[string]any{"source": c.id})
	}
	if errs, ok := envelope["errors"]; ok && errs != nil {
		if arr, ok := errs.([]any); ok && len(arr) > 0 {
			msg := fmt.Sprint(arr[0])
			if m, ok := arr[0].(map[string]any); ok {
				if mm, ok := m["message"].(string); ok && mm != "" {
					msg = mm
				}
			}
			return nil, protocol.NewError(protocol.ErrSourceError,
				"graphql errors: "+msg, map[string]any{"source": c.id})
		}
	}
	return body, nil
}

func extractItems(body []byte, itemsPath string) ([]map[string]any, error) {
	var root any
	if err := json.Unmarshal(body, &root); err != nil {
		return nil, err
	}
	cur := root
	for _, part := range strings.Split(itemsPath, ".") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("itemsPath %q: not an object at %q", itemsPath, part)
		}
		cur, ok = m[part]
		if !ok {
			return nil, fmt.Errorf("itemsPath %q: missing %q", itemsPath, part)
		}
	}
	switch v := cur.(type) {
	case nil:
		return nil, nil
	case []any:
		out := make([]map[string]any, 0, len(v))
		for _, el := range v {
			if m, ok := el.(map[string]any); ok {
				out = append(out, m)
			}
		}
		return out, nil
	case map[string]any:
		return []map[string]any{v}, nil
	default:
		return nil, fmt.Errorf("itemsPath %q: expected array or object", itemsPath)
	}
}

func lookupPath(item map[string]any, physical string) any {
	if item == nil {
		return nil
	}
	if !strings.Contains(physical, ".") {
		return item[physical]
	}
	var cur any = item
	for _, part := range strings.Split(physical, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur = m[part]
	}
	return cur
}

func collectEqFilters(where any) (map[string]string, error) {
	out := map[string]string{}
	if where == nil {
		return out, nil
	}
	if err := walkEq(where, out); err != nil {
		return nil, err
	}
	return out, nil
}

func walkEq(w any, out map[string]string) error {
	m, ok := w.(map[string]any)
	if !ok {
		return nil
	}
	if op, _ := m["op"].(string); op == "and" {
		args, _ := m["args"].([]any)
		for _, a := range args {
			if err := walkEq(a, out); err != nil {
				return err
			}
		}
		return nil
	}
	if _, hasOr := m["op"]; hasOr {
		op, _ := m["op"].(string)
		if op != "eq" && op != "" {
			return protocol.NewError(protocol.ErrUnsupported,
				"graphql pushdown only supports eq filters", map[string]any{"op": op})
		}
	}
	field, _ := m["field"].(string)
	op, _ := m["op"].(string)
	if field == "" {
		return nil
	}
	if i := strings.LastIndex(field, "."); i >= 0 {
		field = field[i+1:]
	}
	if op != "eq" && op != "" {
		return protocol.NewError(protocol.ErrUnsupported,
			"graphql pushdown only supports eq filters", map[string]any{"op": op})
	}
	out[field] = fmt.Sprint(m["value"])
	return nil
}

func (c *Connector) applyAuth(req *http.Request) error {
	if c.auth == nil {
		return nil
	}
	typ, _ := c.auth["type"].(string)
	switch typ {
	case "", "none":
		return nil
	case "bearer":
		token := ""
		if name, ok := c.auth["tokenEnv"].(string); ok {
			token = os.Getenv(name)
		}
		req.Header.Set("Authorization", "Bearer "+token)
	case "header":
		name, _ := c.auth["name"].(string)
		val := ""
		if vn, ok := c.auth["valueEnv"].(string); ok {
			val = os.Getenv(vn)
		}
		req.Header.Set(name, val)
	case "basic":
		user := os.Getenv(fmt.Sprint(c.auth["userEnv"]))
		pass := os.Getenv(fmt.Sprint(c.auth["passwordEnv"]))
		req.SetBasicAuth(user, pass)
	}
	return nil
}

func contains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func asInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int64:
		return int(n), true
	case float64:
		return int(n), true
	default:
		return 0, false
	}
}
