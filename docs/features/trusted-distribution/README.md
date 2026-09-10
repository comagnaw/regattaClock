# Trusted distribution

How to get `regattaClock` binaries to run on Windows and macOS without the operating
system blocking them, starting from a small managed Windows domain and growing toward
public distribution — at zero recurring cost for the near term.

**Related docs in this directory**

- [windows-internal-pki.md](windows-internal-pki.md) — short term: sign for the managed domain with an internal cert
- [windows-public-trust.md](windows-public-trust.md) — long term: SmartScreen / public code‑signing options
- [windows-packaging.md](windows-packaging.md) — portable `.exe` vs a real installer (Inno Setup / WiX MSI)
- [macos-notarization.md](macos-notarization.md) — mid term: Developer ID signing + Apple notarization
- [ci-and-provenance.md](ci-and-provenance.md) — how `release.yml` changes; checksums + build provenance as the zero‑cost baseline

> Dates and prices below are current as of 2026‑09. Signing‑program rules change often —
> confirm with the vendor before acting.

## Where things stand

[`.github/workflows/release.yml`](../../../.github/workflows/release.yml) builds artifacts on
every `v*` tag (and on `workflow_dispatch`, which builds and uploads to the run without
publishing). The `build → sign → provenance → release` split from
[ci-and-provenance.md](ci-and-provenance.md) is in place; the sign stage is scaffolded for
**Option B** (self‑signed cert + `signtool`) and skips cleanly until `WINDOWS_PFX_BASE64` is
set.

| OS | Job | Build | Output | Signed? |
|----|-----|-------|--------|---------|
| Windows | `build-windows` (`windows-latest`, MinGW) | native `fyne package -os windows` | `regattaClock-<version>-windows-amd64-portable.zip` + Inno Setup `…-windows-setup.exe` | not yet (scaffolded) |
| macOS | `build-macos` (`macos-latest`) | `fyne package -os darwin -release` per arch → `lipo` universal → `hdiutil` DMG | `regattaClock-<version>-macos-universal.dmg` | no |
| both | `provenance` (`ubuntu-latest`) | — | `SHA256SUMS` + `attest-build-provenance` over every file | n/a |

**Done:** the zero‑cost verifiability baseline (`SHA256SUMS` + provenance attestations); native
Windows build so `regattaClock -v` carries the real build stamp on Windows too; a
`FyneApp.toml` version resource; a per‑user Inno Setup installer alongside the portable zip.
**Pending:** an actual signing certificate (this doc), macOS notarization
([macos-notarization.md](macos-notarization.md)), a WiX MSI for silent domain deploy
([windows-packaging.md](windows-packaging.md)).

What users see today:

- **Windows** — Microsoft Defender SmartScreen ("Windows protected your PC"), an "unknown
  publisher" prompt, occasionally a Defender quarantine. On locked‑down machines the app may
  not run at all.
- **macOS** — Gatekeeper blocks a downloaded copy. On current macOS (Sequoia removed the
  Finder Control‑click "Open" shortcut) the user must go to System Settings → Privacy &
  Security → **Open Anyway**. The maintainer's own Mac runs it only because a locally‑built
  app is never quarantined.

## Audiences

| Audience | Timeframe | Trust mechanism available |
|----------|-----------|---------------------------|
| Maintainer's Windows domain, 7–10 PCs | now | **Entra / Intune‑managed** (cloud, no on‑prem AD assumed) — full control of every machine; internal PKI trust pushed by Intune |
| macOS users (maintainer + others) | mid | Apple notarization only — no internal‑CA equivalent for Gatekeeper |
| Public, unmanaged Windows | long | A publicly‑trusted code‑signing certificate; SmartScreen reputation |

## Requirements

- **R1** Domain users run the Windows app with no security prompts and no per‑machine manual
  trust steps (the trust is pushed once, centrally).
- **R2** All signing and packaging is automated in GitHub Actions off a `v*` tag. No signing
  on a developer laptop.
- **R3** Signing keys are never committed and never exposed to workflows triggered from forks
  or pull requests.
- **R4** Every released artifact is independently verifiable — checksum plus build provenance —
  even while it is still unsigned.
- **R5** Public Windows users can install with a documented, bounded number of clicks, and a
  path to a zero‑prompt experience exists without replacing the toolchain.
- **R6** macOS users can install a real `.app` from a DMG that passes Gatekeeper (mid term).
  The maintainer's own machine keeps working with no change.
- **R7** No hard lock‑in to a single signing vendor; the approach stays OS‑agnostic in spirit,
  consistent with the rest of the project (see [`personas/logging-options.md`](../personas/logging-options.md) §2).
- **R8** The short‑term internal solution has zero recurring cost.

## Constraints

- **C1** The Windows binary is cross‑compiled on Linux (`fyne-cross`, Docker). The signer must
  run on Linux (`osslsigncode` / `jsign`) or a separate `windows-latest` job must be added.
- **C2** The macOS binary is `lipo`‑merged from two arch slices; sign the merged binary and
  bundle, not the slices.
- **C3** CA/Browser Forum rules since June 2023 require publicly‑trusted code‑signing private
  keys to live on FIPS 140‑2 hardware or in a cloud HSM. A plain `.pfx` file is no longer an
  option for a public certificate (it is still fine for an internal one).
- **C4** Apple notarization requires the paid Apple Developer Program ($99/yr). There is no
  zero‑cost equivalent for distributing to macOS machines you do not manage.
- **C5** SmartScreen reputation is tied to the specific certificate, builds up over
  downloads/time, and resets when the certificate changes. An internal certificate contributes
  nothing to SmartScreen off‑domain.
- **C6** The project is maintained by an individual with no legal entity. Organisation‑validated
  OV/EV certificates and a Microsoft Store *company* account are unavailable or need extra
  identity vetting.
- **C7** Mark‑of‑the‑Web: files downloaded from the internet are quarantined regardless of
  signature. Distributing the signed build from an internal file share, Intune, or SCCM avoids
  MOTW entirely on the domain.
- **C8** Configuration lives in Fyne `Preferences` (per [`CLAUDE.md`](../../../CLAUDE.md)), so
  an uninstaller has almost nothing to clean up — an installer's value is a Start Menu entry
  and an "Apps & Features" record, not file cleanup.
- **C9** GitHub Actions secrets are not available to runs triggered from forks. Every signing
  job must degrade to producing an unsigned artifact, never fail the build.

## Phased roadmap

| Phase | Audience | Approach | Recurring cost | Effort |
|-------|----------|----------|----------------|--------|
| **Now** | Managed domain | Internal code‑signing certificate: one self‑signed cert (zero cost) or **Azure Trusted Signing – Private Trust** (~$10/month, no CI key, Entra‑native). Distribute the trust with **Intune** — a Trusted certificate profile for the root plus a platform script for **Trusted Publishers** (the profile can't reach that store). Sign `.exe` + installer in CI (`osslsigncode` for the self‑signed cert; `azure/trusted-signing-action` + OIDC for Option C). Deliver via Intune / internal share to sidestep MOTW. | $0 or ~$120/yr | S–M |
| **Now, in parallel** | Everyone | Publish `SHA256SUMS` and GitHub build‑provenance attestations with every release; document verification. | $0 | S |
| **Now** | Windows | Add an Inno Setup per‑user installer alongside the portable zip. Add a WiX **MSI** if/when silent Intune/GPO deployment on the domain is wanted. | $0 | M |
| **Mid** | macOS (public) | Apple Developer Program → Developer ID Application cert → hardened runtime → `notarytool` → `stapler`, all in the existing `macos-latest` job. Ship a signed, stapled DMG; optionally a Homebrew cask. | $99/yr | M–L |
| **Long** | Unmanaged Windows | Apply to **SignPath Foundation** for a free OSS code‑signing certificate + signing service. If ineligible, or if Trusted Signing Private Trust is already in use, add a **Public Trust** validation to **Azure Trusted Signing** (~$120/yr, OIDC, same CI wiring). | $0–120/yr | M |
| **Deferred** | — | Microsoft Store (MSIX), winget manifest, EV certificate, reproducible builds / SBOM. | varies | — |

## Decision summary

The domain is Entra / Intune‑managed with no on‑prem AD assumed, so there is no AD CS to
enrol against and Entra ID does not itself issue signing certificates — but trust is still
delivered centrally, by Intune. Do the near‑term work now: an internal signing certificate
(self‑signed for zero cost, or **Azure Trusted Signing – Private Trust** at ~$10/month to keep
the private key out of CI and get an on‑ramp to public trust), `SHA256SUMS` +
build‑provenance attestations for everyone, and an installer. Adopt the Apple Developer
Program for macOS when $99/yr is acceptable — there is no cheaper path to Gatekeeper. For the
public Windows audience, pursue a **SignPath Foundation** OSS certificate (free), or add a
**Public Trust** validation to Azure Trusted Signing (~$120/yr) if Private Trust is already in
place. Do **not** buy a hardware‑token OV/EV certificate — the cost and CI friction are not
justified at this stage.
