from __future__ import annotations

import json
import math
import re
from datetime import date, datetime, timezone
from decimal import Decimal
from typing import Any

_TS_RE = re.compile(
    r"^\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?$"
)


def to_logical_type(name: str, duck_type: str) -> str:
    u = duck_type.upper()
    if "BOOL" in u:
        return "boolean"
    if any(x in u for x in ("INT", "DECIMAL", "DOUBLE", "FLOAT", "HUGE", "REAL", "NUM")):
        return "number"
    if "TIMESTAMP" in u or u in ("DATE", "TIME"):
        return "timestamp"
    if "JSON" in u:
        return "json"
    return "string"


def _parse_ts(v: Any) -> datetime | None:
    if isinstance(v, datetime):
        dt = v
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return dt.astimezone(timezone.utc)
    if isinstance(v, date) and not isinstance(v, datetime):
        return datetime(v.year, v.month, v.day, tzinfo=timezone.utc)
    if isinstance(v, str) and _TS_RE.match(v.strip()):
        s = v.strip().replace("Z", "+00:00")
        if " " in s and "T" not in s:
            s = s.replace(" ", "T", 1)
        try:
            dt = datetime.fromisoformat(s)
        except ValueError:
            return None
        if dt.tzinfo is None:
            dt = dt.replace(tzinfo=timezone.utc)
        return dt.astimezone(timezone.utc)
    return None


def serialize_cell(v: Any) -> Any:
    if v is None:
        return None
    if isinstance(v, bool):
        return v
    if isinstance(v, (int,)):
        return int(v)
    if isinstance(v, Decimal):
        f = float(v)
        if math.isfinite(f) and abs(f - round(f)) < 1e-12:
            return int(round(f))
        return f
    if isinstance(v, float):
        if math.isnan(v) or math.isinf(v):
            return None
        if abs(v - round(v)) < 1e-12:
            return int(round(v))
        return float(v)
    ts = _parse_ts(v)
    if ts is not None:
        return ts.strftime("%Y-%m-%dT%H:%M:%SZ")
    if hasattr(v, "item") and not isinstance(v, (bytes, str, list, dict)):
        try:
            return serialize_cell(v.item())
        except Exception:
            pass
    if isinstance(v, (list, dict)):
        return json.loads(json.dumps(v, default=str))
    if isinstance(v, bytes):
        return v.decode("utf-8", errors="replace")
    return v


def _as_number(v: Any) -> float | None:
    if isinstance(v, bool):
        return None
    if isinstance(v, (int, float)):
        if isinstance(v, float) and (math.isnan(v) or math.isinf(v)):
            return None
        return float(v)
    if isinstance(v, str):
        s = v.strip()
        if s == "":
            return None
        try:
            return float(s)
        except ValueError:
            return None
    return None


def _as_json_text(v: Any) -> str | None:
    if isinstance(v, (dict, list)):
        return json.dumps(v, sort_keys=True, default=str, separators=(",", ":"))
    if isinstance(v, str):
        s = v.strip()
        if s.startswith("{") or s.startswith("["):
            try:
                return json.dumps(json.loads(s), sort_keys=True, default=str, separators=(",", ":"))
            except json.JSONDecodeError:
                return None
    return None


def cells_equal(a: Any, b: Any, rel: float = 1e-6) -> bool:
    if a is None and b is None:
        return True
    ta, tb = _parse_ts(a), _parse_ts(b)
    if ta is not None and tb is not None:
        return abs((ta - tb).total_seconds()) < 1.01
    ja, jb = _as_json_text(a), _as_json_text(b)
    if ja is not None and jb is not None:
        return ja == jb
    if isinstance(a, bool) and isinstance(b, bool):
        return a is b
    na, nb = _as_number(a), _as_number(b)
    if na is not None and nb is not None:
        if na == nb:
            return True
        denom = max(abs(na), abs(nb), 1.0)
        return abs(na - nb) / denom <= rel or abs(na - nb) <= rel
    return a == b


def _row_key(row: list[Any]) -> str:
    return json.dumps(row, sort_keys=True, default=str)


def sort_rows(rows: list[list[Any]]) -> list[list[Any]]:
    return sorted(rows, key=_row_key)


def has_order_by(sql: str) -> bool:
    return re.search(r"\bORDER\s+BY\b", sql, re.IGNORECASE) is not None


def compare_result(expected: dict, got: dict, sql: str, sort_flag: bool | None = None) -> list[str]:
    errs: list[str] = []
    exp_cols = [c["name"] for c in (expected.get("columns") or [])]
    got_cols = [c["name"] for c in (got.get("columns") or [])]
    if exp_cols != got_cols:
        errs.append(f"columns: expected {exp_cols} got {got_cols}")
        return errs
    er = list(expected.get("rows") or [])
    gr = list(got.get("rows") or [])
    do_sort = sort_flag if sort_flag is not None else not has_order_by(sql)
    if do_sort:
        er, gr = sort_rows(er), sort_rows(gr)
    if expected.get("rowCount") != got.get("rowCount") and len(er) != len(gr):
        errs.append(f"rowCount: expected {expected.get('rowCount')} got {got.get('rowCount')}")
    if len(er) != len(gr):
        errs.append(f"rows len: expected {len(er)} got {len(gr)}")
        return errs
    for i, (x, y) in enumerate(zip(er, gr)):
        if len(x) != len(y):
            errs.append(f"row {i} width {len(x)} vs {len(y)}")
            continue
        for j, (a, b) in enumerate(zip(x, y)):
            if not cells_equal(serialize_cell(a), serialize_cell(b)):
                errs.append(f"row {i} col {j} ({exp_cols[j] if j < len(exp_cols) else j}): {a!r} vs {b!r}")
                if len(errs) > 12:
                    return errs
    return errs
