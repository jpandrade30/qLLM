# Changelog

All notable changes to qLLM are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Protocol versions are the `protocolVersion` field (`planning/`). Runtime responses advertise **0.2.0**; **0.1.0** preset/catalog/IR files remain valid. Latest product release: **0.3.5**.

## [Unreleased]

### Added

- Link to the end-to-end demo repo [jpandrade30/SmallDemo](https://github.com/jpandrade30/SmallDemo) from the README and product site (home, Get started, Docs).
- Home page embeds the SmallDemo walkthrough video ([YouTube](https://www.youtube.com/watch?v=1rozeLOgUE0)).

## [0.3.5] - 2026-10-04

### Added

- Explicit experimental / no-liability disclaimer in the root README and on the product site (home note + footer on every page).

## [0.3.4] - 2026-10-04

### Changed

- Product site primary nav follows a reader journey: Get started → Configure → Connectors → Query → Security → Docs → Protocol → Decisions → Changelog → Contribute.

## [0.3.3] - 2026-10-04

### Added

- GitHub Pages product site (`site/`, English): overview, get started, connectors, configure (side menu for preset/catalog/config/env), query, security, Docs index, Decisions (plain-language D17–D22), and Contribute — deployed via Actions to https://jpandrade30.github.io/qLLM/
- Site pages for protocol **0.1.0 → 0.2.0** (`protocol.html`) and a generated **Changelog** (`changelog.html` from root `CHANGELOG.md` via `scripts/dev/render_site_md.py`, also in the Pages workflow).
- Docs clarify units on time/size/row limits (`ms`, `bytes`, `rows`) in `docs/en/field-reference.md` and `project-files.md`; decision ids point at `planning/01-decisions.md` instead of bare `D##` shorthand.
- `CONTRIBUTING.md` and `CODE_OF_CONDUCT.md` (Contributor Covenant 2.1): fork/PR workflow, contract-change checklist, changelog/README expectations.
- Standalone generator (`scripts/standalone/init-standalone.py`) copies `planning/` into the slim folder so LLMs/humans can author config against the contract; Get started / install docs stress that `config/` and monorepo `deploy/` are examples to replace.

### Changed

- Home-page flow sketch sits inside **01 — What** as the example diagram (no longer a separate band above that stage).
- Documentation type labels for `protocolVersion` say **No version** instead of “semver” (field-reference / from-scratch in en/pt/es/zh, plus planning D11 / protocol field table).

### Fixed

- Standalone `config/qllm.env.yaml` template now uses the required root `env:` map.

## [0.3.2] - 2026-10-04

### Added

- Experimental source type `graphql` (no harness): HTTP POST to `baseUrlEnv`, catalog binding `graphql_operation` + `options.operations.<name>` (`document`, `itemsPath`, optional `variables` / `limitVariable`). Documents must be GraphQL **`query` only** — `mutation` / `subscription` and write keywords (`INSERT`, `UPDATE`, `DELETE`, …) fail at open/fetch with `CONFIG_ERROR` before any HTTP. Not a GraphQL agent API (D17 unchanged).

### Changed

- Docs and root README Quick start recommend downloading the GitHub Release **`qllm-standalone-<ver>.zip`** instead of cloning the monorepo to run qLLM.

## [0.3.1] - 2026-10-03

### Added

- GitHub Actions CI (`go vet` / `go test`, `-tags duckdb` job, generate + build + validate the slim folder).
- GitHub Release workflow on tags `v*`: builds `qllm-standalone-<ver>.zip` from `scripts/standalone/init-standalone.py` (LICENSE included) and attaches it using the matching `CHANGELOG.md` section as the release body. Release notes warn that GitHub’s automatic **Source code** archives are the full monorepo — use only the `qllm-standalone-*.zip` asset.

### Fixed

- DuckDB dialect unit tests: parenthesize `UNION ALL` arms that use `LIMIT`, use a window in `QUALIFY`, and express boolean XOR as `<>` (DuckDB has no boolean `XOR` / `xor(bool,bool)`).

## [0.3.0] - 2026-10-03

Product release. Protocol stays **0.2.0** (additive: D19–D22, extra `sources[].type` values).

### Added

- Source `type` values in this release (harness still only postgres / mysql / mongodb / rest):
  - Stable: `postgres`, `mysql`, `mongodb`, `rest`.
  - Experimental (in the binary, no compose/goldens): `mssql`, `sqlite`, `clickhouse`, `dynamodb`, `cassandra`, `ksql` (from 0.2.0), plus **`redis`** and **`kafka`** (D19).
  - Experimental wire aliases (same driver and connection as the parent): MySQL family `mariadb`, `tidb`, `vitess`, `aurora_mysql`, `planetscale`; Postgres family `cockroach`, `yugabyte`, `alloydb`, `aurora_postgres`, `neon`, `supabase`, `timescale`, `redshift`.
- Redis (D19): `binding.kind: key` + `keyPattern`; allowlisted GET/HGETALL/LRANGE/SSCAN/ZRANGE/XRANGE only. Missing key predicate is `UNSUPPORTED`. Never deletes, pops, or `KEYS`.
- Kafka (D19): `binding.kind: topic`; fetch without a consumer group or offset commit (`read_committed`). Missing partition/offset, key, or time predicate is `UNSUPPORTED`. Never produces.
- REST `getById` runs when every `{name}` in the path has an `eq` filter. `list.itemsKey` (or resource-level) picks the JSON array key; `maxPages` / `pageSize` / `limitParam` / `offsetParam` walk offset pages (capped at 20).
- REST catalog field `fromFilter` (D20): when the API omits a key it already received as `eq`, qLLM fills that column so `GROUP BY` and joins work. Missing `eq` is `INVALID_IR`; a mismatched body value is `SOURCE_ERROR`.
- Catalog field `shape` (D22): optional free-text hint of the inner structure of a `type: json` field, shown in `describe_catalog`.
- Parsed `json` cells (objects and arrays) in query responses; SQL column types come from DuckDB instead of labeling every column `string`.
- Scoped keys (D21): one `qllm.access.yaml` entry per app type; derived Bearer `app.user.expiry.hmac` (or a static `scope` map) forces `eq` on catalog `entities[].scope`. Conflicting Query IR filters return `FORBIDDEN_SCOPE` (`scopeMode: reject`, default). Query tools gain no extra field.
- Example stack `Dockerfile.enforced` + `docker-compose.enforced.yml` + `deploy/prd/enforced/` (Postgres seed, LangGraph agent that mints keys and opens an MCP session per `execute_sql`).
- `deploy/prd` split into `default/` (product image) and `enforced/` (scoped-key demo).
- REST entity `api_profiles` (nested `address` / `tags` / `prefs`) plus SQL goldens `rest_json_*` and dataset `fixtures/datasets/v1/api_profiles.json`.
- Docs: `responses.md` (en/pt/es/zh) for the query envelope, column types, and parsed json cells.
- `scripts/standalone/init-standalone.py` (`.ps1` / `.sh`) writes a slim folder (`qllm-<user>`) with the Go runtime, blank SQLite YAML, and a Dockerfile.
- `scripts/dev/check-live.ps1` / `.sh` runs SQL goldens against a live MCP (`-Filter rest_json` for nested JSON).

### Changed

- Root README is reorganized (why, how it works, connector table, quick start, configuration, serving, containers, behavior, development).
- Operator scripts live under `scripts/dev/`, `scripts/prd-tst/`, and `scripts/standalone/` (old flat `scripts/*.ps1` paths no longer exist).
- Compose mounts `deploy/image/config` on `/config` and `fixtures/test-api/data.json` on the fake API so catalog/seed changes apply without rebuilding those images.
- Enforced demo compose project is `qllm-enforced` (does not reuse harness container names). The LangGraph node is `format_answer` so it no longer collides with the `answer` state key.

### Fixed

- Kafka connector no longer sets `DisableAutoCommit` (invalid without a consumer group), which made `Open()` fail whenever a Kafka source was in the preset. Still no group, no commits.
- Catalog SQL (`execute_sql` / `POST /v1/sql`) applies entity `scope` on each source fetch (inject). A spoof `WHERE user_id = '…'` does not leak other users' rows.

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
