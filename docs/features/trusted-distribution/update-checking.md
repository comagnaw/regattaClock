# In-app update checking

Whether regattaClock should check GitHub for a newer release and tell the
operator, and separately, whether it should ever download and install that
release itself.

**Status:** idea captured, not designed in detail or scheduled. Companion to
[README.md](README.md) (the trust/signing posture this depends on) and
[../releases.md](../releases.md) (the version model a check compares against).

## Two very different scopes

- **Check + notify (small, no new trust dependency).** On startup (or from a
  menu item), `GET
  https://api.github.com/repos/comagnaw/regattaClock/releases/latest`
  (public, unauthenticated, no secret needed), compare the returned tag
  against `internal/version.Current.Version`
  ([`internal/version/version.go`](../../../internal/version/version.go)),
  and if newer, show a dialog with a link to the release page — the same
  `widget.NewHyperlink` pattern the Version window
  ([`internal/regatta/version_window.go`](../../../internal/regatta/version_window.go))
  already uses for "View on GitHub". Best-effort, off the startup path, never
  blocks: a failed check is silent or a single `WARN` log line, matching the
  non-disruption posture already established for other background network
  calls (see
  [`regattacentral-integration.md`](../personas/regattacentral-integration.md)'s
  "Non-disruption rules").
- **Download + install (deferred, real trust dependency).** Actually fetching
  and running a new build automatically is a different, much larger question.
  Windows cannot overwrite its own running `.exe`; macOS needs its own
  replace-the-`.app`-from-a-mounted-`.dmg` dance. More importantly: this
  project's binaries are **not yet code-signed**
  (see [README.md](README.md)'s "Where things stand" — SmartScreen and
  Gatekeeper both still flag a manual download today). An auto-updater that
  fetches and silently runs new unsigned code is a meaningfully different
  trust posture than a human clicking through a browser download and the
  same OS warnings — it's the same shape of risk this whole directory exists
  to close (C7's Mark-of-the-Web point, R4's "every artifact independently
  verifiable" requirement), not something to bolt on ahead of it.

## Recommendation

Only the check-and-notify half is worth scoping now, and only after (or
alongside) whatever else is already planned for the Version window / startup
flow — it has no dependency on the signing roadmap. Download-and-install
should not be attempted before code signing (this directory's "Now" phase)
lands; even then, it deserves its own design pass (checksum verification
against `SHA256SUMS`, at minimum, since a compromised or MITM'd GitHub API
response is the exact failure mode an auto-updater must not trust blindly).

## Existing-code reuse analysis

- `internal/version.Current` / `Current.Version` — already the comparison
  target; no new version-parsing needed, it's a plain semver string
  (`0.6.2-alpha`) matching the tag format `release.yml` already produces.
- `internal/regatta/version_window.go`'s hyperlink pattern — direct template
  for surfacing the "a new version is available" link.
- The non-disruption rules already codified in
  [`regattacentral-integration.md`](../personas/regattacentral-integration.md)
  (own goroutine/context, `fyne.Do`, best-effort, never blocks a click) — the
  same posture applies to a GitHub API call as to an RC one.
- `SHA256SUMS` (already published per release, [ci-and-provenance.md](ci-and-provenance.md))
  — the integrity check any future download-and-install phase would need to
  verify against before running anything it fetched.

## Open questions (not resolved here)

- Check on every launch, or only when the operator asks (a menu item)? A
  background check on every launch of a regatta-day timing app is worth
  weighing against R1-style "no surprises during the event" caution even for
  the notify-only version — likely only ever a passive, dismissible notice,
  never a blocking dialog, and possibly disabled entirely once a session is
  bound to a regatta.
- Where the "don't check again for version X" / "checked at time T"
  preference lives, if any (Fyne `Preferences`, matching everything else per
  `AGENTS.md`).
- Pre-release (`-alpha`) tags: does "latest" mean GitHub's own
  `releases/latest` (which by GitHub's own semantics skips pre-releases —
  every tag this project has cut so far is a pre-release, so
  `releases/latest` would currently return **nothing**), or does the check
  need `GET .../releases` and take the first entry instead? This needs
  resolving before implementation, not assumed.
