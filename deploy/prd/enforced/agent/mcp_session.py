"""Open a short MCP session per call (qLLM /mcp is stateless)."""

from __future__ import annotations

import json

from mcp import ClientSession
from mcp.client.streamable_http import streamablehttp_client


async def call_tool(mcp_url: str, token: str, name: str, arguments: dict | None = None) -> str:
    headers = {"Authorization": f"Bearer {token}"}
    async with streamablehttp_client(mcp_url, headers=headers) as (read, write, _):
        async with ClientSession(read, write) as session:
            await session.initialize()
            res = await session.call_tool(name, arguments or {})
            parts = []
            for block in res.content:
                text = getattr(block, "text", None)
                if text:
                    parts.append(text)
            raw = "\n".join(parts) if parts else ""
            if res.isError:
                return json.dumps({"error": raw or "tool error"})
            return raw
