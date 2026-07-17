from __future__ import annotations

import json
import time
from pathlib import Path

from fastapi import FastAPI, Query
from fastapi.responses import JSONResponse

app = FastAPI()

DATA_PATH = Path(__file__).with_name("data.json")


def load_data() -> dict:
    if DATA_PATH.exists():
        return json.loads(DATA_PATH.read_text(encoding="utf-8"))
    return {"users": [], "products": [], "tickets": []}


DATA = load_data()


@app.get("/health")
def health():
    return {
        "ok": True,
        "counts": {k: len(v) for k, v in DATA.items() if isinstance(v, list)},
    }


def _paginate(rows: list, limit: int, offset: int):
    return rows[offset : offset + limit]


@app.get("/users")
def list_users(
    email: str | None = None,
    country: str | None = None,
    limit: int = Query(100, ge=1, le=1000),
    offset: int = 0,
):
    rows = DATA.get("users", [])
    if email:
        rows = [u for u in rows if u.get("email") == email]
    if country:
        rows = [u for u in rows if u.get("country") == country]
    return _paginate(rows, limit, offset)


@app.get("/users/{user_id}")
def get_user(user_id: str):
    for u in DATA.get("users", []):
        if u.get("id") == user_id:
            return u
    return JSONResponse({"error": "not found"}, status_code=404)


@app.get("/products")
def list_products(
    category: str | None = None,
    limit: int = Query(100, ge=1, le=1000),
    offset: int = 0,
):
    rows = DATA.get("products", [])
    if category:
        rows = [p for p in rows if p.get("category") == category]
    return _paginate(rows, limit, offset)


@app.get("/tickets")
def list_tickets(
    status: str | None = None,
    customer_id: str | None = None,
    limit: int = Query(100, ge=1, le=1000),
    offset: int = 0,
):
    rows = DATA.get("tickets", [])
    if status:
        rows = [t for t in rows if t.get("status") == status]
    if customer_id:
        rows = [t for t in rows if t.get("customer_id") == customer_id]
    return _paginate(rows, limit, offset)


@app.get("/slow")
def slow(ms: int = Query(20000, ge=1)):
    time.sleep(ms / 1000.0)
    return {"slept_ms": ms}


if __name__ == "__main__":
    import uvicorn

    uvicorn.run(app, host="0.0.0.0", port=8080)
