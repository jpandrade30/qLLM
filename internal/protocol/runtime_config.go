package protocol

// RuntimeConfig is optional serve/runtime settings (qllm.config.yaml).
// Not part of the Query IR data-plane.
type RuntimeConfig struct {
	Serve ServeConfig `json:"serve,omitempty" yaml:"serve,omitempty"`
}

type ServeConfig struct {
	Addr                 string     `json:"addr,omitempty" yaml:"addr,omitempty"`
	MCPAddr              string     `json:"mcpAddr,omitempty" yaml:"mcpAddr,omitempty"`
	AuthTokenEnv         string     `json:"authTokenEnv,omitempty" yaml:"authTokenEnv,omitempty"`
	InsecureBind         *bool      `json:"insecureBind,omitempty" yaml:"insecureBind,omitempty"`
	MaxBodyBytes         *int64     `json:"maxBodyBytes,omitempty" yaml:"maxBodyBytes,omitempty"`
	MaxRestResponseBytes *int64     `json:"maxRestResponseBytes,omitempty" yaml:"maxRestResponseBytes,omitempty"`
	CORS                 *CORSConfig `json:"cors,omitempty" yaml:"cors,omitempty"`
}

type CORSConfig struct {
	Origins      []string `json:"origins,omitempty" yaml:"origins,omitempty"`
	AllowHeaders []string `json:"allowHeaders,omitempty" yaml:"allowHeaders,omitempty"`
	AllowMethods []string `json:"allowMethods,omitempty" yaml:"allowMethods,omitempty"`
}

// EnvFile seeds process environment from qllm.env.yaml (empty process env only).
type EnvFile struct {
	Env map[string]string `json:"env" yaml:"env"`
}

// ServeSettings is the merged effective serve configuration.
type ServeSettings struct {
	Addr                 string
	MCPAddr              string
	AuthTokenEnv         string
	AuthToken            string // resolved from env; empty = auth off
	InsecureBind         bool
	MaxBodyBytes         int64
	MaxRestResponseBytes int64
	CORS                 CORSConfig
}

const (
	DefaultHTTPAddr            = "127.0.0.1:8088"
	DefaultMCPAddr             = "127.0.0.1:8089"
	DefaultMaxBodyBytes        = int64(1 << 20)  // 1 MiB
	DefaultMaxRestResponseBytes = int64(10 << 20) // 10 MiB
)

func DefaultServeSettings() ServeSettings {
	return ServeSettings{
		Addr:                 DefaultHTTPAddr,
		MCPAddr:              DefaultMCPAddr,
		InsecureBind:         false,
		MaxBodyBytes:         DefaultMaxBodyBytes,
		MaxRestResponseBytes: DefaultMaxRestResponseBytes,
		CORS: CORSConfig{
			AllowHeaders: []string{"Content-Type", "Accept", "Mcp-Session-Id", "Authorization"},
			AllowMethods: []string{"GET", "POST", "DELETE", "OPTIONS"},
		},
	}
}
