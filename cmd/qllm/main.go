package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"qLLM/internal/config"
	"qLLM/internal/connector"
	"qLLM/internal/executor"
	"qLLM/internal/httpserver"
	"qLLM/internal/mcpserver"
	"qLLM/internal/protocol"
	"qLLM/internal/querystore"
	"qLLM/internal/validate"

	"github.com/spf13/cobra"
)

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
			p, c, _, err := config.LoadBundle(opts())
			if err != nil {
				return printErr(err)
			}
			idx, perr := validate.Bundle(p, c)
			if perr != nil {
				return printErr(perr)
			}
			reg, err := connector.OpenAll(p.Sources)
			if err != nil {
				return printErr(err)
			}
			defer reg.Close()
			store := querystore.New(2 * time.Minute)
			exec := executor.New(idx, reg, store)
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

	serveCmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve HTTP and/or MCP",
		RunE: func(cmd *cobra.Command, args []string) error {
			httpMode, _ := cmd.Flags().GetBool("http")
			mcpMode, _ := cmd.Flags().GetBool("mcp")
			addr, _ := cmd.Flags().GetString("addr")
			if !httpMode && !mcpMode {
				httpMode = true
			}
			p, c, _, err := config.LoadBundle(opts())
			if err != nil {
				return printErr(err)
			}
			idx, perr := validate.Bundle(p, c)
			if perr != nil {
				return printErr(perr)
			}
			reg, err := connector.OpenAll(p.Sources)
			if err != nil {
				return printErr(err)
			}
			defer reg.Close()
			store := querystore.New(2 * time.Minute)
			exec := executor.New(idx, reg, store)
			if mcpMode {
				return mcpserver.Run(idx, exec, store)
			}
			if httpMode {
				fmt.Fprintf(os.Stderr, "qllm http listening on %s\n", addr)
				srv := &httpserver.Server{Idx: idx, Exec: exec, Store: store}
				return httpserver.ListenAndServe(addr, srv)
			}
			return nil
		},
	}
	addConfigFlags(serveCmd)
	serveCmd.Flags().Bool("http", false, "serve HTTP /v1 API")
	serveCmd.Flags().Bool("mcp", false, "serve MCP on stdio")
	serveCmd.Flags().String("addr", ":8088", "HTTP listen address")

	root.AddCommand(validateCmd, queryCmd, serveCmd)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func printErr(err error) error {
	if pe, ok := err.(*protocol.ProtocolError); ok {
		enc := json.NewEncoder(os.Stderr)
		_ = enc.Encode(protocol.ErrorResponse{ProtocolVersion: protocol.ProtocolVersion, Error: pe})
		return pe
	}
	return err
}
