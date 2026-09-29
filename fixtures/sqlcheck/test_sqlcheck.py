from __future__ import annotations

import json
import os
import sys
from pathlib import Path

import pytest

FIXTURES = Path(__file__).resolve().parents[1]
if str(FIXTURES) not in sys.path:
    sys.path.insert(0, str(FIXTURES))

from sqlcheck.cases import load_suite, missing_covers
from sqlcheck.compare import compare_result
from sqlcheck.oracle import load_expected, open_oracle, run_sql
from sqlcheck.report import preview_result, print_case_banner, sql_oneline


def _cases():
    return load_suite()["cases"]


def _ids(cases):
    return [c["id"] for c in cases]


@pytest.fixture(scope="session")
def suite():
    return load_suite()


@pytest.fixture(scope="session")
def oracle_con():
    return open_oracle()


def test_required_covers(suite):
    miss = missing_covers(suite["cases"], suite["required_covers"])
    assert not miss, f"uncovered tags: {miss}"


@pytest.mark.parametrize("case", _cases(), ids=_ids(_cases()))
def test_oracle_case(oracle_con, case):
    print_case_banner(case)
    if case.get("expect") == "reject":
        exp = load_expected(case["id"])
        print("  golden: reject", exp.get("errorCode"))
        assert exp.get("expect") == "reject"
        return
    got = run_sql(oracle_con, case["sql"])
    exp = load_expected(case["id"])
    print("oracle:")
    print(preview_result(got))
    errs = compare_result(exp, got, case["sql"], case.get("sort"))
    if errs:
        print("  mismatch vs golden:")
        for e in errs:
            print("   ", e)
    assert not errs, f"{case['id']}: " + "; ".join(errs)


@pytest.mark.parametrize("case", [c for c in _cases() if c.get("expect") != "reject"], ids=lambda c: c["id"])
def test_sql_uses_logical_names(case):
    banned = ("mrr_cents", "total_cents", "price_cents", "unit_cents", "amount_cents", "customerId")
    sql = case["sql"]
    for b in banned:
        assert b not in sql, f"{case['id']} uses physical {b} in {sql_oneline(sql)}"


@pytest.fixture(scope="session")
def mcp_session():
    if os.environ.get("QLLM_SQLCHECK_SKIP_MCP") == "1":
        pytest.skip("QLLM_SQLCHECK_SKIP_MCP=1")
    from sqlcheck.mcp_client import MCPClient, MCPError

    client = MCPClient()
    try:
        client.initialize()
    except Exception as e:
        client.close()
        if os.environ.get("QLLM_SQLCHECK_REQUIRE_MCP") == "1":
            raise
        pytest.skip(f"MCP not reachable: {e}")
    try:
        names = client.tools_list()
        assert set(names) == {"how_to_use_me", "describe_catalog", "execute_sql"}, names
        yield client
    except MCPError as e:
        if os.environ.get("QLLM_SQLCHECK_REQUIRE_MCP") == "1":
            raise
        pytest.skip(str(e))
    finally:
        client.close()


@pytest.mark.mcp
@pytest.mark.parametrize("case", _cases(), ids=_ids(_cases()))
def test_mcp_case(mcp_session, case):
    from sqlcheck.mcp_client import MCPError

    print_case_banner(case)
    try:
        payload = mcp_session.call_execute_sql(case["sql"], str(case.get("version") or "2"))
    except MCPError as e:
        if os.environ.get("QLLM_SQLCHECK_REQUIRE_MCP") == "1":
            raise
        pytest.skip(str(e))
    print("  mcp status:", payload.get("status"), "queryId:", payload.get("queryId"))
    if payload.get("error"):
        print("  error:", json.dumps(payload.get("error"), ensure_ascii=False))
    if case.get("expect") == "reject":
        code = (payload.get("error") or {}).get("code")
        want = case.get("error_code") or "INVALID_SQL"
        print(f"  reject code={code} want={want}")
        assert payload.get("status") == "failed", case["id"]
        assert code == want, (case["id"], payload)
        return
    assert payload.get("status") == "succeeded", (case["id"], payload.get("queryId"), payload.get("error"))
    got = payload.get("result") or {}
    print("mcp result:")
    print(preview_result(got))
    exp = load_expected(case["id"])
    errs = compare_result(exp, got, case["sql"], case.get("sort"))
    if errs:
        print("  mismatch vs golden:")
        for e in errs:
            print("   ", e)
    assert not errs, f"{case['id']} {payload.get('queryId')}: " + "; ".join(errs)
    assert "UNKNOWN_FIELD" not in json.dumps(payload)
