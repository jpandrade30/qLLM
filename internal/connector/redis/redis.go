package redis

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"

	"qLLM/internal/config"
	"qLLM/internal/connector/def"
	"qLLM/internal/connector/keycond"
	"qLLM/internal/protocol"
	"qLLM/internal/result"

	goredis "github.com/redis/go-redis/v9"
)

var patternRE = regexp.MustCompile(`\{([^{}]+)}`)

// allowedRedisCmds is the only Redis surface this connector may use (D19).
var allowedRedisCmds = map[string]struct{}{
	"TYPE": {}, "EXISTS": {}, "GET": {}, "MGET": {},
	"HGET": {}, "HMGET": {}, "HGETALL": {},
	"LRANGE": {}, "SSCAN": {}, "ZRANGE": {}, "ZSCORE": {},
	"XRANGE": {},
}

type KV interface {
	Do(ctx context.Context, cmd string, args ...any) (any, error)
}

type Connector struct {
	id     string
	kv     KV
	caps   def.Caps
	closer func() error
}

type redisKV struct {
	c *goredis.Client
}

func (k redisKV) Do(ctx context.Context, cmd string, args ...any) (any, error) {
	if _, ok := allowedRedisCmds[strings.ToUpper(cmd)]; !ok {
		return nil, protocol.NewError(protocol.ErrForbidden,
			"redis command not allowed: "+cmd, map[string]any{"cmd": cmd})
	}
	full := append([]any{cmd}, args...)
	return k.c.Do(ctx, full...).Result()
}

// Open opens a source or engine.
func Open(src protocol.Source) (*Connector, error) {
	addr := config.OptionalEnvString(src.Connection, "addrEnv")
	if addr == "" {
		host, err := config.EnvString(src.Connection, "hostEnv")
		if err != nil {
			return nil, protocol.NewError(protocol.ErrConfigError, "redis needs addrEnv or hostEnv", map[string]any{"source": src.ID})
		}
		port := config.ConnInt(src.Connection, "port", 6379)
		addr = fmt.Sprintf("%s:%d", host, port)
	}
	timeout := 10 * time.Second
	if src.Options != nil {
		if ms := config.ConnInt(src.Options, "timeoutMs", 0); ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}
	opt := &goredis.Options{
		Addr:         addr,
		DB:           config.ConnInt(src.Connection, "db", 0),
		Username:     config.OptionalEnvString(src.Connection, "userEnv"),
		Password:     config.OptionalEnvString(src.Connection, "passwordEnv"),
		ReadTimeout:  timeout,
		WriteTimeout: timeout,
	}
	if truthy(src.Connection["tls"]) {
		opt.TLSConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}
	cli := goredis.NewClient(opt)
	return New(src.ID, redisKV{c: cli}, cli.Close), nil
}

// New builds a connector around a KV (tests inject a recorder).
func New(id string, kv KV, closer func() error) *Connector {
	if closer == nil {
		closer = func() error { return nil }
	}
	return &Connector{
		id: id, kv: kv, closer: closer,
		caps: def.Caps{Filter: true, Project: true, Limit: true},
	}
}

func (c *Connector) ID() string                { return c.id }
func (c *Connector) Type() protocol.SourceType { return protocol.SourceRedis }
func (c *Connector) Capabilities() def.Caps    { return c.caps }
func (c *Connector) Close() error              { return c.closer() }

func (c *Connector) Query(ctx context.Context, step def.PushdownStep) (*protocol.TabularResult, error) {
	for _, s := range step.Select {
		if s.Agg != "" {
			return nil, protocol.NewError(protocol.ErrUnsupported, "redis does not push down aggregations", map[string]any{"source": c.id})
		}
	}
	pat := step.Entity.Binding.KeyPattern
	if pat == "" {
		return nil, protocol.NewError(protocol.ErrConfigError, "redis entity needs binding.keyPattern", nil)
	}
	need := step.Entity.Binding.AccessPath.PartitionKeys()
	if len(need) == 0 {
		need = patternFields(pat)
	}
	eqs, perr := keycond.RequireEq(step.Where, need)
	if perr != nil {
		return nil, perr
	}
	key, err := fillPattern(pat, eqs)
	if err != nil {
		return nil, err
	}
	typRaw, err := c.kv.Do(ctx, "TYPE", key)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
	}
	typ := fmt.Sprint(typRaw)
	limit := step.Limit
	if limit <= 0 {
		limit = 100
	}
	var items []map[string]any
	switch typ {
	case "none":
	case "string":
		v, err := c.kv.Do(ctx, "GET", key)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
		}
		items = []map[string]any{coerceItem(fmt.Sprint(v))}
	case "hash":
		v, err := c.kv.Do(ctx, "HGETALL", key)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
		}
		items = []map[string]any{hashToMap(v)}
	case "list":
		v, err := c.kv.Do(ctx, "LRANGE", key, 0, limit-1)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
		}
		items = listItems(v, limit)
	case "set":
		v, err := c.kv.Do(ctx, "SSCAN", key, 0, "COUNT", limit)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
		}
		items = listItems(sscanMembers(v), limit)
	case "zset":
		v, err := c.kv.Do(ctx, "ZRANGE", key, 0, limit-1)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
		}
		items = listItems(v, limit)
	case "stream":
		v, err := c.kv.Do(ctx, "XRANGE", key, "-", "+", "COUNT", limit)
		if err != nil {
			return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
		}
		items = streamItems(v, limit)
	default:
		return nil, protocol.NewError(protocol.ErrUnsupported, "redis type "+typ+" is not readable", map[string]any{"source": c.id})
	}
	return project(step, items, limit), nil
}

func patternFields(pat string) []string {
	ms := patternRE.FindAllStringSubmatch(pat, -1)
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m[1])
	}
	return out
}

func fillPattern(pat string, eqs map[string]any) (string, error) {
	out := pat
	for _, m := range patternRE.FindAllStringSubmatch(pat, -1) {
		v, ok := eqs[m[1]]
		if !ok {
			return "", protocol.NewError(protocol.ErrUnsupported, "missing key pattern field "+m[1], nil)
		}
		out = strings.ReplaceAll(out, m[0], fmt.Sprint(v))
	}
	return out, nil
}

func coerceItem(s string) map[string]any {
	var m map[string]any
	if json.Unmarshal([]byte(s), &m) == nil {
		if _, ok := m["value"]; !ok {
			m["value"] = s
		}
		return m
	}
	return map[string]any{"value": s}
}

func hashToMap(v any) map[string]any {
	out := map[string]any{}
	switch t := v.(type) {
	case map[string]string:
		for k, val := range t {
			out[k] = val
		}
	case map[string]any:
		return t
	case []any:
		for i := 0; i+1 < len(t); i += 2 {
			out[fmt.Sprint(t[i])] = t[i+1]
		}
	}
	return out
}

func listItems(v any, limit int) []map[string]any {
	arr, _ := v.([]any)
	if arr == nil {
		if ss, ok := v.([]string); ok {
			for _, s := range ss {
				arr = append(arr, s)
			}
		}
	}
	var out []map[string]any
	for i, el := range arr {
		if i >= limit {
			break
		}
		out = append(out, coerceItem(fmt.Sprint(el)))
	}
	return out
}

func sscanMembers(v any) []any {
	if arr, ok := v.([]any); ok && len(arr) == 2 {
		return listAny(arr[1])
	}
	return listAny(v)
}

func listAny(v any) []any {
	if a, ok := v.([]any); ok {
		return a
	}
	if ss, ok := v.([]string); ok {
		out := make([]any, len(ss))
		for i, s := range ss {
			out[i] = s
		}
		return out
	}
	return nil
}

func streamItems(v any, limit int) []map[string]any {
	arr := listAny(v)
	var out []map[string]any
	for i, el := range arr {
		if i >= limit {
			break
		}
		row := map[string]any{}
		if m, ok := el.(map[string]any); ok {
			row = m
		} else if pair, ok := el.([]any); ok && len(pair) >= 2 {
			row["id"] = pair[0]
			fields := hashToMap(pair[1])
			for k, val := range fields {
				row[k] = val
			}
		} else {
			row["value"] = fmt.Sprint(el)
		}
		out = append(out, row)
	}
	return out
}

func project(step def.PushdownStep, items []map[string]any, limit int) *protocol.TabularResult {
	sel := step.Select
	if len(sel) == 0 {
		for _, f := range step.Entity.Fields {
			sel = append(sel, def.SelectItem{Field: f.Name, As: f.Name})
		}
	}
	cols := make([]protocol.Column, 0, len(sel))
	for _, s := range sel {
		as := s.As
		if as == "" {
			as = s.Field
		}
		cols = append(cols, protocol.Column{Name: as, Type: def.FieldType(step.Entity, s.Field)})
	}
	var rows [][]any
	for _, item := range items {
		row := make([]any, len(sel))
		for i, s := range sel {
			row[i] = item[def.PhysicalName(step.Entity, s.Field)]
		}
		rows = append(rows, row)
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return result.New(cols, rows, limit > 0 && len(rows) >= limit)
}

func truthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	default:
		return false
	}
}
