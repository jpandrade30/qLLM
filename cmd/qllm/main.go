package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"qLLM/internal/access"
	"qLLM/internal/config"
	"qLLM/internal/connector"
	"qLLM/internal/executor"
	"qLLM/internal/httpserver"
	"qLLM/internal/mcpserver"
	"qLLM/internal/protocol"
	"qLLM/internal/querystore"
	"qLLM/internal/validate"

	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
)

// main implements runtime behavior for this package.
func main() {
	root := &cobra.Command{
		Use:   "qllm",
		Short: "qLLM multi-source query runtime",
	}

	var preset, catalog, project, configDir string
	addConfigFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringVar(&preset, "preset", "", "path to preset file")
		cmd.Flags().StringVar(&catalog, "catalog", "", "path to catalog file")
		cmd.Flags().StringVar(&project, "project", "", "path to qllm.project.yaml")
		cmd.Flags().StringVar(&configDir, "config-dir", "", "directory with qllm.preset.yaml and qllm.catalog.yaml")
	}
	opts := func() config.Options {
		return config.Options{Preset: preset, Catalog: catalog, Project: project, ConfigDir: configDir}
	}

	validateCmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate preset, catalog, and optional IR",
		RunE: func(cmd *cobra.Command, args []string) error {
			irPath, _ := cmd.Flags().GetString("ir")
			p, c, paths, err := config.LoadBundle(opts())
			if err != nil {
				return printErr(err)
			}
			idx, perr := validate.Bundle(p, c)
			if perr != nil {
				return printErr(perr)
			}
			fmt.Fprintf(os.Stderr, "ok preset=%s catalog=%s entities=%d\n", paths.Preset, paths.Catalog, len(idx.Catalog.Entities))
			if irPath != "" {
				q, err := config.LoadQueryIR(irPath)
				if err != nil {
					return printErr(err)
				}
				if perr := validate.Query(idx, q); perr != nil {
					return printErr(perr)
				}
				fmt.Fprintln(os.Stderr, "ok ir="+irPath)
			}
			return nil
		},
	}
	addConfigFlags(validateCmd)
	validateCmd.Flags().String("ir", "", "optional query IR file")

	queryCmd := &cobra.Command{
		Use:   "query",
		Short: "Execute a Query IR file",
		RunE: func(cmd *cobra.Command, args []string) error {
			irPath, _ := cmd.Flags().GetString("file")
			if irPath == "" {
				return fmt.Errorf("--file is required")
			}
			if err := config.ApplyEnvFile(opts().ConfigDir); err != nil {
				return printErr(err)
			}
			p, c, _, err := config.LoadBundle(opts())
			if err != nil {
				return printErr(err)
			}
			idx, perr := validate.Bundle(p, c)
			if perr != nil {
				return printErr(perr)
			}
			reg, err := connector.OpenAll(p, connector.OpenOpts{})
			if err != nil {
				return printErr(err)
			}
			defer reg.Close()
			store := querystore.New(2 * time.Minute)
			exec := executor.New(idx, reg, store)
			if af, _, err := config.LoadAccess("", opts().ConfigDir); err != nil {
				return printErr(err)
			} else if af != nil {
				acl, perr := access.Resolve(af, idx)
				if perr != nil {
					return printErr(perr)
				}
				exec.ACL = acl
			}
			appName, _ := cmd.Flags().GetString("app")
			if appName == "" {
				appName = os.Getenv("QLLM_APP")
			}
			exec.StdioApp = appName
			q, err := config.LoadQueryIR(irPath)
			if err != nil {
				return printErr(err)
			}
			resp := exec.Execute(context.Background(), q)
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp)
		},
	}
	addConfigFlags(queryCmd)
	queryCmd.Flags().StringP("file", "f", "", "query IR JSON/YAML file")
	queryCmd.Flags().String("app", "", "app name when qllm.access.yaml is present")

	sqlCmd := &cobra.Command{
		Use:   "sql",
		Short: "Execute a catalog SQL file (dialect latest if --version omitted)",
		RunE: func(cmd *cobra.Command, args []string) error {
			sqlPath, _ := cmd.Flags().GetString("file")
			if sqlPath == "" {
				return fmt.Errorf("--file is required")
			}
			raw, err := os.ReadFile(sqlPath)
			if err != nil {
				return err
			}
			if err := config.ApplyEnvFile(opts().ConfigDir); err != nil {
				return printErr(err)
			}
			p, c, _, err := config.LoadBundle(opts())
			if err != nil {
				return printErr(err)
			}
			idx, perr := validate.Bundle(p, c)
			if perr != nil {
				return printErr(perr)
			}
			reg, err := connector.OpenAll(p, connector.OpenOpts{})
			if err != nil {
				return printErr(err)
			}
			defer reg.Close()
			store := querystore.New(2 * time.Minute)
			exec := executor.New(idx, reg, store)
			if af, _, err := config.LoadAccess("", opts().ConfigDir); err != nil {
				return printErr(err)
			} else if af != nil {
				acl, perr := access.Resolve(af, idx)
				if perr != nil {
					return printErr(perr)
				}
				exec.ACL = acl
			}
			appName, _ := cmd.Flags().GetString("app")
			if appName == "" {
				appName = os.Getenv("QLLM_APP")
			}
			exec.StdioApp = appName
			ver, _ := cmd.Flags().GetString("version")
			resp := exec.ExecuteSQL(context.Background(), &protocol.SQLRequest{SQL: string(raw), Version: ver})
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp)
		},
	}
	addConfigFlags(sqlCmd)
	sqlCmd.Flags().StringP("file", "f", "", "SQL file")
	sqlCmd.Flags().String("version", "", "SQL dialect version (omit = latest)")
	sqlCmd.Flags().String("app", "", "app name when qllm.access.yaml is present")

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve HTTP and/or MCP",
		RunE: func(cmd *cobra.Command, args []string) error {
			httpMode, _ := cmd.Flags().GetBool("http")
			mcpMode, _ := cmd.Flags().GetBool("mcp")
			mcpHTTP, _ := cmd.Flags().GetBool("mcp-http")
			runtimeConfigPath, _ := cmd.Flags().GetString("runtime-config")
			if !httpMode && !mcpMode && !mcpHTTP {
				httpMode = true
			}
			if mcpMode && (httpMode || mcpHTTP) {
				return fmt.Errorf("--mcp (stdio) cannot be combined with --http or --mcp-http")
			}

			cfgOpts := opts()
			if err := config.ApplyEnvFile(cfgOpts.ConfigDir); err != nil {
				return printErr(err)
			}
			rc, _, err := config.LoadRuntimeConfig(runtimeConfigPath, cfgOpts.ConfigDir)
			if err != nil {
				return printErr(err)
			}
			flags := config.ServeFlagOverrides{}
			if cmd.Flags().Changed("addr") {
				v, _ := cmd.Flags().GetString("addr")
				flags.Addr = &v
			}
			if cmd.Flags().Changed("mcp-addr") {
				v, _ := cmd.Flags().GetString("mcp-addr")
				flags.MCPAddr = &v
			}
			if cmd.Flags().Changed("auth-token-env") {
				v, _ := cmd.Flags().GetString("auth-token-env")
				flags.AuthTokenEnv = &v
			}
			if cmd.Flags().Changed("insecure-bind") {
				v, _ := cmd.Flags().GetBool("insecure-bind")
				flags.InsecureBind = &v
			}
			if cmd.Flags().Changed("cors-origin") {
				v, _ := cmd.Flags().GetStringSlice("cors-origin")
				flags.CORSOrigins = v
				flags.CORSOriginsSet = true
			}
			settings, err := config.MergeServeSettings(rc, flags)
			if err != nil {
				return printErr(err)
			}

			p, c, _, err := config.LoadBundle(cfgOpts)
			if err != nil {
				return printErr(err)
			}
			idx, perr := validate.Bundle(p, c)
			if perr != nil {
				return printErr(perr)
			}
			reg, err := connector.OpenAll(p, connector.OpenOpts{
				MaxRestResponseBytes: settings.MaxRestResponseBytes,
			})
			if err != nil {
				return printErr(err)
			}
			defer reg.Close()
			store := querystore.New(2 * time.Minute)
			exec := executor.New(idx, reg, store)
			af, _, err := config.LoadAccess("", cfgOpts.ConfigDir)
			if err != nil {
				return printErr(err)
			}
			var acl *access.Registry
			if af != nil {
				acl, perr = access.Resolve(af, idx)
				if perr != nil {
					return printErr(perr)
				}
			}
			stdioApp, _ := cmd.Flags().GetString("app")
			if stdioApp == "" {
				stdioApp = os.Getenv("QLLM_APP")
			}
			exec.ACL = acl
			exec.StdioApp = stdioApp
			mcpSrv := mcpserver.New(idx, exec, store)

			if mcpMode {
				if acl != nil && exec.StdioApp == "" {
					return printErr(protocol.NewError(protocol.ErrConfigError,
						"qllm.access.yaml present: set --app or QLLM_APP for MCP stdio", nil))
				}
				return mcpserver.RunStdio(mcpSrv)
			}

			authToken := settings.AuthToken
			if acl != nil {
				authToken = "acl"
			}
			if httpMode {
				if err := config.CheckBindPolicy(settings.Addr, authToken, settings.InsecureBind); err != nil {
					return printErr(err)
				}
			}
			if mcpHTTP {
				if err := config.CheckBindPolicy(settings.MCPAddr, authToken, settings.InsecureBind); err != nil {
					return printErr(err)
				}
			}

			g, _ := errgroup.WithContext(context.Background())
			if httpMode {
				addr := settings.Addr
				g.Go(func() error {
					fmt.Fprintf(os.Stderr, "qllm http listening on %s\n", addr)
					srv := &httpserver.Server{
						Idx:          idx,
						Exec:         exec,
						Store:        store,
						AuthToken:    settings.AuthToken,
						ACL:          acl,
						MaxBodyBytes: settings.MaxBodyBytes,
					}
					return httpserver.ListenAndServe(addr, srv)
				})
			}
			if mcpHTTP {
				mcpAddr := settings.MCPAddr
				g.Go(func() error {
					return mcpserver.ListenAndServe(mcpAddr, mcpSrv, mcpserver.HTTPOptions{
						AuthToken:    settings.AuthToken,
						ACL:          acl,
						CORS:         settings.CORS,
						MaxBodyBytes: settings.MaxBodyBytes,
					})
				})
			}
			return g.Wait()
		},
	}
	addConfigFlags(serveCmd)
	serveCmd.Flags().Bool("http", false, "serve HTTP /v1 API")
	serveCmd.Flags().Bool("mcp", false, "serve MCP on stdio (exclusive)")
	serveCmd.Flags().Bool("mcp-http", false, "serve MCP Streamable HTTP (/mcp) + SSE (/sse)")
	serveCmd.Flags().String("addr", protocol.DefaultHTTPAddr, "HTTP /v1 listen address")
	serveCmd.Flags().String("mcp-addr", protocol.DefaultMCPAddr, "MCP HTTP listen address")
	serveCmd.Flags().String("runtime-config", "", "path to qllm.config.yaml (optional)")
	serveCmd.Flags().String("auth-token-env", "", "env var name for Bearer auth token")
	serveCmd.Flags().Bool("insecure-bind", false, "allow non-loopback bind without auth")
	serveCmd.Flags().StringSlice("cors-origin", nil, "allowed CORS origin (repeatable; empty disables CORS)")
	serveCmd.Flags().String("app", "", "app name for MCP stdio when qllm.access.yaml is present (or QLLM_APP)")

	root.AddCommand(validateCmd, queryCmd, sqlCmd, serveCmd, newCatalogCmd(opts, addConfigFlags))
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// printErr implements runtime behavior for this package.
func printErr(err error) error {
	if pe, ok := err.(*protocol.ProtocolError); ok {
		enc := json.NewEncoder(os.Stderr)
		_ = enc.Encode(protocol.ErrorResponse{ProtocolVersion: protocol.ProtocolVersion, Error: pe})
		return pe
	}
	return err
}
