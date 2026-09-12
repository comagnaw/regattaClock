# rcprobe

A developer tool for exercising [`internal/regattacentral`](../../internal/regattacentral)
against the live RegattaCentral v4 API before any UI exists. It de-risks Phase A
of the [RegattaCentral integration](../../docs/features/personas/regattacentral-integration.md)
by confirming the token response, the JSON shapes, whether `/bulk` carries
per-seat eligibility, rate limits, and the `Origin` / referer requirement.

**Not shipped.** `release.yml` packages only `./cmd/regattaClock`, so rcprobe
stays out of releases while `go build ./...` still compiles it in CI. It imports
no Fyne, so it builds without CGO.

> **Never commit rcprobe output.** A real `/bulk` response contains athlete PII
> (names, ages, clubs) and `token` prints a live access token. The commands below
> write to `internal/regattacentral/testdata/`, which is **gitignored** for
> exactly this reason. Any committed test fixture under that path is hand-authored
> with synthetic data - captures inform the model, they are not the model.

## Credentials

rcprobe reads credentials from [`internal/secretstore`](../../internal/secretstore) -
never from a flag. By default it uses the `RC_*` environment variables; the
account must have **staff** access to the regatta.

```sh
export RC_CLIENT_ID=…
export RC_CLIENT_SECRET=…
export RC_USERNAME=…       # your RegattaCentral login
export RC_PASSWORD=…
export RC_API_KEY=…        # optional - see below
```

Or point it at a 0600 JSON file instead:

```sh
go run ./cmd/rcprobe --secrets-file ~/.config/regattaclock/rc-secrets.json …
```

```json
{
  "regattaClock": {
    "regattacentral/client_id": "…",
    "regattacentral/client_secret": "…",
    "regattacentral/username": "…",
    "regattacentral/password": "…",
    "regattacentral/api_key": "…"
  }
}
```

`RC_API_KEY` / `regattacentral/api_key` is **optional** - RegattaCentral's own
client-registration flow issues a `client-id` and an `API-Key` together as a
pair (its account-management page: *"your assigned 'API-key' and 'client-id'
will be displayed"*), and separately documents *"requests made ... using your
API-Key or Client-Id generated token"* - implying the API-Key is really an
**alternative to** the OAuth2 password grant (and, per an earlier reading of
the Cookbook, meant to travel in the `Authorization` header itself, not a
separate header). When set, this client currently sends it as an additional
`X-Api-Key` header alongside the OAuth token, not as an Authorization-header
replacement - a reasonable first guess, not a confirmed mechanism. See
[the investigation doc](../../docs/features/personas/heatsheet-rc-pivot-investigation.md)
for why this was added: a real `/upload` write returned `404` without it
ever being sent, and every read this project has made worked fine without
it, so its actual necessity (and the right way to send it) is still
unconfirmed.

## Capturing responses for reference

`--out DIR` writes each raw JSON response to `DIR/<name>.json`. Use these locally
to write the typed read model and to derive synthetic fixtures; they are **not**
committed (see the warning above).

```sh
RID=<regatta id>          # or use --config <personacfg file>
OUT=internal/regattacentral/testdata   # gitignored

go run ./cmd/rcprobe --regatta "$RID" --out "$OUT" token
go run ./cmd/rcprobe --regatta "$RID" --out "$OUT" walk    # bulk + every event's entries, one call

go run ./cmd/rcprobe --regatta "$RID" --out "$OUT" active
go run ./cmd/rcprobe --regatta "$RID" --out "$OUT" orgs
go run ./cmd/rcprobe --out "$OUT" search-orgs "<club name>"
go run ./cmd/rcprobe --out "$OUT" search-people "<lastname>" 1999-01-01
```

### walk command

`internal/regattacentral`'s Cookbook reading left it unconfirmed whether
`/bulk` nests full per-event entries or just event/regatta metadata (see
[regattacentral-integration.md](../../docs/features/personas/regattacentral-integration.md)).
`walk` gets everything reconciliation needs in one command instead of guessing
or hand-running `entries <eventID>` once per event:

1. Calls `bulk` and saves `bulk.json`, same as the `bulk` command.
2. Calls `orgs` and saves `organizations.json` (best-effort — a failure here is
   logged and does not stop the rest). A real capture showed an entry
   references its organization **by id only**, no inline name at all, which
   `cmd/rcreconcile` resolves against this file — see its README.
3. Walks the bulk response looking for event ids — structurally, not by a
   fixed path: any object with an `eventId` field, or an `id` field one level
   under something named like "events" (see `eventIDsFromBulk` in
   [`walk.go`](walk.go)).
4. Calls `entries <eventID>` for every id it found and saves each as
   `entries-<eventID>.json` — the same files, and same naming, the individual
   `entries` command would produce.

`--out` is required (there would be nowhere to put the results otherwise). A
failure on one event's entries is reported and does not stop the rest; `walk`
exits non-zero listing which ids failed, after saving everything it could.
[`cmd/rcreconcile`](../rcreconcile/README.md) reads a whole `--out` directory
like this at once via `--rc-dir`.

If `walk` finds 0 event ids, or `rcreconcile` still reports 0 entries after
running it, the `/bulk`/`entries` shape itself needs confirming — see the
`shape` command in [`cmd/rcreconcile`'s README](../rcreconcile/README.md).

## Commands

| Command | Endpoint |
|---|---|
| `token` | `POST /oauth2/api/token` (prints the access token) |
| `bulk [regattaID]` | `GET /regattas/{id}/bulk` |
| `walk [regattaID]` | `bulk`, then `entries` for every event id found in it — see [above](#walk-command) |
| `events [regattaID]` | `GET /regattas/{id}/events` |
| `entries <eventID> [regattaID]` | `GET /regattas/{id}/events/{eventID}/entries` |
| `lanes <eventID> [regattaID]` | `GET /regattas/{id}/events/{eventID}/lanes` |
| `results <eventID> [regattaID]` | `GET /regattas/{id}/events/{eventID}/results` |
| `active [regattaID]` | `GET /regattas/{id}/races?active` |
| `orgs [regattaID]` | `GET /regattas/{id}/organizations` |
| `search-orgs <name>` | `GET /organizations?name=…` |
| `search-people <lastname> <yyyy-mm-dd>` | `GET /participants?lastname=…&birthdate=…` |

`regattaID` falls back to `--regatta`, then to the `regattacentral.regattaID` in
a `--config` [deployment file](../../docs/features/personas/persona-config-file.md).

## Flags

| Flag | Purpose |
|---|---|
| `--secrets-file PATH` | JSON secrets file instead of `RC_*` env vars |
| `--config PATH` | personacfg deployment file for `baseURL` / `regattaID` |
| `--base-url URL` | override the API base URL |
| `--token-url URL` | override the OAuth2 token endpoint (point at a mock) |
| `--regatta ID` | regatta id (overrides `--config`) |
| `--origin VALUE` | send an `Origin` header |
| `--out DIR` | also write each raw response to `DIR/<name>.json` |
| `--timeout DUR` | per-request timeout (default `30s`) |

Flags come before the command: `rcprobe [flags] <command> [args]`.
