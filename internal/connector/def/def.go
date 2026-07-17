package def

import (
	"context"

	"qLLM/internal/protocol"
)

type Caps = protocol.Capabilities

type PushdownStep struct {
	SourceID string
	Entity   *protocol.Entity
	Binding  string
	Select   []SelectItem
	Where    map[string]any
	GroupBy  []string
	OrderBy  []protocol.OrderExpr
	Limit    int
	Offset   int
}

type SelectItem struct {
	Field string
	Agg   string
	As    string
}

type Connector interface {
	ID() string
	Type() protocol.SourceType
	Capabilities() Caps
	Query(ctx context.Context, step PushdownStep) (*protocol.TabularResult, error)
	Close() error
}

func PhysicalName(e *protocol.Entity, logical string) string {
	for _, f := range e.Fields {
		if f.Name == logical {
			return f.Physical
		}
	}
	return logical
}

func FieldType(e *protocol.Entity, logical string) protocol.LogicalType {
	for _, f := range e.Fields {
		if f.Name == logical {
			return f.Type
		}
	}
	return protocol.TypeString
}
