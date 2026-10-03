#!/usr/bin/env python3
"""Create a slim qLLM repo (runtime + blank SQLite project) for GitHub/GitLab."""

from __future__ import annotations

import argparse
import re
import shutil
import sqlite3
import sys
from pathlib import Path

REPO = Path(__file__).resolve().parents[2]
UPSTREAM = "https://github.com/jpandrade30/qLLM"


def slug_user(raw: str) -> str:
    s = re.sub(r"[^a-z0-9]+", "-", raw.strip().lower()).strip("-")
    if not s or not s[0].isalpha():
        s = "user" if not s else f"user-{s}"
    return s


def project_id(slug: str) -> str:
    return "qllm_" + slug.replace("-", "_")


def copy_runtime(dest: Path) -> None:
    shutil.copy2(REPO / "go.mod", dest / "go.mod")
    shutil.copy2(REPO / "go.sum", dest / "go.sum")
    cmd = dest / "cmd" / "qllm"
    cmd.mkdir(parents=True, exist_ok=True)
    for p in (REPO / "cmd" / "qllm").glob("*.go"):
        if p.name.endswith("_test.go"):
            continue
        shutil.copy2(p, cmd / p.name)
    src_root = REPO / "internal"
    dst_root = dest / "internal"
    for p in src_root.rglob("*"):
        if not p.is_file():
            continue
        if p.suffix == ".go":
            if p.name.endswith("_test.go"):
                continue
        elif p.suffix == ".json" and "schemas" in p.parts:
            pass
        else:
            continue
        rel = p.relative_to(src_root)
        out = dst_root / rel
        out.parent.mkdir(parents=True, exist_ok=True)
        shutil.copy2(p, out)


def write_sqlite(path: Path) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    if path.exists():
        path.unlink()
    conn = sqlite3.connect(path)
    try:
        conn.execute("CREATE TABLE items (id TEXT PRIMARY KEY, name TEXT)")
        conn.execute("INSERT INTO items (id, name) VALUES ('1', 'hello')")
        conn.commit()
    finally:
        conn.close()


def write_text(path: Path, body: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(body, encoding="utf-8", newline="\n")


def generate_config(dest: Path, project: str) -> None:
    cfg = dest / "config"
    write_text(
        cfg / "qllm.preset.yaml",
        f"""protocolVersion: "0.2.0"
project: {project}
limits:
  maxSyncMs: 15000
  maxSourceMs: 12000
  defaultLimit: 100
  maxLimit: 1000
  readOnly: true
sources:
  - id: local_sqlite
    type: sqlite
    connection:
      pathEnv: QLLM_SQLITE_PATH
""",
    )
    write_text(
        cfg / "qllm.catalog.yaml",
        f"""protocolVersion: "0.2.0"
project: {project}
entities:
  - name: items
    source: local_sqlite
    binding:
      kind: table
      schema: main
      table: items
    fields:
      - name: id
        type: string
        physical: id
      - name: name
        type: string
        physical: name
""",
    )
    write_text(
        cfg / "qllm.config.yaml",
        """serve:
  addr: "0.0.0.0:8088"
  mcpAddr: "0.0.0.0:8089"
  authTokenEnv: "QLLM_AUTH_TOKEN"
  insecureBind: false
  maxBodyBytes: 1048576
  maxRestResponseBytes: 10485760
  cors:
    origins: []
""",
    )
    write_text(
        cfg / "qllm.env.yaml",
        """QLLM_SQLITE_PATH: /data/app.db
QLLM_AUTH_TOKEN: ${QLLM_AUTH_TOKEN}
""",
    )
    write_text(
        cfg / "qllm.access.yaml",
        """apps:
  - name: demo-agent
    key: ${QLLM_AUTH_TOKEN}
    tables: [items]
""",
    )


def generate_dockerfile(dest: Path) -> None:
    write_text(
        dest / "Dockerfile",
        """# syntax=docker/dockerfile:1
FROM golang:1.26.6-bookworm AS build
WORKDIR /src
RUN apt-get update && apt-get install -y --no-install-recommends gcc libc6-dev && rm -rf /var/lib/apt/lists/*
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ENV CGO_ENABLED=1
RUN go build -tags duckdb -o /out/qllm ./cmd/qllm

FROM debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/* \\
    && mkdir -p /config /data
COPY --from=build /out/qllm /usr/local/bin/qllm
COPY config/ /config/
COPY data/app.db /data/app.db

EXPOSE 8088 8089
ENTRYPOINT ["qllm"]
CMD ["serve", "--http", "--mcp-http", "--config-dir", "/config"]
""",
    )


def generate_readme(dest: Path, slug: str, project: str) -> None:
    write_text(
        dest / "README.md",
        f"""# qllm-{slug}

Slim qLLM runtime for **{project}**. Generated from [{UPSTREAM}]({UPSTREAM}). Host this folder on GitHub or GitLab — it is enough to build and run.

## Run (Windows, Linux, macOS)

Docker or a compatible engine (`docker`, `nerdctl`, `podman`):

```bash
cp .env.example .env
# edit .env — set QLLM_AUTH_TOKEN

docker build -t qllm-{slug} .
docker run --rm -p 8088:8088 -p 8089:8089 --env-file .env qllm-{slug}
```

```bash
curl -s http://127.0.0.1:8088/v1/health
curl -s -H "Authorization: Bearer change-me" http://127.0.0.1:8088/v1/catalog
```

Replace `change-me` with the token in `.env`. `GET /v1/health` does not need a token.

The image includes a SQLite file with one table `items` (row `id=1`, `name=hello`). Catalog SQL:

```bash
curl -s -X POST http://127.0.0.1:8088/v1/sql \\
  -H "Authorization: Bearer change-me" \\
  -H "Content-Type: application/json" \\
  -d '{{"sql":"SELECT id, name FROM items LIMIT 10"}}'
```

## Validate without Docker

If you have Go **1.26.6+** (pure Go build, no CGO):

```bash
go build -o qllm ./cmd/qllm
./qllm validate --config-dir ./config
```

On the host, set `QLLM_SQLITE_PATH` to `data/app.db` (relative to where you run the binary) and `QLLM_AUTH_TOKEN` before `serve`.

## Add your own sources

Edit `config/qllm.preset.yaml` and `config/qllm.catalog.yaml`. Secrets stay in env vars (`*Env` keys), not in YAML.

Guides in the upstream repo:

- [{UPSTREAM}/blob/main/docs/en/from-scratch.md]({UPSTREAM}/blob/main/docs/en/from-scratch.md)
- [{UPSTREAM}/blob/main/docs/en/field-reference.md]({UPSTREAM}/blob/main/docs/en/field-reference.md)
- [{UPSTREAM}/tree/main/planning]({UPSTREAM}/tree/main/planning)

Rebuild the image after YAML changes, or mount `./config` over `/config`.
""",
    )


def generate_dotfiles(dest: Path) -> None:
    write_text(dest / ".env.example", "QLLM_AUTH_TOKEN=change-me\n")
    write_text(
        dest / ".gitignore",
        """*.exe
/qllm
/qllm.exe
.env
.DS_Store
tmp/
*.log
.vscode/
.idea/
""",
    )


def parse_args(argv: list[str]) -> argparse.Namespace:
    p = argparse.ArgumentParser(
        description="Create a slim qLLM project folder (runtime + blank SQLite config).",
    )
    p.add_argument("--user", required=True, help="Owner name (folder qllm-<slug>)")
    p.add_argument(
        "--out",
        default=".",
        help="Parent directory for the new folder (default: current directory)",
    )
    p.add_argument("--force", action="store_true", help="Overwrite an existing folder")
    return p.parse_args(argv)


def main(argv: list[str] | None = None) -> int:
    args = parse_args(argv if argv is not None else sys.argv[1:])
    slug = slug_user(args.user)
    project = project_id(slug)
    dest = Path(args.out).expanduser().resolve() / f"qllm-{slug}"
    if dest.exists():
        if not args.force:
            print(f"refusing to overwrite {dest} (pass --force)", file=sys.stderr)
            return 1
        shutil.rmtree(dest)
    dest.mkdir(parents=True)
    copy_runtime(dest)
    license_src = REPO / "LICENSE.md"
    if license_src.is_file():
        shutil.copy2(license_src, dest / "LICENSE.md")
    generate_config(dest, project)
    write_sqlite(dest / "data" / "app.db")
    generate_dockerfile(dest)
    generate_readme(dest, slug, project)
    generate_dotfiles(dest)
    print(dest)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
