package connector

import (
	"fmt"
	"net/http"
	"strings"

	"qLLM/internal/connector/cassandra"
	"qLLM/internal/connector/def"
	"qLLM/internal/connector/dynamodb"
	"qLLM/internal/connector/graphql"
	"qLLM/internal/connector/kafka"
	"qLLM/internal/connector/ksql"
	"qLLM/internal/connector/mongo"
	"qLLM/internal/connector/redis"
	"qLLM/internal/connector/rest"
	"qLLM/internal/connector/sqldb"
	"qLLM/internal/protocol"
)

type (
	Caps         = def.Caps
	PushdownStep = def.PushdownStep
	SelectItem   = def.SelectItem
	Connector    = def.Connector
)

var (
	PhysicalName = def.PhysicalName
	FieldType    = def.FieldType
)

type Registry struct {
	byID map[string]def.Connector
}

// NewRegistry builds a registry from already-open connectors (tests).
func NewRegistry(cs map[string]def.Connector) *Registry {
	if cs == nil {
		cs = map[string]def.Connector{}
	}
	return &Registry{byID: cs}
}

// OpenOpts carries runtime settings into connector open.
type OpenOpts struct {
	MaxRestResponseBytes int64
}

// OpenAll opens a source or engine.
func OpenAll(p *protocol.Preset, opts OpenOpts) (*Registry, error) {
	if p == nil {
		return nil, protocol.NewError(protocol.ErrConfigError, "preset is nil", nil)
	}
	if err := validateReadOnlyREST(p); err != nil {
		return nil, err
	}
	maxSourceMs := p.Limits.MaxSourceMs
	readOnly := p.Limits.ReadOnly
	r := &Registry{byID: make(map[string]def.Connector)}
	for _, s := range p.Sources {
		var c def.Connector
		var err error
		switch protocol.WireFamily(s.Type) {
		case protocol.SourcePostgres:
			c, err = sqldb.OpenPostgres(s, maxSourceMs)
		case protocol.SourceMySQL:
			c, err = sqldb.OpenMySQL(s, maxSourceMs)
		case protocol.SourceMSSQL:
			c, err = sqldb.OpenMSSQL(s, maxSourceMs)
		case protocol.SourceSQLite:
			c, err = sqldb.OpenSQLite(s, maxSourceMs)
		case protocol.SourceClickHouse:
			c, err = sqldb.OpenClickHouse(s, maxSourceMs)
		case protocol.SourceMongoDB:
			c, err = mongo.Open(s)
		case protocol.SourceREST:
			c, err = rest.Open(s, rest.OpenOpts{
				ReadOnly:             readOnly,
				MaxResponseBodyBytes: opts.MaxRestResponseBytes,
			})
		case protocol.SourceDynamoDB:
			c, err = dynamodb.Open(s)
		case protocol.SourceCassandra:
			c, err = cassandra.Open(s)
		case protocol.SourceKSQL:
			c, err = ksql.Open(s)
		case protocol.SourceRedis:
			c, err = redis.Open(s)
		case protocol.SourceKafka:
			c, err = kafka.Open(s)
		case protocol.SourceGraphQL:
			c, err = graphql.Open(s, graphql.OpenOpts{
				MaxResponseBodyBytes: opts.MaxRestResponseBytes,
			})
		default:
			err = protocol.NewError(protocol.ErrConfigError, "unknown source type: "+string(s.Type), nil)
		}
		if err != nil {
			_ = r.Close()
			return nil, err
		}
		r.byID[s.ID] = c
	}
	return r, nil
}

// validateReadOnlyREST implements runtime behavior for this package.
func validateReadOnlyREST(p *protocol.Preset) error {
	if !p.Limits.ReadOnly {
		return nil
	}
	for _, s := range p.Sources {
		if s.Type != protocol.SourceREST || s.Options == nil {
			continue
		}
		resources, _ := s.Options["resources"].(map[string]any)
		for name, raw := range resources {
			resDef, _ := raw.(map[string]any)
			for _, opName := range []string{"list", "getById"} {
				op, _ := resDef[opName].(map[string]any)
				if op == nil {
					continue
				}
				method, _ := op["method"].(string)
				if method == "" {
					method = http.MethodGet
				}
				m := strings.ToUpper(method)
				if m != http.MethodGet && m != http.MethodHead {
					return protocol.NewError(protocol.ErrConfigError,
						fmt.Sprintf("readOnly preset forbids REST resource %q %s method %s on source %s", name, opName, m, s.ID),
						map[string]any{"source": s.ID, "resource": name, "method": m})
				}
			}
		}
	}
	return nil
}

// Get returns a stored value.
func (r *Registry) Get(id string) (def.Connector, error) {
	c, ok := r.byID[id]
	if !ok {
		return nil, protocol.NewError(protocol.ErrConfigError, fmt.Sprintf("connector %s not open", id), nil)
	}
	return c, nil
}

// Close releases resources.
func (r *Registry) Close() error {
	var first error
	for _, c := range r.byID {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
