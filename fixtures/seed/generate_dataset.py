"""Write frozen logical JSON under fixtures/datasets/v1/. Does not touch DBs."""

from __future__ import annotations

import argparse
import hashlib
import random
from datetime import datetime, timezone
from pathlib import Path

from faker import Faker

from jsonutil import dump_json, utc_iso
from paths import DATASET_DIR, LOGICAL_TABLES


def build_tables(n: int, seed: int) -> dict[str, list[dict]]:
    fake = Faker()
    Faker.seed(seed)
    random.seed(seed)

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
            abbr = fake.state_abbr() if hasattr(fake, "state_abbr") else fake.country_code()
            addresses.append(
                {
                    "id": f"a{c['id']}_{j}",
                    "customer_id": c["id"],
                    "city": fake.city(),
                    "state": abbr,
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
                    "mrr": mrr,
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
                "price": random.randint(500, 50000),
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
                unit = prod["price"]
                total += qty * unit
                invoice_items.append(
                    {
                        "id": f"{inv_id}_l{k}",
                        "invoice_id": inv_id,
                        "product_id": prod["id"],
                        "qty": qty,
                        "unit": unit,
                    }
                )
            issued = fake.date_time_between(start_date="-18m", end_date="now", tzinfo=timezone.utc)
            invoices.append(
                {
                    "id": inv_id,
                    "customer_id": c["id"],
                    "total": total,
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
                        "amount": total,
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
                    "id": f"e{c['id']}_{j}",
                    "customer_id": c["id"],
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
                    "id": f"sess{c['id']}_{j}",
                    "customer_id": c["id"],
                    "device": random.choice(["web", "ios", "android"]),
                    "started_at": fake.date_time_between(start_date="-3m", end_date="now", tzinfo=timezone.utc),
                    "active": random.random() < 0.3,
                }
            )

    notifications = []
    for c in random.sample(customers, k=min(len(customers), max(1, n * 2 // 3))):
        for j in range(random.randint(1, 4)):
            notifications.append(
                {
                    "id": f"n{c['id']}_{j}",
                    "customer_id": c["id"],
                    "channel": random.choice(["email", "sms", "push"]),
                    "title": fake.sentence(nb_words=4)[:80],
                    "read": random.random() < 0.5,
                    "created_at": fake.date_time_between(start_date="-3m", end_date="now", tzinfo=timezone.utc),
                }
            )

    audit_logs = []
    for i in range(n * 3):
        c = random.choice(customers)
        audit_logs.append(
            {
                "id": f"aud{i}",
                "customer_id": c["id"],
                "action": random.choice(["create", "update", "delete", "login", "export"]),
                "resource": random.choice(["invoice", "user", "ticket", "subscription"]),
                "at": fake.date_time_between(start_date="-1y", end_date="now", tzinfo=timezone.utc),
            }
        )

    legacy_users = [
        {"id": c["id"], "email": c["email"], "full_name": c["full_name"], "country": c["country"]}
        for c in customers
    ]
    api_tickets = [
        {
            "id": t["id"],
            "customer_id": t["customer_id"],
            "subject": t["subject"],
            "status": t["status"],
            "priority": t["priority"],
        }
        for t in tickets
    ]

    return {
        "customers": customers,
        "addresses": addresses,
        "support_tickets": tickets,
        "subscriptions": subscriptions,
        "products": products,
        "invoices": invoices,
        "invoice_items": invoice_items,
        "payments": payments,
        "events": events,
        "sessions": sessions,
        "notifications": notifications,
        "audit_logs": audit_logs,
        "legacy_users": legacy_users,
        "api_products": products,
        "api_tickets": api_tickets,
    }


def write_dataset(out_dir: Path, tables: dict[str, list[dict]], n: int, seed: int) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)
    files: dict[str, str] = {}
    for name in LOGICAL_TABLES:
        path = out_dir / f"{name}.json"
        dump_json(path, tables[name])
        digest = hashlib.sha256(path.read_bytes()).hexdigest()
        files[path.name] = digest

    manifest = {
        "version": "v1",
        "seed": seed,
        "n_customers": n,
        "generated_at": utc_iso(datetime.now(timezone.utc)),
        "files": files,
    }
    dump_json(out_dir / "manifest.json", manifest)
    print(f"wrote {out_dir} customers={n} seed={seed}")


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--customers", type=int, default=40)
    p.add_argument("--seed", type=int, default=42)
    p.add_argument("--out", type=Path, default=DATASET_DIR)
    args = p.parse_args()
    tables = build_tables(args.customers, args.seed)
    write_dataset(args.out, tables, args.customers, args.seed)


if __name__ == "__main__":
    main()
