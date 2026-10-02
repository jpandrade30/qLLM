package kafka

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"qLLM/internal/config"
	"qLLM/internal/connector/def"
	"qLLM/internal/connector/keycond"
	"qLLM/internal/protocol"
	"qLLM/internal/result"

	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/kmsg"
	"github.com/twmb/franz-go/pkg/sasl/plain"
	"github.com/twmb/franz-go/pkg/sasl/scram"
)

// Record is one Kafka message returned as stored (key and value unchanged).
type Record struct {
	Partition int32
	Offset    int64
	Timestamp time.Time
	Key       []byte
	Value     []byte
	Headers   map[string]string
}

type FetchReq struct {
	Topic     string
	Partition int32
	Offset    int64
	Limit     int
}

// Fetcher is the only broker surface. Implementations must not join a group or commit.
type Fetcher interface {
	Fetch(ctx context.Context, req FetchReq) ([]Record, error)
	OffsetAt(ctx context.Context, topic string, partition int32, at time.Time) (int64, error)
}

type Connector struct {
	id             string
	fetch          Fetcher
	maxRecords     int
	maxScanRecords int
	caps           def.Caps
	closer         func() error
}

// Open opens a source or engine.
func Open(src protocol.Source) (*Connector, error) {
	brokers := config.OptionalEnvString(src.Connection, "brokersEnv")
	if brokers == "" {
		return nil, protocol.NewError(protocol.ErrConfigError, "kafka connection.brokersEnv is required", map[string]any{"source": src.ID})
	}
	timeout := 12 * time.Second
	maxRec := 100
	maxScan := 200
	if src.Options != nil {
		if ms := config.ConnInt(src.Options, "timeoutMs", 0); ms > 0 {
			timeout = time.Duration(ms) * time.Millisecond
		}
		if n := config.ConnInt(src.Options, "maxRecords", 0); n > 0 {
			maxRec = n
		}
		if n := config.ConnInt(src.Options, "maxScanRecords", 0); n > 0 {
			maxScan = n
		}
	}
	opts := []kgo.Opt{
		kgo.SeedBrokers(splitCSV(brokers)...),
		kgo.DisableAutoCommit(),
		kgo.FetchIsolationLevel(kgo.ReadCommitted()),
		kgo.ConnIdleTimeout(timeout),
	}
	if truthy(src.Connection["tls"]) {
		opts = append(opts, kgo.DialTLSConfig(&tls.Config{MinVersion: tls.VersionTLS12}))
	}
	mech := strings.ToLower(config.ConnString(src.Connection, "sasl", "none"))
	user := config.OptionalEnvString(src.Connection, "userEnv")
	pass := config.OptionalEnvString(src.Connection, "passwordEnv")
	switch mech {
	case "", "none":
	case "plain":
		opts = append(opts, kgo.SASL(plain.Auth{User: user, Pass: pass}.AsMechanism()))
	case "scram":
		opts = append(opts, kgo.SASL(scram.Auth{User: user, Pass: pass}.AsSha256Mechanism()))
	default:
		return nil, protocol.NewError(protocol.ErrConfigError, "kafka sasl must be none, plain, or scram", nil)
	}
	cl, err := kgo.NewClient(opts...)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), map[string]any{"source": src.ID})
	}
	return New(src.ID, kgoFetch{cl: cl}, maxRec, maxScan, func() error {
		cl.Close()
		return nil
	}), nil
}

// New builds a connector around a Fetcher (tests inject a recorder).
func New(id string, f Fetcher, maxRec, maxScan int, closer func() error) *Connector {
	if maxRec <= 0 {
		maxRec = 100
	}
	if maxScan <= 0 {
		maxScan = 200
	}
	if closer == nil {
		closer = func() error { return nil }
	}
	return &Connector{
		id: id, fetch: f, maxRecords: maxRec, maxScanRecords: maxScan, closer: closer,
		caps: def.Caps{Filter: true, Project: true, Limit: true},
	}
}

func (c *Connector) ID() string                { return c.id }
func (c *Connector) Type() protocol.SourceType { return protocol.SourceKafka }
func (c *Connector) Capabilities() def.Caps    { return c.caps }
func (c *Connector) Close() error              { return c.closer() }

func (c *Connector) Query(ctx context.Context, step def.PushdownStep) (*protocol.TabularResult, error) {
	for _, s := range step.Select {
		if s.Agg != "" {
			return nil, protocol.NewError(protocol.ErrUnsupported, "kafka does not push down aggregations", map[string]any{"source": c.id})
		}
	}
	topic := step.Entity.Binding.Topic
	if topic == "" {
		topic = step.Entity.Binding.Table
	}
	if topic == "" {
		return nil, protocol.NewError(protocol.ErrConfigError, "kafka entity needs binding.topic", nil)
	}
	eqs, err := keycond.EqValues(step.Where)
	if err != nil {
		return nil, err
	}
	partField := ""
	pks := step.Entity.Binding.AccessPath.PartitionKeys()
	if len(pks) > 0 {
		partField = pks[0]
	}
	keyField := step.Entity.Binding.AccessPath.MessageKey()
	part, hasPart := int32Eq(eqs, partField, "partition")
	off, hasOff := int64Eq(eqs, "offset")
	keyVal, hasKey := stringEq(eqs, keyField, "key")
	ts, hasTS := timeEq(eqs, "timestamp")
	if !hasPart && !hasKey && !hasTS {
		return nil, protocol.NewError(protocol.ErrUnsupported,
			"kafka query needs equality on partition+offset, accessPath.key, or timestamp", nil)
	}
	if hasPart && !hasOff && !hasKey && !hasTS {
		return nil, protocol.NewError(protocol.ErrUnsupported,
			"kafka partition filter also needs offset, key, or timestamp (no unbounded consume)", nil)
	}
	limit := step.Limit
	if limit <= 0 || limit > c.maxRecords {
		if limit <= 0 {
			limit = c.maxRecords
		}
	}
	if limit > c.maxRecords {
		limit = c.maxRecords
	}
	start := off
	if hasTS {
		at, oerr := c.fetch.OffsetAt(ctx, topic, part, ts)
		if oerr != nil {
			return nil, protocol.NewError(protocol.ErrSourceError, oerr.Error(), map[string]any{"source": c.id})
		}
		start = at
	}
	fetchLimit := limit
	if hasKey && !hasOff && !hasTS {
		fetchLimit = c.maxScanRecords
	}
	recs, ferr := c.fetch.Fetch(ctx, FetchReq{Topic: topic, Partition: part, Offset: start, Limit: fetchLimit})
	if ferr != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, ferr.Error(), map[string]any{"source": c.id})
	}
	var items []map[string]any
	for _, r := range recs {
		if hasKey && string(r.Key) != keyVal {
			continue
		}
		items = append(items, recordMap(r))
		if len(items) >= limit {
			break
		}
	}
	return project(step, items, limit), nil
}

func recordMap(r Record) map[string]any {
	m := map[string]any{
		"partition": r.Partition,
		"offset":    r.Offset,
		"timestamp": r.Timestamp.UTC().Format(time.RFC3339Nano),
		"key":       string(r.Key),
		"value":     string(r.Value),
		"headers":   r.Headers,
	}
	var obj map[string]any
	if json.Unmarshal(r.Value, &obj) == nil {
		for k, v := range obj {
			if _, exists := m[k]; !exists {
				m[k] = v
			}
			m["value."+k] = v
		}
	}
	return m
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
			phys := def.PhysicalName(step.Entity, s.Field)
			row[i] = lookup(item, phys)
		}
		rows = append(rows, row)
	}
	if limit > 0 && len(rows) > limit {
		rows = rows[:limit]
	}
	return result.New(cols, rows, limit > 0 && len(rows) >= limit)
}

func lookup(m map[string]any, phys string) any {
	if v, ok := m[phys]; ok {
		return v
	}
	cur := any(m)
	for _, p := range strings.Split(phys, ".") {
		obj, ok := cur.(map[string]any)
		if !ok {
			return nil
		}
		cur, ok = obj[p]
		if !ok {
			return nil
		}
	}
	return cur
}

func int32Eq(eqs map[string]any, names ...string) (int32, bool) {
	for _, n := range names {
		if n == "" {
			continue
		}
		if v, ok := eqs[n]; ok {
			return int32(asInt64(v)), true
		}
	}
	return 0, false
}

func int64Eq(eqs map[string]any, names ...string) (int64, bool) {
	for _, n := range names {
		if v, ok := eqs[n]; ok {
			return asInt64(v), true
		}
	}
	return 0, false
}

func stringEq(eqs map[string]any, names ...string) (string, bool) {
	for _, n := range names {
		if n == "" {
			continue
		}
		if v, ok := eqs[n]; ok {
			return fmt.Sprint(v), true
		}
	}
	return "", false
}

func timeEq(eqs map[string]any, name string) (time.Time, bool) {
	v, ok := eqs[name]
	if !ok {
		return time.Time{}, false
	}
	switch t := v.(type) {
	case time.Time:
		return t, true
	case string:
		if parsed, err := time.Parse(time.RFC3339, t); err == nil {
			return parsed, true
		}
		if parsed, err := time.Parse(time.RFC3339Nano, t); err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func asInt64(v any) int64 {
	switch n := v.(type) {
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case int64:
		return n
	case float64:
		return int64(n)
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	default:
		i, _ := strconv.ParseInt(fmt.Sprint(v), 10, 64)
		return i
	}
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
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

type kgoFetch struct {
	cl *kgo.Client
}

func (k kgoFetch) Fetch(ctx context.Context, req FetchReq) ([]Record, error) {
	k.cl.AddConsumePartitions(map[string]map[int32]kgo.Offset{
		req.Topic: {req.Partition: kgo.NewOffset().At(req.Offset)},
	})
	defer k.cl.RemoveConsumePartitions(map[string][]int32{req.Topic: {req.Partition}})
	fetches := k.cl.PollRecords(ctx, req.Limit)
	if err := fetches.Err(); err != nil && ctx.Err() == nil {
		return nil, err
	}
	var out []Record
	fetches.EachRecord(func(r *kgo.Record) {
		if req.Limit > 0 && len(out) >= req.Limit {
			return
		}
		hs := map[string]string{}
		for _, h := range r.Headers {
			hs[h.Key] = string(h.Value)
		}
		out = append(out, Record{
			Partition: r.Partition,
			Offset:    r.Offset,
			Timestamp: r.Timestamp,
			Key:       append([]byte(nil), r.Key...),
			Value:     append([]byte(nil), r.Value...),
			Headers:   hs,
		})
	})
	return out, nil
}

func (k kgoFetch) OffsetAt(ctx context.Context, topic string, partition int32, at time.Time) (int64, error) {
	req := kmsg.NewPtrListOffsetsRequest()
	req.IsolationLevel = 1
	rt := kmsg.NewListOffsetsRequestTopic()
	rt.Topic = topic
	rp := kmsg.NewListOffsetsRequestTopicPartition()
	rp.Partition = partition
	rp.Timestamp = at.UnixMilli()
	rt.Partitions = append(rt.Partitions, rp)
	req.Topics = append(req.Topics, rt)
	resp, err := req.RequestWith(ctx, k.cl)
	if err != nil {
		return 0, err
	}
	if len(resp.Topics) == 0 || len(resp.Topics[0].Partitions) == 0 {
		return 0, fmt.Errorf("list offsets empty")
	}
	p := resp.Topics[0].Partitions[0]
	if p.ErrorCode != 0 {
		return 0, fmt.Errorf("list offsets error code %d", p.ErrorCode)
	}
	return p.Offset, nil
}
