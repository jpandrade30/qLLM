# Installation guide

This page covers every way to get qLLM running: what each option does, what it needs, and how to check it worked. GitHub Releases ship a slim source zip (`qllm-standalone-<ver>.zip`); you still build the binary or image yourself.

## 1. Pick an option

| Option | Needs | You get | Use it when |
|--------|-------|---------|-------------|
| **A. Container image** (`Dockerfile`) | Docker, nerdctl, or podman | Full build (embedded DuckDB) with your `deploy/prd` YAML baked in | Production, or you do not want a C toolchain |
| **B. Go build, pure Go** | Go 1.26.6+ | `qllm` without CGO. No catalog SQL | Quick `validate`, Query IR, CI without CGO |
| **C. Go build, embedded DuckDB** (`-tags duckdb`) | Go 1.26.6+, CGO, a C compiler (+ `duckdblib` on Windows) | Everything, including `execute_sql` / `qllm sql` | Local development of the full product |
| **D. Standalone zip** (prefer Release asset; or `init-standalone.*`) | Docker (to run); Python 3 only if regenerating from a clone | A small folder you can host on GitHub/GitLab | **Default suggestion:** run without cloning this monorepo |
| **E. Compose harness** (`docker-compose.yml`) | nerdctl compose (Rancher Desktop) | qLLM + Postgres, MySQL, MongoDB, fake API | Running the goldens and trying the demo |
| **F. Kubernetes simulation** (`deploy/prd-tst`) | Rancher Desktop with Kubernetes | A fleet-ops style cluster | Testing rollouts. Separate world, see [environments.md](environments.md) |
| **G. Scoped-key demo** (`docker-compose.enforced.yml`) | nerdctl/docker compose | qLLM + Postgres + a LangGraph agent | Seeing row scope (D21) end to end |

Not sure? Use **D** (Release zip) or **A** to run it; **C** to develop this repo.

## 2. The two engines: pure Go vs embedded DuckDB

qLLM has one local join engine behind a single `Engine` API. Which one you get depends on a build tag.

| | Pure Go (default) | Embedded DuckDB (`-tags duckdb`) |
|---|---|---|
| Build command | `go build ./cmd/qllm` | `go build -tags duckdb ./cmd/qllm` |
| CGO / C compiler | Not needed | Required |
| Query IR (`/v1/queries`, `qllm query`) | Yes, including cross-source joins, REST aggregations, `where`, offset | Yes |
| Catalog SQL (`/v1/sql`, MCP `execute_sql`, `qllm sql`) | **No**. It cannot execute the SQL | **Yes** |
| `go test ./...` | CGO-free | Only the tagged packages need the tag |

The container images and production use `-tags duckdb`. If a SQL request fails because the build lacks DuckDB, you are running a pure Go binary.

## 3. Option A: container image

The root [`Dockerfile`](../../Dockerfile) builds in two stages.

1. **Build stage** (`golang:1.26.6-bookworm`): installs `gcc` and `libc6-dev`, downloads Go modules, sets `CGO_ENABLED=1`, runs `go build -tags duckdb`.
2. **Runtime stage** (`debian:bookworm-slim`): adds `ca-certificates`, copies the binary to `/usr/local/bin/qllm`, and bakes these files from `deploy/prd/default/` into `/config`: `qllm.preset.yaml`, `qllm.catalog.yaml`, `qllm.config.yaml`, `qllm.env.yaml`, `qllm.access.yaml`.

The container starts with `qllm serve --http --mcp-http --config-dir /config` and exposes **8088** (HTTP `/v1`) and **8089** (MCP HTTP).

```bash
nerdctl build -t qllm .          # or: docker build -t qllm .
nerdctl run --rm -p 8088:8088 -p 8089:8089 qllm
```

To use your own YAML, either edit `deploy/prd/default/*.yaml` and rebuild, or mount your folder over the baked one and skip the rebuild:

```bash
docker run --rm -p 8088:8088 -p 8089:8089 -v "$PWD/my-project:/config" qllm
```

Secrets are environment variables named by the `*Env` keys in the preset, passed with `-e NAME=value` or `--env-file`. They never go in the YAML.

Inside a container the server must listen on a non-loopback address, which `qllm` only allows with auth set (or `serve.insecureBind: true`). Keep `serve.addr` and `authTokenEnv` consistent in the baked `qllm.config.yaml`.

The three Dockerfiles differ only in what they bake:

| File | Bakes into `/config` | Used by |
|------|----------------------|---------|
| `Dockerfile` | `deploy/prd/default/` | Product image |
| `Dockerfile.dev` | `deploy/image/config/` | Compose harness and `prd-tst` (Kubernetes mounts a ConfigMap over `/config`) |
| `Dockerfile.enforced` | `deploy/prd/enforced/config/` | Scoped-key demo |

### Rancher Desktop and Kubernetes

Kubernetes in Rancher Desktop reads images from the `k8s.io` containerd namespace:

```bash
nerdctl --namespace k8s.io build -t qllm:local .
```

If you built in the default namespace, copy the image:

```bash
nerdctl save qllm:local -o qllm-local.tar
nerdctl --namespace k8s.io load -i qllm-local.tar
```

`ErrImagePull` in the simulation means the image is not in that namespace (the manifests use `imagePullPolicy: Never`).

## 4. Option B: Go build, pure Go

Install **Go 1.26.6 or newer** (`go.mod` sets `go 1.26.6`).

```bash
go build -o qllm ./cmd/qllm        # Windows: -o qllm.exe
go test ./...
./qllm validate --config-dir ./my-project
```

No C compiler is involved. Use it to validate files and to run Query IR. Catalog SQL will not work (section 2).

## 5. Option C: Go build with embedded DuckDB

Add `-tags duckdb` and enable CGO. Everything else is the same command.

### Linux and macOS

Install a C compiler (`gcc`, or Xcode command line tools: `xcode-select --install`). The Go bindings (`duckdb-go-bindings`, see `go.mod`) include the DuckDB library for linux and darwin on amd64 and arm64, so you do not download DuckDB yourself.

```bash
source ./scripts/dev/dev-shell.sh      # sets CGO_ENABLED=1 and the demo QLLM_* variables
go test -tags duckdb ./internal/duckdblocal/
go build -tags duckdb -o qllm ./cmd/qllm
```

`dev-shell.sh` must be **sourced**, not executed, or the variables disappear when it ends. Its demo variables use `${VAR:-default}`, so anything you already exported wins. If you only need CGO:

```bash
CGO_ENABLED=1 go build -tags duckdb -o qllm ./cmd/qllm
```

### Windows

On Windows the repository links against the official DuckDB library through two things: a MinGW gcc and the `duckdblib/` folder.

**Step 1. gcc (MSYS2 UCRT64).** Install [MSYS2](https://www.msys2.org/), open the **UCRT64** shell, and run:

```bash
pacman -S mingw-w64-ucrt-x86_64-gcc
```

`scripts/dev/dev-shell.ps1` looks for `gcc.exe` in `C:\ghcup\msys64\ucrt64\bin`. If MSYS2 lives elsewhere (the default is `C:\msys64\ucrt64\bin`), edit the `$MsysGccBin` line at the top of the script. Without gcc the script prints the install hint and exits.

**Step 2. DuckDB library.** From the [official DuckDB installation page](https://duckdb.org/docs/installation/), download the C/C++ library for Windows (`libduckdb-windows-amd64.zip`). Extract it and copy **`duckdb.dll`**, **`duckdb.lib`**, and the header **`duckdb.h`** into the `duckdblib/` folder at the repo root. These are large binaries; do not commit them to your own fork.

**Step 3. Load the environment.** In PowerShell:

```powershell
.\scripts\dev\dev-shell.ps1
```

or double-click `scripts\dev\dev-shell.cmd`, which opens a PowerShell with the script loaded (execution policy bypassed for that window only). The script:

| Action | Why |
|--------|-----|
| Puts the MSYS2 `bin` and `duckdblib/` first on `PATH` | The linker finds gcc, and `qllm.exe` finds `duckdb.dll` **at run time** |
| `CGO_ENABLED=1`, `CC=gcc` | Turns on CGO |
| `CGO_CFLAGS=-I<duckdblib>` | Compiler finds `duckdb.h` |
| `CGO_LDFLAGS=-L<duckdblib> -lduckdb` | Linker finds the library |
| Sets demo `QLLM_*` variables (local Postgres, MySQL, Mongo, fake API) | Matches the Compose harness. Ignore them for your own project |
| Changes directory to the repo root and sets the prompt to `qLLM-dev` | Convenience |

It only changes the **current** session. Open a new terminal and you must run it again.

**Step 4. Build and check.**

```powershell
go test -tags duckdb ./internal/duckdblocal/
go build -tags duckdb -o qllm.exe ./cmd/qllm
go run -tags duckdb .\scripts\dev\duckdb_smoke.go
```

`duckdb_smoke.go` opens the embedded engine and runs a tiny query, so it is the fastest way to prove CGO and the library are wired correctly.

To run `qllm.exe` from a normal terminal later, make sure `duckdb.dll` is next to the executable or on `PATH`.

## 6. Option D: standalone zip (preferred over clone)

Do **not** clone this monorepo just to host qLLM. Download **`qllm-standalone-<ver>.zip`** from the [GitHub Release](https://github.com/jpandrade30/qLLM/releases) (ignore automatic **Source code** archives), unzip, then `docker build` as in that folder's README.

Only if you already have a clone and need a named folder (no implementer `docs/`, fixtures, or harness):

```bash
python scripts/standalone/init-standalone.py --user Alice --out ..
./scripts/standalone/init-standalone.sh  --user Alice --out ..      # Linux/macOS, calls python3 or python
.\scripts\standalone\init-standalone.ps1 --user Alice --out ..      # Windows
```

| Flag | Meaning |
|------|---------|
| `--user` | Required. The name becomes a slug: lower case, anything outside `a-z0-9` turns into `-`. A slug that does not start with a letter gets a `user-` prefix |
| `--out` | **Parent** directory for the new folder. Default: current directory. `--out ..` puts it next to this repo |
| `--force` | Delete and recreate the target folder if it exists. Without it the script refuses |

The result is `<out>/qllm-<slug>/` and the script prints that path. It contains:

| Path | What it is |
|------|-----------|
| `go.mod`, `go.sum`, `cmd/qllm/`, `internal/` | The Go runtime, without tests |
| `planning/` | Protocol prose + JSON Schemas — keep this so humans/LLMs can author real config (`planning/` wins) |
| `config/` | **Example** YAML only (SQLite `items`). Replace every value for your sources; see the [Configure](https://jpandrade30.github.io/qLLM/configure.html) product page |
| `data/app.db` | The SQLite smoke file (`items`: `id=1`, `name=hello`) |
| `Dockerfile` | Same two-stage build with `-tags duckdb`, bakes `config/` and `data/` |
| `LICENSE.md`, `.env.example`, `.gitignore`, `README.md` | MIT license, token template, ignores, run instructions |

In the full monorepo, `deploy/` is the same kind of **example** layout — copy the shape, replace the content.

Run it in the new folder:

```bash
cp .env.example .env          # set QLLM_AUTH_TOKEN
docker build -t qllm-alice .
docker run --rm -p 8088:8088 -p 8089:8089 --env-file .env qllm-alice
curl -s http://127.0.0.1:8088/v1/health
```

Without Docker, a pure Go build can run `validate` (no SQL). Then edit the preset and catalog to add your own sources.

## 7. Option E: Compose harness

```bash
nerdctl compose up --build
```

Builds `Dockerfile.dev` and starts qLLM, Postgres, MySQL, MongoDB, and the fake API. Demo Bearer token: `change-me`. Host ports: HTTP 8088, MCP 8089. Load data with `.\scripts\dev\dev-seed-fake.ps1` or `./scripts/dev/dev-seed-fake.sh`. Run the SQL goldens with `pytest fixtures/sqlcheck`. Passwords are weak and Mongo is open, so keep it on localhost. Details: [environments.md](environments.md).

## 8. Options F and G

- **F.** `.\scripts\prd-tst\prd-tst-up.ps1` (or `.sh`) builds `qllm:local` in `k8s.io` and applies `deploy/prd-tst`. Do not run it together with Compose. See [environments.md](environments.md).
- **G.** Scoped-key demo with its own compose file: `deploy/prd/enforced/README.md` and [multi-user-safety.md](multi-user-safety.md).

## 9. After installing

Configuration is **files plus environment variables**. Changing YAML only needs a restart (or a Deployment rollout). Rebuild only when Go code changes.

Minimum to start serving:

1. A folder with `qllm.preset.yaml` and `qllm.catalog.yaml` ([from-scratch.md](from-scratch.md)). Without them `serve` fails with `CONFIG_ERROR`; there is no fallback to `fixtures/`.
2. The env vars named by the preset's `*Env` keys.
3. Check and start:

```bash
./qllm validate --config-dir ./my-project
./qllm serve --http --mcp-http --config-dir ./my-project
curl -s http://127.0.0.1:8088/v1/health
```

Defaults listen on `127.0.0.1:8088` (HTTP) and `127.0.0.1:8089` (MCP). All commands and flags: [cli.md](cli.md). Auth, bind, and CORS: [http-mcp.md](http-mcp.md).

## 10. Troubleshooting

| Symptom | Cause and fix |
|---------|---------------|
| SQL returns `UNSUPPORTED` mentioning DuckDB | Pure Go binary. Rebuild with `-tags duckdb` and CGO |
| `gcc not found at …` from `dev-shell.ps1` | Install the MSYS2 gcc package or fix `$MsysGccBin` |
| `Warning: duckdb.dll not found` | Put `duckdb.dll`, `duckdb.lib`, `duckdb.h` in `duckdblib/` |
| Build fails with `cannot find -lduckdb` or `duckdb.h` | `duckdblib/` incomplete, or you did not run `dev-shell.ps1` in this terminal |
| `qllm.exe` exits at start with a missing DLL | `duckdb.dll` is not on `PATH` or beside the exe |
| Linux/macOS: `gcc: command not found` | Install a C compiler |
| `CONFIG_ERROR` at `serve` | No `qllm.preset.yaml` / `qllm.catalog.yaml` in `--config-dir` or the CWD |
| Refuses to bind a non-loopback address | Set `serve.authTokenEnv` (and export the variable) or `--insecure-bind` for local tests only |
| Auth env set but server will not start | The variable named by `authTokenEnv` is empty |
| `ErrImagePull` in the simulation | Build or load the image in the `k8s.io` namespace |
| `refusing to overwrite …` from init-standalone | Folder exists. Choose another `--user` or pass `--force` |

Other build details (pinned `mcp-go`, rebuild rules): [build.md](build.md).
