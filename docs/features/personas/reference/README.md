# Reference material

Vendored copies of external documents the persona design work depends on, kept
in-tree so the design docs can cite specific sections and so review does not
require chasing a live URL (some of which need a login).

## `RegattaCentral_APIV4_Cookbook.pdf`

RegattaCentral API V4.0 Cookbook — the vendor's integration guide, *"geared
toward the regatta management/timing application"*, which is regattaClock's role.

- **Source:** <https://api.regattacentral.com/v4/RegattaCentral_APIV4_Cookbook.pdf>
- **Companion online guide:** <https://api.regattacentral.com/v4/apiV4.jsp>
  (endpoint list, auth, headers)
- **Schema:** `rc-api.xsd`, linked from the online guide; used to generate model
  types.
- **Retrieved:** 2026-09-10 (8 pages, 15 numbered sections).

Cited by section number from
[../regattacentral-integration.md](../regattacentral-integration.md) — e.g. §3
Authentication, §5 Downloading Regatta Entry Information (`/bulk`), §14 Timing
Milestones and Results, §15 Lane Draw. Re-fetch and diff against the live copy
before starting implementation; RegattaCentral revises it without versioning.
