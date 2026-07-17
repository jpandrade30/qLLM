---
name: qllm-spec-change
description: >-
  Safely evolve qLLM planning docs and JSON Schemas with versioning and
  checklist discipline. Use when the user asks to change protocol decisions,
  add IR operators, alter connection shapes, or revise API responses.
---

# qLLM Spec Change

## Process

Copy and track:

```text
Spec change:
- [ ] Problem statement (why)
- [ ] Additive vs breaking
- [ ] Update planning/01-decisions.md if policy changes
- [ ] Update planning/03-protocol-schemas.md
- [ ] Update planning/schemas/*.schema.json
- [ ] Update 04-connectors.md if capabilities change
- [ ] Update 05/06 if harness/roadmap affected
- [ ] protocolVersion bump decision recorded
- [ ] List code touch points (do not code unless asked)
```

## Breaking examples

- Renaming IR fields (`from` → `source`)
- Changing row encoding (objects instead of arrays)
- Removing error codes
- Making secrets inline-only (disallowing env)

## Additive examples

- New compare op with connector support flags
- Optional `meta` fields
- New source `type` with new connection def

## Output to user

Summarize: what changed, version impact, migration notes for presets/catalogs, open questions.
