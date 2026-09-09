# Releases: branching & versioning

Why the release process looks the way it does, how a build's version string is
put together, and the escape hatch for maintaining an older major line if that
need ever arrives. The operational step-by-step lives in
[AGENTS.md](../../AGENTS.md) under **Releases**; this note is the rationale.

## Model today

- Pull requests target **`develop`**. `develop` is the integration branch for
  the one line under active development.
- **`main`** is the release branch for that line. It moves only when a
  `develop` → `main` **promotion PR** is merged.
- The repo-root **`version`** file (`export VERSION=<x.y.z[-pre]>`) is the single
  source of truth for the version string, and it changes **only** in the
  promotion PR — nowhere else. `main`'s `version` therefore always equals the
  newest release; `develop` carries the last released value until the next
  promotion.
- A release is an annotated git tag `v<VERSION>` cut from `main`, plus the
  GitHub release and artifacts that `.github/workflows/release.yml` builds from
  that tag.

One `develop`, one `main`. That is the whole model in use.

## What a build reports

`regattaClock -v` and the app's **Version** window read `internal/version`,
whose fields are set at link time by `scripts/compile` (local `make build`) and
by `release.yml` (tag builds). A plain `go run` / `go build` sets none of them
and every field reads `dev`.

| Field | Example | Meaning |
|---|---|---|
| `version` | `0.4.1-alpha` | The release line the build sits on — the `version` file's value, verbatim. |
| `build` | `127.gc0ffee` | Commits since the last tag, then `g` + short commit. Empty when the build is an exact, clean tag. |
| `dirty` | `false` | The working tree had uncommitted changes at build time. |
| `full` | `0.4.1-alpha+127.gc0ffee` | Headline string: `version`, plus `+<build>` when past the tag, plus `.dirty` / `+dirty` for an unclean tree. Collapses to just `version` on a clean tag. |
| `branch`, `commit`, `date`, `user`, `source` | | Build branch, short SHA, UTC RFC 3339 timestamp, building user, and a "View on GitHub" URL to the built commit. |

`build` is semver `+build` metadata (`0.4.1-alpha+127.gc0ffee` is a valid semver
string), so tools that understand semver still sort it by the `version` part.

The count comes from `git rev-list --count <last tag>..HEAD`, so it answers "how
far past the last release is this?" even though the `version` file itself has
not moved.

### Known gaps

- A checkout with **no tags fetched** (a shallow CI clone, for instance) makes
  `commitsSinceTag` fall back to `0`, so `build` is empty and the build looks
  like a tag build. `release.yml` sidesteps this by passing `BuildCount=0`
  explicitly — a tag build is on the tag by construction.
- Between the promotion PR **merging** and the tag being **pushed**, a local
  `main` build counts from the *previous* tag, so `build`'s N is larger than the
  commits into the new line. It corrects itself once the tag exists.

## Versioning

Pre-1.0 (`0.x`), following semver's initial-development clause:

- **Major** stays `0`.
- **Minor** — any user-facing change since the last tag: a feature, a behaviour
  change, *or* a breaking change. A breaking change pre-1.0 is still a minor
  bump, but it must be called out in the release notes.
- **Patch** — bug fixes, docs, chore, tests, or CI only; nothing user-facing.

"Breaking" for this app means a change to:

- the finish-line operator workflow (start / lap / winning time / order-of-finish
  / save);
- an on-disk format read across sessions or by another operator — `data.json`,
  `regattaSchedule.json`, the persona files;
- exported PNG naming or layout;
- the `regattaClock -v` JSON shape.

The bump for a given release is chosen **ad-hoc at promotion time** by reviewing
the merged-PR titles since the last tag. There is no label convention or
changelog file; the release notes are written by hand from those PRs.

## Declaring 1.0

A deliberate future decision, **not** triggered by branching or by any single
feature. It will mean committing to stability of the operator workflow and the
on-disk formats — after 1.0, breaking either forces a major bump. Dropping the
`-alpha` / `-beta` suffix is usually the same decision, but need not be. The
exact bar is TBD and out of scope until there is a reason to set it.

## Maintained older lines (future — not in use)

Today `main` is the only release line, so there is nothing to maintain in
parallel. If `main` ever advances to a new major while the previous major still
needs bug fixes, the standard git-flow answer applies — a **release branch**,
created only when the need is real:

```text
today:                          if v0 later needs upkeep past v1:

  ●──●──●──●  main (v0 line)       ●──●──●──●──────●──●  main (v1 line)
        \                               \        (v1.0.0 tag) …
         ●──●──●  develop                ●──●──●  develop  (VERSION = 1.x)
                                          \
                                           ●──●  release/v0  (VERSION = 0.x)
                                              (v0.4.2, v0.4.3 tagged here)
```

- Cut `release/v<N>` from that line's **last tag** (`release/v0` from
  `v0.4.1-alpha`).
- Its patch releases are tagged from `release/v<N>`. Its `version` file carries
  `0.x.y`; `main` / `develop` carry `1.x.y`. No contention — they are different
  branches — and the "bump only in the promotion PR" rule is unchanged, with the
  promotion target generalised from `main` to that line's release branch.
- Fixes reach the old line by cherry-pick or a PR **targeting** `release/v<N>` —
  never a second long-lived `develop`. A maintenance line takes backported
  fixes, not parallel feature development.
- `main` always tracks the newest major line.

Why not set this up now: a `release/*` branch is a minutes-long operation when a
real need appears, and pre-creating parallel lines is upkeep — extra branches to
watch, build, and reason about — for zero present benefit.
