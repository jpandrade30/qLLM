package connector

import (
	"fmt"

	"qLLM/internal/connector/def"
	"qLLM/internal/connector/mongo"
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

func OpenAll(sources []protocol.Source) (*Registry, error) {
	r := &Registry{byID: make(map[string]def.Connector)}
	for _, s := range sources {
		var c def.Connector
		var err error
		switch s.Type {
		case protocol.SourcePostgres:
			c, err = sqldb.OpenPostgres(s)
		case protocol.SourceMySQL:
			c, err = sqldb.OpenMySQL(s)
		case protocol.SourceMongoDB:
			c, err = mongo.Open(s)
		case protocol.SourceREST:
			c, err = rest.Open(s)
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

func (r *Registry) Get(id string) (def.Connector, error) {
	c, ok := r.byID[id]
	if !ok {
		return nil, protocol.NewError(protocol.ErrConfigError, fmt.Sprintf("connector %s not open", id), nil)
	}
	return c, nil
}

func (r *Registry) Close() error {
	var first error
	for _, c := range r.byID {
		if err := c.Close(); err != nil && first == nil {
			first = err
		}
	}
	return first
}
