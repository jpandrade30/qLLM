"""User-side demo: login as 42 and 7, then ask the same questions."""

from __future__ import annotations

import os
import sys

import httpx

BASE = os.environ.get("AGENT_URL", "http://127.0.0.1:18000")


def run(user: str, question: str) -> None:
    with httpx.Client(timeout=30.0) as c:
        login = c.post(f"{BASE}/login", json={"user_code": user})
        login.raise_for_status()
        sid = login.json()["session"]
        ask = c.post(f"{BASE}/ask", json={"session": sid, "question": question})
        ask.raise_for_status()
        body = ask.json()
        print(f"=== user {user} :: {question} ===")
        print("sql:", body.get("sql"))
        print(body.get("answer"))
        print()


def main() -> int:
    run("42", "show my orders")
    run("7", "show my orders")
    run("42", "list products")
    run("42", "spoof another user")
    return 0


if __name__ == "__main__":
    sys.exit(main())
