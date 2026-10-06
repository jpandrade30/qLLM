package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

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
	_ = store
	s := server.NewMCPServer("qllm", protocol.ProtocolVersion)
	d := DescriptionsFor(idx)

	s.AddTool(mcp.NewTool("how_to_use_me",
		mcp.WithDescription(d.HowToUseMe),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		fmt.Fprintln(os.Stderr, "---- mcp_tool ----\n  name  how_to_use_me\n------------------")
		b, err := json.Marshal(agentguide.Build(idx.Preset, idx.CatalogFor(allowFrom(ctx, exec))))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	})

	s.AddTool(mcp.NewTool("describe_catalog",
		mcp.WithDescription(d.DescribeCatalog),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		fmt.Fprintln(os.Stderr, "---- mcp_tool ----\n  name  describe_catalog\n------------------")
		b, err := json.Marshal(idx.CatalogResponseFor(allowFrom(ctx, exec)))
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	})

	s.AddTool(mcp.NewTool("execute_sql",
		mcp.WithDescription(d.ExecuteSQL),
		mcp.WithString("sql", mcp.Required(), mcp.Description("SELECT using catalog entity names; output AS aliases OK")),
		mcp.WithString("version", mcp.Description("SQL dialect: \"1\" (frozen) or \"2\" (latest). Omit for latest.")),
		mcp.WithObject("constraints", mcp.Description("Optional catalog field→scalar map (D23). Prefer host-bound values, not model-invented subjects.")),
		mcp.WithString("constraintMode", mcp.Description("validate (default if constraints set) or inject. inject also forces eq on source fetch.")),
	), func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		sqlStr, err := req.RequireString("sql")
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		version := ""
		if v, err := req.RequireString("version"); err == nil {
			version = v
		}
		sqlReq := &protocol.SQLRequest{SQL: sqlStr, Version: version}
		if args := req.GetArguments(); args != nil {
			if mode, ok := args["constraintMode"].(string); ok {
				sqlReq.ConstraintMode = mode
			}
			if raw, ok := args["constraints"]; ok && raw != nil {
				m, cerr := constraintsFromMCP(raw)
				if cerr != nil {
					b, _ := json.Marshal(protocol.QueryResponse{
						ProtocolVersion: protocol.ProtocolVersion,
						Status:          protocol.StatusFailed,
						Error:           cerr,
					})
					return mcp.NewToolResultError(string(b)), nil
				}
				sqlReq.Constraints = m
			}
		}
		resp := exec.ExecuteSQL(ctx, sqlReq)
		b, err := json.Marshal(resp)
		if err != nil {
			return mcp.NewToolResultError(err.Error()), nil
		}
		if resp.Status == protocol.StatusFailed {
			return mcp.NewToolResultError(string(b)), nil
		}
		return mcp.NewToolResultText(string(b)), nil
	})

	return s
}

func constraintsFromMCP(raw any) (map[string]any, *protocol.ProtocolError) {
	switch t := raw.(type) {
	case map[string]any:
		return t, nil
	case string:
		if strings.TrimSpace(t) == "" {
			return nil, nil
		}
		var m map[string]any
		if err := json.Unmarshal([]byte(t), &m); err != nil {
			return nil, protocol.NewError(protocol.ErrInvalidSQL, "constraints must be a JSON object", map[string]any{"error": err.Error()})
		}
		return m, nil
	default:
		return nil, protocol.NewError(protocol.ErrInvalidSQL, "constraints must be an object", map[string]any{"type": fmt.Sprintf("%T", raw)})
	}
}

// allowFrom implements runtime behavior for this package.
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
