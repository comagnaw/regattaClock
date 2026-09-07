# Windows: public trust (unmanaged machines)

Long‑term path to **R5** — people who are not on the maintainer's domain can install
`regattaClock` without fighting SmartScreen. Framed for an **individual maintainer with no
legal entity** (**C6**) and **no budget for now** (**R8** applies only to the short term; this
is where a small spend eventually becomes worthwhile).

> Program names, prices, and eligibility below are current as of 2026‑09 and change often.
> Verify with each vendor before committing.

## What public users actually hit

- **SmartScreen App Reputation.** Microsoft scores a download by the *signing certificate's*
  accumulated reputation (downloads over time without malware reports) and by the file hash.
  A brand‑new certificate has **no** reputation, so early downloads still show "Windows
  protected your PC → More info → Run anyway" even when correctly signed. Reputation builds
  over weeks/months and is **reset** if you change certificates (**C5**).
- **"Unknown publisher"** on unsigned binaries, and in some enterprise configs a hard block.
- **Microsoft Defender** may quarantine an unsigned, low‑prevalence executable outright.

The internal certificate from [windows-internal-pki.md](windows-internal-pki.md) does
**nothing** here — it is trusted only on machines where you pushed it by Group Policy / Intune.

> **If you adopted Azure Trusted Signing *Private Trust* for the domain** (Option C in
> [windows-internal-pki.md](windows-internal-pki.md)), going public is a **validation‑type
> change in the same service**, not a new system: you add a
> **Public Trust** identity validation (Microsoft verifies your identity), point the same
> `azure/trusted-signing-action` at the public certificate profile, and keep the same OIDC
> auth and CI wiring. That makes "Azure Trusted Signing" below the natural continuation rather
> than a fresh adoption.

## Options

| Option | Who can get it | Cost | CI fit | SmartScreen reputation | Notes |
|--------|----------------|------|--------|------------------------|-------|
| **SignPath Foundation** (OSS program) | Established open‑source projects that meet their criteria | **Free** | GitHub Actions integration provided | Accrues like any OV cert | They supply the certificate and a signing service; you do not hold the key. Approval is a process, not instant. |
| **Azure Trusted Signing**, individual tier (Public Trust) | Individuals + orgs; identity verified by Microsoft | **~$9.99/month** (~$120/yr) | First‑class: `azure/trusted-signing-action`, **OIDC** — no key stored in CI | Accrues; chains to a Microsoft root | Certificates are short‑lived (issued per‑signing); Microsoft operates the HSM. Cheapest reliable paid path. **Same service and CI wiring as Private Trust** — if that is already in use for the domain, this is a validation‑type upgrade, not a migration. |
| **OV certificate on a hardware token** (Sectigo, DigiCert, …) | Individuals (with ID vetting) or orgs | **~$200–400/yr** | Poor: since 2023 (**C3**) the key must be on a FIPS token or cloud HSM. CI needs the token on a self‑hosted runner, or a Key Vault + `jsign` variant | Accrues over time | High friction for a solo maintainer. |
| **EV certificate** | Historically orgs only; a few CAs now do individuals | **~$300–600/yr** | Same hardware‑key friction as OV | **Immediate** — EV bypasses the reputation warm‑up | Overkill for this project's stage and budget. |
| **Microsoft Store** (MSIX package) | Individual dev account, **$19 one‑time** | $19 once | Separate packaging + Store submission pipeline | Store apps are SmartScreen‑trusted by default | Requires MSIX packaging and passing Store certification. Out of current scope (no MSIX in this plan); revisit if Store distribution becomes a goal. |

## Reputation mechanics worth knowing

- Reputation is **per certificate**, not per project. Moving from SignPath to Azure Trusted
  Signing (or renewing onto a new cert identity) starts the warm‑up again.
- Signing **consistently from the first public release** is what accumulates it. An
  intermittently‑signed history helps little.
- **EV** is the only option that skips the warm‑up. Nothing else — including a paid OV cert —
  gives day‑one silent installs.
- Distributing the same signed file through more channels (GitHub Releases + a website +
  winget) accelerates reputation because total download volume is what counts.

## Recommendation

1. **Apply to [SignPath Foundation](https://signpath.org/)** for the OSS program. It is the
   only path that is both free and produces genuine public trust, and it keeps the private key
   out of your hands entirely. Expect a review; start early.
2. **Until a public certificate is in place**, ship the public Windows build **unsigned but
   with strong provenance** — `SHA256SUMS` and GitHub build‑provenance attestations
   ([ci-and-provenance.md](ci-and-provenance.md)) — and give users a short, honest
   "SmartScreen will warn you; here's how to verify the download" section in the README.
3. **If SignPath is not an option** (declined, or you want control of timing), adopt
   **Azure Trusted Signing** at ~$120/yr. Use GitHub **OIDC** so no signing key or long‑lived
   credential is stored in CI (**R3**). This chains to a Microsoft root and warms up
   SmartScreen reputation over the following weeks. **If Trusted Signing Private Trust is
   already signing the domain build** (Option C in
   [windows-internal-pki.md](windows-internal-pki.md)), this is the default choice — add a
   Public Trust validation to the same account rather than onboarding SignPath.
4. **Do not** buy a hardware‑token OV or EV certificate at this stage. The recurring cost plus
   the CI friction from mandatory hardware key storage (**C3**) is not justified for an
   alpha‑stage, individually‑maintained tool.
5. Whichever certificate you land on, **stick with it** — switching resets reputation
   (**C5**). Always RFC 3161 timestamp (as in [windows-internal-pki.md](windows-internal-pki.md)).

## Alternatives considered

| Approach | Verdict |
|----------|---------|
| Stay unsigned forever, document the "Run anyway" click | Acceptable as a stopgap; not acceptable long‑term for **R5**. Some corporate environments block unsigned binaries entirely. |
| Buy the cheapest OV cert now | Spends money for a key‑handling problem (**C3**) and still no day‑one trust. SignPath (free) or Trusted Signing (cheaper, OIDC) dominate it. |
| Go straight to EV for instant trust | Real benefit (no warm‑up) but wrong cost tier for now, and historically needs an entity (**C6**). Reconsider if the project gains an organisation and a wide audience. |
| Microsoft Store as the primary channel | Solves trust cleanly but adds MSIX packaging + Store certification, which this plan explicitly scoped out. Keep as a future option. |
| Ask users to disable SmartScreen | Never. |
