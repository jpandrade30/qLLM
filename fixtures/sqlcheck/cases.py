from __future__ import annotations

import yaml

from sqlcheck.paths import CASES_YAML

REQUIRED_COVERS = [
    "postgres",
    "mysql",
    "mongodb",
    "rest",
    "filter",
    "having",
    "distinct",
    "case",
    "like",
    "between",
    "cte",
    "subquery",
    "arithmetic",
    "join",
    "agg",
    "coalesce",
    "string",
    "numeric",
    "cast",
    "json",
    "array",
    "datetime",
    "extra-agg",
    "set-ops",
    "qualify",
    "window",
    "xor",
    "count-distinct",
    "reject",
]


def load_suite() -> dict:
    raw = yaml.safe_load(CASES_YAML.read_text(encoding="utf-8"))
    cases = raw.get("cases") or []
    required = list(raw.get("required_covers") or REQUIRED_COVERS)
    return {"cases": cases, "required_covers": required}


def missing_covers(cases: list[dict], required: list[str] | None = None) -> list[str]:
    need = required or REQUIRED_COVERS
    seen: set[str] = set()
    for c in cases:
        for t in c.get("covers") or []:
            seen.add(str(t))
    return [t for t in need if t not in seen]
