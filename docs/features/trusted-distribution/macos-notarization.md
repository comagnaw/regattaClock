# macOS: signing and notarization

Mid‑term path to **R6** — macOS users install a real `.app` from a DMG that Gatekeeper
accepts, while the maintainer's own machine keeps working unchanged. Companion to
[README.md](README.md); CI wiring in [ci-and-provenance.md](ci-and-provenance.md).

**There is no zero‑cost path here** (**C4**). Distributing to Macs you do not manage requires
Apple notarization, which requires the paid **Apple Developer Program ($99/yr)**. macOS has no
internal‑CA equivalent for Gatekeeper the way Windows has Trusted Publishers.

> `spctl`/`codesign` behaviour and menu paths below are current as of 2026‑09 (macOS Sequoia
> era). Verify against current Apple documentation before implementing.

## Where things stand

[`release.yml`](../../../.github/workflows/release.yml)'s `build-macos` job runs
`fyne package -os darwin -release` for `amd64` and `arm64`, `lipo`s them into a universal
binary, assembles a `.app`, and wraps it in a DMG with `hdiutil`. Nothing is code‑signed with
a Developer ID and nothing is notarized.

Why it still runs for the maintainer: a locally built `.app` is never quarantined, and Apple
Silicon accepts the ad‑hoc signature `fyne`/the toolchain applies. A **downloaded** copy is a
different story:

- The download carries `com.apple.quarantine`.
- Gatekeeper refuses it. On current macOS the old Finder **Control‑click → Open** shortcut is
  gone; the user must open **System Settings → Privacy & Security** and click **Open Anyway**,
  then confirm again on first launch.
- An unsigned (not even ad‑hoc) binary on Apple Silicon will not execute at all.

## Zero‑cost interim (trusted testers only)

For a handful of people you can talk to directly — not public release:

1. Ensure the binary is at least **ad‑hoc signed** (`codesign -s - --deep`). Confirm whether
   `fyne package -release` already does this; if so, nothing to add.
2. Document the unblock steps in the release notes:
   - System Settings → Privacy & Security → **Open Anyway**, or
   - `xattr -dr com.apple.quarantine /Applications/regattaClock.app` in Terminal.

This is a stopgap. It does not scale and asks users to bypass a security control, so it is not
acceptable for a public download.

## Real path — Developer ID + notarization ($99/yr)

Once enrolled in the Apple Developer Program:

### 1. Certificate

Create a **Developer ID Application** certificate (Xcode or the Developer portal). Export it
as a `.p12` with a password for CI. Note the **Team ID**.

### 2. Sign (inside‑out)

```bash
# Any nested dylibs / helper binaries first, then the bundle.
codesign --force --options runtime --timestamp \
  --sign "Developer ID Application: <Name> (<TEAMID>)" \
  regattaClock.app/Contents/MacOS/regattaClock

codesign --force --options runtime --timestamp \
  --sign "Developer ID Application: <Name> (<TEAMID>)" \
  regattaClock.app
```

- `--options runtime` enables the **hardened runtime**, which notarization requires.
- Sign the **universal binary after `lipo`** (**C2**), not the per‑arch slices.
- **Entitlements**: a Fyne/GLFW GUI app normally needs none. Add
  `com.apple.security.cs.disable-library-validation` *only* if the app loads unsigned
  third‑party dylibs (it should not). Do not add JIT/`allow-unsigned-executable-memory`
  entitlements unless a real failure demands it.

### 3. Notarize

```bash
# Prefer an App Store Connect API key (.p8 + key id + issuer id) over an app‑specific password.
xcrun notarytool submit regattaClock.dmg \
  --key   AuthKey_XXXX.p8 \
  --key-id   "$ASC_KEY_ID" \
  --issuer   "$ASC_ISSUER_ID" \
  --wait
```

Notarize the DMG (Apple inspects the `.app` inside it). Fix any `codesign`/hardened‑runtime
findings it reports.

### 4. Staple

```bash
xcrun stapler staple regattaClock.app
xcrun stapler staple regattaClock.dmg
```

Stapling attaches the notarization ticket so Gatekeeper passes **offline**. Also sign the DMG
itself with the Developer ID before stapling it.

### 5. CI (on the existing `macos-latest` job)

- Create a temporary keychain, `security import` the base64‑decoded `.p12` with its password,
  `security set-key-partition-list` so `codesign` can use it unattended.
- Provide notary credentials as secrets: the `.p8` (base64), `ASC_KEY_ID`, `ASC_ISSUER_ID`.
- Gate the whole sign/notarize path on secret presence (**C9**) — forks still produce an
  unsigned DMG.
- Tools that cut boilerplate if wanted: `create-dmg` for the DMG layout, `quill` (Anchore) for
  sign+notarize without Xcode. The native `codesign`/`notarytool` route is fine since the job
  already runs on macOS.

## Distribution

- **GitHub Releases**: the signed, stapled universal DMG.
- **Homebrew Cask** (low effort once notarized): a cask needs only a stable URL + SHA‑256 and
  `depends_on macos`. Good reach for the CLI‑comfortable audience.
- **Mac App Store**: separate, sandboxed submission path with its own entitlement constraints.
  Out of scope.

## Verification

On a second Mac (not the build machine), after downloading the DMG:

```bash
spctl -a -vvv --type execute /Applications/regattaClock.app
#   → "accepted"  source=Notarized Developer ID
xcrun stapler validate /Applications/regattaClock.app
codesign --verify --deep --strict --verbose=2 /Applications/regattaClock.app
```

Gatekeeper should open the app on first launch with at most the standard "downloaded from the
internet" one‑time confirmation — no "cannot be opened" / "unidentified developer".

## Alternatives considered

| Approach | Verdict |
|----------|---------|
| Stay unsigned, document the Privacy & Security bypass | Fine for named testers; fails **R6** for public distribution and trains users to bypass Gatekeeper. |
| Self‑signed cert users manually trust | No real Gatekeeper path for this; notarization is still required. Not worth the friction. |
| MDM‑pushed Gatekeeper policy | Only works on machines you manage — the maintainer does not manage the target Macs. |
| `quill` / third‑party notarizers | Useful convenience wrappers; still need the $99/yr Developer ID. No cost saving, just less scripting. |
| Mac App Store | Solves trust but adds sandboxing + review; different goal. Revisit later. |
| Skip macOS distribution entirely | Acceptable *now* (maintainer runs a local build), but the stated mid‑term goal is an installable app, so plan for notarization. |

## Recommendation

Keep shipping the unsigned universal DMG **and the local‑build workflow** for the maintainer
in the near term. When $99/yr is acceptable, enrol in the Apple Developer Program and add
**Developer ID signing → hardened runtime → `notarytool` → `stapler`** to the existing
`macos-latest` job, gated on secrets. Publish the stapled DMG on GitHub Releases and add a
Homebrew cask. Do not pursue the Mac App Store.
