# CI and provenance

How [`release.yml`](../../../.github/workflows/release.yml) changes to sign and package, how
signing material is handled without leaking it (**R3**) or breaking fork builds (**C9**), and
the **zero‑cost verifiability baseline to ship first** (**R4**). Companion to every other doc
in this directory.

## Zero‑cost baseline — do this first

Independent of any certificate. Gives every release a verifiable identity even while it is
unsigned, and is useful forever afterward.

### SHA‑256 checksums

Emit a single `SHA256SUMS` file covering every released artifact and attach it to the release.
Document verification in the README:

```bash
# macOS / Linux
shasum -a 256 -c SHA256SUMS
```
```powershell
# Windows
(Get-FileHash .\regattaClock-amd64.exe.zip -Algorithm SHA256).Hash
```

### GitHub build‑provenance attestations

Add [`actions/attest-build-provenance`](https://github.com/actions/attest-build-provenance) to
the release workflow. It produces a signed, publicly‑verifiable statement that *this exact
artifact was built by this workflow at this commit* — free for public repositories, no key to
manage.

```yaml
permissions:
  id-token: write
  attestations: write
  contents: write
# ...
- uses: actions/attest-build-provenance@v1
  with:
    subject-path: |
      dist/*.zip
      dist/*.dmg
      dist/*.exe
```

Users verify with:

```bash
gh attestation verify regattaClock-amd64.exe.zip --repo comagnaw/regattaClock
```

### Nice‑to‑have

- `-trimpath` and a pinned Go toolchain for reproducible‑ish builds.
- An SBOM per release via `cyclonedx-gomod` or `syft`, attached to the release.
- Sigstore `cosign` keyless signatures of the artifacts — largely redundant with the
  attestations above, add only if a downstream consumer asks for it.

## Restructured release workflow

Today `release.yml` has `build-macos` then `build-windows` (`needs: build-macos`), each doing
build + publish in one job. Split build from sign/package so signing is isolated and skippable:

```
build-windows  (ubuntu-latest, fyne-cross)        → unsigned regattaClock-<arch>.exe  (artifact)
   └─ sign-windows        [if signing configured]  → sign .exe  +  build & sign installer (Inno / WiX)
                                                     ·  self‑signed cert  → osslsigncode + .pfx secret
                                                     ·  Trusted Signing   → azure/login (OIDC) + trusted-signing-action, no key
build-macos    (macos-latest, fyne package+lipo)   → unsigned regattaClock.app / .dmg  (artifact)
   └─ sign-macos          [if secrets present]     → codesign → notarytool → stapler
provenance     (ubuntu-latest)                     → SHA256SUMS  +  attest-build-provenance
release        (ubuntu-latest)                     → gh release: signed installers, portable zips, .dmg, SHA256SUMS
```

- Keep the `v*` tag trigger.
- If using the Trusted Signing path, give `sign-windows` `permissions: id-token: write` for OIDC.
- The `sign-*` jobs `needs:` their `build-*` job and consume its uploaded artifact.
- `provenance` and `release` `needs:` whatever sign/build jobs actually ran.
- Packaging (Inno / WiX / `create-dmg`) lives in the sign jobs so the installer is built
  around the already‑signed executable — see
  [windows-packaging.md](windows-packaging.md#signing-order).

## Fork / PR safety (C9)

Secrets are absent on fork‑triggered runs. A signing step must **skip**, not fail. GitHub does
not allow `secrets.*` directly in a job‑level `if`, so map it into an output or env first:

```yaml
jobs:
  check-secrets:
    runs-on: ubuntu-latest
    outputs:
      can-sign-windows: ${{ steps.c.outputs.win }}
      can-sign-macos:   ${{ steps.c.outputs.mac }}
    steps:
      - id: c
        env:
          WIN_PFX: ${{ secrets.WINDOWS_PFX_BASE64 }}
          MAC_P12: ${{ secrets.MACOS_CERT_P12_BASE64 }}
        run: |
          echo "win=${{ env.WIN_PFX != '' }}" >> "$GITHUB_OUTPUT"
          echo "mac=${{ env.MAC_P12 != '' }}" >> "$GITHUB_OUTPUT"

  sign-windows:
    needs: [build-windows, check-secrets]
    if: needs.check-secrets.outputs.can-sign-windows == 'true'
    # ...
```

When a `sign-*` job is skipped, `release` still publishes the **unsigned** artifact plus its
checksums and attestation. Forks get a working build; only the canonical repo produces signed
output.

## Secrets vs. OIDC

| Signing path | Credential in CI | Mechanism |
|--------------|------------------|-----------|
| Internal PKI, self‑signed cert ([windows-internal-pki.md](windows-internal-pki.md) Option B) | base64 `.pfx` + password | GitHub Actions **secrets**. Acceptable for an internal LOB cert (**C3** does not apply to internal certs). |
| Apple notarization ([macos-notarization.md](macos-notarization.md)) | base64 `.p12` + password; App Store Connect `.p8` + key id + issuer id | GitHub Actions **secrets**; imported into a temporary keychain. |
| Azure Trusted Signing — **Private Trust** (domain, [windows-internal-pki.md](windows-internal-pki.md) Option C) **or Public Trust** ([windows-public-trust.md](windows-public-trust.md)) | none | `azure/login` + GitHub **OIDC → Entra** federated credential, then `azure/trusted-signing-action`. No key or long‑lived secret stored (**R3** in full). Same wiring for both trust types — only the certificate profile differs. |
| Cloud‑HSM OV cert (if ever) | none | Workload identity to the KMS; `jsign` signs via the HSM. |

Prefer OIDC whenever a path supports it — Azure Trusted Signing (either trust type) needs no
stored key. If you use the self‑signed `.pfx` instead, it is the one stored key and it is
internal‑only — do not reuse it for public signing.

For the OIDC path, the `check-secrets` gate above keys off the Entra federated‑credential
inputs (e.g. `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID`) rather than a
`.pfx` secret; forks without them still get an unsigned build (**C9**).

Suggested secret names: `WINDOWS_PFX_BASE64`, `WINDOWS_PFX_PASSWORD`,
`MACOS_CERT_P12_BASE64`, `MACOS_CERT_PASSWORD`, `MACOS_KEYCHAIN_PASSWORD`,
`ASC_API_KEY_P8_BASE64`, `ASC_API_KEY_ID`, `ASC_API_ISSUER_ID`.

## Timestamping

Every Authenticode signature (`osslsigncode -ts` / `signtool /tr`) and the `codesign
--timestamp` calls must use a timestamp authority. Timestamped signatures keep validating
after the certificate rotates or expires, so old releases do not "go bad" when you renew.

## Verification

- Push a throwaway tag on a branch (`v0.0.0-test1`): with repo secrets present, `sign-windows`
  and `sign-macos` run and the release carries signed artifacts. From a fork PR, both are
  skipped and the build still goes green with unsigned artifacts + `SHA256SUMS` + attestation.
- `gh attestation verify <artifact> --repo comagnaw/regattaClock` succeeds for every released
  file.
- `shasum -a 256 -c SHA256SUMS` passes on a fresh download.
- Per‑OS launch checks: see [windows-internal-pki.md](windows-internal-pki.md#verifying-a-signed-build),
  [windows-packaging.md](windows-packaging.md#verification),
  [macos-notarization.md](macos-notarization.md#verification).

## Alternatives considered

| Approach | Verdict |
|----------|---------|
| Keep build + publish in one job per OS | Signing material would be in scope for the build step and harder to gate. Splitting is cheap and clarifies fork behaviour. |
| Sign on a `windows-latest` job with `signtool` | Works, adds a runner and a toolchain. `osslsigncode` on the existing Linux job is fewer moving parts (**C1**); keep `signtool` as the fallback. |
| Store the Apple cert unlocked in the repo keychain | Never. Temporary keychain per run, deleted after. |
| Skip attestations, rely on signatures only | Attestations are free, work *before* any cert exists, and cover provenance (who/what/where built it) that a code signature does not. Do both. |
| `goreleaser` to orchestrate all of this | Viable and would replace much of the hand‑rolled YAML, but it is a larger change than this plan needs. Note it as a possible later consolidation. |
