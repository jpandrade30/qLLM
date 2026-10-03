"""Load fixtures/datasets/v1 into Postgres, MySQL, MongoDB, and test-api JSON. Never calls Faker."""

from __future__ import annotations

import argparse
import hashlib
import os
from datetime import datetime, timezone
from pathlib import Path
from typing import Any

from jsonutil import load_json
from paths import API_FIXTURE, DATASET_DIR, LOGICAL_TABLES


def env(name: str, default: str) -> str:
    return os.environ.get(name, default)


def parse_dt(v: Any) -> datetime:
    if isinstance(v, datetime):
        return v
    s = str(v).replace("Z", "+00:00")
    dt = datetime.fromisoformat(s)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=timezone.utc)
    return dt


def verify_manifest(dataset_dir: Path) -> dict:
    manifest = load_json(dataset_dir / "manifest.json")
    files = manifest.get("files") or {}
    for name in LOGICAL_TABLES:
        path = dataset_dir / f"{name}.json"
        if not path.is_file():
            raise SystemExit(f"missing dataset file {path}")
        got = hashlib.sha256(path.read_bytes()).hexdigest()
        want = files.get(path.name)
        if want and got != want:
            raise SystemExit(f"hash mismatch {path.name}: expected {want} got {got} (regenerate or restore v1)")
    return manifest


def load_tables(dataset_dir: Path) -> dict[str, list[dict]]:
    return {name: load_json(dataset_dir / f"{name}.json") for name in LOGICAL_TABLES}


def write_api(tables: dict[str, list[dict]]) -> None:
    payload = {
        "users": tables["legacy_users"],
        "products": [
            {
                "id": p["id"],
                "sku": p["sku"],
                "name": p["name"],
                "price_cents": p["price"],
                "category": p["category"],
            }
            for p in tables["api_products"]
        ],
        "tickets": tables["api_tickets"],
        "profiles": tables["api_profiles"],
    }
    API_FIXTURE.parent.mkdir(parents=True, exist_ok=True)
    API_FIXTURE.write_text(__import__("json").dumps(payload), encoding="utf-8")
    print(f"wrote {API_FIXTURE} ({API_FIXTURE.stat().st_size // 1024} KB)")


def load_postgres(tables: dict[str, list[dict]]) -> None:
    import psycopg2
    from psycopg2.extras import execute_batch

    customers = tables["customers"]
    addresses = tables["addresses"]
    tickets = tables["support_tickets"]
    subscriptions = tables["subscriptions"]

    conn = psycopg2.connect(
        host=env("QLLM_CRM_PG_HOST", "127.0.0.1"),
        port=int(env("QLLM_CRM_PG_PORT", "5432")),
        dbname="crm",
        user=env("QLLM_CRM_PG_USER", "qllm"),
        password=env("QLLM_CRM_PG_PASSWORD", "qllm"),
    )
    conn.autocommit = True
    cur = conn.cursor()
    cur.execute(
        """
        DROP TABLE IF EXISTS support_tickets CASCADE;
        DROP TABLE IF EXISTS subscriptions CASCADE;
        DROP TABLE IF EXISTS addresses CASCADE;
        DROP TABLE IF EXISTS customers CASCADE;
        CREATE TABLE customers (
          id TEXT PRIMARY KEY,
          email TEXT NOT NULL UNIQUE,
          full_name TEXT NOT NULL,
          phone TEXT,
          country TEXT,
          created_at TIMESTAMPTZ NOT NULL
        );
        CREATE TABLE addresses (
          id TEXT PRIMARY KEY,
          customer_id TEXT NOT NULL REFERENCES customers(id),
          city TEXT,
          state TEXT,
          country TEXT,
          postal_code TEXT
        );
        CREATE TABLE support_tickets (
          id TEXT PRIMARY KEY,
          customer_id TEXT NOT NULL REFERENCES customers(id),
          subject TEXT,
          status TEXT,
          priority TEXT,
          created_at TIMESTAMPTZ NOT NULL
        );
        CREATE TABLE subscriptions (
          id TEXT PRIMARY KEY,
          customer_id TEXT NOT NULL REFERENCES customers(id),
          plan TEXT,
          status TEXT,
          mrr_cents INT,
          started_at TIMESTAMPTZ NOT NULL
        );
        """
    )
    execute_batch(
        cur,
        "INSERT INTO customers (id,email,full_name,phone,country,created_at) VALUES (%s,%s,%s,%s,%s,%s)",
        [(c["id"], c["email"], c["full_name"], c["phone"], c["country"], parse_dt(c["created_at"])) for c in customers],
        page_size=200,
    )
    execute_batch(
        cur,
        "INSERT INTO addresses (id,customer_id,city,state,country,postal_code) VALUES (%s,%s,%s,%s,%s,%s)",
        [(a["id"], a["customer_id"], a["city"], a["state"], a["country"], a["postal_code"]) for a in addresses],
        page_size=200,
    )
    execute_batch(
        cur,
        "INSERT INTO support_tickets (id,customer_id,subject,status,priority,created_at) VALUES (%s,%s,%s,%s,%s,%s)",
        [(t["id"], t["customer_id"], t["subject"], t["status"], t["priority"], parse_dt(t["created_at"])) for t in tickets],
        page_size=200,
    )
    execute_batch(
        cur,
        "INSERT INTO subscriptions (id,customer_id,plan,status,mrr_cents,started_at) VALUES (%s,%s,%s,%s,%s,%s)",
        [(s["id"], s["customer_id"], s["plan"], s["status"], s["mrr"], parse_dt(s["started_at"])) for s in subscriptions],
        page_size=200,
    )
    cur.close()
    conn.close()
    print(f"postgres: customers={len(customers)} addresses={len(addresses)} tickets={len(tickets)} subs={len(subscriptions)}")


def load_mysql(tables: dict[str, list[dict]]) -> None:
    import pymysql

    products = tables["products"]
    invoices = tables["invoices"]
    invoice_items = tables["invoice_items"]
    payments = tables["payments"]

    conn = pymysql.connect(
        host=env("QLLM_BILLING_MYSQL_HOST", "127.0.0.1"),
        port=int(env("QLLM_BILLING_MYSQL_PORT", "3306")),
        user=env("QLLM_BILLING_MYSQL_USER", "qllm"),
        password=env("QLLM_BILLING_MYSQL_PASSWORD", "qllm"),
        database="billing",
        autocommit=True,
    )
    cur = conn.cursor()
    cur.execute("SET FOREIGN_KEY_CHECKS=0")
    for t in ("payments", "invoice_items", "invoices", "products"):
        cur.execute(f"DROP TABLE IF EXISTS {t}")
    cur.execute(
        """
        CREATE TABLE products (
          id VARCHAR(64) PRIMARY KEY,
          sku VARCHAR(64) NOT NULL,
          name VARCHAR(120) NOT NULL,
          price_cents INT NOT NULL,
          category VARCHAR(64) NOT NULL
        )
        """
    )
    cur.execute(
        """
        CREATE TABLE invoices (
          id VARCHAR(64) PRIMARY KEY,
          customer_id VARCHAR(64) NOT NULL,
          total_cents INT NOT NULL,
          status VARCHAR(32) NOT NULL,
          issued_at DATETIME NOT NULL,
          KEY idx_invoices_customer (customer_id),
          KEY idx_invoices_status (status)
        )
        """
    )
    cur.execute(
        """
        CREATE TABLE invoice_items (
          id VARCHAR(64) PRIMARY KEY,
          invoice_id VARCHAR(64) NOT NULL,
          product_id VARCHAR(64) NOT NULL,
          qty INT NOT NULL,
          unit_cents INT NOT NULL,
          KEY idx_items_invoice (invoice_id)
        )
        """
    )
    cur.execute(
        """
        CREATE TABLE payments (
          id VARCHAR(64) PRIMARY KEY,
          invoice_id VARCHAR(64) NOT NULL,
          customer_id VARCHAR(64) NOT NULL,
          amount_cents INT NOT NULL,
          method VARCHAR(32) NOT NULL,
          paid_at DATETIME NOT NULL,
          KEY idx_payments_customer (customer_id)
        )
        """
    )
    cur.executemany(
        "INSERT INTO products (id,sku,name,price_cents,category) VALUES (%s,%s,%s,%s,%s)",
        [(p["id"], p["sku"], p["name"], p["price"], p["category"]) for p in products],
    )
    cur.executemany(
        "INSERT INTO invoices (id,customer_id,total_cents,status,issued_at) VALUES (%s,%s,%s,%s,%s)",
        [(i["id"], i["customer_id"], i["total"], i["status"], parse_dt(i["issued_at"]).replace(tzinfo=None)) for i in invoices],
    )
    cur.executemany(
        "INSERT INTO invoice_items (id,invoice_id,product_id,qty,unit_cents) VALUES (%s,%s,%s,%s,%s)",
        [(x["id"], x["invoice_id"], x["product_id"], x["qty"], x["unit"]) for x in invoice_items],
    )
    cur.executemany(
        "INSERT INTO payments (id,invoice_id,customer_id,amount_cents,method,paid_at) VALUES (%s,%s,%s,%s,%s,%s)",
        [(p["id"], p["invoice_id"], p["customer_id"], p["amount"], p["method"], parse_dt(p["paid_at"]).replace(tzinfo=None)) for p in payments],
    )
    cur.execute("SET FOREIGN_KEY_CHECKS=1")
    cur.close()
    conn.close()
    print(f"mysql: products={len(products)} invoices={len(invoices)} items={len(invoice_items)} payments={len(payments)}")


def _mongo_doc(row: dict, field_map: dict[str, str]) -> dict:
    out = {}
    for k, v in row.items():
        nk = field_map.get(k, k)
        if nk in ("ts", "startedAt", "createdAt", "at") or k in ("ts", "started_at", "created_at", "at"):
            out[nk] = parse_dt(v)
        else:
            out[nk] = v
    return out


def load_mongo(tables: dict[str, list[dict]]) -> None:
    from pymongo import MongoClient

    specs = (
        ("app_events", tables["events"], {"id": "_id", "customer_id": "customerId"}),
        ("sessions", tables["sessions"], {"id": "_id", "customer_id": "customerId", "started_at": "startedAt"}),
        ("notifications", tables["notifications"], {"id": "_id", "customer_id": "customerId", "created_at": "createdAt"}),
        ("audit_logs", tables["audit_logs"], {"id": "_id", "customer_id": "customerId"}),
    )
    uri = env("QLLM_EVENTS_MONGO_URI", "mongodb://127.0.0.1:27017")
    client = MongoClient(uri)
    db = client["events"]
    for coll, rows, fmap in specs:
        docs = [_mongo_doc(r, fmap) for r in rows]
        db[coll].drop()
        if docs:
            db[coll].insert_many(docs, ordered=False)
        print(f"mongo.{coll}={len(docs)}")
    client.close()


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--dataset", type=Path, default=DATASET_DIR)
    p.add_argument("--skip-load", action="store_true", help="Only write API JSON from dataset")
    args = p.parse_args()
    verify_manifest(args.dataset)
    tables = load_tables(args.dataset)
    write_api(tables)
    if args.skip_load:
        return
    load_postgres(tables)
    load_mysql(tables)
    load_mongo(tables)
    print("load complete")


if __name__ == "__main__":
    main()
