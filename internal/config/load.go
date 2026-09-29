package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"qLLM/internal/protocol"
	"qLLM/internal/validate"

	"gopkg.in/yaml.v3"
)

var envPlaceholder = regexp.MustCompile(`^\$\{([A-Za-z_][A-Za-z0-9_]*)\}$`)

type Paths struct {
	Preset  string
	Catalog string
}

type Options struct {
	Preset    string
	Catalog   string
	Project   string
	ConfigDir string
}

func Resolve(opts Options) (Paths, error) {
	if opts.Preset != "" && opts.Catalog != "" {
		return Paths{Preset: opts.Preset, Catalog: opts.Catalog}, nil
	}
	if opts.Preset != "" || opts.Catalog != "" {
		return Paths{}, protocol.NewError(protocol.ErrConfigError,
			"both --preset and --catalog are required when either is set", nil)
	}
	if opts.Project != "" {
		return resolveProject(opts.Project)
	}
	dir := opts.ConfigDir
	if dir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return Paths{}, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
		}
		dir = cwd
	}
	preset, err := findNamed(dir, "qllm.preset")
	if err != nil {
		return Paths{}, err
	}
	catalog, err := findNamed(dir, "qllm.catalog")
	if err != nil {
		return Paths{}, err
	}
	return Paths{Preset: preset, Catalog: catalog}, nil
}

func resolveProject(path string) (Paths, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Paths{}, protocol.NewError(protocol.ErrConfigError,
			fmt.Sprintf("read project file: %v", err), nil)
	}
	var pf protocol.ProjectFile
	if err := unmarshalFlexible(path, raw, &pf); err != nil {
		return Paths{}, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	if pf.Preset == "" || pf.Catalog == "" {
		return Paths{}, protocol.NewError(protocol.ErrConfigError,
			"project file must set preset and catalog", nil)
	}
	base := filepath.Dir(path)
	preset := filepath.Clean(filepath.Join(base, pf.Preset))
	catalog := filepath.Clean(filepath.Join(base, pf.Catalog))
	preset, err = ConfinePath(base, preset)
	if err != nil {
		return Paths{}, err
	}
	catalog, err = ConfinePath(base, catalog)
	if err != nil {
		return Paths{}, err
	}
	return Paths{Preset: preset, Catalog: catalog}, nil
}

func findNamed(dir, base string) (string, error) {
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		p := filepath.Join(dir, base+ext)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", protocol.NewError(protocol.ErrConfigError,
		fmt.Sprintf("missing %s.{yaml|yml|json} in %s", base, dir), nil)
}

func LoadPreset(path string) (*protocol.Preset, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError,
			fmt.Sprintf("read preset: %v", err), nil)
	}
	var p protocol.Preset
	if err := unmarshalFlexible(path, raw, &p); err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	return &p, nil
}

func LoadCatalog(path string) (*protocol.Catalog, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError,
			fmt.Sprintf("read catalog: %v", err), nil)
	}
	var c protocol.Catalog
	if err := unmarshalFlexible(path, raw, &c); err != nil {
		return nil, protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	return &c, nil
}

func LoadAccess(explicitPath, configDir string) (*protocol.AccessFile, string, error) {
	var path string
	var err error
	if explicitPath != "" {
		path = explicitPath
	} else {
		dir := configDir
		if dir == "" {
			dir, err = os.Getwd()
			if err != nil {
				return nil, "", protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
			}
		}
		path, err = findNamedOptional(dir, "qllm.access")
		if err != nil {
			return nil, "", err
		}
		if path == "" {
			return nil, "", nil
		}
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, "", protocol.NewError(protocol.ErrConfigError,
			fmt.Sprintf("read access config: %v", err), nil)
	}
	var af protocol.AccessFile
	if err := unmarshalFlexible(path, raw, &af); err != nil {
		return nil, "", protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	return &af, path, nil
}

// ApplyEnvFile loads optional qllm.env.yaml and sets empty process env keys only.
func ApplyEnvFile(configDir string) error {
	dir := configDir
	if dir == "" {
		var err error
		dir, err = os.Getwd()
		if err != nil {
			return protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
		}
	}
	path, err := findNamedOptional(dir, "qllm.env")
	if err != nil {
		return err
	}
	if path == "" {
		return nil
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return protocol.NewError(protocol.ErrConfigError,
			fmt.Sprintf("read env file: %v", err), nil)
	}
	var ef protocol.EnvFile
	if err := unmarshalFlexible(path, raw, &ef); err != nil {
		return protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	if perr := validate.EnvFile(&ef); perr != nil {
		return perr
	}
	for k, v := range ef.Env {
		if os.Getenv(k) != "" {
			continue
		}
		val, skip, perr := expandEnvFileValue(k, v)
		if perr != nil {
			return perr
		}
		if skip {
			continue
		}
		if err := os.Setenv(k, val); err != nil {
			return protocol.NewError(protocol.ErrConfigError, err.Error(), map[string]any{"env": k})
		}
	}
	return nil
}

// expandEnvFileValue: literal YAML, or exactly ${NAME} from the process env.
// Missing NAME is skip (do not write the placeholder string). Malformed ${} is CONFIG_ERROR.
func expandEnvFileValue(k, v string) (string, bool, *protocol.ProtocolError) {
	if !strings.Contains(v, "${") {
		return v, false, nil
	}
	m := envPlaceholder.FindStringSubmatch(v)
	if m == nil {
		return "", false, protocol.NewError(protocol.ErrConfigError,
			"env value placeholder must be exactly ${ENV_NAME}", map[string]any{"env": k})
	}
	from := os.Getenv(m[1])
	if from == "" {
		return "", true, nil
	}
	return from, false, nil
}

func LoadQueryIR(path string) (*protocol.QueryIR, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, protocol.NewError(protocol.ErrInvalidIR,
			fmt.Sprintf("read IR: %v", err), nil)
	}
	var q protocol.QueryIR
	if err := unmarshalFlexible(path, raw, &q); err != nil {
		return nil, protocol.NewError(protocol.ErrInvalidIR, err.Error(), nil)
	}
	return &q, nil
}

func LoadBundle(opts Options) (*protocol.Preset, *protocol.Catalog, Paths, error) {
	paths, err := Resolve(opts)
	if err != nil {
		return nil, nil, Paths{}, err
	}
	preset, err := LoadPreset(paths.Preset)
	if err != nil {
		return nil, nil, Paths{}, err
	}
	catalog, err := LoadCatalog(paths.Catalog)
	if err != nil {
		return nil, nil, Paths{}, err
	}
	return preset, catalog, paths, nil
}

func unmarshalFlexible(path string, raw []byte, dest any) error {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return json.Unmarshal(raw, dest)
	case ".yaml", ".yml", "":
		// YAML path, or try YAML then JSON
		if err := yaml.Unmarshal(raw, dest); err != nil {
			if ext == "" {
				return json.Unmarshal(raw, dest)
			}
			return err
		}
		return nil
	default:
		if err := yaml.Unmarshal(raw, dest); err != nil {
			return json.Unmarshal(raw, dest)
		}
		return nil
	}
}

// EnvString reads connection env key from map.
func EnvString(conn map[string]any, key string) (string, error) {
	v, ok := conn[key]
	if !ok {
		return "", fmt.Errorf("missing connection.%s", key)
	}
	name, ok := v.(string)
	if !ok || name == "" {
		return "", fmt.Errorf("connection.%s must be a non-empty string", key)
	}
	val := os.Getenv(name)
	if val == "" {
		return "", fmt.Errorf("environment variable %s is empty", name)
	}
	return val, nil
}

func OptionalEnvString(conn map[string]any, key string) string {
	v, ok := conn[key]
	if !ok {
		return ""
	}
	name, _ := v.(string)
	if name == "" {
		return ""
	}
	return os.Getenv(name)
}

func ConnInt(conn map[string]any, key string, def int) int {
	v, ok := conn[key]
	if !ok {
		return def
	}
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return def
	}
}

func ConnString(conn map[string]any, key, def string) string {
	v, ok := conn[key]
	if !ok {
		return def
	}
	s, ok := v.(string)
	if !ok {
		return def
	}
	return s
}

func ConnBool(conn map[string]any, key string, def bool) bool {
	v, ok := conn[key]
	if !ok {
		return def
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		return strings.EqualFold(t, "true") || t == "1"
	default:
		return def
	}
}
