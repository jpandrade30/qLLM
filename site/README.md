# qLLM product site

Static English product pages for GitHub Pages (`https://jpandrade30.github.io/qLLM/`).

qLLM is **experimental**; every page footer (and the home hero) states that the software is provided as-is and the author accepts no responsibility for damage caused by its use — keep that aligned with the root README disclaimer.

## Edit

- HTML lives in this folder (`index.html`, `get-started.html`, `docs.html`, …). Primary nav order (all pages): Home → Get started → Configure → Connectors → Query → Security → Docs → Protocol → Decisions → Changelog → Contribute (keep in sync with `scripts/dev/render_site_md.py` `NAV_LINKS`).
- Shared CSS/JS, favicon (`StudyingCat.ico`), hero image, and Excalidraw-style `assets/flow-diagram.svg` (sketch under home **01 — What**): `assets/`.
- Links and asset paths are **relative** so the site works under the `/qLLM/` base path and when opened locally.
- Copy must stay faithful to the root `README.md` and `docs/en/` — do not invent HTTP/MCP APIs.
- `docs.html` is an index into the deep manuals on GitHub (`docs/en`, plus `planning/`). Keep that table in sync when guides are added or renamed; do not duplicate field-by-field content into HTML.
- `protocol.html` summarizes additive differences between `protocolVersion` **0.1.0** and **0.2.0**.
- `changelog.html` is generated from root `CHANGELOG.md` via `python scripts/dev/render_site_md.py` (also run in the Pages workflow). Re-run after editing the changelog.
- `configure.html` uses a side menu (overview / preset+catalog / config / env). Keep those field tables aligned with `docs/en/field-reference.md`.
- `decisions.html` explains product labels `D17`–`D22` in plain language. Prefer “decision D21” + link over a bare id. Always state units (`ms`, `rows`, `bytes`) next to numeric limits.
- `contribute.html` summarizes fork/PR + CoC; keep it aligned with root `CONTRIBUTING.md` and `CODE_OF_CONDUCT.md`.

## Design

Visual direction comes from the [Hallmark](https://www.usehallmark.com/) skill (Nutlope): Cobalt-inspired tokens + narrative-workflow macrostructure on the home page. Skill copy in the repo: `.cursor/skills/hallmark` (and `.agents/skills/hallmark` if installed via `npx skills add`).

## Deploy

Push to `main`/`master` with changes under `site/**` (or `.github/workflows/pages.yml`). Workflow uploads this folder as the Pages artifact.

One-time on GitHub: **Settings → Pages → Source → GitHub Actions**.
