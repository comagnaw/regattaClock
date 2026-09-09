# Windows: packaging (portable exe and installer)

`regattaClock` should ship **both** a portable `.exe` and a real installer. This is orthogonal
to signing — a signed portable exe is fine, and an installer is useful whether or not it is
signed — but the two interact, and an **MSI** specifically unlocks silent domain deployment.
Companion to [windows-internal-pki.md](windows-internal-pki.md) and
[ci-and-provenance.md](ci-and-provenance.md).

## Where things stand

[`release.yml`](../../../.github/workflows/release.yml) runs
`fyne-cross windows -arch amd64,386` and publishes `regattaClock-<arch>.exe.zip` — a bare
executable, no installer. Two version surfaces, only one of them fed:

- **Go level** — the git tag *is* compiled into the binary via `-ldflags -X` on
  `internal/version` (`regattaClock -v`; see [`AGENTS.md`](../../../AGENTS.md)
  "Releases").
- **Win32 `VERSIONINFO`** — the version Explorer shows under right‑click →
  Properties → **Details**. There is still **no `FyneApp.toml`** in the repo, so
  this is whatever `fyne`/`fyne-cross` defaults to; the tag does **not** reach it
  yet (see [Version metadata](#version-metadata) below).

Consequences of portable‑only:

- No Start Menu entry, no "Apps & Features" record, no uninstaller.
- Nothing to clean up on removal anyway — configuration is in Fyne `Preferences`
  (registry/AppData), per [`CLAUDE.md`](../../../CLAUDE.md) (**C8**). So an installer's job is
  *discoverability and lifecycle*, not file management.

## Keep: the portable zip

Retain `regattaClock-<arch>.exe.zip` as one release artifact. Improvements:

- Include a short `README.txt` and the per‑file line from `SHA256SUMS`
  ([ci-and-provenance.md](ci-and-provenance.md)) inside the zip.
- Name consistently: `regattaClock-<version>-windows-<arch>-portable.zip`.
- This is the artifact for "just run it from a USB stick at the venue" and for operators who
  cannot install software.

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
