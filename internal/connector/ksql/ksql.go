package ksql

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
	"qLLM/internal/connector/keycond"
	"qLLM/internal/protocol"
	"qLLM/internal/result"
)

type Connector struct {
	id      string
	baseURL string
	client  *http.Client
	auth    map[string]any
	caps    def.Caps
}

func Open(src protocol.Source) (*Connector, error) {
	base, err := config.EnvString(src.Connection, "baseUrlEnv")
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	auth, _ := src.Connection["auth"].(map[string]any)
	timeout := 12 * time.Second
	if src.Options != nil {
		if ms := config.ConnInt(src.Options, "timeoutMs", 0); ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
	}
	return &Connector{
		id:      src.ID,
		baseURL: strings.TrimRight(base, "/"),
		client:  &http.Client{Timeout: timeout},
		auth:    auth,
		caps:    def.Caps{Filter: true, Project: true, Limit: true},
	}, nil
}

func (c *Connector) ID() string                { return c.id }
func (c *Connector) Type() protocol.SourceType { return protocol.SourceKSQL }
func (c *Connector) Capabilities() def.Caps    { return c.caps }
func (c *Connector) Close() error              { return nil }

func (c *Connector) Query(ctx context.Context, step def.PushdownStep) (*protocol.TabularResult, error) {
	for _, s := range step.Select {
		if s.Agg != "" {
			return nil, protocol.NewError(protocol.ErrUnsupported, "ksql connector does not push down aggregations", map[string]any{"source": c.id})
		}
	}
	tbl := step.Entity.Binding.Table
	if tbl == "" {
		return nil, protocol.NewError(protocol.ErrConfigError, "ksql entity needs binding.table", nil)
	}
	key := step.Entity.Binding.AccessPath.KsqlKey
	if key == "" {
		pks := step.Entity.Binding.AccessPath.PartitionKeys()
		if len(pks) == 1 {
			key = pks[0]
		}
	}
	eqs, perr := keycond.RequireEq(step.Where, []string{key})
	if perr != nil {
		return nil, perr
	}
	sel := step.Select
	if len(sel) == 0 {
		for _, f := range step.Entity.Fields {
			sel = append(sel, def.SelectItem{Field: f.Name, As: f.Name})
		}
	}
	proj := make([]string, 0, len(sel))
	cols := make([]protocol.Column, 0, len(sel))
	for _, s := range sel {
		as := s.As
		if as == "" {
			as = s.Field
		}
		phys := def.PhysicalName(step.Entity, s.Field)
		proj = append(proj, ident(phys)+" AS "+ident(as))
		cols = append(cols, protocol.Column{Name: as, Type: def.FieldType(step.Entity, s.Field)})
	}
	val := fmt.Sprintf("%v", eqs[key])
	sql := fmt.Sprintf("SELECT %s FROM %s WHERE %s = '%s'",
		strings.Join(proj, ", "), ident(tbl), ident(def.PhysicalName(step.Entity, key)), escapeSQL(val))
	if step.Limit > 0 {
		sql += fmt.Sprintf(" LIMIT %d", step.Limit)
	}
	sql += ";"
	if strings.Contains(strings.ToUpper(sql), "EMIT CHANGES") {
		return nil, protocol.NewError(protocol.ErrUnsupported, "ksql push queries (EMIT CHANGES) are not supported", nil)
	}
	body, _ := json.Marshal(map[string]any{"ksql": sql, "streamsProperties": map[string]any{}})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/query", bytes.NewReader(body))
	if err != nil {
		return nil, protocol.NewError(protocol.ErrInternal, err.Error(), nil)
	}
	req.Header.Set("Content-Type", "application/vnd.ksql.v1+json")
	if err := c.applyAuth(req); err != nil {
		return nil, err
	}
	resp, err := c.client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, protocol.NewError(protocol.ErrTimeout, fmt.Sprintf("source %s exceeded timeout", c.id), map[string]any{"source": c.id})
		}
		return nil, protocol.NewError(protocol.ErrSourceError, fmt.Sprintf("source %s query failed", c.id), map[string]any{"source": c.id, "cause": err.Error()})
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
	}
	if resp.StatusCode >= 400 {
		return nil, protocol.NewError(protocol.ErrSourceError, fmt.Sprintf("ksql HTTP %d", resp.StatusCode), map[string]any{"source": c.id})
	}
	rows, err := parseKSQL(raw, len(sel))
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
	}
	truncated := step.Limit > 0 && len(rows) >= step.Limit
	return result.New(cols, rows, truncated), nil
}

func (c *Connector) applyAuth(req *http.Request) error {
	if c.auth == nil {
		return nil
	}
	typ, _ := c.auth["type"].(string)
	switch typ {
	case "", "none":
	case "bearer":
		name, _ := c.auth["tokenEnv"].(string)
		req.Header.Set("Authorization", "Bearer "+os.Getenv(name))
	case "basic":
		u, _ := c.auth["userEnv"].(string)
		p, _ := c.auth["passwordEnv"].(string)
		req.SetBasicAuth(os.Getenv(u), os.Getenv(p))
	}
	return nil
}

func parseKSQL(raw []byte, width int) ([][]any, error) {
	var recs []map[string]any
	if err := json.Unmarshal(raw, &recs); err != nil {
		return nil, err
	}
	var rows [][]any
	for _, rec := range recs {
		rowObj, _ := rec["row"].(map[string]any)
		if rowObj == nil {
			continue
		}
		cols, _ := rowObj["columns"].([]any)
		if cols == nil {
			continue
		}
		if width > 0 && len(cols) > width {
			cols = cols[:width]
		}
		rows = append(rows, cols)
	}
	return rows, nil
}

func ident(s string) string {
	return "`" + strings.ReplaceAll(s, "`", "``") + "`"
}

func escapeSQL(s string) string {
	return strings.ReplaceAll(s, `'`, `''`)
}
