package mongo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"qLLM/internal/config"
	"qLLM/internal/connector/def"
	"qLLM/internal/protocol"
	"qLLM/internal/result"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Connector struct {
	id   string
	cli  *mongo.Client
	db   *mongo.Database
	caps def.Caps
}

func Open(src protocol.Source) (*Connector, error) {
	uri, err := config.EnvString(src.Connection, "uriEnv")
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	dbName := config.ConnString(src.Connection, "database", "")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cli, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, protocol.NewError(protocol.ErrSourceError, err.Error(), nil)
	}
	return &Connector{
		id:  src.ID,
		cli: cli,
		db:  cli.Database(dbName),
		caps: def.Caps{
			Filter: true, Project: true, Agg: true, GroupBy: true,
			OrderBy: true, Limit: true,
		},
	}, nil
}

func (c *Connector) ID() string                   { return c.id }
func (c *Connector) Type() protocol.SourceType    { return protocol.SourceMongoDB }
func (c *Connector) Capabilities() def.Caps { return c.caps }
func (c *Connector) Close() error                 { return c.cli.Disconnect(context.Background()) }

func (c *Connector) Query(ctx context.Context, step def.PushdownStep) (*protocol.TabularResult, error) {
	collName := step.Entity.Binding.Collection
	coll := c.db.Collection(collName)

	filter, err := whereBSON(step.Entity, step.Where)
	if err != nil {
		return nil, err
	}

	hasAgg := false
	for _, s := range step.Select {
		if s.Agg != "" {
			hasAgg = true
			break
		}
	}

	if hasAgg {
		return c.aggQuery(ctx, coll, step, filter)
	}

	opts := options.Find()
	if step.Limit > 0 {
		lim := int64(step.Limit)
		opts.SetLimit(lim)
	}
	if step.Offset > 0 {
		off := int64(step.Offset)
		opts.SetSkip(off)
	}
	if len(step.OrderBy) > 0 {
		sort := bson.D{}
		for _, o := range step.OrderBy {
			field := stripQual(o.Field)
			phys := def.PhysicalName(step.Entity, field)
			dir := 1
			if strings.EqualFold(o.Dir, "desc") {
				dir = -1
			}
			sort = append(sort, bson.E{Key: phys, Value: dir})
		}
		opts.SetSort(sort)
	}
	proj := bson.M{}
	columns := []protocol.Column{}
	for _, s := range step.Select {
		field := s.Field
		as := s.As
		if as == "" {
			as = field
		}
		phys := def.PhysicalName(step.Entity, field)
		proj[phys] = 1
		columns = append(columns, protocol.Column{Name: as, Type: def.FieldType(step.Entity, field)})
	}
	if len(proj) > 0 {
		opts.SetProjection(proj)
	}

	cur, err := coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, mapErr(c.id, ctx, err)
	}
	defer cur.Close(ctx)

	rows := [][]any{}
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			return nil, mapErr(c.id, ctx, err)
		}
		row := make([]any, len(step.Select))
		for i, s := range step.Select {
			phys := def.PhysicalName(step.Entity, s.Field)
			row[i] = normalize(doc[phys])
		}
		rows = append(rows, row)
	}
	truncated := step.Limit > 0 && len(rows) >= step.Limit
	return result.New(columns, rows, truncated), nil
}

func (c *Connector) aggQuery(ctx context.Context, coll *mongo.Collection, step def.PushdownStep, filter bson.M) (*protocol.TabularResult, error) {
	pipeline := mongo.Pipeline{}
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$match", Value: filter}})
	}
	groupID := bson.M{}
	for _, g := range step.GroupBy {
		field := stripQual(g)
		phys := def.PhysicalName(step.Entity, field)
		groupID[field] = "$" + phys
	}
	group := bson.M{"_id": groupID}
	columns := []protocol.Column{}
	outFields := []string{}
	for _, g := range step.GroupBy {
		field := stripQual(g)
		columns = append(columns, protocol.Column{Name: field, Type: def.FieldType(step.Entity, field)})
		outFields = append(outFields, field)
	}
	for _, s := range step.Select {
		if s.Agg == "" {
			continue
		}
		as := s.As
		switch strings.ToLower(s.Agg) {
		case "count":
			group[as] = bson.M{"$sum": 1}
			columns = append(columns, protocol.Column{Name: as, Type: protocol.TypeNumber})
		case "sum", "avg", "min", "max":
			phys := def.PhysicalName(step.Entity, stripQual(s.Field))
			group[as] = bson.M{"$" + strings.ToLower(s.Agg): "$" + phys}
			columns = append(columns, protocol.Column{Name: as, Type: protocol.TypeNumber})
		default:
			return nil, protocol.NewError(protocol.ErrUnsupported, "unsupported agg: "+s.Agg, nil)
		}
		outFields = append(outFields, as)
	}
	pipeline = append(pipeline, bson.D{{Key: "$group", Value: group}})
	if step.Limit > 0 {
		pipeline = append(pipeline, bson.D{{Key: "$limit", Value: step.Limit}})
	}

	cur, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, mapErr(c.id, ctx, err)
	}
	defer cur.Close(ctx)
	rows := [][]any{}
	for cur.Next(ctx) {
		var doc bson.M
		if err := cur.Decode(&doc); err != nil {
			return nil, mapErr(c.id, ctx, err)
		}
		row := make([]any, len(outFields))
		idMap, _ := doc["_id"].(bson.M)
		for i, name := range outFields {
			if idMap != nil {
				if v, ok := idMap[name]; ok {
					row[i] = normalize(v)
					continue
				}
			}
			row[i] = normalize(doc[name])
		}
		rows = append(rows, row)
	}
	return result.New(columns, rows, false), nil
}

func whereBSON(e *protocol.Entity, w map[string]any) (bson.M, error) {
	if w == nil {
		return bson.M{}, nil
	}
	if op, ok := w["op"].(string); ok {
		switch op {
		case "and", "or":
			args, _ := w["args"].([]any)
			parts := make([]bson.M, 0, len(args))
			for _, a := range args {
				m, _ := a.(map[string]any)
				sub, err := whereBSON(e, m)
				if err != nil {
					return nil, err
				}
				parts = append(parts, sub)
			}
			key := "$and"
			if op == "or" {
				key = "$or"
			}
			return bson.M{key: parts}, nil
		case "not":
			args, _ := w["args"].([]any)
			if len(args) != 1 {
				return nil, protocol.NewError(protocol.ErrInvalidIR, "not requires one arg", nil)
			}
			m, _ := args[0].(map[string]any)
			sub, err := whereBSON(e, m)
			if err != nil {
				return nil, err
			}
			return bson.M{"$nor": []bson.M{sub}}, nil
		}
	}
	field, _ := w["field"].(string)
	op, _ := w["op"].(string)
	field = stripQual(field)
	phys := def.PhysicalName(e, field)
	switch op {
	case "eq":
		return bson.M{phys: w["value"]}, nil
	case "neq":
		return bson.M{phys: bson.M{"$ne": w["value"]}}, nil
	case "gt":
		return bson.M{phys: bson.M{"$gt": w["value"]}}, nil
	case "gte":
		return bson.M{phys: bson.M{"$gte": w["value"]}}, nil
	case "lt":
		return bson.M{phys: bson.M{"$lt": w["value"]}}, nil
	case "lte":
		return bson.M{phys: bson.M{"$lte": w["value"]}}, nil
	case "in":
		return bson.M{phys: bson.M{"$in": w["value"]}}, nil
	case "nin":
		return bson.M{phys: bson.M{"$nin": w["value"]}}, nil
	case "contains":
		return bson.M{phys: bson.M{"$regex": fmt.Sprint(w["value"])}}, nil
	case "is_null":
		return bson.M{phys: nil}, nil
	case "not_null":
		return bson.M{phys: bson.M{"$ne": nil}}, nil
	default:
		return nil, protocol.NewError(protocol.ErrUnsupported, "unsupported op: "+op, nil)
	}
}

func stripQual(f string) string {
	if i := strings.LastIndex(f, "."); i >= 0 {
		return f[i+1:]
	}
	return f
}

func normalize(v any) any {
	switch t := v.(type) {
	case time.Time:
		return t.UTC().Format(time.RFC3339)
	default:
		return t
	}
}

func mapErr(id string, ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return protocol.NewError(protocol.ErrTimeout,
			fmt.Sprintf("source %s exceeded timeout", id), map[string]any{"source": id})
	}
	return protocol.NewError(protocol.ErrSourceError, err.Error(), map[string]any{"source": id})
}
