"""LangGraph compiled once. The user token lives in RunnableConfig, not in state."""

from __future__ import annotations

import json
import os
from typing import Any, TypedDict

from langchain_core.runnables import RunnableConfig
from langgraph.graph import END, StateGraph

from mcp_session import call_tool

MCP_URL = os.environ.get("QLLM_MCP_URL", "http://127.0.0.1:18089/mcp")


class AgentState(TypedDict, total=False):
    question: str
    catalog: str
    guide: str
    sql: str
    raw: str
    answer: str


def _token(config: RunnableConfig) -> str:
    conf = (config or {}).get("configurable") or {}
    tok = conf.get("qllm_token")
    if not tok:
        raise ValueError("qllm_token missing from config.configurable")
    return str(tok)


async def load_context(state: AgentState, config: RunnableConfig) -> dict[str, Any]:
    token = _token(config)
    guide = await call_tool(MCP_URL, token, "how_to_use_me")
    catalog = await call_tool(MCP_URL, token, "describe_catalog")
    return {"guide": guide, "catalog": catalog}


def plan_query(state: AgentState, config: RunnableConfig) -> dict[str, Any]:
    q = (state.get("question") or "").lower()
    if "spoof" in q:
        sql = "SELECT id, user_id, item, total_cents FROM orders WHERE user_id = '7' LIMIT 50"
    elif "product" in q:
        sql = "SELECT id, name, price_cents FROM products LIMIT 50"
    else:
        sql = "SELECT id, user_id, item, total_cents FROM orders LIMIT 50"
    return {"sql": sql}


async def run_sql(state: AgentState, config: RunnableConfig) -> dict[str, Any]:
    """Wrapper around MCP execute_sql. Session opened per call with the user's key."""
    token = _token(config)
    raw = await call_tool(MCP_URL, token, "execute_sql", {"sql": state.get("sql") or "SELECT 1 LIMIT 1"})
    return {"raw": raw}


def format_answer(state: AgentState, config: RunnableConfig) -> dict[str, Any]:
    raw = state.get("raw") or ""
    try:
        payload = json.loads(raw)
    except json.JSONDecodeError:
        return {"answer": raw}
    if isinstance(payload, dict) and payload.get("error"):
        err = payload["error"]
        if isinstance(err, dict):
            return {"answer": f"{err.get('code')}: {err.get('message')}"}
        return {"answer": str(err)}
    if isinstance(payload, dict) and payload.get("status") == "failed" and payload.get("error"):
        err = payload["error"]
        return {"answer": f"{err.get('code')}: {err.get('message')}"}
    result = payload.get("result") if isinstance(payload, dict) else None
    if not result:
        return {"answer": raw}
    cols = [c.get("name") for c in result.get("columns") or []]
    rows = result.get("rows") or []
    lines = [", ".join(cols)] + [", ".join(str(v) for v in row) for row in rows]
    if not rows:
        lines.append("(no rows — if you filtered another user, the fetch was already scoped)")
    return {"answer": "\n".join(lines)}


def compile_graph():
    g = StateGraph(AgentState)
    g.add_node("load_context", load_context)
    g.add_node("plan_query", plan_query)
    g.add_node("run_sql", run_sql)
    g.add_node("format_answer", format_answer)
    g.set_entry_point("load_context")
    g.add_edge("load_context", "plan_query")
    g.add_edge("plan_query", "run_sql")
    g.add_edge("run_sql", "format_answer")
    g.add_edge("format_answer", END)
    return g.compile()
