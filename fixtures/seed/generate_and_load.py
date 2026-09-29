"""Deprecated wrapper. Prefer generate_dataset.py (Faker) and load_dataset.py (no Faker)."""

from __future__ import annotations

import argparse
import runpy
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent


def main() -> None:
    p = argparse.ArgumentParser()
    p.add_argument("--customers", type=int, default=40)
    p.add_argument("--seed", type=int, default=42)
    p.add_argument("--skip-load", action="store_true")
    p.add_argument("--regenerate", action="store_true", help="Run Faker and overwrite fixtures/datasets/v1")
    args, extra = p.parse_known_args()
    if args.regenerate:
        sys.argv = ["generate_dataset.py", "--customers", str(args.customers), "--seed", str(args.seed), *extra]
        runpy.run_path(str(HERE / "generate_dataset.py"), run_name="__main__")
    load_argv = ["load_dataset.py"]
    if args.skip_load:
        load_argv.append("--skip-load")
    sys.argv = load_argv
    runpy.run_path(str(HERE / "load_dataset.py"), run_name="__main__")


if __name__ == "__main__":
    main()
