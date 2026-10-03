"""Customer backend: login mints a scoped key; /ask runs the compiled graph."""

from __future__ import annotations

import os
import uuid
from typing import Any

from fastapi import FastAPI, HTTPException
from pydantic import BaseModel

from graph import compile_graph
from mint import mint_key

SECRET = os.environ.get("QLLM_MOBILE_SECRET", "change-me-mobile")
sessions: dict[str, dict[str, str]] = {}
graph = compile_graph()
app = FastAPI(title="qLLM enforced-scope demo")


class LoginIn(BaseModel):
    user_code: str


class AskIn(BaseModel):
    session: str
    question: str


@app.post("/login")
def login(body: LoginIn) -> dict[str, str]:
    if not body.user_code:
        raise HTTPException(400, "user_code required")
    token = mint_key("mobile", body.user_code, SECRET)
    sid = str(uuid.uuid4())
    sessions[sid] = {"user_code": body.user_code, "token": token}
    return {"session": sid, "user_code": body.user_code}


@app.post("/ask")
async def ask(body: AskIn) -> dict[str, Any]:
    sess = sessions.get(body.session)
    if not sess:
        raise HTTPException(401, "unknown session")
    out = await graph.ainvoke(
        {"question": body.question},
        config={"configurable": {"qllm_token": sess["token"], "user_code": sess["user_code"]}},
    )
    return {
        "user_code": sess["user_code"],
        "sql": out.get("sql"),
        "answer": out.get("answer"),
    }


@app.get("/health")
def health() -> dict[str, str]:
    return {"ok": "true"}
