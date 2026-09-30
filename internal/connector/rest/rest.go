package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"qLLM/internal/config"
	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
	"qLLM/internal/result"
)

type Connector struct {
	id               string
	baseURL          string
	client           *http.Client
	auth             map[string]any
	res              map[string]any
	caps             def.Caps
	readOnly         bool
	maxResponseBytes int64
}

type OpenOpts struct {
	ReadOnly             bool
	MaxResponseBodyBytes int64
}

// Open opens a source or engine.
func Open(src protocol.Source, opts OpenOpts) (*Connector, error) {
	base, err := config.EnvString(src.Connection, "baseUrlEnv")
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	auth, _ := src.Connection["auth"].(map[string]any)
	var resources map[string]any
	if src.Options != nil {
		resources, _ = src.Options["resources"].(map[string]any)
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
		baseURL:          strings.TrimRight(base, "/"),
		client:           &http.Client{Timeout: timeout},
		auth:             auth,
		res:              resources,
		readOnly:         opts.ReadOnly,
		maxResponseBytes: maxBytes,
		caps: def.Caps{
			Filter: true, Project: true, Limit: true,
		},
	}, nil
}

// ID implements runtime behavior for this package.
func (c *Connector) ID() string { return c.id }

// Type implements runtime behavior for this package.
func (c *Connector) Type() protocol.SourceType { return protocol.SourceREST }

// Capabilities implements runtime behavior for this package.
func (c *Connector) Capabilities() def.Caps { return c.caps }

// Close releases resources.
func (c *Connector) Close() error { return nil }

// Query fetches rows from a source.
func (c *Connector) Query(ctx context.Context, step def.PushdownStep) (*protocol.TabularResult, error) {
	for _, s := range step.Select {
		if s.Agg != "" {
			return nil, protocol.NewError(protocol.ErrUnsupported,
				"REST connector does not push down aggregations", map[string]any{"source": c.id})
		}
	}
	resourceName := step.Entity.Binding.Resource
	if c.res == nil {
		return nil, protocol.NewError(protocol.ErrConfigError, "REST source missing options.resources", nil)
	}
	resDef, ok := c.res[resourceName].(map[string]any)
	if !ok {
		return nil, protocol.NewError(protocol.ErrConfigError, "unknown REST resource: "+resourceName, nil)
	}
	list, _ := resDef["list"].(map[string]any)
	if list == nil {
		return nil, protocol.NewError(protocol.ErrConfigError, "REST resource missing list", nil)
	}
	method, _ := list["method"].(string)
	if method == "" {
		method = http.MethodGet
	}
	if c.readOnly {
		m := strings.ToUpper(method)
		if m != http.MethodGet && m != http.MethodHead {
			return nil, protocol.NewError(protocol.ErrForbidden,
				fmt.Sprintf("readOnly forbids REST method %s on source %s", m, c.id),
				map[string]any{"source": c.id, "method": m})
		}
	}
	path, _ := list["path"].(string)
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError,
			fmt.Sprintf("source %s invalid resource URL", c.id), map[string]any{"source": c.id})
	}
	q := u.Query()
	if step.Where != nil {
		if err := applyWhereParams(q, step.Where); err != nil {
			return nil, err
		}
	}
	if step.Limit > 0 {
		q.Set("limit", strconv.Itoa(step.Limit))
	}
	if step.Offset > 0 {
		q.Set("offset", strconv.Itoa(step.Offset))
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError,
			fmt.Sprintf("source %s request build failed", c.id), map[string]any{"source": c.id})
	}
	if err := c.applyAuth(req); err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, protocol.NewError(protocol.ErrTimeout,
				fmt.Sprintf("source %s exceeded timeout", c.id), map[string]any{"source": c.id})
		}
		return nil, protocol.NewError(protocol.ErrSourceError,
			fmt.Sprintf("source %s request failed", c.id), map[string]any{"source": c.id})
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, c.maxResponseBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError,
			fmt.Sprintf("source %s read failed", c.id), map[string]any{"source": c.id})
	}
	if int64(len(body)) > c.maxResponseBytes {
		return nil, protocol.NewError(protocol.ErrSourceError,
			fmt.Sprintf("source %s response exceeded maxRestResponseBytes", c.id),
			map[string]any{"source": c.id, "maxBytes": c.maxResponseBytes})
	}
	if resp.StatusCode >= 400 {
		return nil, protocol.NewError(protocol.ErrSourceError,
			fmt.Sprintf("REST %s returned %d", c.id, resp.StatusCode),
			map[string]any{"source": c.id, "status": resp.StatusCode})
	}

	items, err := parseList(body)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError,
			fmt.Sprintf("source %s unsupported list payload", c.id), map[string]any{"source": c.id})
	}

	columns := []protocol.Column{}
	for _, s := range step.Select {
		as := s.As
		if as == "" {
			as = s.Field
		}
		columns = append(columns, protocol.Column{Name: as, Type: def.FieldType(step.Entity, s.Field)})
	}
	rows := [][]any{}
	for _, item := range items {
		row := make([]any, len(step.Select))
		for i, s := range step.Select {
			phys := def.PhysicalName(step.Entity, s.Field)
			row[i] = item[phys]
		}
		rows = append(rows, row)
	}
	if step.Limit > 0 && len(rows) > step.Limit {
		rows = rows[:step.Limit]
	}
	truncated := step.Limit > 0 && len(rows) >= step.Limit
	return result.New(columns, rows, truncated), nil
}

// applyAuth implements runtime behavior for this package.
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

// applyWhereParams implements runtime behavior for this package.
func applyWhereParams(q url.Values, w map[string]any) error {
	if op, ok := w["op"].(string); ok && (op == "and" || op == "or") {
		args, _ := w["args"].([]any)
		for _, a := range args {
			m, _ := a.(map[string]any)
			if err := applyWhereParams(q, m); err != nil {
				return err
			}
		}
		return nil
	}
	field, _ := w["field"].(string)
	op, _ := w["op"].(string)
	if field == "" {
		return nil
	}
	if i := strings.LastIndex(field, "."); i >= 0 {
		field = field[i+1:]
	}
	if op != "eq" && op != "" {
		// only eq maps cleanly to query params in MVP
		if op != "eq" {
			return protocol.NewError(protocol.ErrUnsupported,
				"REST pushdown only supports eq filters in MVP", nil)
		}
	}
	q.Set(field, fmt.Sprint(w["value"]))
	return nil
}

// parseList implements runtime behavior for this package.
func parseList(body []byte) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}
	var wrap map[string]any
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, err
	}
	for _, key := range []string{"data", "items", "results", "users"} {
		if v, ok := wrap[key]; ok {
			b, _ := json.Marshal(v)
			if err := json.Unmarshal(b, &arr); err == nil {
				return arr, nil
			}
		}
	}
	return nil, fmt.Errorf("unsupported REST list payload")
}

// asInt implements runtime behavior for this package.
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
