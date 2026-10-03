package protocol

const ProtocolVersion = "0.2.0"

type ErrorCode string

const (
	ErrInvalidIR          ErrorCode = "INVALID_IR"
	ErrUnknownEntity      ErrorCode = "UNKNOWN_ENTITY"
	ErrUnknownField       ErrorCode = "UNKNOWN_FIELD"
	ErrAmbiguousField     ErrorCode = "AMBIGUOUS_FIELD"
	ErrAmbiguousAlias     ErrorCode = "AMBIGUOUS_ALIAS"
	ErrLimitExceeded      ErrorCode = "LIMIT_EXCEEDED"
	ErrForbidden          ErrorCode = "FORBIDDEN"
	ErrUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrUnsupported        ErrorCode = "UNSUPPORTED"
	ErrUnsupportedVersion ErrorCode = "UNSUPPORTED_VERSION"
	ErrInvalidSQL         ErrorCode = "INVALID_SQL"
	ErrConfigError        ErrorCode = "CONFIG_ERROR"
	ErrTimeout            ErrorCode = "TIMEOUT"
	ErrSourceError        ErrorCode = "SOURCE_ERROR"
	ErrNotReady           ErrorCode = "NOT_READY"
	ErrNotFound           ErrorCode = "NOT_FOUND"
	ErrInternal           ErrorCode = "INTERNAL"
)

type ProtocolError struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// Error implements runtime behavior for this package.
func (e *ProtocolError) Error() string {
	return string(e.Code) + ": " + e.Message
}

// NewError constructs a value.
func NewError(code ErrorCode, message string, details map[string]any) *ProtocolError {
	return &ProtocolError{Code: code, Message: message, Details: details}
}

type ErrorResponse struct {
	ProtocolVersion string         `json:"protocolVersion"`
	Error           *ProtocolError `json:"error"`
}

type Limits struct {
	MaxSyncMs    int  `json:"maxSyncMs" yaml:"maxSyncMs"`
	MaxSourceMs  int  `json:"maxSourceMs" yaml:"maxSourceMs"`
	DefaultLimit int  `json:"defaultLimit" yaml:"defaultLimit"`
	MaxLimit     int  `json:"maxLimit" yaml:"maxLimit"`
	ReadOnly     bool `json:"readOnly" yaml:"readOnly"`
}

type SourceType string

const (
	SourcePostgres   SourceType = "postgres"
	SourceMySQL      SourceType = "mysql"
	SourceMongoDB    SourceType = "mongodb"
	SourceREST       SourceType = "rest"
	SourceMSSQL      SourceType = "mssql"
	SourceSQLite     SourceType = "sqlite"
	SourceClickHouse SourceType = "clickhouse"
	SourceDynamoDB   SourceType = "dynamodb"
	SourceCassandra  SourceType = "cassandra"
	SourceKSQL       SourceType = "ksql"
	SourceRedis      SourceType = "redis"
	SourceKafka      SourceType = "kafka"

	SourceMariaDB     SourceType = "mariadb"
	SourceTiDB        SourceType = "tidb"
	SourceVitess      SourceType = "vitess"
	SourceAuroraMySQL SourceType = "aurora_mysql"
	SourcePlanetScale SourceType = "planetscale"

	SourceCockroach      SourceType = "cockroach"
	SourceYugabyte       SourceType = "yugabyte"
	SourceAlloyDB        SourceType = "alloydb"
	SourceAuroraPostgres SourceType = "aurora_postgres"
	SourceNeon           SourceType = "neon"
	SourceSupabase       SourceType = "supabase"
	SourceTimescale      SourceType = "timescale"
	SourceRedshift       SourceType = "redshift"
)

// WireFamily maps a source type to the driver it reuses (itself if none).
func WireFamily(t SourceType) SourceType {
	switch t {
	case SourceMariaDB, SourceTiDB, SourceVitess, SourceAuroraMySQL, SourcePlanetScale:
		return SourceMySQL
	case SourceCockroach, SourceYugabyte, SourceAlloyDB, SourceAuroraPostgres,
		SourceNeon, SourceSupabase, SourceTimescale, SourceRedshift:
		return SourcePostgres
	default:
		return t
	}
}

type Source struct {
	ID         string         `json:"id" yaml:"id"`
	Type       SourceType     `json:"type" yaml:"type"`
	Connection map[string]any `json:"connection" yaml:"connection"`
	Options    map[string]any `json:"options,omitempty" yaml:"options,omitempty"`
}

type Preset struct {
	ProtocolVersion string   `json:"protocolVersion" yaml:"protocolVersion"`
	Project         string   `json:"project" yaml:"project"`
	Limits          Limits   `json:"limits" yaml:"limits"`
	Sources         []Source `json:"sources" yaml:"sources"`
}

type LogicalType string

const (
	TypeString    LogicalType = "string"
	TypeNumber    LogicalType = "number"
	TypeBoolean   LogicalType = "boolean"
	TypeTimestamp LogicalType = "timestamp"
	TypeJSON      LogicalType = "json"
)

type AccessPath struct {
	Partition []string `json:"partition,omitempty" yaml:"partition,omitempty"`
	PK        []string `json:"pk,omitempty" yaml:"pk,omitempty"`
	Sort      string   `json:"sort,omitempty" yaml:"sort,omitempty"`
	SK        string   `json:"sk,omitempty" yaml:"sk,omitempty"`
	KsqlKey   string   `json:"ksqlKey,omitempty" yaml:"ksqlKey,omitempty"`
	Key       string   `json:"key,omitempty" yaml:"key,omitempty"`
}

// PartitionKeys implements runtime behavior for this package.
func (a AccessPath) PartitionKeys() []string {
	if len(a.Partition) > 0 {
		return a.Partition
	}
	return a.PK
}

// SortKey implements runtime behavior for this package.
func (a AccessPath) SortKey() string {
	if a.Sort != "" {
		return a.Sort
	}
	return a.SK
}

// MessageKey is the Kafka record-key field (or ksqlKey fallback).
func (a AccessPath) MessageKey() string {
	if a.Key != "" {
		return a.Key
	}
	return a.KsqlKey
}

type Binding struct {
	Kind       string     `json:"kind" yaml:"kind"`
	Schema     string     `json:"schema,omitempty" yaml:"schema,omitempty"`
	Table      string     `json:"table,omitempty" yaml:"table,omitempty"`
	Collection string     `json:"collection,omitempty" yaml:"collection,omitempty"`
	Resource   string     `json:"resource,omitempty" yaml:"resource,omitempty"`
	KeyPattern string     `json:"keyPattern,omitempty" yaml:"keyPattern,omitempty"`
	Topic      string     `json:"topic,omitempty" yaml:"topic,omitempty"`
	AccessPath AccessPath `json:"accessPath,omitempty" yaml:"accessPath,omitempty"`
}

type Field struct {
	Name        string      `json:"name" yaml:"name"`
	Type        LogicalType `json:"type" yaml:"type"`
	Physical    string      `json:"physical" yaml:"physical"`
	Description string      `json:"description,omitempty" yaml:"description,omitempty"`
	FromFilter  bool        `json:"fromFilter,omitempty" yaml:"fromFilter,omitempty"`
}

type Relation struct {
	Name string     `json:"name" yaml:"name"`
	To   string     `json:"to" yaml:"to"`
	Type string     `json:"type" yaml:"type"`
	On   [][]string `json:"on" yaml:"on"`
}

type Entity struct {
	Name        string     `json:"name" yaml:"name"`
	Aliases     []string   `json:"aliases,omitempty" yaml:"aliases,omitempty"`
	Description string     `json:"description,omitempty" yaml:"description,omitempty"`
	Source      string     `json:"source" yaml:"source"`
	Binding     Binding    `json:"binding" yaml:"binding"`
	PrimaryKey  []string   `json:"primaryKey,omitempty" yaml:"primaryKey,omitempty"`
	Fields      []Field    `json:"fields" yaml:"fields"`
	Relations   []Relation `json:"relations,omitempty" yaml:"relations,omitempty"`
}

type Catalog struct {
	ProtocolVersion string   `json:"protocolVersion" yaml:"protocolVersion"`
	Project         string   `json:"project" yaml:"project"`
	Entities        []Entity `json:"entities" yaml:"entities"`
}

type ProjectFile struct {
	ProtocolVersion string `json:"protocolVersion" yaml:"protocolVersion"`
	Preset          string `json:"preset" yaml:"preset"`
	Catalog         string `json:"catalog" yaml:"catalog"`
}

type JoinOn struct {
	Left  string `json:"left" yaml:"left"`
	Right string `json:"right" yaml:"right"`
}

type Join struct {
	Type string   `json:"type" yaml:"type"`
	From string   `json:"from" yaml:"from"`
	As   string   `json:"as,omitempty" yaml:"as,omitempty"`
	On   []JoinOn `json:"on" yaml:"on"`
}

type AggExpr struct {
	Agg   string `json:"agg" yaml:"agg"`
	Field string `json:"field,omitempty" yaml:"field,omitempty"`
	As    string `json:"as" yaml:"as"`
}

type OrderExpr struct {
	Field string `json:"field" yaml:"field"`
	Dir   string `json:"dir,omitempty" yaml:"dir,omitempty"`
}

// SelectItem is either a field ref string or an agg object.
// Unmarshaled via custom logic in validate/query parsing.
type QueryIR struct {
	ProtocolVersion string         `json:"protocolVersion,omitempty" yaml:"protocolVersion,omitempty"`
	From            string         `json:"from" yaml:"from"`
	As              string         `json:"as,omitempty" yaml:"as,omitempty"`
	Joins           []Join         `json:"joins,omitempty" yaml:"joins,omitempty"`
	Select          []any          `json:"select" yaml:"select"`
	Where           map[string]any `json:"where,omitempty" yaml:"where,omitempty"`
	GroupBy         []string       `json:"groupBy,omitempty" yaml:"groupBy,omitempty"`
	OrderBy         []OrderExpr    `json:"orderBy,omitempty" yaml:"orderBy,omitempty"`
	Limit           *int           `json:"limit,omitempty" yaml:"limit,omitempty"`
	Offset          *int           `json:"offset,omitempty" yaml:"offset,omitempty"`
	Mode            string         `json:"mode,omitempty" yaml:"mode,omitempty"`
}

type Column struct {
	Name string      `json:"name"`
	Type LogicalType `json:"type"`
}

type TabularResult struct {
	Columns   []Column `json:"columns"`
	Rows      [][]any  `json:"rows"`
	RowCount  int      `json:"rowCount"`
	Truncated bool     `json:"truncated"`
}

type PlanStepMeta struct {
	Source    string `json:"source"`
	Pushdown  bool   `json:"pushdown"`
	ElapsedMs int64  `json:"elapsedMs"`
}

type PlanMeta struct {
	UsedDuckDB bool           `json:"usedDuckDB"`
	Steps      []PlanStepMeta `json:"steps,omitempty"`
}

type QueryMeta struct {
	ElapsedMs int64     `json:"elapsedMs"`
	Mode      string    `json:"mode"`
	App       string    `json:"app,omitempty"`
	Plan      *PlanMeta `json:"plan,omitempty"`
}

type QueryStatus string

const (
	StatusAccepted  QueryStatus = "accepted"
	StatusRunning   QueryStatus = "running"
	StatusSucceeded QueryStatus = "succeeded"
	StatusFailed    QueryStatus = "failed"
	StatusCanceled  QueryStatus = "canceled"
)

type QueryResponse struct {
	ProtocolVersion string         `json:"protocolVersion"`
	QueryID         string         `json:"queryId"`
	Status          QueryStatus    `json:"status"`
	Result          *TabularResult `json:"result,omitempty"`
	Meta            *QueryMeta     `json:"meta,omitempty"`
	Error           *ProtocolError `json:"error,omitempty"`
}

type Capabilities struct {
	Filter         bool `json:"filter"`
	Project        bool `json:"project"`
	Agg            bool `json:"agg"`
	GroupBy        bool `json:"groupBy"`
	JoinSameSource bool `json:"joinSameSource"`
	OrderBy        bool `json:"orderBy"`
	Limit          bool `json:"limit"`
}

type SourceInfo struct {
	ID           string       `json:"id"`
	Type         SourceType   `json:"type"`
	Capabilities Capabilities `json:"capabilities"`
}

type CatalogResponse struct {
	ProtocolVersion string       `json:"protocolVersion"`
	Project         string       `json:"project"`
	Entities        []Entity     `json:"entities"`
	Sources         []SourceInfo `json:"sources"`
}

type HealthResponse struct {
	OK              bool   `json:"ok"`
	ProtocolVersion string `json:"protocolVersion"`
}

// HowToUseMeResponse is an LLM-oriented guide for querying this runtime.
type HowToUseMeResponse struct {
	ProtocolVersion string                `json:"protocolVersion"`
	Purpose         string                `json:"purpose"`
	Workflow        []string              `json:"workflow"`
	Never           []string              `json:"never"`
	NotSupported    []string              `json:"notSupported"`
	Endpoints       []HowToEndpoint       `json:"endpoints"`
	Grammar         string                `json:"grammar,omitempty"`
	Where           *HowToWhereGuide      `json:"where,omitempty"`
	FieldRefRules   []string              `json:"fieldRefRules,omitempty"`
	JoinRules       []string              `json:"joinRules,omitempty"`
	AggregateRules  []string              `json:"aggregateRules,omitempty"`
	OrderByRules    []string              `json:"orderByRules,omitempty"`
	QueryIR         *HowToQueryIR         `json:"queryIR,omitempty"`
	SQL             HowToSQLGuide         `json:"sql"`
	Rules           []string              `json:"rules"`
	Examples        []HowToExample        `json:"examples,omitempty"`
	InvalidExamples []HowToInvalidExample `json:"invalidExamples,omitempty"`
	Errors          []HowToError          `json:"errors"`
	Project         HowToProject          `json:"project"`
}

type HowToSQLGuide struct {
	LatestVersion     string            `json:"latestVersion"`
	SupportedVersions []string          `json:"supportedVersions"`
	Tool              string            `json:"tool"`
	HTTP              string            `json:"http"`
	Rules             []string          `json:"rules"`
	Supported         []string          `json:"supported"`
	Dialect2Only      []string          `json:"dialect2Only"`
	Reject            []string          `json:"reject"`
	Examples          []HowToSQLExample `json:"examples"`
}

type HowToSQLExample struct {
	Title   string `json:"title"`
	SQL     string `json:"sql"`
	Version string `json:"version,omitempty"`
}

type HowToWhereGuide struct {
	Shapes         []string       `json:"shapes"`
	ValidExample   map[string]any `json:"validExample"`
	InvalidExample map[string]any `json:"invalidExample"`
	Fix            map[string]any `json:"fix"`
}

type HowToInvalidExample struct {
	Wrong map[string]any `json:"wrong"`
	Why   string         `json:"why"`
	Fix   map[string]any `json:"fix"`
}

type HowToEndpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Use    string `json:"use"`
}

type HowToQueryIR struct {
	Shape      string   `json:"shape"`
	CompareOps []string `json:"compareOps"`
	AggOps     []string `json:"aggOps"`
	JoinTypes  []string `json:"joinTypes"`
	FieldRef   string   `json:"fieldRef"`
	Notes      []string `json:"notes"`
	Required   []string `json:"required"`
	Optional   []string `json:"optional"`
}

type HowToExample struct {
	Title string         `json:"title"`
	IR    map[string]any `json:"ir"`
}

type HowToError struct {
	Code string `json:"code"`
	When string `json:"when"`
}

type HowToProject struct {
	Name         string   `json:"name"`
	EntityNames  []string `json:"entityNames"`
	DefaultLimit int      `json:"defaultLimit"`
	MaxLimit     int      `json:"maxLimit"`
	MaxSyncMs    int      `json:"maxSyncMs"`
	ReadOnly     bool     `json:"readOnly"`
}
