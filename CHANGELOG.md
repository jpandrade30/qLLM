# Changelog

All notable changes to qLLM are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Protocol versions are the `protocolVersion` field (`planning/`). Runtime responses currently advertise **0.2.0**; **0.1.0** preset/catalog/IR files remain valid.

## [Unreleased]

### Added

- Experimental wire-compatible source types (same driver as the parent, no harness): MySQL family `mariadb`, `tidb`, `vitess`, `aurora_mysql`, `planetscale`; Postgres family `cockroach`, `yugabyte`, `alloydb`, `aurora_postgres`, `neon`, `supabase`, `timescale`, `redshift`.
- REST `getById` is executed when every `{name}` in the path has an `eq` filter. `list.itemsKey` (or resource-level) picks the JSON array key; `maxPages` / `pageSize` / `limitParam` / `offsetParam` walk offset pages (capped at 20).
- `scripts/standalone/init-standalone.py` (`.ps1` / `.sh` wrappers) writes a slim folder (`qllm-<user>`) with the Go runtime, blank SQLite YAML, and a Dockerfile so the project can be hosted without harness or docs.
- Experimental read-only `redis` and `kafka` sources (D19): Redis allowlists GET/HGETALL/LRANGE/SSCAN/ZRANGE/XRANGE; Kafka fetches without a consumer group or offset commit. Missing key/offset predicates return `UNSUPPORTED`. No harness.
- REST catalog field `fromFilter` (D20): when the API omits a key it already received as `eq` (for example `{"saldo":5300}`), qLLM fills that column so `GROUP BY` and joins work. Missing `eq` is `INVALID_IR`; a mismatched body value is `SOURCE_ERROR`.
- Scoped keys (D21): one `qllm.access.yaml` entry per app type; derived Bearer `app.user.expiry.hmac` (or a static `scope` map) forces `eq` on catalog `entities[].scope`. Conflicting filters return `FORBIDDEN_SCOPE` (`scopeMode: reject`, default). Query tools gain no extra field.
- Example stack `Dockerfile.enforced` + `docker-compose.enforced.yml` + `deploy/prd/enforced/` (Postgres seed, LangGraph agent that mints keys and opens an MCP session per `execute_sql` call).
- Catalog field `shape` (D22): optional free-text hint of the inner structure of a `type: json` field, shown in `describe_catalog`.
- Docs: `responses.md` (en/pt/es/zh) for the query envelope, column types, and parsed json cells.
- REST entity `api_profiles` (nested `address` / `tags` / `prefs`) plus SQL goldens `rest_json_*` and dataset `fixtures/datasets/v1/api_profiles.json`.
- `deploy/prd` split into `default/` (product image) and `enforced/` (scoped-key demo).

### Changed

- Root README is reorganized (why, how it works, connector table, quick start, configuration, serving, containers, behavior, development).
- Operator scripts live under `scripts/dev/`, `scripts/prd-tst/`, and `scripts/standalone/` (old flat `scripts/*.ps1` paths no longer exist).
- `scripts/dev/check-live.ps1` / `.sh` runs the SQL goldens against a live MCP (`-Filter rest_json` for nested JSON cases).
- Compose mounts `deploy/image/config` on `/config` so catalog/preset changes apply without rebuilding the `qllm` image.
- Enforced demo compose project is `qllm-enforced` (does not reuse harness container names). The LangGraph node is `format_answer` so it no longer collides with the `answer` state key.

### Fixed

- Kafka connector no longer sets `DisableAutoCommit` (invalid without a consumer group), which made `Open()` fail and kept qLLM from starting whenever a Kafka source was in the preset. Still no group, no commits, `read_committed`.
- Catalog SQL (`execute_sql` / `POST /v1/sql`) now applies entity `scope` on each source fetch (inject). Without this, a scoped key only constrained Query IR.
- Catalog SQL column types come from DuckDB (no longer every column as `string`). `json` cells are parsed objects/lists across SQL, REST, and Mongo; the response schema now allows array cells.

## [0.2.0] - 2026-09-30

### Added

- Docs (en/pt/es/zh): REST `options.resources` explained in detail (`list`, `getById`, `method`, `path`, `queryParams`, how queries map to HTTP, response shapes) and every `options` key with defaults; `getById` is documented as not called by the runtime. Root README gains a "Why qLLM" section.
- Experimental source types (no compose/goldens): `mssql`, `sqlite`, `clickhouse`, `dynamodb`, `cassandra`, `ksql` (pull only).
- Catalog `binding.accessPath` (`pk`/`partition`, `sk`/`sort`, `ksqlKey`). Queries without the required key equality return `UNSUPPORTED` (no Dynamo Scan / Cassandra `ALLOW FILTERING` / ksql `EMIT CHANGES`).
- `qllm.env.yaml` values may be exactly `${ENV_NAME}`; empty names are not written as the placeholder string. Compose injects harness secrets via `environment:`.
- SQL catalog path: CTE aliases, richer dialect `"2"` coverage in goldens (`fixtures/sqlcheck`).
- `scripts/prd-tst/prd-tst-up` / `prd-tst-down` (`.ps1` / `.sh`) to apply or remove the fleet-ops Kubernetes sim.
- `deploy/prd/` example YAML baked by the product `Dockerfile` (copy of harness shapes). Fleet-ops K8s sim lives in `deploy/prd-tst/`.
- POSIX twins: `scripts/*.sh` for seed, CGO shell, and `prd-tst-*` port-forward/Argo.
- `docs/` implementer manual in four languages (`docs/en`, `docs/pt`, `docs/es`, `docs/zh`): from-scratch, field reference, `Dockerfile` vs `.dev`, and more. The README links to each language.
- `execute_sql` logs a multiline block on stderr (status, queryId, SQL as written). MCP logs `how_to_use_me` / `describe_catalog` the same way.
- Cursor rules: keep `README.md` and this changelog in the same change set as user-visible work.

### Changed

- Runtime `protocolVersion` is **0.2.0**. Query IR shape is unchanged from 0.1.0.
- Go module toolchain requirement is **1.26** (deps). Image/harness still demo postgres, mysql, mongodb, REST only.
- Compose builds `Dockerfile.dev` (`deploy/image/config`). Default `Dockerfile` bakes `deploy/prd`.
- K8s sim scripts renamed `prd-tst-*.ps1` (old `prd-*.ps1` names removed).
- Production Go functions have Godoc comments.
- The manual moved from a flat `docs/` folder into per-language folders, with clearer English and Brazilian Portuguese text.

### Fixed

- Docker build image is `golang:1.26.6-bookworm` so `go mod download` matches `go.mod` (was 1.25 with `GOTOOLCHAIN=local`).
- PRD sim: `qllm:local` uses `imagePullPolicy: Never` so kubelet does not pull `docker.io/library/qllm:local`. Build with `nerdctl --namespace k8s.io`.

### Security

- Toolchain is **Go 1.26.6** (`go.mod` + `golang:1.26.6-bookworm`) so `govulncheck` stdlib findings on 1.26.0 (url/tls/http/x509/net/mail/xml/asn1) are closed. `golang.org/x/crypto` is **v0.56.0** (SSH DoS). Remaining module-only advisory GO-2026-5932 is `x/crypto/openpgp` (unmaintained, no fix; qLLM does not import it).
- Bearer compare uses HMAC-SHA256 + `hmac.Equal` (fixed-size digest; no `len` short-circuit). ACL lookup always compares against every app key.
- Demo tokens/passwords are not baked as literals in `deploy/image/config/qllm.env.yaml`; they come from process/compose env.

## [0.1.0] - 2026-09

Baseline shipped in this repo before the 0.2.0 source-type bump:

- Preset + logical catalog + JSON Query IR + HTTP `/v1` and MCP (`how_to_use_me`, `describe_catalog`, `execute_sql`).
- Connectors in the harness: postgres, mysql, mongodb, REST; DuckDB local join / `execute_sql` (`-tags duckdb`).
- `qllm.access.yaml`, SQL dialect `"1"`/`"2"`, catalog `introspect` / `from-openapi`, D17 (no GraphQL API), D18 (harness isolated from the binary).
