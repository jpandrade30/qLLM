package dynamodb

import (
	"context"
	"fmt"
	"strconv"

	"qLLM/internal/config"
	"qLLM/internal/connector/def"
	"qLLM/internal/connector/keycond"
	"qLLM/internal/protocol"
	"qLLM/internal/result"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	ddb "github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

type Connector struct {
	id     string
	client *ddb.Client
	caps   def.Caps
}

// Open opens a source or engine.
func Open(src protocol.Source) (*Connector, error) {
	region := config.ConnString(src.Connection, "region", "")
	if region == "" {
		return nil, protocol.NewError(protocol.ErrConfigError, "dynamodb connection.region is required", map[string]any{"source": src.ID})
	}
	cfg, err := awsconfig.LoadDefaultConfig(context.Background(), awsconfig.WithRegion(region))
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, "aws config: "+err.Error(), map[string]any{"source": src.ID})
	}
	var optFns []func(*ddb.Options)
	if ep := config.OptionalEnvString(src.Connection, "endpointEnv"); ep != "" {
		endpoint := ep
		optFns = append(optFns, func(o *ddb.Options) {
			o.BaseEndpoint = aws.String(endpoint)
		})
	}
	return &Connector{
		id:     src.ID,
		client: ddb.NewFromConfig(cfg, optFns...),
		caps: def.Caps{
			Filter: true, Project: true, Limit: true,
		},
	}, nil
}

// ID implements runtime behavior for this package.
func (c *Connector) ID() string { return c.id }

// Type implements runtime behavior for this package.
func (c *Connector) Type() protocol.SourceType { return protocol.SourceDynamoDB }

// Capabilities implements runtime behavior for this package.
func (c *Connector) Capabilities() def.Caps { return c.caps }

// Close releases resources.
func (c *Connector) Close() error { return nil }

// Query fetches rows from a source.
func (c *Connector) Query(ctx context.Context, step def.PushdownStep) (*protocol.TabularResult, error) {
	for _, s := range step.Select {
		if s.Agg != "" {
			return nil, protocol.NewError(protocol.ErrUnsupported, "dynamodb does not push down aggregations", map[string]any{"source": c.id})
		}
	}
	table := step.Entity.Binding.Table
	if table == "" {
		return nil, protocol.NewError(protocol.ErrConfigError, "dynamodb entity needs binding.table", nil)
	}
	pks := step.Entity.Binding.AccessPath.PartitionKeys()
	eqs, perr := keycond.RequireEq(step.Where, pks)
	if perr != nil {
		return nil, perr
	}
	names := map[string]string{}
	values := map[string]types.AttributeValue{}
	var cond []string
	n := 0
	addEq := func(logical string) error {
		phys := def.PhysicalName(step.Entity, logical)
		nk := "#n" + strconv.Itoa(n)
		vk := ":v" + strconv.Itoa(n)
		n++
		names[nk] = phys
		av, err := attributevalue.Marshal(eqs[logical])
		if err != nil {
			return err
		}
		values[vk] = av
		cond = append(cond, nk+" = "+vk)
		return nil
	}
	for _, k := range pks {
		if err := addEq(k); err != nil {
			return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
		}
	}
	if sk := step.Entity.Binding.AccessPath.SortKey(); sk != "" {
		if _, ok := eqs[sk]; ok {
			if err := addEq(sk); err != nil {
				return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": c.id})
			}
		}
	}
	in := &ddb.QueryInput{
		TableName:                 aws.String(table),
		KeyConditionExpression:    aws.String(joinAnd(cond)),
		ExpressionAttributeNames:  names,
		ExpressionAttributeValues: values,
	}
	if step.Limit > 0 {
		in.Limit = aws.Int32(int32(step.Limit))
	}
	out, err := c.client.Query(ctx, in)
	if err != nil {
		if ctx.Err() != nil {
			return nil, protocol.NewError(protocol.ErrTimeout, fmt.Sprintf("source %s exceeded timeout", c.id), map[string]any{"source": c.id})
		}
		return nil, protocol.NewError(protocol.ErrSourceError, fmt.Sprintf("source %s query failed", c.id), map[string]any{"source": c.id, "cause": err.Error()})
	}
	columns, rows := projectItems(step, out.Items)
	truncated := step.Limit > 0 && len(rows) >= step.Limit
	return result.New(columns, rows, truncated), nil
}

// joinAnd implements runtime behavior for this package.
func joinAnd(parts []string) string {
	s := ""
	for i, p := range parts {
		if i > 0 {
			s += " AND "
		}
		s += p
	}
	return s
}

// projectItems implements runtime behavior for this package.
func projectItems(step def.PushdownStep, items []map[string]types.AttributeValue) ([]protocol.Column, [][]any) {
	sel := step.Select
	if len(sel) == 0 {
		sel = []def.SelectItem{}
		for _, f := range step.Entity.Fields {
			sel = append(sel, def.SelectItem{Field: f.Name, As: f.Name})
		}
	}
	cols := make([]protocol.Column, len(sel))
	for i, s := range sel {
		as := s.As
		if as == "" {
			as = s.Field
		}
		cols[i] = protocol.Column{Name: as, Type: def.FieldType(step.Entity, s.Field)}
	}
	var rows [][]any
	for _, item := range items {
		var m map[string]any
		_ = attributevalue.UnmarshalMap(item, &m)
		row := make([]any, len(sel))
		for i, s := range sel {
			row[i] = m[def.PhysicalName(step.Entity, s.Field)]
		}
		rows = append(rows, row)
	}
	return cols, rows
}
