# Feature backlog

Measurable follow-ups already captured across `docs/features/**`, grouped by
theme. This file only tracks work that is already written down somewhere — new
ideas belong in the relevant design doc first, then here.

## Trusted distribution — Windows

- [ ] Obtain a Windows code-signing certificate — one self-signed cert ($0) or an Azure Trusted Signing **Private Trust** account (~$10/mo, no CI key) — [windows-internal-pki.md](trusted-distribution/windows-internal-pki.md#recommendation)
- [ ] Activate the `sign-windows` job — `signtool` / `osslsigncode` (self-signed) or `azure/trusted-signing-action` + OIDC — RFC 3161 timestamped, signing the exe then the installer built around it — [ci-and-provenance.md](trusted-distribution/ci-and-provenance.md#secrets-vs-oidc)
- [ ] Push the cert to the managed fleet via Intune — a Trusted-certificate profile for the root plus a platform script for the Trusted Publishers store; deliver releases from an internal share / Intune so Mark-of-the-Web never engages — [windows-internal-pki.md](trusted-distribution/windows-internal-pki.md#deploying-trust-to-the-fleet)
- [ ] Add a WiX / MSI build alongside the Inno installer for silent `msiexec /qn` deployment via GPO / Intune / SCCM — [windows-packaging.md](trusted-distribution/windows-packaging.md)
- [ ] Native Windows ARM64 build — today's release ships amd64 only; an ARM-based Windows machine (e.g. Surface Pro X, Surface Pro 9 5G/11 with Snapdragon) falls back to Windows' x64 emulation, untested with this app. Not pursued unless real demand shows up (no regatta hardware runs ARM today) — [README.md](../../README.md#downloading-and-verifying)

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

## Testing — sample regattaData (built)

- [ ] Full-size manual run on the multi-machine Windows domain (RD, PST/SST, PFT/SFT, AWD on the share): time, approve, and publish the last five races; watch load latency, watcher churn, and memory — [sample-regatta.md](testing/sample-regatta.md#verification)

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

- [ ] Represent Heat Sheet **lunch breaks**. The reader skips a break block (a 3-row block with no race number and `Break` in column B; its length is set in the workbook's "Regatta Attributes" sheet), so a break shows up only as a gap in scheduled times. Nothing in the schedule, race trees, or ST/FT views marks it. Needs a design: a schedule field, how the trees show it, and whether countdowns account for it — [sample-regatta.md](testing/sample-regatta.md#constraints-and-gotchas)
- [ ] `ScheduleOrigin` interface (`Fingerprint()` / `Load()`) generalising the Excel reader, with a later in-app-authored heat sheet (`Origin.Type = "heatsheet"`) built from the RegattaCentral roster and uploaded back to RegattaCentral, via `regattaDataFromSchedule` / `reader.NewRegattaData` (one entry point, no forked constructor) — [persona-plan.md](personas/persona-plan.md#3b-schedule-origin-refresh-rd-only), [schedule-data-model.md](personas/closed/schedule-data-model.md), [regattacentral-integration.md](personas/regattacentral-integration.md#phase-c--in-app-heat-sheet-authoring)
- [ ] **PFT: cancel a race, with a reason.** Races are sometimes cancelled, usually for weather and rowers' safety (e.g. Walter Mess 2026 races 1–3), and today a cancelled race just looks like one with no results. Not scheduled yet; capture so the finish.json model and the publish output leave room for it.
  - **UX:** in the PFT's race tree (`showRaceTree`), one button per race that toggles with its state. **Cancel** (on a race with no approved result) opens a modal with radio-button reasons: **Weather**, **Water / course conditions**, or **Other** (a short note required). **Un-cancel** (on a cancelled race) clears it after a confirmation.
  - **A race state, shown like "On the Water":** a new `StateCancelled` in `store.DeriveTeamState`, derived from the result and taking precedence over every other state ([race-state-machine.md](personas/closed/race-state-machine.md)). Its display text carries the reason ("Cancelled: Weather", "Cancelled: Course blocked"), so it reaches every place state is shown today: PFT and SFT race trees, the RD tree, and the clock window's status label.
  - **finish.json:** `RaceResult.Cancelled *Cancellation{Reason, Note, At}`. `Reason` is a stored ID (`weather`, `water`, `other`), so labels can change without migrating data. A cancelled race has no rows and can't be approved until it is un-cancelled.
  - **Publish:** no Referee Approval. `store.CanPublish` and `publish.BuildView` accept `Approved` **or** `Cancelled`, and `Revision` covers the cancellation, so cancelling and un-cancelling both republish. Un-cancel follows the carry-forward rule: the published cancellation stays until the race is approved and published again.
  - **Output:** web JSON gets a race-level `"cancelled": {"reason": "weather", "note": ""}` with empty `results` and the lanes kept as entries ([web-results format](../../examples/web-results/README.md#the-file-format-schemaversion-1)). The results page shows "Cancelled · Weather" in place of the results table, and the workbook writes "Cancelled: Weather" in the race's block. The past-season archive import already writes this field, so archive and live files agree.
  - **Out of scope for now:** cancelling the rest of a day, or a whole regatta, in one action.
- [ ] **Carry a heat's advancement rule into regattaData and the published results.** The heat sheet already states it: the third row of each race block holds "2 to Advance" in column 0 (see the sample regatta). The reader captures that row but maps nothing from it (`RawData`, [regattaData.go](../../internal/reader/regattaData.go)), so 2026 results lost it. Past seasons' imported results carry it, and the results page shows it after the race title (#148).
  - **Read:** a `Progression` field from `RawData` row 2, column 0, on `reader.RaceData`. The same row's lane columns are the rowers' names in 1x/2x boats; leave those alone.
  - **Store:** `store.ScheduleRace.Progression` in `regattaSchedule.json`, set at import and reload like `FlightInfo`, so it reaches every persona.
  - **Publish:** `publish.PublishableRace.Progression`, part of the revision, written as the web JSON's optional `races[].progression` ([format](../../examples/web-results/README.md#the-file-format-schemaversion-1)) and shown in the workbook's race block.
  - **Open question:** wording. Keep the heat sheet's text ("2 to Advance"), or normalize it the way the archive import does ("Top 2 to Final")?
- [ ] Windows folder-picker hang on a stale/disconnected mapped network drive — `dialog.NewFolderOpen`'s sidebar enumerates every drive letter and calls `os.Stat` on each before showing, which can hang indefinitely. Current mitigation is operator-side (README.md's Troubleshooting section); investigate a Fyne version bump that might fix this upstream, or a custom enumeration-free directory picker, before this becomes an app-level fix — [README.md](../../README.md#troubleshooting)

## Persona additions — requirements & constraints not yet charted

Placeholders. Each needs its own design pass (requirements, constraints, a
`persona.Role` / `Definition` / challenge) before any implementation.

- [ ] **Heat Sheet Creator (HSC)** — Executive-team, **standalone persona**, not a sidecar. Foundational: originates regatta data from RegattaCentral (roster/events/entries) so the RD can turn it into `regattaSchedule.json`, replacing the current manual RC-export-to-Excel process. Staged: **v1** (read-only local working copy, participant/waiver filtering, RC-id fields added directly to `store.ScheduleRace`/`store.ScheduleEntry`, deadline/finalize workflow, fully-manual post-finalize sync) has no blockers and does not depend on the RC write-path investigation; **v2** (pushing the finalized Heat Sheet back to RC) is hard-gated on that investigation concluding; **v3** (heat→semi/final progression seeding, per a VASRA reference algorithm — computes a later race's lane draw from an earlier heat's approved result and proposes it for the RD to apply) is a separate later increment with no RC dependency, but needs its own design pass for round-type/progression metadata and placeholder-race creation first. The editing-UI question (Fyne in-app authoring, resolved by a dedicated throwaway feasibility spike, vs. generate-and-reconcile-Excel) is deliberately left open pending that spike. Does not block Developer/Results Publisher/SOM/Streamer, but its RC-id indexing decision should inform their own later RC-dependent slices — [heat-sheet-creator.md](personas/new/heat-sheet-creator.md)
- [ ] **Regatta operational state** (not a persona — shared config) — the
  RD's regatta-creation-time choice of schedule-ingest source (the
  results-publish destination moved to the PFT, see
  [publish-destinations.md](publish-destinations.md)), plus social
  platform selection for Social Media (SOM, below) and any future
  social-publishing work. A dependency of Results Publisher and SOM below,
  not the other way around — [operational-state.md](personas/operational-state.md)
- [ ] **Results Publisher — RegattaCentral destination (deferred)** — REP is a **native PFT feature**, and its **spreadsheet destination has shipped** (#126, #127; documented in PFT's section of [personas/README.md](personas/README.md#finish-timer-ft)). What remains is the RegattaCentral destination, a second branch in the same publish path. It is deferred until [heatsheet-rc-pivot-investigation.md](personas/heatsheet-rc-pivot-investigation.md) concludes. It supersedes the "Register Results" scoping below; see the doc for why — [results-publisher.md](personas/new/results-publisher.md)
- [ ] **Social Media (SOM)** — resolved as a **sidecar capability**, not a persona: formalizes `sidecar-personas.md`'s `social-post` capability, hosted only by Finish Timers (PFT/SFT), reusing the already-sketched `internal/publish` package (`PublishableRace`/`BuildView`/`RenderText`) as its shared text-formatting contract. Designs the automated-X-post pop-up (editable text, Publish/Close, retry-on-failure) that `sidecar-personas.md` only placeholder'd. Depends on that doc's Phase 0 landing first; X's API authentication requirements are flagged as their own open investigation, not resolved here — [social-media.md](personas/new/social-media.md)
- [ ] **Streamer (STM)** — resolved as a **standalone persona** (Executive team, Media UI), not a sidecar: runs on the OBS/YouTube-streaming machine, generating lane-assignment and results PNGs (the latter gated on `RaceResult.Approved`, sharing `internal/publish`'s text-format contract with SOM) plus a fixed-size, NTP-corrected wall-clock window for the stream. Supersedes the old "Streaming persona (Media tab)" placeholder. No hard blockers — does not depend on `sidecar-personas.md`'s Phase 0c landing first, only Phase 0a — [streamer.md](personas/new/streamer.md)
- [ ] **Developer (DEV)** persona (Admins tab) — Executive-team, standalone, read-only. A more detailed regatta-status tree than the RD's, plus global buttons to view/follow every persona's already-JSON logs (`regattaData/logs/`) in a new filterable table, collect/export them (clipboard / zip) for a support report, and to view regatta/deployment metadata. Owns log collection and visibility outright — this supersedes the earlier, narrower idea of a Director-side "collect logs" button. No blockers found; log-follow needs its own watcher-free mechanism (`internal/watcher` must not watch `logs/`) — [developer.md](personas/new/developer.md)
- [ ] **Standalone "Publisher" persona** (Media tab, a publish-only machine) — a new `persona.Role` (name TBD), a `Definition`, a picker entry, `startPublisherFlow`, and an RD-style read-only oversight tree; whether it is ever built is itself an open decision — [sidecar-personas.md](personas/sidecar-personas.md#recommended-increments)
- [ ] Cross-cutting decisions to settle first — which Leads may toggle a sidecar; where non-secret config lives; the keyring library and its headless-Linux fallback; a synchronous outbox vs fire-and-forget; per-machine vs per-regatta config; render-for-a-human as the v1 line vs going straight to automated push — [sidecar-personas.md](personas/sidecar-personas.md#open-decisions)

## Integrations

- [ ] **RegattaCentral v4 API** — one `internal/regattacentral` client (OAuth2 password grant + refresh, `net/http` transport with retry/backoff, typed JSON model) plus `internal/secretstore` (OS keyring: `client_id` / `client_secret` / operator `username` / `password`). Phase A: client + secret store + `cmd/rcprobe` dev CLI, no UI. Phase B: read-only in-app roster view from `/bulk`. Phase C: in-app heat-sheet authoring (`Origin.Type = "heatsheet"`) that retires the Excel ingest and uploads schedule + lane draws back — own persona doc. Phase D: Register Results write path (sidecar increment 1). Auth flow / endpoints / wire format confirmed against the [V4 Cookbook](personas/reference/RegattaCentral_APIV4_Cookbook.pdf); open items remain (see doc) — [regattacentral-integration.md](personas/regattacentral-integration.md)
