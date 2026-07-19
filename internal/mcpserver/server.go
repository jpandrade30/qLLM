package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"qLLM/internal/agentguide"
	"qLLM/internal/catalogidx"
	"qLLM/internal/executor"
	"qLLM/internal/protocol"
	"qLLM/internal/querystore"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// New registers MCP tools on a transport-agnostic MCPServer.
func New(idx *catalogidx.Index, exec *executor.Executor, store *querystore.Store) *server.MCPServer {
	s := server.NewMCPServer("qllm", protocol.ProtocolVersion)

	s.AddTool(mcp.NewTool("how_to_use_me",
		mcp.WithDescription("Return the closed Query IR contract for LLMs: never-rules, where shapes, examples, and invalidExamples. Call this BEFORE execute_query."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		b, err := json.Marshal(agentguide.Build(idx.Preset, idx.Catalog))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	})

	s.AddTool(mcp.NewTool("describe_catalog",
		mcp.WithDescription("Return the logical catalog, entities, fields, relations, and source capabilities. Call after how_to_use_me."),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		b, err := json.Marshal(idx.CatalogResponse())
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

// RunStdio serves MCP over stdio (Inspector / local agents).
func RunStdio(mcpServer *server.MCPServer) error {
	fmt.Fprintln(os.Stderr, "qllm mcp server starting on stdio")
	return server.ServeStdio(mcpServer)
}

// Run is kept for callers that still pass idx/exec/store directly (stdio).
func Run(idx *catalogidx.Index, exec *executor.Executor, store *querystore.Store) error {
	return RunStdio(New(idx, exec, store))
}

// Handler returns the MCP HTTP mux (Streamable /mcp + SSE /sse,/message) with CORS.
func Handler(mcpServer *server.MCPServer) http.Handler {
	streamable := server.NewStreamableHTTPServer(mcpServer,
		server.WithEndpointPath("/mcp"),
		server.WithStateLess(true),
	)
	sseServer := server.NewSSEServer(mcpServer,
		server.WithSSEEndpoint("/sse"),
		server.WithMessageEndpoint("/message"),
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", withCORS(streamable))
	mux.Handle("/sse", withCORS(sseServer))
	mux.Handle("/message", withCORS(sseServer))
	return mux
}

// ListenAndServe serves Streamable HTTP at /mcp and SSE at /sse + /message.
func ListenAndServe(addr string, mcpServer *server.MCPServer) error {
	fmt.Fprintf(os.Stderr, "qllm mcp-http listening on %s (/mcp streamable, /sse SSE)\n", addr)
	return http.ListenAndServe(addr, Handler(mcpServer))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Accept, Mcp-Session-Id, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}
