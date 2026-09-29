package protocol

const (
	SQLDialect1      = "1"
	SQLDialect2      = "2"
	SQLDialectLatest = SQLDialect2
)

type SQLRequest struct {
	Version string `json:"version,omitempty" yaml:"version,omitempty"`
	SQL     string `json:"sql" yaml:"sql"`
}

func ResolveSQLVersion(version string) (string, *ProtocolError) {
	if version == "" {
		return SQLDialectLatest, nil
	}
	switch version {
	case SQLDialect1, SQLDialect2:
		return version, nil
	default:
		return "", NewError(ErrUnsupportedVersion, "unsupported SQL dialect version: "+version, map[string]any{
			"version": version,
			"latest":  SQLDialectLatest,
		})
	}
}

type AccessFile struct {
	Apps []AccessApp `json:"apps" yaml:"apps"`
}

type AccessApp struct {
	Name   string   `json:"name" yaml:"name"`
	Key    string   `json:"key" yaml:"key"`
	Tables []string `json:"tables" yaml:"tables"`
}
