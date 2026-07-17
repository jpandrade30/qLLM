package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"qLLM/internal/agentguide"
	"qLLM/internal/catalogidx"
	"qLLM/internal/executor"
	"qLLM/internal/protocol"
	"qLLM/internal/querystore"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func Run(idx *catalogidx.Index, exec *executor.Executor, store *querystore.Store) error {
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
		// Prefer structured arguments matching IR fields
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

	fmt.Fprintln(os.Stderr, "qllm mcp server starting on stdio")
	return server.ServeStdio(s)
}
