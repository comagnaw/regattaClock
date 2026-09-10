# Feature backlog

Measurable follow-ups already captured across `docs/features/**`, grouped by
theme. This file only tracks work that is already written down somewhere — new
ideas belong in the relevant design doc first, then here.

## Trusted distribution — Windows

- [ ] Obtain a Windows code-signing certificate — one self-signed cert ($0) or an Azure Trusted Signing **Private Trust** account (~$10/mo, no CI key) — [windows-internal-pki.md](trusted-distribution/windows-internal-pki.md#recommendation)
- [ ] Activate the `sign-windows` job — `signtool` / `osslsigncode` (self-signed) or `azure/trusted-signing-action` + OIDC — RFC 3161 timestamped, signing the exe then the installer built around it — [ci-and-provenance.md](trusted-distribution/ci-and-provenance.md#secrets-vs-oidc)
- [ ] Push the cert to the managed fleet via Intune — a Trusted-certificate profile for the root plus a platform script for the Trusted Publishers store; deliver releases from an internal share / Intune so Mark-of-the-Web never engages — [windows-internal-pki.md](trusted-distribution/windows-internal-pki.md#deploying-trust-to-the-fleet)
- [ ] Add a WiX / MSI build alongside the Inno installer for silent `msiexec /qn` deployment via GPO / Intune / SCCM — [windows-packaging.md](trusted-distribution/windows-packaging.md)

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

- [ ] Wire the config UI: `PrefLogging` / `PrefDebug` actually drive `internal/applog`, and surface `PrefStorageMode` / `PrefNTPServers` (the checkboxes exist but drive nothing) — [persona-plan.md](personas/persona-plan.md#12-windows-storage-modes-cloud-synced-folder-and-local-smb)
- [ ] Local write-ahead journal — collect each value to a local file first, then flush to the shared path, so an SMB outage or cloud stall never blocks collection — [persona-plan.md](personas/persona-plan.md#13-open-items), [shared-storage-options.md](personas/shared-storage-options.md)
- [ ] `ScheduleOrigin` interface (`Fingerprint()` / `Load()`) generalising the Excel reader, with a later HTTP-API origin adapter (RD-only, URI + API key) — [persona-plan.md](personas/persona-plan.md#3b-schedule-origin-refresh-rd-only), [schedule-data-model.md](personas/schedule-data-model.md)
- [ ] `data.json` → slim `regattaSchedule.json` migration — strip `Place` / `Split` / `Time` / `Saved` / `Approved` on the first RD open of a legacy file — [schedule-data-model.md](personas/schedule-data-model.md#migration)
- [ ] Route exporter and persona-derived paths through `sanitizeForFilename` for Windows-reserved names and device names — [persona-plan.md](personas/persona-plan.md#12-windows-storage-modes-cloud-synced-folder-and-local-smb)
- [ ] Timer-side staleness indicator in the race tree — "start times last updated N s ago" from the watcher's last-change time — [persona-plan.md](personas/persona-plan.md#12-windows-storage-modes-cloud-synced-folder-and-local-smb)
- [ ] Resolve the remaining open items — cross-team FT→ST start fallback (with explicit confirmation) and an RD override for an ST locked on the wrong race — [persona-plan.md](personas/persona-plan.md#13-open-items)
- [ ] Optional niceties — a user-initiated `w32tm /resync` button in Director config; size-based log rotation plus a Director "collect logs" (clipboard / zip) action; remote syslog export behind a second preference — [persona-plan.md](personas/persona-plan.md#13-open-items), [logging-options.md](personas/logging-options.md)

## Persona additions — requirements & constraints not yet charted

Placeholders. Each needs its own design pass (requirements, constraints, a
`persona.Role` / `Definition` / challenge) before any implementation.

- [ ] **Result-publishing personas** — Executive-team, read-only, each with its own challenge: a social-**text** publisher (plaintext results table) and a social-**image** publisher (PNG via `internal/exporter`). Decision so far: **defer**. Needs a new `persona.Role` with `File: ""`, a `Session.WritePath()` branch, the `regattaData/results/` schema + a re-publish `revision` contract, and where published state is recorded — [future-result-driven-persona.md](personas/future-result-driven-persona.md#what-to-reserve-now-no-code), [sidecar-personas.md](personas/sidecar-personas.md#recommended-increments)
- [ ] **Register Results** — automated push to an external results service (sidecar increment 1). Needs: non-secret config + secret storage (OS keyring), an outbox / retry design, a field-mapping spec; this is the app's first outbound HTTP and first stored secret — [sidecar-personas.md](personas/sidecar-personas.md#recommended-increments)
- [ ] **Social Post** — automated X / social posting (sidecar increment 2). Needs: OAuth2, media upload, rate-limit handling — [sidecar-personas.md](personas/sidecar-personas.md#recommended-increments)
- [ ] **Streaming** persona (Media tab) — Standalone; entirely unscoped — [sidecar-personas.md](personas/sidecar-personas.md)
- [ ] **Developer** persona (Admins tab) — Standalone; entirely unscoped — [persona-plan.md](personas/persona-plan.md#7-the-persona-registry)
- [ ] **Standalone "Publisher" persona** (Media tab, a publish-only machine) — a new `persona.Role` (name TBD), a `Definition`, a picker entry, `startPublisherFlow`, and an RD-style read-only oversight tree; whether it is ever built is itself an open decision — [sidecar-personas.md](personas/sidecar-personas.md#recommended-increments)
- [ ] Cross-cutting decisions to settle first — which Leads may toggle a sidecar; where non-secret config lives; the keyring library and its headless-Linux fallback; a synchronous outbox vs fire-and-forget; per-machine vs per-regatta config; render-for-a-human as the v1 line vs going straight to automated push — [sidecar-personas.md](personas/sidecar-personas.md#open-decisions)
