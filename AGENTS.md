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

## Text styling

`internal/text` is the single home for UI type styling. Every `widget.Label` or
`canvas.Text` that sets `TextStyle`, `Alignment`, `Truncation`, `Wrapping`, or
`TextSize` — `widget.NewLabelWithStyle` included — comes from a factory there
(`BoldLabel`, `Header3`, `Truncating`, `Note`, `Wrapping`, …), so the app keeps
one consistent fit and finish and one place to change a given style.

- Do not hand-roll a styled label or text in a feature package
  (`internal/regatta`, `internal/clock`, …), and do not add a local
  `*widget.Label`-returning helper. If `internal/text` lacks the style you need,
  add the factory there — with a test, following the existing `Header*` /
  `*Label` / `Truncating*` naming — then call it.
- A bare `widget.NewLabel(text)` with no styling is fine inline; the rule is
  about *styled* text. Setting `Importance` on a label is also fine inline —
  importance is per-use, not type styling.
- Keep the user-facing string itself in `internal/common` and pass it to the
  factory; `internal/text` never hard-codes copy.

## Releases

A **release** is a point-in-time compiled version of regattaClock, announced by an
annotated git **tag** (semantic version) and a **GitHub release**.

The version string lives in the repo-root **`version`** file
(`export VERSION=<x.y.z[-pre]>`) — the single source of truth. It is:

- read by the `Makefile` (`include version`) and `scripts/compile`, which compile
  it into the binary via `go build -ldflags -X` alongside the build branch,
  commit, timestamp, and the source URL (assembled from `GH_HOST`/`ORG`/`REPO`);
- shown by `regattaClock -v` / `-version` (indented JSON of `internal/version`'s
  `Current` struct) and by the app menu's **Version** item (a `key: value` window
  with a "View on GitHub" link to the built commit);
- `dev` for every attribute in a plain `go run` / `go build` — the `-ldflags` are
  only applied by `scripts/compile` and `release.yml`.

The git **tag** is `v` + the `version` file's `VERSION`; the two must match.

Releases are cut from **`main`**. When asked to generate a release:

1. **Confirm the release point.** `git fetch origin`, then
   `git log --first-parent --oneline origin/main..origin/develop`. If `main` is behind
   `develop`, tell the team and ask whether to release from `main` as it stands or wait
   for a `develop` → `main` merge. **Do not merge `develop` into `main` yourself unless
   explicitly told to.**

2. **Find the last tag and the changes since it.**
   `git tag --list 'v*' --sort=-v:refname | head -1`, then
   `git log --first-parent --oneline <lastTag>..origin/main` (and
   `gh pr list --state merged --base main --limit 50` for titles).

3. **Choose the version** — `v<major>.<minor>.<patch>[-<pre>]`, semver.
   - **Major stays `0`.** Bump it only when the team explicitly asks for a major
     release; pre-1.0, even a breaking change is a minor bump.
   - **Minor** (`v0.4.x` → `v0.5.0`) — any new feature or user-facing behaviour change
     since the last tag. The usual bump.
   - **Patch** (`v0.4.1` → `v0.4.2`) — only bug fixes, docs, chore, tests, or CI since
     the last tag; no user-facing change.
   - The chosen `x.y.z[-pre]` (no `v`) is written to the repo-root `version` file
     **only in the `develop` → `main` promotion PR** — nowhere else does that
     file change. `main`'s `version` therefore always equals the newest release;
     `develop` keeps the last released value until the next promotion, so a dev
     build's `-v` shows that version with a newer `commit`/`branch`.

4. **Choose the pre-release suffix.** If the previous tag ends in `-alpha` or `-beta`,
   ask the team whether the new tag keeps that suffix, advances it (`-alpha` → `-beta`),
   or drops it for a stable `v0.x.y`. Default: keep the previous suffix (every tag so
   far is `-alpha`).

5. **Draft the notes** — a short, summarised **bullet list of changes since the last
   tag**, grouped by area (Personas, Finish Timer, Docs, CI, …), written from the merged
   PRs. A human summary, not a raw `git log` dump. These go in the GitHub release body
   only; there is no changelog file.

6. **Get explicit sign-off** on the version, pre-release status, and notes before
   tagging — pushing the tag publishes a GitHub release and builds artifacts, an
   outward-facing action.

7. **Tag and push** (annotated, one-line message in the terse style of the existing
   tags — the bullets live in the GitHub release):

   ```sh
   git checkout main && git pull origin main
   . ./version && [ "v$VERSION" = "<version>" ] || { echo "version file != tag"; exit 1; }
   git tag -a <version> -m "<one-line summary>"
   git push origin <version>
   ```

   The `version`-file check guards against tagging before the `develop` → `main`
   promotion PR bumped the file. A published tag is not moved; a wrong version
   means a new tag.

8. **Let `release.yml` run** (it triggers on the `v*` tag push):
   `gh run watch` / `gh run list --workflow=release.yml`. It builds
   `regattaClock.dmg`, `regattaClock-amd64.exe.zip`, `regattaClock-386.exe.zip` and
   **auto-creates the GitHub release** — title `Release <version>`, **empty body**, and
   **not marked pre-release** whatever the suffix. Wait for **both** jobs (`build-macos`,
   then `build-windows`) to finish. If a job fails, report it and stop — do not retag.

9. **Finalise the GitHub release** (the workflow does not):
   - `gh release edit <version> --notes-file <notes>` — add the bullet-point notes.
   - If it should be a pre-release: `gh release edit <version> --prerelease` (the
     workflow forced `prerelease: false`).
   - `gh release view <version>` — confirm the three assets are attached.

10. **Report** the tag, the reason for that bump, the pre-release status, and the
    release URL.

`test.yml` does not run on tags, so make sure `main` is green before tagging. If a
`FyneApp.toml` / `-app-version` is added later (see
`docs/features/trusted-distribution/windows-packaging.md`), it is a separate
OS-level version resource — derive it from the `version` file too, never
hand-edit it independently.
