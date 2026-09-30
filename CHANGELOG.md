# Changelog

All notable changes to qLLM are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Protocol versions are the `protocolVersion` field (`planning/`). Runtime responses currently advertise **0.2.0**; **0.1.0** preset/catalog/IR files remain valid.

## [0.2.0] - 2026-09-30

### Added

- Docs (en/pt/es/zh): REST `options.resources` explained in detail (`list`, `getById`, `method`, `path`, `queryParams`, how queries map to HTTP, response shapes) and every `options` key with defaults; `getById` is documented as not called by the runtime. Root README gains a "Why qLLM" section.
- Experimental source types (no compose/goldens): `mssql`, `sqlite`, `clickhouse`, `dynamodb`, `cassandra`, `ksql` (pull only).
- Catalog `binding.accessPath` (`pk`/`partition`, `sk`/`sort`, `ksqlKey`). Queries without the required key equality return `UNSUPPORTED` (no Dynamo Scan / Cassandra `ALLOW FILTERING` / ksql `EMIT CHANGES`).
- `qllm.env.yaml` values may be exactly `${ENV_NAME}`; empty names are not written as the placeholder string. Compose injects harness secrets via `environment:`.
- SQL catalog path: CTE aliases, richer dialect `"2"` coverage in goldens (`fixtures/sqlcheck`).
- `scripts/prd-tst-up` / `prd-tst-down` (`.ps1` / `.sh`) to apply or remove the fleet-ops Kubernetes sim.
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
