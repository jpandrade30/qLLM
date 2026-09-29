package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"qLLM/internal/access"
	"qLLM/internal/agentguide"
	"qLLM/internal/appctx"
	"qLLM/internal/catalogidx"
	"qLLM/internal/executor"
	"qLLM/internal/protocol"
	"qLLM/internal/querystore"
	"qLLM/internal/serveauth"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// New registers MCP tools on a transport-agnostic MCPServer.
func New(idx *catalogidx.Index, exec *executor.Executor, store *querystore.Store) *server.MCPServer {
	s := server.NewMCPServer("qllm", protocol.ProtocolVersion)

	s.AddTool(mcp.NewTool("how_to_use_me",
		mcp.WithDescription("Return the Query IR contract and optional SQL dialect for LLMs. Call this BEFORE execute_query or execute_sql."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		b, err := json.Marshal(agentguide.Build(idx.Preset, idx.CatalogFor(allowFrom(ctx, exec))))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	})

	s.AddTool(mcp.NewTool("describe_catalog",
		mcp.WithDescription("Return the logical catalog, entities, fields, relations, and source capabilities. Call after how_to_use_me."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		b, err := json.Marshal(idx.CatalogResponseFor(allowFrom(ctx, exec)))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	})

	s.AddTool(mcp.NewTool("execute_query",
		mcp.WithDescription("Execute a qLLM Query IR JSON document. Call how_to_use_me first. Query IR is NOT SQL — use {op,args} for AND/OR, CompareOp tokens like eq/gte."),
		mcp.WithString("ir", mcp.Required(), mcp.Description("Query IR as JSON string or object fields at top-level")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		var q protocol.QueryIR
		raw, _ := json.Marshal(req.Params.Arguments)
		if err := json.Unmarshal(raw, &q); err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if q.From == "" {
			if ir, err := req.RequireString("ir"); err == nil {
				if err := json.Unmarshal([]byte(ir), &q); err != nil {
					return mcp.NewToolResultError(err.Error()), nil
				}
			}
		}
		resp := exec.Execute(ctx, &q)
		b, err := json.Marshal(resp)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if resp.Status == protocol.StatusFailed {
			return mcp.NewToolResultError(string(b)), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	})

	s.AddTool(mcp.NewTool("execute_sql",
		mcp.WithDescription("Execute catalog SQL (SELECT). Tables are catalog entity names. Call how_to_use_me and describe_catalog first. version omitted = latest dialect."),
		mcp.WithString("sql", mcp.Required(), mcp.Description("SELECT statement using catalog tables")),
		mcp.WithString("version", mcp.Description("SQL dialect version; omit for latest")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sqlStr, err := req.RequireString("sql")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		version := ""
		if v, err := req.RequireString("version"); err == nil {
			version = v
		}
		resp := exec.ExecuteSQL(ctx, &protocol.SQLRequest{SQL: sqlStr, Version: version})
		b, err := json.Marshal(resp)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if resp.Status == protocol.StatusFailed {
			return mcp.NewToolResultError(string(b)), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	})

	s.AddTool(mcp.NewTool("get_query",
		mcp.WithDescription("Get status/result for an async queryId"),
		mcp.WithString("queryId", mcp.Required(), mcp.Description("Query id returned by execute_query")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		id, err := req.RequireString("queryId")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		resp, perr := store.Get(id)
		if perr != nil {
			return mcp.NewToolResultError(perr.Error()), nil
		}
		b, err := json.Marshal(resp)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	})

	return s
}

func allowFrom(ctx context.Context, exec *executor.Executor) map[string]struct{} {
	if exec == nil || exec.ACL == nil {
		return nil
	}
	if app := appctx.App(ctx); app != nil {
		return app.TableSet()
	}
	if exec.StdioApp != "" {
		if app := exec.ACL.LookupName(exec.StdioApp); app != nil {
			return app.TableSet()
		}
	}
	return map[string]struct{}{}
}

// RunStdio serves MCP over stdio (Inspector / local agents).
func RunStdio(mcpServer *server.MCPServer) error {
	fmt.Fprintln(os.Stderr, "qllm mcp server starting on stdio")
	return server.ServeStdio(mcpServer)
}

// Run is kept for callers that still pass idx/exec/store directly (stdio).
func Run(idx *catalogidx.Index, exec *executor.Executor, store *querystore.Store) error {
	return RunStdio(New(idx, exec, store))
}

// HTTPOptions configures MCP HTTP auth, CORS, and body limits.
type HTTPOptions struct {
	AuthToken    string
	ACL          *access.Registry
	CORS         protocol.CORSConfig
	MaxBodyBytes int64
}

// Handler returns the MCP HTTP mux (Streamable /mcp + SSE /sse,/message).
func Handler(mcpServer *server.MCPServer, opts HTTPOptions) http.Handler {
	streamable := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)
	sseServer := server.NewSSEServer(mcpServer,
		server.WithSSEEndpoint("/sse"),
		server.WithMessageEndpoint("/message"),
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", streamable)
	mux.Handle("/sse", sseServer)
	mux.Handle("/message", sseServer)

	var h http.Handler = mux
	h = serveauth.MaxBytes(opts.MaxBodyBytes, h)
	if opts.ACL != nil {
		h = serveauth.AppsMiddleware(opts.ACL, h)
	} else {
		h = serveauth.Middleware(opts.AuthToken, h)
	}
	h = serveauth.CORS(opts.CORS, h)
	return h
}

// ListenAndServe serves Streamable HTTP at /mcp and SSE at /sse + /message.
func ListenAndServe(addr string, mcpServer *server.MCPServer, opts HTTPOptions) error {
	fmt.Fprintf(os.Stderr, "qllm mcp-http listening on %s (/mcp streamable, /sse SSE)\n", addr)
	return http.ListenAndServe(addr, Handler(mcpServer, opts))
}
