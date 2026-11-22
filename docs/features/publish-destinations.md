# Publish destinations: Excel, web JSON, and later RegattaCentral

**Status (2026-11-21): design approved, not yet built.** Delivery is three PRs. See [Delivery](#delivery).

Today the Primary Finish Timer's (PFT) per-race **Publish** button writes exactly one thing: the regatta's results workbook (`<Name> - <Date> Results.xlsx`) into a folder the PFT picks, normally an OneDrive-synced one. This doc makes the destination a **per-regatta choice made on the PFT**. The PFT can enable **one or more** destinations, and one Publish click writes every enabled destination.

## Why

- **The OneDrive/Excel path is painful to run,** but the public may still expect the workbook, so it stays as an option rather than being replaced.
- **A web page is now proven.** [`examples/google-sites-poc/`](../../examples/google-sites-poc/README.md) serves results to the general public from the club's Google Sites website:
  - an Apps Script web app reads `results-*.json` from a **private Google Drive folder**;
  - Google Drive for Desktop on the PFT laptop keeps that folder synced;
  - the app is embedded in a Google Sites page.

  The SharePoint `.aspx` variant ([`examples/sharepoint-poc/`](../../examples/sharepoint-poc/README.md)) rendered too, but SharePoint Online can't host a page for anonymous visitors.
- **RegattaCentral (RC)** is a plausible third destination. Its write path exists only as a CLI on the `regattacentral-heatsheet-investigation` branch, so it isn't a guaranteed delivery. It needs a slot in the design, not an implementation.

## Decisions (author, 2026-11-21)

1. **Several destinations at once.** Publish fans out to every enabled destination. This lets the workbook stay as a fallback while the web page beds in.
2. **The PFT chooses, not the RD.** Every destination so far is a folder that only the PFT laptop has mapped (an OneDrive library, or a Google Drive for Desktop folder). The RD machine usually has neither, so it can't sensibly choose or check them.

   This **supersedes** the RD-owned `ResultsDestination` in [personas/operational-state.md](personas/operational-state.md). That field (`store.PublishConfig` in `regattaSchedule.json`) never had a writer and is removed.
3. **The choice is stored per regatta, in the PFT's `finish.json`,** next to the results folder the PFT already records there (`FinishLog.ResultsDir`).
   - `finish.json` lives in the shared `regattaData`, so a replacement PFT laptop inherits the choice.
   - A new regatta starts clean, the same rule as today's results folder.
4. **Every destination renders the same in-memory view** (`publish.BuildView` / `publish.ScheduleView`) and is never a source of truth. This is the "cache, never authority" rule in [future-result-driven-persona.md](personas/future-result-driven-persona.md).

## Destinations

| Kind ID | Label | Needs a folder | Output | Status |
| --- | --- | --- | --- | --- |
| `spreadsheet` | Results workbook (Excel) | yes | `<Name> - <Date> Results.xlsx`, with the hidden `_regattaClock` ledger sheet | shipped (#126, #127), unchanged |
| `webjson` | Web results (JSON) | yes | `results-<season>-<slug>.json` (schema below) | new |
| `regattacentral` | RegattaCentral | no | RC v4 API upload | registered as **coming soon**, disabled until the RC branch lands |

## Design

### Destination interface (`internal/publish`)

`Meta`, `LedgerEntry`, `Ledger` and `Published` (with `CheckRegatta`) move up from `internal/publish/spreadsheet`, because none of them is spreadsheet-specific. `spreadsheet` keeps type aliases, so its callers don't change.

```go
// A Destination renders the publish view into one external form.
type Destination interface {
    Read(dir string, meta Meta) (Published, error)   // prior state + ledger; missing = empty
    Write(dir string, meta Meta, races []PublishableRace, ledger Ledger) error
    FileName(meta Meta) string                       // for prompts and error messages
}

type Kind struct {
    ID          string // "spreadsheet", "webjson", "regattacentral"
    Label       string // from internal/common
    NeedsFolder bool
    Available   bool   // false = shown disabled, "coming soon"
    New         func() Destination
}

var Kinds = []Kind{ /* spreadsheet, webjson, regattacentral */ }
func KindByID(id string) (Kind, bool)
```

This follows two existing patterns: `persona.Registry` (a list of struct literals) and `secretstore.Backend` (one interface, several implementations).

- `dir` is ignored when `NeedsFolder` is false.
- RC's own parameters (base URL, regatta ID) already live in the deployment-level `personacfg`, and its credentials in `secretstore`.

### `webjson`: the format the Google Sites page reads (schemaVersion 1)

The file is `results-<season>-<slug>.json`:

- `season` is the regatta date's year;
- `slug` is the regatta name lower-cased, with runs of non-alphanumerics turned into `-`.

The Apps Script lists `results-*.json` in its folder, so **no `index.json`** is written or maintained.

```json
{
  "schemaVersion": 1,
  "generator": "regattaClock",
  "regatta": { "name": "…", "date": "YYYY-MM-DD", "season": "2026", "regattaKey": "12 hex" },
  "publishedAt": "RFC 3339 UTC",
  "races": [{
    "raceNumber": 1, "title": "Race 1 - W-1x - Final", "scheduledTime": "09:00 AM",
    "boatClass": "W-1x", "flightInfo": "Final", "note": "", "winningTime": "07:03.6",
    "approvedAt": "RFC 3339 UTC", "revision": "16 hex", "publishedAt": "RFC 3339 UTC",
    "lanes":   { "3": { "school": "…", "additionalInfo": "…", "status": "" } },
    "results": [{ "place": "1", "lane": "3", "school": "…", "additionalInfo": "…",
                  "split": "00:00.0", "time": "07:03.6" }]
  }]
}
```

This is the PoC format plus two fields. **`generator`** marks the file as RegattaClock's, so `Read` can refuse one it didn't write. A per-race **`publishedAt`** is the second. `regatta.date` is ISO when the schedule's date parses, so the page can sort regattas, and is passed through as written otherwise.

With `publishedAt`, the file carries its own ledger (`revision` + `publishedAt` per race), as the workbook's hidden sheet does. `Read` rebuilds `Published{RegattaKey, Ledger, Rows}` from the file, so re-publish detection and the carry-forward of un-approved races (`mergePublished`) work the same as for Excel.

**Writing:**

- The file is written in place with `filesystem.SaveBytesFileInPlace`, the same as the workbook. Keeping the file's identity keeps its Drive file ID stable, and the Apps Script's cache is keyed on that ID plus the last-modified time.
- A file with a different `regattaKey` is refused (`ErrOtherRegatta`). A `.json` file that isn't a regattaClock results file is refused as foreign. Both match the workbook's guards.

### PFT-owned settings (`finish.json`)

```go
type PublishSettings struct {
    Destinations []string          `json:",omitempty"` // kind IDs, in display order
    Dirs         map[string]string `json:",omitempty"` // kind ID -> folder, for NeedsFolder kinds
}
// FinishLog gains: Publish *PublishSettings `json:",omitempty"`
```

- **Old files keep working.** With no `Publish` block, the destinations are `["spreadsheet"]` and the spreadsheet folder is the legacy `ResultsDir`. Saving writes the new form and clears `ResultsDir`.
- **Per-machine "last used" folders** stay preferences and are only offered as defaults:
  - `PrefResultsDir` for the spreadsheet;
  - `PrefResultsDir + "." + kind` for the others.
- **Saving keeps today's `setResultsDir` guards:** publisher only, refused while writes are blocked, rolled back if the save fails.

### PFT UI

- **"Publish Destinations…" menu item.** It's PFT-only and appears when `isPublisher()` is true.
  - The dialog has one row per kind: a checkbox, the folder, and **Choose…** for kinds that need a folder.
  - RC is disabled with "coming soon".
  - Ticking a folder kind that has no folder yet opens the folder picker.
  - At least one destination must stay enabled.
- **Startup.** The existing folder flow (reachable / unreachable / offer last-used / pick) runs once for each enabled folder kind, in order. Prompts name the destination, for example "Choose the folder for Web results (JSON)…".
- **Configuration screen.** The single results-folder row becomes a read-only summary of the enabled destinations and folders, with a button that opens the same dialog.
- **Strings** live in `internal/common`. Styled text comes from `internal/text` factories (AGENTS.md).

### Publishing and button state

- **Each destination is read, merged and written independently.** A workbook open in Excel (`ErrLocked`) doesn't block the web JSON. Successful destinations record their ledger, and failures are collected into one dialog, worded per destination through the existing `publishErrorMessage` mapping.
- **"Published" means published everywhere.** The PFT keeps a ledger per kind. A race's "published revision" is its revision only when every enabled destination holds that same revision; otherwise it's blank. `derivePublishState` is unchanged: a destination that lags, failed, or was just added shows **Re-publish**, and re-publishing back-fills it.
- **Changing destinations mid-regatta** re-runs the ledger load for the new set.

## Existing-code reuse

- **Publish view:** `publish.BuildView`, `ScheduleView` and `Revision` (`internal/publish/publish.go`). Every destination renders from these.
- **Carry-forward:** `mergePublished` and `publishResults` (`internal/regatta/publish_results.go`). They become generic over `Destination` instead of calling `spreadsheet` directly.
- **Folder flow:** `ensureResultsDir`, `pickResultsFolder` and `setResultsDir`. They become parameterized by kind.
- **File writes:** `filesystem.SaveBytesFileInPlace`, which keeps file identity for OneDrive, Drive and Excel.
- **RC:** `internal/regattacentral`, `personacfg.RegattaCentral` and `secretstore` for the future RC `Destination`.

## Delivery

1. **This doc** (and the doc cross-references).
2. **Engine:**
   - the `Destination` interface and the registry;
   - `spreadsheet` adapted behind the interface;
   - the `webjson` package;
   - `PublishSettings` in `finish.json`, with the legacy fallback;
   - per-kind ledgers and fan-out publish;
   - `store.PublishConfig` removed.

   With nothing configured, behavior is exactly today's: spreadsheet only.
3. **PFT UI:** the "Publish Destinations…" dialog, the per-kind startup prompts, and the Configuration summary.

**When this can start:** now. Nothing blocks it. The RC destination alone stays gated on [heatsheet-rc-pivot-investigation.md](personas/heatsheet-rc-pivot-investigation.md) and the RC branch landing.

## Open

- **Excel's future.** Whether the workbook stays enabled by default for new regattas or becomes opt-in once the web page is established. Today's default is unchanged: `spreadsheet` only.
- **Cross-season school search** on the web page (loading every season's file, or a generated index). This is a page feature, independent of the app.
- **Season boundaries.** `season` is the calendar year of the regatta date. Revisit if a season ever spans a year boundary.
