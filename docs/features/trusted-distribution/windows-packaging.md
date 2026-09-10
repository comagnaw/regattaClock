# Windows: packaging (portable exe and installer)

`regattaClock` should ship **both** a portable `.exe` and a real installer. This is orthogonal
to signing — a signed portable exe is fine, and an installer is useful whether or not it is
signed — but the two interact, and an **MSI** specifically unlocks silent domain deployment.
Companion to [windows-internal-pki.md](windows-internal-pki.md) and
[ci-and-provenance.md](ci-and-provenance.md).

## Where things stand

[`release.yml`](../../../.github/workflows/release.yml)'s `build-windows` job runs on
**`windows-latest`** with MinGW and builds the exe with native `fyne package -os windows`
(no more `fyne-cross`). It produces **two artifacts**:

- `regattaClock-<version>-windows-amd64-portable.zip` — the exe plus a `README.txt`;
- `regattaClock-<version>-windows-setup.exe` — a per‑user Inno Setup installer
  (`packaging/windows/regattaClock.iss`), Start Menu entry and uninstaller.

Both version surfaces are now fed: the **Go level** (`regattaClock -v`, from `-ldflags -X`
on `internal/version` — the native build means Windows gets the same stamp as macOS) and the
**Win32 `VERSIONINFO`** (via `cmd/regattaClock/FyneApp.toml` + `-app-version` in CI). The
32‑bit (386) build was dropped.

Neither artifact is code‑signed yet — the `sign-windows` job is scaffolded for
[Option B](windows-internal-pki.md) and stays inert until a certificate secret is set.

## Keep: the portable zip

`regattaClock-<version>-windows-amd64-portable.zip` already bundles a `README.txt` with the
`Get-FileHash` verify line — the artifact for "just run it from a USB stick at the venue" and
for operators who cannot install software. It is kept alongside the installer.

## Add: an installer

### Inno Setup — recommended default

A small `.iss` script produces a single `setup.exe`:

- `PrivilegesRequired=lowest` → **per‑user install**, no UAC prompt, installs under
  `%LocalAppData%\Programs\regattaClock`.
- Creates a Start Menu shortcut and a proper uninstaller / "Apps & Features" entry.
- `AppVersion` / `VersionInfoVersion` set from the git tag at build time.
- Build it either in a `windows-latest` job, or with Inno Setup under Wine on the existing
  Linux job (community actions exist, e.g. `amake/innosetup`).

Sign the produced `setup.exe` (see [signing order](#signing-order)).

### WiX / MSI — add for domain deployment

An **MSI** is what makes silent, centrally‑managed rollout possible on the maintainer's
domain:

- Deployable via **GPO Software Installation**, **Intune**, or **SCCM** — an Inno `setup.exe`
  is not.
- `msiexec /i regattaClock.msi /qn` for unattended install; clean per‑machine or per‑user
  modes; automatic repair/upgrade semantics.
- Build with [WiX Toolset](https://wixtoolset.org/) (v4+ runs on .NET, works in a
  `windows-latest` job) or [`msitools`](https://gitlab.gnome.org/GNOME/msitools) /
  [`go-msi`](https://github.com/mh-cbon/go-msi) on Linux.

Recommendation: add the MSI **in addition to** Inno once domain mass‑deployment is actually
wanted (it pairs directly with the "distribute via Intune/SCCM to avoid MOTW" step in
[windows-internal-pki.md](windows-internal-pki.md)). Until then, Inno + portable zip is enough.

## Signing order

Sign inside‑out, so every layer a user can execute is signed:

1. Sign `regattaClock.exe` (the Fyne output).
2. Build the installer (`setup.exe` and/or `.msi`) **embedding the already‑signed exe**.
3. Sign the installer itself.

All with the same certificate — internal ([windows-internal-pki.md](windows-internal-pki.md))
for now, public ([windows-public-trust.md](windows-public-trust.md)) later — and always RFC
3161 timestamped.

## Version metadata

Add a `FyneApp.toml` at the repo root so the exe carries real product/version info:

```toml
[Details]
Icon = "Icon.png"
Name = "regattaClock"
ID = "com.github.comagnaw.regattaClock"
Version = "0.0.0"   # overridden in CI
Build = 1
```

In CI, derive the version from `github.ref_name` (strip the leading `v`) and pass it through —
`fyne-cross`/`fyne package` accept `-app-version` / `-app-build`, and the same value feeds the
Inno `AppVersion` and the WiX `ProductVersion`. One version string, one source (the tag).

## Verification

On a clean Windows VM (no dev tools):

- **Portable**: unzip, double‑click `regattaClock.exe`, app starts. `Get-FileHash` matches
  `SHA256SUMS`.
- **Inno installer**: run `setup.exe` → no UAC prompt (per‑user) → Start Menu entry present →
  "Apps & Features" lists regattaClock with a working **Uninstall**.
- **MSI**: `msiexec /i regattaClock.msi /qn` installs silently; `/x` removes it;
  GPO/Intune assignment installs it on next policy refresh.
- **Signed**: `Get-AuthenticodeSignature` reports `Valid` on the exe *and* the installer.

## Alternatives considered

| Approach | Verdict |
|----------|---------|
| Portable zip only (today) | Keep it, but it fails discoverability and can't be centrally deployed. Not sufficient alone. |
| NSIS instead of Inno Setup | Comparable; Inno's script is simpler and per‑user mode is one line. Either is fine — Inno chosen for brevity. |
| MSIX instead of Inno/MSI | Cleanest Store/SmartScreen story but a different packaging pipeline; explicitly out of scope for this plan. Revisit with [windows-public-trust.md](windows-public-trust.md). |
| MSI only (drop Inno) | MSI authoring is heavier; Inno is the faster win for the non‑domain audience. Do both, Inno first. |
| `fyne package`'s own installer output | `fyne` produces the packaged `.exe`, not a Start‑Menu installer or an MSI. Still need Inno/WiX on top. |
