# rcprobe

A developer tool for exercising [`internal/regattacentral`](../../internal/regattacentral)
against the live RegattaCentral v4 API before any UI exists. It de-risks Phase A
of the [RegattaCentral integration](../../docs/features/personas/regattacentral-integration.md)
by confirming the token response, the JSON shapes, whether `/bulk` carries
per-seat eligibility, rate limits, and the `Origin` / referer requirement.

**Not shipped.** `release.yml` packages only `./cmd/regattaClock`, so rcprobe
stays out of releases while `go build ./...` still compiles it in CI. It imports
no Fyne, so it builds without CGO.

## Credentials

rcprobe reads credentials from [`internal/secretstore`](../../internal/secretstore) -
never from a flag. By default it uses the `RC_*` environment variables; the
account must have **staff** access to the regatta.

```sh
export RC_CLIENT_ID=…
export RC_CLIENT_SECRET=…
export RC_USERNAME=…       # your RegattaCentral login
export RC_PASSWORD=…
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
    "regattacentral/password": "…"
  }
}
```

## Capturing the Phase A goldens

`--out DIR` writes each raw JSON response to `DIR/<name>.json`. These captures
become the fixtures that drive the typed read model.

```sh
RID=<regatta id>          # or use --config <personacfg file>
OUT=internal/regattacentral/testdata

go run ./cmd/rcprobe --regatta "$RID" --out "$OUT" token
go run ./cmd/rcprobe --regatta "$RID" --out "$OUT" bulk
go run ./cmd/rcprobe --regatta "$RID" --out "$OUT" events

# pick an <eventID> from events.json, then:
go run ./cmd/rcprobe --regatta "$RID" --out "$OUT" entries <eventID>
go run ./cmd/rcprobe --regatta "$RID" --out "$OUT" lanes   <eventID>
go run ./cmd/rcprobe --regatta "$RID" --out "$OUT" results <eventID>

go run ./cmd/rcprobe --regatta "$RID" --out "$OUT" active
go run ./cmd/rcprobe --regatta "$RID" --out "$OUT" orgs
go run ./cmd/rcprobe --out "$OUT" search-orgs "<club name>"
go run ./cmd/rcprobe --out "$OUT" search-people "<lastname>" 1999-01-01
```

## Commands

| Command | Endpoint |
|---|---|
| `token` | `POST /oauth2/api/token` (prints the access token) |
| `bulk [regattaID]` | `GET /regattas/{id}/bulk` |
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
