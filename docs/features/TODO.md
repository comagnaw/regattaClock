# Feature backlog

Measurable follow-ups already captured across `docs/features/**`, grouped by
theme. This file only tracks work that is already written down somewhere — new
ideas belong in the relevant design doc first, then here.

## Trusted distribution — Windows

- [ ] Obtain a Windows code-signing certificate — one self-signed cert ($0) or an Azure Trusted Signing **Private Trust** account (~$10/mo, no CI key) — [windows-internal-pki.md](trusted-distribution/windows-internal-pki.md#recommendation)
- [ ] Activate the `sign-windows` job — `signtool` / `osslsigncode` (self-signed) or `azure/trusted-signing-action` + OIDC — RFC 3161 timestamped, signing the exe then the installer built around it — [ci-and-provenance.md](trusted-distribution/ci-and-provenance.md#secrets-vs-oidc)
- [ ] Push the cert to the managed fleet via Intune — a Trusted-certificate profile for the root plus a platform script for the Trusted Publishers store; deliver releases from an internal share / Intune so Mark-of-the-Web never engages — [windows-internal-pki.md](trusted-distribution/windows-internal-pki.md#deploying-trust-to-the-fleet)
- [ ] Add a WiX / MSI build alongside the Inno installer for silent `msiexec /qn` deployment via GPO / Intune / SCCM — [windows-packaging.md](trusted-distribution/windows-packaging.md)
- [ ] Native Windows ARM64 build — today's release ships amd64 only; an ARM-based Windows machine (e.g. Surface Pro X, Surface Pro 9 5G/11 with Snapdragon) falls back to Windows' x64 emulation, untested with this app. Not pursued unless real demand shows up (no regatta hardware runs ARM today) — [README.md](../README.md#downloading-and-verifying)

## Trusted distribution — macOS

- [ ] Verify `fyne package -release` already ad-hoc signs the binary; add `codesign -s - --deep` if not — [macos-notarization.md](trusted-distribution/macos-notarization.md)
- [ ] Apple Developer Program ($99/yr): Developer ID Application cert → hardened runtime → `notarytool submit --wait` → `stapler staple` on the universal binary, `.app`, and `.dmg`; add a secret-gated `sign-macos` job to the `macos-latest` job — [macos-notarization.md](trusted-distribution/macos-notarization.md), [ci-and-provenance.md](trusted-distribution/ci-and-provenance.md#restructured-release-workflow)
- [ ] Publish a Homebrew cask once the DMG is notarized — [macos-notarization.md](trusted-distribution/macos-notarization.md#distribution)

## Trusted distribution — public trust (unmanaged machines)

- [ ] Apply to the SignPath Foundation OSS code-signing program (free; key stays out of CI; expect a review) — [windows-public-trust.md](trusted-distribution/windows-public-trust.md#recommendation)
- [ ] Fallback: add an Azure Trusted Signing **Public Trust** validation (~$120/yr, OIDC) if SignPath is declined or Private Trust is already in use — [windows-public-trust.md](trusted-distribution/windows-public-trust.md#recommendation)

## Release pipeline & CI

- [ ] `check-secrets` job that maps `secrets.*` into outputs so every `sign-*` job's `if:` **skips** (not fails) on fork / PR runs — [ci-and-provenance.md](trusted-distribution/ci-and-provenance.md#fork--pr-safety-c9)
- [ ] Produce an SBOM per release (`cyclonedx-gomod` or `syft`) and attach it to the GitHub release — [ci-and-provenance.md](trusted-distribution/ci-and-provenance.md)
- [ ] In-app "check for updates" — notify-and-link (GitHub releases API + `internal/version.Current.Version` compare, a dialog linking to the release page) is small and independent; auto-download-and-install is deferred until code signing lands (an unsigned auto-updater fetching and running new code is the same trust problem the rest of trusted-distribution/ exists to close). Note: GitHub's `releases/latest` endpoint skips pre-releases, and every tag so far is `-alpha` — the check needs the full `releases` list, not `/latest` — [update-checking.md](trusted-distribution/update-checking.md)

## Testing — integration lane (proposed, not built)

- [ ] `//go:build integration` lane under `test/integration/` with a `spawnPersonas(t, root, defs…)` helper; default `go test ./...` skips it — [integration-testing.md](testing/integration-testing.md#mechanics)
- [ ] Scenario: multi-writer round-trip — two persona sessions writing `start.json` / `finish.json` into one root while a Director watcher runs — [integration-testing.md](testing/integration-testing.md#scenarios)
- [ ] Scenario: atomic rename vs a held handle — `SaveJSONFileAtomic` racing a reader that holds the target open (Linux + Windows)
- [ ] Scenario: `filepath.FromSlash` on a real Fyne `file:///C:/…` URI through the folder-pick callbacks
- [ ] Scenario: mtime goes backwards on a watched file — older stamp with identical, then changed, bytes
- [ ] Scenario: schedule change while a clock is open — the stale-lane-map mark, the schedule-conflict banner, and the flagged committed result, end to end
- [ ] Add the integration CI job to `test.yml` — `ubuntu-latest` + `windows-latest` matrix, `push` / nightly `schedule` / `workflow_dispatch`, **not** `pull_request` — [integration-testing.md](testing/integration-testing.md#ci-lane)

## Releases & versioning

- [ ] Define the criteria for declaring **1.0** — a deliberate commitment to freeze the operator workflow and the on-disk formats; usually drops the `-alpha` suffix — [releases.md](releases.md#declaring-10)
- [ ] `release/v<N>` maintained-line branch procedure — cut only if an older major genuinely needs upkeep past a newer major — [releases.md](releases.md)
- [ ] Close the build-stamp gaps — empty `build` on a tag-less shallow clone; inflated `build` count on a local `main` build between the promotion merge and the tag — [releases.md](releases.md#known-gaps)

## Personas — feature follow-ups

The multi-persona operating model is largely built (`persona-plan.md` phases
0–8d), plus the clock-window visual pass and the primary FT's read-only
**Compare Secondary** window; the RD tree was simplified back to primary-team
values only (the phase 8b-2 per-value `·2nd` fallback was removed). Captured
remaining work:

- [ ] Local write-ahead journal — collect each value to a local file first, then flush to the shared path, so an SMB outage or cloud stall never blocks collection — [persona-plan.md](personas/persona-plan.md#13-open-items), [shared-storage-options.md](personas/shared-storage-options.md)
- [ ] `ScheduleOrigin` interface (`Fingerprint()` / `Load()`) generalising the Excel reader, with a later in-app-authored heat sheet (`Origin.Type = "heatsheet"`) built from the RegattaCentral roster and uploaded back to RegattaCentral, via `regattaDataFromSchedule` / `reader.NewRegattaData` (one entry point, no forked constructor) — [persona-plan.md](personas/persona-plan.md#3b-schedule-origin-refresh-rd-only), [schedule-data-model.md](personas/closed/schedule-data-model.md), [regattacentral-integration.md](personas/regattacentral-integration.md#phase-c--in-app-heat-sheet-authoring)
- [ ] Route exporter and persona-derived paths through `sanitizeForFilename` for Windows-reserved names and device names — [persona-plan.md](personas/persona-plan.md#12-windows-storage-modes-cloud-synced-folder-and-local-smb)
- [ ] Timer-side staleness indicator in the race tree — "start times last updated N s ago" from the watcher's last-change time — [persona-plan.md](personas/persona-plan.md#12-windows-storage-modes-cloud-synced-folder-and-local-smb)
- [ ] Resolve the remaining open items — cross-team FT→ST start fallback (with explicit confirmation) and an RD override for an ST locked on the wrong race — [persona-plan.md](personas/persona-plan.md#13-open-items)
- [ ] Optional niceties — a user-initiated `w32tm /resync` button in Director config; size-based log rotation plus a Director "collect logs" (clipboard / zip) action; remote syslog export behind a second preference — [persona-plan.md](personas/persona-plan.md#13-open-items), [logging-options.md](personas/logging-options.md)
- [ ] Windows folder-picker hang on a stale/disconnected mapped network drive — `dialog.NewFolderOpen`'s sidebar enumerates every drive letter and calls `os.Stat` on each before showing, which can hang indefinitely. Current mitigation is operator-side (README.md's Troubleshooting section); investigate a Fyne version bump that might fix this upstream, or a custom enumeration-free directory picker, before this becomes an app-level fix — [README.md](../README.md#troubleshooting)
- [ ] FT clock: the Winning Time helper note (`c.winningNote`, `internal/clock/content.go`'s `winningTimeInput()`) can wrap to 3 lines at its current fixed `winningNoteWidth` (300px) for the longer messages (`WinningTimeStaleNote` / `WinningTimeNegativeNote`, `internal/common/consts.go`), bleeding into the Results banner below. There's more horizontal room to the right to widen it, and the note could be top-aligned within its cell instead of vertically centered — flagged as provisional when this note moved beside the entry field (see that change's own "if the verbiage fits" caveat) — [content.go](../../internal/clock/content.go)

## Persona additions — requirements & constraints not yet charted

Placeholders. Each needs its own design pass (requirements, constraints, a
`persona.Role` / `Definition` / challenge) before any implementation.

- [ ] **Heat Sheet Creator (HSC)** — Executive-team, **standalone persona**, not a sidecar. Foundational: originates regatta data from RegattaCentral (roster/events/entries) so the RD can turn it into `regattaSchedule.json`, replacing the current manual RC-export-to-Excel process. Staged: **v1** (read-only local working copy, participant/waiver filtering, RC-id fields added directly to `store.ScheduleRace`/`store.ScheduleEntry`, deadline/finalize workflow, fully-manual post-finalize sync) has no blockers and does not depend on the RC write-path investigation; **v2** (pushing the finalized Heat Sheet back to RC) is hard-gated on that investigation concluding; **v3** (heat→semi/final progression seeding, per a VASRA reference algorithm — computes a later race's lane draw from an earlier heat's approved result and proposes it for the RD to apply) is a separate later increment with no RC dependency, but needs its own design pass for round-type/progression metadata and placeholder-race creation first. The editing-UI question (Fyne in-app authoring, resolved by a dedicated throwaway feasibility spike, vs. generate-and-reconcile-Excel) is deliberately left open pending that spike. Does not block Developer/Results Publisher/SOM/Streamer, but its RC-id indexing decision should inform their own later RC-dependent slices — [heat-sheet-creator.md](personas/new/heat-sheet-creator.md)
- [ ] **Regatta operational state** (not a persona — shared config) — the
  RD's regatta-creation-time choice of schedule-ingest source and official
  results-publish destination (spreadsheet or RegattaCentral), plus social
  platform selection for Social Media (SOM, below) and any future
  social-publishing work. A dependency of Results Publisher and SOM below,
  not the other way around — [operational-state.md](personas/operational-state.md)
- [ ] **Results Publisher** — resolved as a **native PFT feature**, not a persona: a per-race Publish button, enabled once `RaceResult.Approved`, writing to a configured destination (a new standalone results spreadsheet first; RegattaCentral later, once the investigation concludes). Supersedes the "Register Results" scoping below — see the doc for why. Depends on Regatta operational state above for its destination config — [results-publisher.md](personas/new/results-publisher.md)
- [ ] **Social Media (SOM)** — resolved as a **sidecar capability**, not a persona: formalizes `sidecar-personas.md`'s `social-post` capability, hosted only by Finish Timers (PFT/SFT), reusing the already-sketched `internal/publish` package (`PublishableRace`/`BuildView`/`RenderText`) as its shared text-formatting contract. Designs the automated-X-post pop-up (editable text, Publish/Close, retry-on-failure) that `sidecar-personas.md` only placeholder'd. Depends on that doc's Phase 0 landing first; X's API authentication requirements are flagged as their own open investigation, not resolved here — [social-media.md](personas/new/social-media.md)
- [ ] **Streamer (STM)** — resolved as a **standalone persona** (Executive team, Media UI), not a sidecar: runs on the OBS/YouTube-streaming machine, generating lane-assignment and results PNGs (the latter gated on `RaceResult.Approved`, sharing `internal/publish`'s text-format contract with SOM) plus a fixed-size, NTP-corrected wall-clock window for the stream. Supersedes the old "Streaming persona (Media tab)" placeholder. No hard blockers — does not depend on `sidecar-personas.md`'s Phase 0c landing first, only Phase 0a — [streamer.md](personas/new/streamer.md)
- [ ] **Developer (DEV)** persona (Admins tab) — Executive-team, standalone, read-only. A more detailed regatta-status tree than the RD's, plus global buttons to view/follow every persona's already-JSON logs (`regattaData/logs/`) in a new filterable table, and to view regatta/deployment metadata. No blockers found; log-follow needs its own watcher-free mechanism (`internal/watcher` must not watch `logs/`) — [developer.md](personas/new/developer.md)
- [ ] **Standalone "Publisher" persona** (Media tab, a publish-only machine) — a new `persona.Role` (name TBD), a `Definition`, a picker entry, `startPublisherFlow`, and an RD-style read-only oversight tree; whether it is ever built is itself an open decision — [sidecar-personas.md](personas/sidecar-personas.md#recommended-increments)
- [ ] Cross-cutting decisions to settle first — which Leads may toggle a sidecar; where non-secret config lives; the keyring library and its headless-Linux fallback; a synchronous outbox vs fire-and-forget; per-machine vs per-regatta config; render-for-a-human as the v1 line vs going straight to automated push — [sidecar-personas.md](personas/sidecar-personas.md#open-decisions)

## Integrations

- [ ] **RegattaCentral v4 API** — one `internal/regattacentral` client (OAuth2 password grant + refresh, `net/http` transport with retry/backoff, typed JSON model) plus `internal/secretstore` (OS keyring: `client_id` / `client_secret` / operator `username` / `password`). Phase A: client + secret store + `cmd/rcprobe` dev CLI, no UI. Phase B: read-only in-app roster view from `/bulk`. Phase C: in-app heat-sheet authoring (`Origin.Type = "heatsheet"`) that retires the Excel ingest and uploads schedule + lane draws back — own persona doc. Phase D: Register Results write path (sidecar increment 1). Auth flow / endpoints / wire format confirmed against the [V4 Cookbook](personas/reference/RegattaCentral_APIV4_Cookbook.pdf); open items remain (see doc) — [regattacentral-integration.md](personas/regattacentral-integration.md)
