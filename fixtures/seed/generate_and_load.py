"""
Generate and load fake demo data into Postgres, MySQL, MongoDB, and the test API JSON.

Usage (with port-forwards up):
  pip install -r fixtures/seed/requirements.txt
  python fixtures/seed/generate_and_load.py --customers 200
"""

from __future__ import annotations

import argparse
import json
import os
import random
from datetime import datetime, timezone
from pathlib import Path

from faker import Faker

ROOT = Path(__file__).resolve().parents[2]
API_DATA = ROOT / "deploy" / "dev" / "test-api" / "data.json"
API_FIXTURE = ROOT / "fixtures" / "test-api" / "data.json"


def env(name: str, default: str) -> str:
    return os.environ.get(name, default)


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--customers", type=int, default=200)
    p.add_argument("--seed", type=int, default=42)
    p.add_argument("--skip-load", action="store_true", help="Only write API JSON")
    args = p.parse_args()

    fake = Faker()
    Faker.seed(args.seed)
    random.seed(args.seed)

    n = args.customers
    print(f"generating dataset customers={n} seed={args.seed}")

    customers = []
    for i in range(1, n + 1):
        cid = f"c{i}"
        customers.append(
            {
                "id": cid,
                "email": fake.unique.email(),
                "full_name": fake.name(),
                "phone": fake.phone_number()[:32],
                "country": fake.country_code(),
                "created_at": fake.date_time_between(start_date="-2y", end_date="now", tzinfo=timezone.utc),
            }
        )

    addresses = []
    for c in customers:
        for j in range(random.randint(1, 2)):
            addresses.append(
                {
                    "id": f"a{c['id']}_{j}",
                    "customer_id": c["id"],
                    "city": fake.city(),
                    "state": fake.state_abv() if hasattr(fake, "state_abv") else fake.country_code(),
                    "country": c["country"],
                    "postal_code": fake.postcode()[:16],
                }
            )

    tickets = []
    for c in random.sample(customers, k=min(len(customers), max(1, n // 2))):
        for j in range(random.randint(1, 3)):
            tickets.append(
                {
                    "id": f"t{c['id']}_{j}",
                    "customer_id": c["id"],
                    "subject": fake.sentence(nb_words=6)[:120],
                    "status": random.choice(["open", "pending", "closed"]),
                    "priority": random.choice(["low", "medium", "high"]),
                    "created_at": fake.date_time_between(start_date="-1y", end_date="now", tzinfo=timezone.utc),
                }
            )

    subscriptions = []
    plans = ["free", "pro", "business", "enterprise"]
    for c in customers:
        if random.random() < 0.7:
            plan = random.choice(plans)
            mrr = {"free": 0, "pro": 2900, "business": 9900, "enterprise": 49900}[plan]
            subscriptions.append(
                {
                    "id": f"s{c['id']}",
                    "customer_id": c["id"],
                    "plan": plan,
                    "status": random.choice(["active", "past_due", "canceled"]),
                    "mrr_cents": mrr,
                    "started_at": fake.date_time_between(start_date="-2y", end_date="now", tzinfo=timezone.utc),
                }
            )

    products = []
    categories = ["software", "addon", "support", "hardware"]
    for i in range(1, 41):
        products.append(
            {
                "id": f"p{i}",
                "sku": f"SKU-{i:04d}",
                "name": fake.catch_phrase()[:80],
                "price_cents": random.randint(500, 50000),
                "category": random.choice(categories),
            }
        )

    invoices = []
    invoice_items = []
    payments = []
    for idx, c in enumerate(customers, start=1):
        for j in range(random.randint(0, 4)):
            inv_id = f"i{idx}_{j}"
            status = random.choice(["paid", "paid", "open", "void"])
            items_n = random.randint(1, 4)
            total = 0
            for k in range(items_n):
                prod = random.choice(products)
                qty = random.randint(1, 5)
                unit = prod["price_cents"]
                total += qty * unit
                invoice_items.append(
                    {
                        "id": f"{inv_id}_l{k}",
                        "invoice_id": inv_id,
                        "product_id": prod["id"],
                        "qty": qty,
                        "unit_cents": unit,
                    }
                )
            issued = fake.date_time_between(start_date="-18m", end_date="now", tzinfo=timezone.utc)
            invoices.append(
                {
                    "id": inv_id,
                    "customer_id": c["id"],
                    "total_cents": total,
                    "status": status,
                    "issued_at": issued,
                }
            )
            if status == "paid":
                payments.append(
                    {
                        "id": f"pay_{inv_id}",
                        "invoice_id": inv_id,
                        "customer_id": c["id"],
                        "amount_cents": total,
                        "method": random.choice(["card", "pix", "boleto", "wire"]),
                        "paid_at": issued,
                    }
                )

    events = []
    event_types = ["login", "logout", "purchase", "signup", "view", "click", "refund"]
    for c in customers:
        for j in range(random.randint(2, 8)):
            events.append(
                {
                    "_id": f"e{c['id']}_{j}",
                    "customerId": c["id"],
                    "type": random.choice(event_types),
                    "ts": fake.date_time_between(start_date="-6m", end_date="now", tzinfo=timezone.utc),
                    "meta": {"ip": fake.ipv4_public(), "ua": fake.user_agent()[:80]},
                }
            )

    sessions = []
    for c in random.sample(customers, k=min(len(customers), n)):
        for j in range(random.randint(1, 3)):
            sessions.append(
                {
                    "_id": f"sess{c['id']}_{j}",
                    "customerId": c["id"],
                    "device": random.choice(["web", "ios", "android"]),
                    "startedAt": fake.date_time_between(start_date="-3m", end_date="now", tzinfo=timezone.utc),
                    "active": random.random() < 0.3,
                }
            )

    notifications = []
    for c in random.sample(customers, k=min(len(customers), max(1, n * 2 // 3))):
        for j in range(random.randint(1, 4)):
            notifications.append(
                {
                    "_id": f"n{c['id']}_{j}",
                    "customerId": c["id"],
                    "channel": random.choice(["email", "sms", "push"]),
                    "title": fake.sentence(nb_words=4)[:80],
                    "read": random.random() < 0.5,
                    "createdAt": fake.date_time_between(start_date="-3m", end_date="now", tzinfo=timezone.utc),
                }
            )

    audit_logs = []
    for i in range(n * 3):
        c = random.choice(customers)
        audit_logs.append(
            {
                "_id": f"aud{i}",
                "customerId": c["id"],
                "action": random.choice(["create", "update", "delete", "login", "export"]),
                "resource": random.choice(["invoice", "user", "ticket", "subscription"]),
                "at": fake.date_time_between(start_date="-1y", end_date="now", tzinfo=timezone.utc),
            }
        )

    api_payload = {
        "users": [
            {
                "id": c["id"],
                "email": c["email"],
                "full_name": c["full_name"],
                "country": c["country"],
            }
            for c in customers
        ],
        "products": products,
        "tickets": [
            {
                "id": t["id"],
                "customer_id": t["customer_id"],
                "subject": t["subject"],
                "status": t["status"],
                "priority": t["priority"],
            }
            for t in tickets
        ],
    }
    for path in (API_DATA, API_FIXTURE):
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(json.dumps(api_payload), encoding="utf-8")
        print(f"wrote {path} ({path.stat().st_size // 1024} KB)")

    if args.skip_load:
        return

    load_postgres(customers, addresses, tickets, subscriptions)
    load_mysql(products, invoices, invoice_items, payments)
    load_mongo(events, sessions, notifications, audit_logs)
    print("load complete")


def load_postgres(customers, addresses, tickets, subscriptions) -> None:
    import psycopg2
    from psycopg2.extras import execute_batch

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
        [(c["id"], c["email"], c["full_name"], c["phone"], c["country"], c["created_at"]) for c in customers],
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
        [(t["id"], t["customer_id"], t["subject"], t["status"], t["priority"], t["created_at"]) for t in tickets],
        page_size=200,
    )
    execute_batch(
        cur,
        "INSERT INTO subscriptions (id,customer_id,plan,status,mrr_cents,started_at) VALUES (%s,%s,%s,%s,%s,%s)",
        [(s["id"], s["customer_id"], s["plan"], s["status"], s["mrr_cents"], s["started_at"]) for s in subscriptions],
        page_size=200,
    )
    cur.close()
    conn.close()
    print(f"postgres: customers={len(customers)} addresses={len(addresses)} tickets={len(tickets)} subs={len(subscriptions)}")


def load_mysql(products, invoices, invoice_items, payments) -> None:
    import pymysql

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
        [(p["id"], p["sku"], p["name"], p["price_cents"], p["category"]) for p in products],
    )
    cur.executemany(
        "INSERT INTO invoices (id,customer_id,total_cents,status,issued_at) VALUES (%s,%s,%s,%s,%s)",
        [(i["id"], i["customer_id"], i["total_cents"], i["status"], i["issued_at"].replace(tzinfo=None)) for i in invoices],
    )
    cur.executemany(
        "INSERT INTO invoice_items (id,invoice_id,product_id,qty,unit_cents) VALUES (%s,%s,%s,%s,%s)",
        [(x["id"], x["invoice_id"], x["product_id"], x["qty"], x["unit_cents"]) for x in invoice_items],
    )
    cur.executemany(
        "INSERT INTO payments (id,invoice_id,customer_id,amount_cents,method,paid_at) VALUES (%s,%s,%s,%s,%s,%s)",
        [(p["id"], p["invoice_id"], p["customer_id"], p["amount_cents"], p["method"], p["paid_at"].replace(tzinfo=None)) for p in payments],
    )
    cur.execute("SET FOREIGN_KEY_CHECKS=1")
    cur.close()
    conn.close()
    print(f"mysql: products={len(products)} invoices={len(invoices)} items={len(invoice_items)} payments={len(payments)}")


def load_mongo(events, sessions, notifications, audit_logs) -> None:
    from pymongo import MongoClient

    uri = env("QLLM_EVENTS_MONGO_URI", "mongodb://127.0.0.1:27017")
    client = MongoClient(uri)
    db = client["events"]
    for name, docs in (
        ("app_events", events),
        ("sessions", sessions),
        ("notifications", notifications),
        ("audit_logs", audit_logs),
    ):
        db[name].drop()
        if docs:
            db[name].insert_many(docs, ordered=False)
        print(f"mongo.{name}={len(docs)}")
    client.close()


if __name__ == "__main__":
    main()
