# Windows: internal PKI for the managed domain

Short‑term path to **R1** (domain users, no prompts). The zero‑cost route (**R8**) is a
self‑signed cert distributed centrally; a managed ~$10/month route (Option C below) removes
the CI private key and doubles as the on‑ramp to public trust. Companion to
[README.md](README.md); packaging
is in [windows-packaging.md](windows-packaging.md); CI wiring is in
[ci-and-provenance.md](ci-and-provenance.md).

> Rules and menu paths below are current as of 2026‑09. Confirm against current Microsoft
> documentation before rolling this out.

## How Windows decides to trust an `.exe`

Three independent gates. All three must pass for a silent launch.

1. **Authenticode signature chain.** The binary carries a signature whose certificate chains
   to a root in the machine's **Trusted Root Certification Authorities** store. Without this
   the file is "unsigned"; with it but no further trust, the publisher shows as verified but
   still triggers a prompt in some contexts.
2. **Trusted Publishers.** If the *signing* certificate (or its issuer) is also present in the
   machine's **Trusted Publishers** store, Windows treats the publisher as pre‑approved: no
   "Do you want to allow this app from an unknown publisher?" prompt, and AppLocker / WDAC
   publisher rules can allow‑list the app by its certificate subject.
3. **SmartScreen + Mark‑of‑the‑Web (MOTW).** Separate from Authenticode. A file downloaded
   from the internet zone gets an alternate data stream (`Zone.Identifier`); SmartScreen then
   checks the file/publisher reputation with Microsoft. An internal certificate has **no**
   reputation, so a downloaded internally‑signed `.exe` can still get "Windows protected your
   PC". The fix on a managed fleet is to **not** deliver it through the internet zone — see
   [MOTW](#mark-of-the-web) below.

On the domain you control all three. Off the domain you control only #1 — that is what
[windows-public-trust.md](windows-public-trust.md) is about.

## Getting a code‑signing certificate

Three ways to obtain the certificate. Pick by what the domain already runs and how it is
managed.

> **Cloud‑managed (Entra / Intune) domains.** If devices are Entra‑joined and managed from
> [entra.microsoft.com](https://entra.microsoft.com) / Intune with **no on‑prem Active
> Directory**, then **Option A does not apply** — there is no AD CS to enrol against, and
> Entra ID itself does not issue code‑signing certificates. Choose **Option B** (zero cost) or
> **Option C** (managed, ~$10/month). Trust is still delivered centrally — by Intune instead of
> Group Policy, see [Deploying trust to the fleet](#deploying-trust-to-the-fleet). A *hybrid*
> domain (on‑prem AD DS synced to Entra) can still use Option A.

### Option A — Enterprise CA (AD CS)

Requires **on‑prem Active Directory Certificate Services** with an enterprise root (not
available on an Entra‑only domain). If you have it:

1. On the CA, duplicate the built‑in **Code Signing** certificate template. Give it a long
   validity (e.g. 3–5 years), require the private key be exportable only if you need it off the
   issuing machine, and publish the template.
2. Enrol for a certificate from a **dedicated signing account/box** (not every developer
   laptop). Keep the private key on that box or export it once, password‑protected, for CI.
3. Nothing else to distribute: the enterprise root is already in every domain machine's
   Trusted Root store via autoenrollment, and you can publish the issuing CA to **Trusted
   Publishers** with Group Policy (below). Revocation (CRL/OCSP) is available if a key leaks.

**Recommended when AD CS already exists** — you get revocation and central lifecycle for free.

### Option B — one self‑signed code‑signing certificate

If there is no CA (or standing one up is not worth it for 7–10 PCs), skip building a CA
hierarchy and issue a single long‑lived code‑signing certificate:

```powershell
# On a trusted admin box. One cert, used directly for signing.
$cert = New-SelfSignedCertificate `
  -Type CodeSigningCert `
  -Subject "CN=regattaClock Signing, O=<your org>" `
  -KeyExportPolicy Exportable `
  -KeyUsage DigitalSignature `
  -KeyAlgorithm RSA -KeyLength 3072 `
  -CertStoreLocation Cert:\CurrentUser\My `
  -NotAfter (Get-Date).AddYears(5)

# Public cert, to push to the fleet:
Export-Certificate -Cert $cert -FilePath regattaClock-signing.cer

# Private key + cert for the CI signer, password‑protected:
$pw = Read-Host -AsSecureString "PFX password"
Export-PfxCertificate -Cert $cert -FilePath regattaClock-signing.pfx -Password $pw
```

There is no separate root here — the self‑signed certificate **is** the trust anchor, so it
goes into **both** Trusted Root and Trusted Publishers on the fleet. Rotating it before expiry
means re‑pushing the new `.cer` by Group Policy and re‑signing subsequent releases; old
releases keep validating if they were timestamped (see [Timestamping](#timestamping)).

**Recommended when there is no CA and cost must be zero** — minimal moving parts for a small
fleet. The tradeoff is no revocation: if the key leaks, your remedy is to remove the cert by
GPO/Intune and issue a new one, and you hold a private key in CI.

### Option C — Azure Trusted Signing, Private Trust (~$10/month)

A Microsoft‑managed signing service, authenticated with your **Entra** identity — a natural
fit for a cloud‑managed domain. In **Private Trust** mode it signs `regattaClock` with a
certificate that chains to a root **you** own and distribute internally (it is *not* publicly
trusted — that is the separate **Public Trust** mode in
[windows-public-trust.md](windows-public-trust.md), same service).

- **No private key anywhere you manage.** Microsoft holds the key in its HSM; CI signs by
  calling the service. GitHub Actions authenticates with **Entra OIDC / workload identity
  federation** — no `.pfx`, no long‑lived secret (satisfies **R3** fully).
- **Certificates are short‑lived** and minted per signing operation; nothing to rotate on the
  fleet as long as the internal root you publish stays constant.
- **Intune‑documented rollout.** Microsoft documents pushing the Trusted Signing internal
  root + a **Trusted Publishers** entry, and matching **App Control for Business / WDAC**
  rules, through Intune — the same distribution described below.
- **One tool, both audiences.** When you later want the public build trusted off‑domain, you
  switch that pipeline to a **Public Trust** account rather than adopting a second system.

Cost is roughly **$9.99/month** plus per‑signature fees at high volume (negligible here).
*Verify current Private Trust availability, identity‑validation requirements, and Intune
integration steps before committing.*

**Recommended when ~$10/month is acceptable** — removes the CI private key, matches an
Entra/Intune shop, and is the on‑ramp to public trust.

## Deploying trust to the fleet

The certificate (Option B's `.cer`, or the internal root from Option A / Option C) must reach
two certificate stores on every managed PC. Two delivery mechanisms, same end state.

### Group Policy (on‑prem / hybrid AD)

Group Policy, `Computer Configuration → Policies → Windows Settings → Security Settings →
Public Key Policies`:

| Store | What goes here | Why |
|-------|----------------|-----|
| **Trusted Root Certification Authorities** | Option A: the enterprise root (usually already there). Option B: the self‑signed `.cer`. Option C: the Trusted Signing internal root. | Makes the Authenticode chain valid (gate #1). |
| **Trusted Publishers** | Option A: the issuing CA cert. Option B: the same self‑signed `.cer`. Option C: the Trusted Signing internal root / issuing CA. | Suppresses the "unknown publisher" prompt and enables publisher‑based App Control / WDAC rules (gate #2). |

GPO has a dedicated policy node for **both** stores, so this is one step.

### Intune (Entra‑managed)

Intune is the Group Policy replacement for an Entra‑joined fleet, but the two stores are
reached differently:

| Store | How | Notes |
|-------|-----|-------|
| **Trusted Root** (and Intermediate) | `Devices → Configuration → Create profile → Templates → Trusted certificate` | Built‑in profile type. Upload the `.cer`; scope to a device group. This is the *only* store the Trusted certificate profile can target. |
| **Trusted Publishers** | **Settings Catalog / custom OMA‑URI**, or an Intune **platform script** (PowerShell, runs as SYSTEM) | The Trusted certificate profile **cannot** write here. Use a one‑line script: `Import-Certificate -FilePath "$PSScriptRoot\regattaClock-signing.cer" -CertStoreLocation Cert:\LocalMachine\TrustedPublisher` (ship the `.cer` inside a small Win32 app, or inline it as base64). |

If you also want publisher allow‑listing, deploy the matching **App Control for Business /
WDAC** policy from `Endpoint security → App Control for Business`.

### Verify (either mechanism)

On a managed test machine:

```powershell
Get-ChildItem Cert:\LocalMachine\Root, Cert:\LocalMachine\TrustedPublisher |
  Where-Object Subject -match regattaClock
```

### Mark-of-the-Web

Even correctly signed, a build **downloaded from GitHub Releases** carries MOTW and can still
hit SmartScreen. On the managed fleet, deliver releases so the internet zone flag is never
set:

- Copy the signed installer to an **internal file share** (UNC path in the Local Intranet
  zone) and point users there, or
- Push it with **Intune** / **SCCM** / a **GPO Software Installation** package (MSI — see
  [windows-packaging.md](windows-packaging.md)).

Belt‑and‑braces: the **"Configure App Install Control"** / SmartScreen Group Policy can be set
to warn‑only or off for managed devices, but avoiding MOTW is cleaner and keeps SmartScreen
protection intact for everything else.

## Key handling at this scale

Applies to **Options A and B** (you hold the private key). **Option C has no key to
handle** — skip this section if you go that route.

- Store the `.pfx` password‑protected, in a restricted location (admin box or a small secrets
  manager). For CI, hold it as a **base64‑encoded GitHub Actions secret** plus a separate
  password secret (details in [ci-and-provenance.md](ci-and-provenance.md)).
- This file‑based key is acceptable **for an internal LOB certificate**. It is **not**
  permitted for a publicly‑trusted certificate (constraint **C3**) — do not reuse this pattern
  when you move to [windows-public-trust.md](windows-public-trust.md).
- Limit the certificate's reach: code‑signing EKU only, no other usages.

## Signing in CI

The Windows binary is cross‑built on Linux (**C1**), so sign it there with
[`osslsigncode`](https://github.com/mtrojnar/osslsigncode) (in Debian/Ubuntu repos) — no
Windows runner needed:

```bash
osslsigncode sign \
  -pkcs12 signing.pfx -pass "$PFX_PASSWORD" \
  -n "regattaClock" -i "https://github.com/comagnaw/regattaClock" \
  -h sha256 \
  -ts http://timestamp.digicert.com \
  -in  regattaClock.exe \
  -out regattaClock-signed.exe
```

Alternatives for Options A/B: [`jsign`](https://ebourg.github.io/jsign/) (Java, also speaks
Azure Key Vault / PKCS#11 for later), or a dedicated `windows-latest` job running `signtool
sign /fd SHA256 /tr <tsa> /td SHA256` (mirrors the MinGW‑on‑Windows job already in
[`.github/workflows/test.yml`](../../../.github/workflows/test.yml)).

**Option C (Azure Trusted Signing)** uses the
[`azure/trusted-signing-action`](https://github.com/Azure/trusted-signing-action) instead of a
`.pfx` — no key material in the workflow. Authenticate the job to Entra with
`azure/login` + **OIDC** (`id-token: write`), point the action at your Trusted Signing account
/ certificate profile, and give it the file to sign. Timestamping is handled by the service.
See [ci-and-provenance.md](ci-and-provenance.md#secrets-vs-oidc).

Sign **both** the raw `.exe` and, once it exists, the installer
([windows-packaging.md](windows-packaging.md)).

### Timestamping

Always pass an RFC 3161 timestamp URL (`-ts` / `/tr`). A timestamped signature stays valid
after the signing certificate expires or is rotated, so releases you cut today keep verifying
next year. Free TSAs: `http://timestamp.digicert.com`, `http://timestamp.sectigo.com`.

## Verifying a signed build

On any Windows machine:

```powershell
Get-AuthenticodeSignature .\regattaClock.exe   # Status should be 'Valid', SignerCertificate = your cert
signtool verify /pa /v .\regattaClock.exe       # if the Windows SDK is present
```

End‑to‑end, on a domain test VM with the GPO/Intune trust applied and the file opened from the
internal share or an Intune deployment: it should launch with **no** SmartScreen and **no**
unknown‑publisher prompt.

## Alternatives considered

| Approach | Verdict |
|----------|---------|
| Do nothing; tell operators to click "Run anyway" | Rejected — fails **R1**; fragile on locked‑down machines; poor look for a race‑day tool. |
| Ship unsigned, allow‑list by **file hash** in AppLocker/WDAC | Works but every release needs a policy update pushed before operators can run it. Publisher rules with a signed binary are set‑once. |
| Full internal **two‑tier CA** (offline root + issuing CA) just for this | Overkill for 7–10 PCs and one app. Revisit only if the org needs internal PKI for other reasons. |
| **Microsoft Cloud PKI** (Intune Suite add‑on) as the issuing CA | It is a real hosted CA managed from Intune, but it is built to enrol *users/devices* through Intune certificate profiles, not to hand a signing cert to a build server — awkward for CI, and heavier licensing. Option C (Trusted Signing) is the cloud‑native fit. *Verify whether a Code Signing EKU is even a supported Cloud PKI scenario.* |
| Buy a public OV/EV cert now and use it internally too | Spends money now (**R8**), and **C3** forces hardware‑token/HSM key storage that complicates CI. It also does not help the managed fleet any more than an internal cert does. |
| Disable SmartScreen fleet‑wide by GPO/Intune | Reduces protection for *all* software. Avoiding MOTW via internal distribution is narrower and safer. |

## Recommendation

1. **Pick the certificate source by domain type and budget:**
   - Hybrid domain with **AD CS** → Option A.
   - Entra‑only domain, **zero cost required** → Option B (one self‑signed code‑signing cert;
     no CA hierarchy).
   - Entra‑only domain, **~$10/month acceptable** → **Option C, Azure Trusted Signing Private
     Trust** — no CI key, Entra‑native, and the on‑ramp to public trust.
2. **Distribute the trust centrally:**
   - Group Policy (hybrid) → one policy node covers **Trusted Root + Trusted Publishers**.
   - Intune (Entra) → **Trusted certificate** profile for the root, plus a **platform
     script / OMA‑URI** to add the same cert to **Trusted Publishers** (the profile cannot).
3. Deliver releases to the fleet from an **internal file share or Intune/SCCM**, not by having
   users download from GitHub, so MOTW/SmartScreen never engages.
4. **Sign in CI:**
   - Options A/B → **`osslsigncode`** on the existing Linux job, always **RFC 3161
     timestamped**, gated on secret presence (**C9**).
   - Option C → **`azure/trusted-signing-action`** with `azure/login` + OIDC — no stored key.
5. For Options A/B, keep the `.pfx` as encrypted CI secrets and treat the key as
   internal‑only; do not carry it over to public signing. Option C has no key to protect.
