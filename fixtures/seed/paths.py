from __future__ import annotations

from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
DATASET_DIR = ROOT / "fixtures" / "datasets" / "v1"
API_FIXTURE = ROOT / "fixtures" / "test-api" / "data.json"
CATALOG_YAML = ROOT / "deploy" / "image" / "config" / "qllm.catalog.yaml"
GOLDENS_DIR = ROOT / "fixtures" / "goldens" / "sql-v1"
CASES_YAML = GOLDENS_DIR / "cases.yaml"
EXPECTED_DIR = GOLDENS_DIR / "expected"

LOGICAL_TABLES = (
    "customers",
    "addresses",
    "support_tickets",
    "subscriptions",
    "products",
    "invoices",
    "invoice_items",
    "payments",
    "events",
    "sessions",
    "notifications",
    "audit_logs",
    "legacy_users",
    "api_products",
    "api_tickets",
    "api_profiles",
)
