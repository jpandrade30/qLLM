from __future__ import annotations

import json
from pathlib import Path
from typing import Any

import duckdb
import pandas as pd
import yaml

from sqlcheck.compare import serialize_cell, to_logical_type
from sqlcheck.paths import CATALOG_YAML, DATASET_DIR, EXPECTED_DIR
from sqlcheck.cases import load_suite


def catalog_fields() -> dict[str, list[tuple[str, str]]]:
    raw = yaml.safe_load(CATALOG_YAML.read_text(encoding="utf-8"))
    out: dict[str, list[tuple[str, str]]] = {}
    for ent in raw.get("entities") or []:
        name = ent["name"]
        fields = [(f["name"], f.get("type") or "string") for f in ent.get("fields") or []]
        out[name] = fields
    return out


def _frame_for(entity: str, fields: list[tuple[str, str]], dataset_dir: Path) -> pd.DataFrame:
    path = dataset_dir / f"{entity}.json"
    rows = json.loads(path.read_text(encoding="utf-8"))
    slim = [{name: r.get(name) for name, _ in fields} for r in rows]
    df = pd.DataFrame(slim)
    for name, typ in fields:
        if name not in df.columns:
            continue
        if typ == "boolean":
            df[name] = df[name].map(lambda x: None if x is None else bool(x))
    return df


def _select_sql(entity: str, fields: list[tuple[str, str]], view: str) -> str:
    parts = []
    for name, typ in fields:
        ident = f'"{name}"'
        if typ == "timestamp":
            parts.append(f'CAST({ident} AS TIMESTAMP) AS {ident}')
        elif typ == "boolean":
            parts.append(f'CAST({ident} AS BOOLEAN) AS {ident}')
        else:
            parts.append(ident)
    return f'CREATE OR REPLACE VIEW "{entity}" AS SELECT {", ".join(parts)} FROM {view}'


def open_oracle(dataset_dir: Path | None = None) -> duckdb.DuckDBPyConnection:
    dataset_dir = dataset_dir or DATASET_DIR
    con = duckdb.connect(":memory:")
    for entity, fields in catalog_fields().items():
        df = _frame_for(entity, fields, dataset_dir)
        view = f"_src_{entity}"
        con.register(view, df)
        con.execute(_select_sql(entity, fields, view))
    return con


def run_sql(con: duckdb.DuckDBPyConnection, sql: str) -> dict[str, Any]:
    rel = con.sql(sql.strip().rstrip(";"))
    names = list(rel.columns)
    types = [str(t) for t in rel.types]
    raw_rows = rel.fetchall()
    columns = [{"name": n, "type": to_logical_type(n, t)} for n, t in zip(names, types)]
    rows = [[serialize_cell(v) for v in row] for row in raw_rows]
    return {"columns": columns, "rows": rows, "rowCount": len(rows)}


def write_goldens(con: duckdb.DuckDBPyConnection | None = None) -> None:
    suite = load_suite()
    con = con or open_oracle()
    EXPECTED_DIR.mkdir(parents=True, exist_ok=True)
    for case in suite["cases"]:
        cid = case["id"]
        dest = EXPECTED_DIR / f"{cid}.json"
        if case.get("expect") == "reject":
            payload = {"expect": "reject", "errorCode": case.get("error_code") or "INVALID_SQL"}
        else:
            payload = run_sql(con, case["sql"])
        dest.write_text(json.dumps(payload, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
        print(f"wrote {dest}")


def load_expected(case_id: str) -> dict[str, Any]:
    path = EXPECTED_DIR / f"{case_id}.json"
    return json.loads(path.read_text(encoding="utf-8"))
