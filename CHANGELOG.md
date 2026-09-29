# Changelog

All notable changes to qLLM are documented here.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
Protocol versions are the `protocolVersion` field (`planning/`). Runtime responses currently advertise **0.2.0**; **0.1.0** preset/catalog/IR files remain valid.

## [Unreleased]

## [0.2.0] - 2026-09-29

### Added

- Experimental source types (no compose/goldens): `mssql`, `sqlite`, `clickhouse`, `dynamodb`, `cassandra`, `ksql` (pull only).
- Catalog `binding.accessPath` (`pk`/`partition`, `sk`/`sort`, `ksqlKey`). Queries without the required key equality return `UNSUPPORTED` (no Dynamo Scan / Cassandra `ALLOW FILTERING` / ksql `EMIT CHANGES`).
- `qllm.env.yaml` values may be exactly `${ENV_NAME}`; empty names are not written as the placeholder string. Compose injects harness secrets via `environment:`.
- SQL catalog path: CTE aliases, richer dialect `"2"` coverage in goldens (`fixtures/sqlcheck`).
- Cursor rules: keep `README.md` and this changelog in the same change set as user-visible work.

### Changed

- Runtime `protocolVersion` is **0.2.0**. Query IR shape is unchanged from 0.1.0.
- Go module toolchain requirement is **1.26** (deps). Image/harness still demo postgres, mysql, mongodb, REST only.

### Security

- Bearer compare uses HMAC-SHA256 + `hmac.Equal` (fixed-size digest; no `len` short-circuit). ACL lookup always compares against every app key.
- Demo tokens/passwords are not baked as literals in `deploy/image/config/qllm.env.yaml`; they come from process/compose env.

## [0.1.0] - 2026-09

Baseline shipped in this repo before the 0.2.0 source-type bump:

- Preset + logical catalog + JSON Query IR + HTTP `/v1` and MCP (`how_to_use_me`, `describe_catalog`, `execute_sql`).
- Connectors in the harness: postgres, mysql, mongodb, REST; DuckDB local join / `execute_sql` (`-tags duckdb`).
- `qllm.access.yaml`, SQL dialect `"1"`/`"2"`, catalog `introspect` / `from-openapi`, D17 (no GraphQL API), D18 (harness isolated from the binary).
