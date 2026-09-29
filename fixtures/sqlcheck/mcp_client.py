from __future__ import annotations

import json
import os
from typing import Any

import httpx

MCP_URL = os.environ.get("QLLM_MCP_URL", "http://127.0.0.1:8089/mcp")
MCP_TOKEN = os.environ.get("QLLM_MCP_TOKEN", "change-me")


class MCPError(RuntimeError):
    pass


class MCPClient:
    def __init__(self, url: str = MCP_URL, token: str = MCP_TOKEN, timeout: float = 60.0) -> None:
        self.url = url
        self.token = token
        self.session_id: str | None = None
        self._n = 0
        self.http = httpx.Client(timeout=timeout)

    def close(self) -> None:
        self.http.close()

    def _headers(self) -> dict[str, str]:
        h = {
            "Content-Type": "application/json",
            "Accept": "application/json, text/event-stream",
            "Authorization": f"Bearer {self.token}",
        }
        if self.session_id:
            h["Mcp-Session-Id"] = self.session_id
        return h

    def _parse_body(self, resp: httpx.Response) -> dict[str, Any] | None:
        sid = resp.headers.get("mcp-session-id") or resp.headers.get("Mcp-Session-Id")
        if sid:
            self.session_id = sid
        text = resp.text or ""
        ctype = (resp.headers.get("content-type") or "").lower()
        if "text/event-stream" in ctype:
            data_lines: list[str] = []
            for line in text.splitlines():
                if line.startswith("data:"):
                    data_lines.append(line[5:].strip())
            if not data_lines:
                return None
            return json.loads(data_lines[-1])
        if not text.strip():
            return None
        return json.loads(text)

    def rpc(self, method: str, params: dict[str, Any] | None = None, notification: bool = False) -> dict[str, Any] | None:
        self._n += 1
        body: dict[str, Any] = {"jsonrpc": "2.0", "method": method}
        if not notification:
            body["id"] = self._n
        if params is not None:
            body["params"] = params
        resp = self.http.post(self.url, headers=self._headers(), json=body)
        if resp.status_code >= 400:
            raise MCPError(f"HTTP {resp.status_code} {method}: {resp.text[:500]}")
        parsed = self._parse_body(resp)
        if notification:
            return parsed
        if parsed is None:
            raise MCPError(f"empty RPC response for {method}")
        if "error" in parsed:
            raise MCPError(str(parsed["error"]))
        return parsed

    def initialize(self) -> None:
        self.rpc(
            "initialize",
            {
                "protocolVersion": "2024-11-05",
                "capabilities": {},
                "clientInfo": {"name": "qllm-sqlcheck", "version": "0.1"},
            },
        )
        self.rpc("notifications/initialized", {}, notification=True)

    def tools_list(self) -> list[str]:
        res = self.rpc("tools/list", {})
        tools = ((res or {}).get("result") or {}).get("tools") or []
        return [t.get("name") for t in tools]

    def call_execute_sql(self, sql: str, version: str | None = None) -> dict[str, Any]:
        args: dict[str, Any] = {"sql": sql}
        if version:
            args["version"] = str(version)
        res = self.rpc("tools/call", {"name": "execute_sql", "arguments": args})
        result = (res or {}).get("result") or {}
        is_error = bool(result.get("isError"))
        content = result.get("content") or []
        text = ""
        if content and isinstance(content[0], dict):
            text = content[0].get("text") or ""
        if not text:
            raise MCPError(f"empty execute_sql content: {result!r}")
        try:
            payload = json.loads(text)
        except json.JSONDecodeError as e:
            raise MCPError(f"execute_sql not JSON: {text[:400]}") from e
        if is_error and payload.get("status") != "failed":
            payload.setdefault("status", "failed")
        return payload
