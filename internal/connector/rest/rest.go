package rest

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"qLLM/internal/config"
	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
	"qLLM/internal/result"
)

var pathParamRE = regexp.MustCompile(`\{([^{}]+)}`)

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

type restOp struct {
	method      string
	path        string
	itemsKey    string
	limitParam  string
	offsetParam string
	pageSize    int
	maxPages    int
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
	eqs, err := collectEqFilters(step.Where)
	if err != nil {
		return nil, err
	}
	op, path, queryEqs, byID := pickOperation(resDef, eqs)
	if op.path == "" && !byID {
		return nil, protocol.NewError(protocol.ErrConfigError, "REST resource missing list", nil)
	}
	if err := c.guardMethod(op.method); err != nil {
		return nil, err
	}

	var items []map[string]any
	if byID {
		body, err := c.doHTTP(ctx, op.method, path, queryEqs, 0, 0, "", "")
		if err != nil {
			return nil, err
		}
		one, err := parseOne(body, op.itemsKey)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrSourceError,
				fmt.Sprintf("source %s unsupported item payload", c.id), map[string]any{"source": c.id})
		}
		if one != nil {
			items = []map[string]any{one}
		}
	} else {
		items, err = c.fetchPages(ctx, op, path, queryEqs, step.Limit, step.Offset)
		if err != nil {
			return nil, err
		}
	}

	fillEqs := strictTopLevelEqs(step.Where)
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
			val, err := projectCell(c.id, step.Entity, s.Field, item, fillEqs)
			if err != nil {
				return nil, err
			}
			row[i] = val
		}
		rows = append(rows, row)
	}
	if step.Limit > 0 && len(rows) > step.Limit {
		rows = rows[:step.Limit]
	}
	truncated := step.Limit > 0 && len(rows) >= step.Limit
	return result.New(columns, rows, truncated), nil
}

func (c *Connector) fetchPages(ctx context.Context, op restOp, path string, eqs map[string]string, limit, offset int) ([]map[string]any, error) {
	maxPages := op.maxPages
	if maxPages <= 0 {
		maxPages = 1
	}
	if maxPages > 20 {
		maxPages = 20
	}
	limitParam := op.limitParam
	if limitParam == "" {
		limitParam = "limit"
	}
	offsetParam := op.offsetParam
	if offsetParam == "" {
		offsetParam = "offset"
	}
	pageSize := limit
	if op.pageSize > 0 {
		pageSize = op.pageSize
	}
	off := offset
	var all []map[string]any
	for page := 0; page < maxPages; page++ {
		body, err := c.doHTTP(ctx, op.method, path, eqs, pageSize, off, limitParam, offsetParam)
		if err != nil {
			return nil, err
		}
		got, err := parseList(body, op.itemsKey)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrSourceError,
				fmt.Sprintf("source %s unsupported list payload", c.id), map[string]any{"source": c.id})
		}
		all = append(all, got...)
		if len(got) == 0 {
			break
		}
		if limit > 0 && len(all) >= limit {
			break
		}
		if pageSize > 0 && len(got) < pageSize {
			break
		}
		if maxPages == 1 {
			break
		}
		off += len(got)
	}
	return all, nil
}

func (c *Connector) doHTTP(ctx context.Context, method, path string, eqs map[string]string, limit, offset int, limitParam, offsetParam string) ([]byte, error) {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError,
			fmt.Sprintf("source %s invalid resource URL", c.id), map[string]any{"source": c.id})
	}
	q := u.Query()
	for k, v := range eqs {
		q.Set(k, v)
	}
	if limit > 0 && limitParam != "" {
		q.Set(limitParam, strconv.Itoa(limit))
	}
	if offset > 0 && offsetParam != "" {
		q.Set(offsetParam, strconv.Itoa(offset))
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
	return body, nil
}

func (c *Connector) guardMethod(method string) error {
	if method == "" {
		method = http.MethodGet
	}
	if !c.readOnly {
		return nil
	}
	m := strings.ToUpper(method)
	if m != http.MethodGet && m != http.MethodHead {
		return protocol.NewError(protocol.ErrForbidden,
			fmt.Sprintf("readOnly forbids REST method %s on source %s", m, c.id),
			map[string]any{"source": c.id, "method": m})
	}
	return nil
}

func pickOperation(resDef map[string]any, eqs map[string]string) (restOp, string, map[string]string, bool) {
	resItemsKey, _ := resDef["itemsKey"].(string)
	getRaw, _ := resDef["getById"].(map[string]any)
	if getRaw != nil && eqs != nil {
		op := parseOp(getRaw, resItemsKey)
		if filled, remain, ok := fillPath(op.path, eqs); ok {
			return op, filled, remain, true
		}
	}
	list, _ := resDef["list"].(map[string]any)
	op := parseOp(list, resItemsKey)
	return op, op.path, eqs, false
}

func parseOp(raw map[string]any, inheritItemsKey string) restOp {
	op := restOp{method: http.MethodGet, maxPages: 1, itemsKey: inheritItemsKey}
	if raw == nil {
		return op
	}
	if m, _ := raw["method"].(string); m != "" {
		op.method = m
	}
	op.path, _ = raw["path"].(string)
	if k, _ := raw["itemsKey"].(string); k != "" {
		op.itemsKey = k
	}
	if p, _ := raw["limitParam"].(string); p != "" {
		op.limitParam = p
	}
	if p, _ := raw["offsetParam"].(string); p != "" {
		op.offsetParam = p
	}
	if n, ok := asInt(raw["maxPages"]); ok && n > 0 {
		op.maxPages = n
	}
	if n, ok := asInt(raw["pageSize"]); ok && n > 0 {
		op.pageSize = n
	}
	return op
}

func fillPath(path string, eqs map[string]string) (string, map[string]string, bool) {
	names := pathParamRE.FindAllStringSubmatch(path, -1)
	if len(names) == 0 {
		return "", nil, false
	}
	remain := make(map[string]string, len(eqs))
	for k, v := range eqs {
		remain[k] = v
	}
	out := path
	for _, m := range names {
		name := m[1]
		val, ok := remain[name]
		if !ok {
			return "", nil, false
		}
		out = strings.ReplaceAll(out, m[0], url.PathEscape(val))
		delete(remain, name)
	}
	return out, remain, true
}

func projectCell(sourceID string, ent *protocol.Entity, field string, item map[string]any, fillEqs map[string]any) (any, error) {
	f := lookupField(ent, field)
	phys := def.PhysicalName(ent, field)
	if f == nil || !f.FromFilter {
		return item[phys], nil
	}
	filterVal, ok := fillEqs[field]
	if !ok {
		return nil, protocol.NewError(protocol.ErrInvalidIR,
			"field "+field+" is not returned by the source; filter it with eq",
			map[string]any{"field": field})
	}
	filled := castFilterValue(f.Type, filterVal)
	if bodyVal, present := item[phys]; present {
		if !sameAsTyped(f.Type, bodyVal, filled) {
			return nil, protocol.NewError(protocol.ErrSourceError,
				fmt.Sprintf("source %s returned %s that does not match the filter", sourceID, field),
				map[string]any{"source": sourceID, "field": field})
		}
		return castFilterValue(f.Type, bodyVal), nil
	}
	return filled, nil
}

func lookupField(e *protocol.Entity, name string) *protocol.Field {
	if e == nil {
		return nil
	}
	for i := range e.Fields {
		if e.Fields[i].Name == name {
			return &e.Fields[i]
		}
	}
	return nil
}

// strictTopLevelEqs collects eq values from a single eq or an AND of eqs.
// Predicates under or/not are ignored and cannot feed fromFilter.
func strictTopLevelEqs(w map[string]any) map[string]any {
	out := map[string]any{}
	collectStrictEq(w, out)
	return out
}

func collectStrictEq(w map[string]any, out map[string]any) {
	if w == nil {
		return
	}
	op, _ := w["op"].(string)
	if op == "or" || op == "not" {
		return
	}
	if op == "and" {
		args, _ := w["args"].([]any)
		for _, a := range args {
			m, _ := a.(map[string]any)
			collectStrictEq(m, out)
		}
		return
	}
	field, _ := w["field"].(string)
	if field == "" {
		return
	}
	if i := strings.LastIndex(field, "."); i >= 0 {
		field = field[i+1:]
	}
	if op != "eq" && op != "" {
		return
	}
	out[field] = w["value"]
}

func castFilterValue(t protocol.LogicalType, v any) any {
	if v == nil {
		return nil
	}
	switch t {
	case protocol.TypeNumber:
		switch n := v.(type) {
		case float64:
			return n
		case float32:
			return float64(n)
		case int:
			return float64(n)
		case int64:
			return float64(n)
		case json.Number:
			f, err := n.Float64()
			if err != nil {
				return fmt.Sprint(v)
			}
			return f
		default:
			f, err := strconv.ParseFloat(fmt.Sprint(v), 64)
			if err != nil {
				return fmt.Sprint(v)
			}
			return f
		}
	case protocol.TypeBoolean:
		switch b := v.(type) {
		case bool:
			return b
		case string:
			return b == "true" || b == "1"
		default:
			s := strings.ToLower(fmt.Sprint(v))
			return s == "true" || s == "1"
		}
	default:
		if s, ok := v.(string); ok {
			return s
		}
		return fmt.Sprint(v)
	}
}

func sameAsTyped(t protocol.LogicalType, body, want any) bool {
	return fmt.Sprint(castFilterValue(t, body)) == fmt.Sprint(castFilterValue(t, want))
}

func collectEqFilters(w map[string]any) (map[string]string, error) {
	out := map[string]string{}
	if w == nil {
		return out, nil
	}
	if err := walkEq(w, out); err != nil {
		return nil, err
	}
	return out, nil
}

func walkEq(w map[string]any, out map[string]string) error {
	if op, ok := w["op"].(string); ok && (op == "and" || op == "or") {
		args, _ := w["args"].([]any)
		for _, a := range args {
			m, _ := a.(map[string]any)
			if err := walkEq(m, out); err != nil {
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
		return protocol.NewError(protocol.ErrUnsupported,
			"REST pushdown only supports eq filters in MVP", nil)
	}
	out[field] = fmt.Sprint(w["value"])
	return nil
}

func parseList(body []byte, itemsKey string) ([]map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		return arr, nil
	}
	var wrap map[string]any
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, err
	}
	keys := []string{"data", "items", "results", "users"}
	if itemsKey != "" {
		keys = []string{itemsKey}
	}
	for _, key := range keys {
		if v, ok := wrap[key]; ok {
			b, err := json.Marshal(v)
			if err != nil {
				continue
			}
			if err := json.Unmarshal(b, &arr); err == nil {
				return arr, nil
			}
		}
	}
	return nil, fmt.Errorf("unsupported REST list payload")
}

func parseOne(body []byte, itemsKey string) (map[string]any, error) {
	var arr []map[string]any
	if err := json.Unmarshal(body, &arr); err == nil {
		if len(arr) == 0 {
			return nil, nil
		}
		return arr[0], nil
	}
	var wrap map[string]any
	if err := json.Unmarshal(body, &wrap); err != nil {
		return nil, err
	}
	keys := []string{"data", "item", "result"}
	if itemsKey != "" {
		keys = append([]string{itemsKey}, keys...)
	}
	for _, key := range keys {
		if v, ok := wrap[key]; ok {
			if m, ok := v.(map[string]any); ok {
				return m, nil
			}
		}
	}
	return wrap, nil
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
