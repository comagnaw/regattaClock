# AGENTS.md

Guidance for AI coding agents working in this repository. Applies to the whole
repo; see also `CLAUDE.md` (local, not committed) for any machine-specific notes.

## Markdown

Run **markdownlint** on every Markdown file you create or change, and fix (or
consciously suppress) what it reports, **before publishing** — before you commit
or open a PR.

- Tool: `npx -y markdownlint-cli2 "<changed files>"` — e.g.
  `npx -y markdownlint-cli2 "docs/**/*.md" "*.md"`. No repo install; the first run
  fetches `markdownlint-cli2` and needs network.
- Config: `.markdownlint-cli2.jsonc` at the repo root is authoritative. It turns
  off line-length (`MD013`) and table-cell padding (`MD060`), allows tabs inside
  fenced code (`MD010` `code_blocks: false`), scopes duplicate-heading checks to
  siblings, and allows bold lead-in lines (`MD036`). Everything else is default.
- A finding you deliberately keep must be suppressed narrowly — an inline
  `<!-- markdownlint-disable-next-line MDxxx -->` with a reason, not a blanket
  rule change. Changing `.markdownlint-cli2.jsonc` is a deliberate, explained edit.
- `docs/features/**/*.md` is expected to lint clean; keep it that way.
