from __future__ import annotations

import argparse
import json
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
FIXTURES = HERE.parent
if str(FIXTURES) not in sys.path:
    sys.path.insert(0, str(FIXTURES))

from sqlcheck.cases import load_suite, missing_covers
from sqlcheck.compare import compare_result
from sqlcheck.oracle import load_expected, open_oracle, run_sql, write_goldens
from sqlcheck.report import preview_result, print_case_banner


def cmd_oracle(_: argparse.Namespace) -> int:
    write_goldens()
    return 0


def cmd_coverage(_: argparse.Namespace) -> int:
    suite = load_suite()
    miss = missing_covers(suite["cases"], suite["required_covers"])
    if miss:
        print("missing covers:", ", ".join(miss))
        return 1
    print(f"covers ok ({len(suite['cases'])} cases)")
    return 0


def cmd_check_oracle(_: argparse.Namespace) -> int:
    suite = load_suite()
    con = open_oracle()
    failed = 0
    for case in suite["cases"]:
        print_case_banner(case)
        if case.get("expect") == "reject":
            print("  skip exec (reject)")
            continue
        got = run_sql(con, case["sql"])
        print("oracle:")
        print(preview_result(got))
        exp = load_expected(case["id"])
        errs = compare_result(exp, got, case["sql"], case.get("sort"))
        if errs:
            failed += 1
            print("  FAIL")
            for e in errs:
                print("   ", e)
        else:
            print("  OK vs golden")
    print()
    print("=" * 72)
    if failed:
        print(f"{failed} oracle mismatches")
        return 1
    print("oracle goldens match")
    return 0


def cmd_mcp(_: argparse.Namespace) -> int:
    from sqlcheck.mcp_client import MCPClient, MCPError

    suite = load_suite()
    client = MCPClient()
    try:
        client.initialize()
        names = client.tools_list()
        want = {"how_to_use_me", "describe_catalog", "execute_sql"}
        extra = set(names) - want
        missing = want - set(names)
        if extra or missing:
            print("tools/list", names, "missing", missing, "extra", extra)
            return 1
        failed = 0
        print("tools/list:", names)
        for case in suite["cases"]:
            print_case_banner(case)
            sql = case["sql"]
            ver = str(case.get("version") or "2")
            try:
                payload = client.call_execute_sql(sql, ver)
            except MCPError as e:
                failed += 1
                print("  FAIL mcp error", e)
                continue
            print("  mcp status:", payload.get("status"), "queryId:", payload.get("queryId"))
            if payload.get("error"):
                print("  error:", json.dumps(payload.get("error"), ensure_ascii=False))
            if case.get("expect") == "reject":
                code = (payload.get("error") or {}).get("code")
                want_code = case.get("error_code") or "INVALID_SQL"
                if payload.get("status") != "failed" or code != want_code:
                    failed += 1
                    print(f"  FAIL reject want={want_code} got={code}")
                else:
                    print("  OK reject")
                continue
            if payload.get("status") != "succeeded":
                failed += 1
                print("  FAIL")
                continue
            got = payload.get("result") or {}
            print("mcp result:")
            print(preview_result(got))
            exp = load_expected(case["id"])
            errs = compare_result(exp, got, sql, case.get("sort"))
            if errs:
                failed += 1
                print("  FAIL vs golden")
                for e in errs:
                    print("   ", e)
            else:
                print("  OK vs golden")
        print()
        print("=" * 72)
        if failed:
            print(f"{failed} MCP mismatches")
            return 1
        print("MCP goldens match")
        return 0
    finally:
        client.close()


def main() -> None:
    p = argparse.ArgumentParser(prog="sqlcheck")
    sub = p.add_subparsers(dest="cmd", required=True)
    sub.add_parser("oracle", help="regenerate fixtures/goldens/sql-v1/expected")
    sub.add_parser("coverage")
    sub.add_parser("check-oracle")
    sub.add_parser("mcp")
    args = p.parse_args()
    fn = {"oracle": cmd_oracle, "coverage": cmd_coverage, "check-oracle": cmd_check_oracle, "mcp": cmd_mcp}[args.cmd]
    raise SystemExit(fn(args))


if __name__ == "__main__":
    main()
