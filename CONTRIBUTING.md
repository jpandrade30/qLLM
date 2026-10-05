# Contributing to qLLM

Thanks for helping improve qLLM. By participating you agree to follow the
[Code of Conduct](CODE_OF_CONDUCT.md).

This repository is a **portable query runtime** (Go): YAML preset + logical
catalog, Query IR / catalog SQL, HTTP `/v1` and MCP. Contributions should keep
that product shape honest — no invented APIs, no GraphQL agent surface, no
warehouse ambitions.

## Before you start

1. Read the product overview and [Decisions](https://jpandrade30.github.io/qLLM/decisions.html)
   (`D17`–`D22`) so labels in docs mean something.
2. For operators and implementers, prefer the Release zip over cloning just to
   *run* qLLM. Clone this repo when you intend to **change** code, specs, harness,
   or docs.
3. Skim [`planning/`](planning/) — contracts live there. If prose and schemas
   disagree, **`planning/` wins**.

## Ways to contribute

| Kind | Where to work | Notes |
|------|---------------|--------|
| Bug fix / connector fix | `internal/`, tests | Prefer a failing test or golden when possible |
| Protocol / schema change | `planning/` **first**, then code | See [Contract changes](#contract-changes) |
| Docs (implementer manuals) | `docs/{en,pt,es,zh}/` | Keep languages in sync when you change meaning |
| Product site | `site/` | English only; deep manuals stay in `docs/` |
| Harness / goldens | `fixtures/`, compose, `scripts/dev/` | Rancher Desktop + **nerdctl compose** |

Out of scope for drive-by PRs unless discussed in an issue first: Python/Node
SDKs, custom domains for Pages, turning the agent API into GraphQL, or adding
slow “warehouse” batch semantics.

## Development setup

- Go **1.26.6+** (see `go.mod` / `toolchain`).
- Optional DuckDB path: CGO + `-tags duckdb` (see root `README.md`).
- Local harness: Rancher Desktop (containerd) + `nerdctl compose` — see
  [`docs/en/environments.md`](docs/en/environments.md).

```bash
git clone https://github.com/<you>/qLLM.git
cd qLLM
go vet ./...
go test ./...
```

CI also runs `go test -tags duckdb` on Linux and builds the slim standalone
folder. Match that locally when you touch DuckDB or release packaging.

## Fork and pull request workflow

1. **Fork** [jpandrade30/qLLM](https://github.com/jpandrade30/qLLM) on GitHub.
2. **Clone** your fork and add upstream:

   ```bash
   git remote add upstream https://github.com/jpandrade30/qLLM.git
   git fetch upstream
   git checkout -b topic/short-description
   ```

3. Make a **focused** change. Prefer small PRs over mixed refactors + features.
4. Keep the branch updated:

   ```bash
   git fetch upstream
   git rebase upstream/main   # or merge, if you prefer
   ```

5. Push to your fork and open a **Pull Request** against `main` (or `master` if
   that is the default).
6. Fill the PR description with:
   - **Why** (problem / goal)
   - **What** changed (user-visible behavior)
   - **How tested** (`go test`, harness, manual curls)
7. Respond to review comments; do not force-push over review history unless the
   maintainer asks for a clean rebase.

### PR checklist

- [ ] Change matches product rules (read-only default, limits, no invented HTTP/MCP shapes)
- [ ] Contract edits updated `planning/` (and schemas) **before** or **with** code
- [ ] [`CHANGELOG.md`](CHANGELOG.md) `[Unreleased]` updated for user-visible or contract changes
- [ ] [`README.md`](README.md) updated when operators/agents would notice (flags, auth, connectors, Pages, protocol)
- [ ] Docs: if you change meaning in `docs/en/`, update other language folders or note the lag in the PR
- [ ] Site: if you add a guide, update `site/docs.html` / field tables as needed; keep units (`ms`, `rows`, `bytes`) explicit
- [ ] Decision labels: prefer “decision D21” + link to Decisions / `planning/01-decisions.md`, not a bare `D##`
- [ ] No secrets, compose demo passwords, or real tokens in the PR

## Contract changes

Changing Query IR, response envelopes, preset/catalog shapes, or connector
capabilities is a **spec change**:

1. State the problem and whether the change is additive or breaking.
2. Update `planning/01-decisions.md` if policy changes.
3. Update `planning/03-protocol-schemas.md` and `planning/schemas/*.schema.json`.
4. Update `planning/04-connectors.md` when capabilities change.
5. Record any `protocolVersion` bump decision.
6. Then implement code and tests.

Do not ship code that contradicts frozen schemas “for convenience.”

## Code and docs style

- **Go:** match neighboring packages; keep connectors read-oriented; fail with
  typed errors (`TIMEOUT`, `UNSUPPORTED`, `CONFIG_ERROR`, …).
- **YAML examples:** `*Env` holds the **name** of an environment variable, never
  the secret. Prefer non-CRM examples when teaching configuration.
- **Units:** always state dimension next to numbers (`15000 ms`, `100 rows`,
  `1048576 bytes`).
- **Commits:** clear why; Conventional Commits welcome but not required.
- **AI-assisted PRs:** you are still responsible for correctness, secrets, and
  license compliance. Do not paste proprietary data into public issues.

## Reporting bugs and security

- **Bugs / features:** [GitHub Issues](https://github.com/jpandrade30/qLLM/issues).
  Include qLLM version, OS, minimal preset/catalog snippets (redact secrets), and
  the typed error code when relevant.
- **Security:** use
  [GitHub Security Advisories](https://github.com/jpandrade30/qLLM/security/advisories/new)
  — do not open a public issue for vulnerabilities.

## License

Contributions are accepted under the same terms as the project license — see
[`LICENSE.md`](LICENSE.md).
