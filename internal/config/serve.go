package config

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"qLLM/internal/protocol"
)

// LoadRuntimeConfig loads optional qllm.config.* from an explicit path or directory.
// Missing file returns (nil, nil).
func LoadRuntimeConfig(explicitPath, configDir string) (*protocol.RuntimeConfig, string, error) {
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
		path, err = findNamedOptional(dir, "qllm.config")
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
			fmt.Sprintf("read runtime config: %v", err), nil)
	}
	var rc protocol.RuntimeConfig
	if err := unmarshalFlexible(path, raw, &rc); err != nil {
		return nil, "", protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	if err := validateRuntimeConfig(&rc); err != nil {
		return nil, "", err
	}
	return &rc, path, nil
}

// findNamedOptional implements runtime behavior for this package.
func findNamedOptional(dir, base string) (string, error) {
	for _, ext := range []string{".yaml", ".yml", ".json"} {
		p := filepath.Join(dir, base+ext)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, nil
		}
	}
	return "", nil
}

// validateRuntimeConfig implements runtime behavior for this package.
func validateRuntimeConfig(rc *protocol.RuntimeConfig) error {
	if rc.Serve.CORS != nil {
		for _, o := range rc.Serve.CORS.Origins {
			if o == "*" {
				return protocol.NewError(protocol.ErrConfigError,
					"serve.cors.origins must not include wildcard *", nil)
			}
		}
	}
	return nil
}

// MergeServeSettings applies defaults → file → flag overrides.
type ServeFlagOverrides struct {
	Addr                 *string
	MCPAddr              *string
	AuthTokenEnv         *string
	InsecureBind         *bool
	CORSOrigins          []string
	CORSOriginsSet       bool
	MaxBodyBytes         *int64
	MaxRestResponseBytes *int64
}

// MergeServeSettings merges values.
func MergeServeSettings(file *protocol.RuntimeConfig, flags ServeFlagOverrides) (protocol.ServeSettings, error) {
	s := protocol.DefaultServeSettings()
	if file != nil {
		applyServeFile(&s, file.Serve)
	}
	if flags.Addr != nil {
		s.Addr = *flags.Addr
	}
	if flags.MCPAddr != nil {
		s.MCPAddr = *flags.MCPAddr
	}
	if flags.AuthTokenEnv != nil {
		s.AuthTokenEnv = *flags.AuthTokenEnv
	}
	if flags.InsecureBind != nil {
		s.InsecureBind = *flags.InsecureBind
	}
	if flags.CORSOriginsSet {
		s.CORS.Origins = append([]string(nil), flags.CORSOrigins...)
	}
	if flags.MaxBodyBytes != nil {
		s.MaxBodyBytes = *flags.MaxBodyBytes
	}
	if flags.MaxRestResponseBytes != nil {
		s.MaxRestResponseBytes = *flags.MaxRestResponseBytes
	}
	for _, o := range s.CORS.Origins {
		if o == "*" {
			return s, protocol.NewError(protocol.ErrConfigError,
				"serve.cors.origins must not include wildcard *", nil)
		}
	}
	if s.AuthTokenEnv != "" {
		s.AuthToken = os.Getenv(s.AuthTokenEnv)
		if s.AuthToken == "" {
			return s, protocol.NewError(protocol.ErrConfigError,
				fmt.Sprintf("environment variable %s is empty (required by serve.authTokenEnv)", s.AuthTokenEnv),
				map[string]any{"authTokenEnv": s.AuthTokenEnv})
		}
	}
	return s, nil
}

// applyServeFile implements runtime behavior for this package.
func applyServeFile(s *protocol.ServeSettings, f protocol.ServeConfig) {
	if f.Addr != "" {
		s.Addr = f.Addr
	}
	if f.MCPAddr != "" {
		s.MCPAddr = f.MCPAddr
	}
	if f.AuthTokenEnv != "" {
		s.AuthTokenEnv = f.AuthTokenEnv
	}
	if f.InsecureBind != nil {
		s.InsecureBind = *f.InsecureBind
	}
	if f.MaxBodyBytes != nil {
		s.MaxBodyBytes = *f.MaxBodyBytes
	}
	if f.MaxRestResponseBytes != nil {
		s.MaxRestResponseBytes = *f.MaxRestResponseBytes
	}
	if f.CORS != nil {
		if f.CORS.Origins != nil {
			s.CORS.Origins = append([]string(nil), f.CORS.Origins...)
		}
		if len(f.CORS.AllowHeaders) > 0 {
			s.CORS.AllowHeaders = append([]string(nil), f.CORS.AllowHeaders...)
		}
		if len(f.CORS.AllowMethods) > 0 {
			s.CORS.AllowMethods = append([]string(nil), f.CORS.AllowMethods...)
		}
	}
}

// CheckBindPolicy refuses non-loopback listen without auth unless insecureBind.
func CheckBindPolicy(addr string, authToken string, insecureBind bool) error {
	if IsLoopbackAddr(addr) {
		return nil
	}
	if authToken != "" {
		return nil
	}
	if insecureBind {
		return nil
	}
	return protocol.NewError(protocol.ErrConfigError,
		fmt.Sprintf("refusing non-loopback bind %q without auth: set serve.authTokenEnv (non-empty token) or insecureBind / --insecure-bind", addr),
		map[string]any{"addr": addr})
}

// IsLoopbackAddr reports whether addr listens only on loopback.
// Bare ":port" / "0.0.0.0:port" / "[::]:port" are not loopback.
func IsLoopbackAddr(addr string) bool {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// Allow host without port only if it's clearly loopback name (unlikely for Listen).
		host = addr
	}
	host = strings.Trim(host, "[]")
	if host == "" || host == "0.0.0.0" || host == "::" || host == "*" {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback()
}

// ConfinePath ensures resolved is under base (after Abs/Clean/EvalSymlinks).
func ConfinePath(base, resolved string) (string, error) {
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	absRes, err := filepath.Abs(resolved)
	if err != nil {
		return "", protocol.NewError(protocol.ErrConfigError, err.Error(), nil)
	}
	absBase = filepath.Clean(absBase)
	absRes = filepath.Clean(absRes)
	if real, err := filepath.EvalSymlinks(absBase); err == nil {
		absBase = real
	}
	if real, err := filepath.EvalSymlinks(absRes); err == nil {
		absRes = real
	} else {
		// File may not exist yet; resolve parent and rejoin base name.
		parent, name := filepath.Split(absRes)
		if parent != "" {
			if realParent, err := filepath.EvalSymlinks(filepath.Clean(parent)); err == nil {
				absRes = filepath.Join(realParent, name)
			}
		}
	}
	rel, err := filepath.Rel(absBase, absRes)
	if err != nil || strings.HasPrefix(rel, "..") || rel == ".." {
		return "", protocol.NewError(protocol.ErrConfigError,
			fmt.Sprintf("path %q escapes project base %q", resolved, base), nil)
	}
	return absRes, nil
}
