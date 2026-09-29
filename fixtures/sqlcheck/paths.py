from __future__ import annotations

import sys
from pathlib import Path

SEED = Path(__file__).resolve().parents[1] / "seed"
if str(SEED) not in sys.path:
    sys.path.insert(0, str(SEED))

from paths import CASES_YAML, CATALOG_YAML, DATASET_DIR, EXPECTED_DIR, GOLDENS_DIR, ROOT  # noqa: E402

__all__ = ["CASES_YAML", "CATALOG_YAML", "DATASET_DIR", "EXPECTED_DIR", "GOLDENS_DIR", "ROOT"]
