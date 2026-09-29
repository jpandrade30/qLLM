from __future__ import annotations

import json
from typing import Any


def sql_oneline(sql: str) -> str:
    return " ".join((sql or "").split())


def preview_result(result: dict[str, Any] | None, max_rows: int = 8) -> str:
    if not result:
        return "  (no result)"
    cols = [c.get("name") for c in (result.get("columns") or [])]
    rows = list(result.get("rows") or [])
    n = result.get("rowCount", len(rows))
    lines = [f"  columns: {cols}", f"  rowCount: {n}"]
    show = rows[:max_rows]
    for i, row in enumerate(show):
        lines.append(f"  [{i}] {json.dumps(row, ensure_ascii=False, default=str)}")
    if len(rows) > max_rows:
        lines.append(f"  ... {len(rows) - max_rows} more rows")
    return "\n".join(lines)


def print_case_banner(case: dict[str, Any]) -> None:
    cid = case.get("id")
    expect = case.get("expect")
    ver = case.get("version") or "2"
    covers = ",".join(case.get("covers") or [])
    print()
    print("=" * 72)
    print(f"{cid}  expect={expect}  dialect={ver}  covers=[{covers}]")
    print("-" * 72)
    print(sql_oneline(case.get("sql") or ""))
