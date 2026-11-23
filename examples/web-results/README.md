# Publishing regatta results to the public

regattaClock's Primary Finish Timer (PFT) publishes each race once the referee approves it. **Publish Destinations…** (PFT app menu, also on the Configuration screen) chooses where the results go, per regatta. Tick as many as you like; one **Publish** click writes them all.

| Destination | What the public sees | What you need | Effort |
| --- | --- | --- | --- |
| **Results workbook (Excel)** | An `.xlsx` results sheet, shared by link | An OneDrive (or other) folder you can share | None: built in |
| **Web results (JSON) on Google Sites** | A results page on your Google Sites website | A Google account and a Google Sites site | About an hour, once. See [`google-sites/`](google-sites/README.md) |
| **Web results (JSON) on your own web host** | The same results page, on your own site | Any web server (Apache, nginx, shared hosting) | About an hour, once. See [`static/`](static/README.md) |
| **RegattaCentral** | Results on RegattaCentral | — | Coming soon: not available yet |

The two web options use the **same page and the same files**. Choose by where your website already lives. Both can run from one PFT folder at the same time.

**Audience size matters for Google Sites.** Google allows only 30 of the script's executions at once, so a large regatta's audience reloading after popular races can hit that limit. See the [worked example and mitigations](google-sites/README.md#capacity-and-rate-limits-a-risk-for-large-regattas). A plain web host has no such limit.

## What the PFT writes

With **Web results (JSON)** ticked, every Publish rewrites one file per regatta in the folder the PFT chose:

```text
results-<season>-<regatta-name>.json      e.g. results-2026-spring-sprints.json
```

- The season is the regatta date's year.
- The file is rewritten in place, so its name, and its identity in a synced folder, never change during a regatta.
- It holds every race published so far. A race is only published once approved, and if the referee later reopens it, the file keeps its last published result until it is approved and published again.

The [`sample-data/`](sample-data/) folder has three real examples, written by the app itself, covering two seasons and the scratched, DQ and DNF cases.

## The file format (schemaVersion 1)

Anything that can read JSON can show these results. Readers **must ignore fields they don't know**, so the format can grow without breaking existing pages.

| Field | Meaning |
| --- | --- |
| `schemaVersion` | `1`. A higher number means a format this page may not understand. |
| `generator` | Always `"regattaClock"`. Tools skip files without it. |
| `regatta.name`, `regatta.date`, `regatta.season` | The regatta. `date` is `YYYY-MM-DD` when the schedule's date could be read, otherwise as written. |
| `regatta.regattaKey` | A stable ID for this regatta, which tells same-named regattas in different years apart. |
| `publishedAt` | When the latest race in this file was published (UTC, RFC 3339). |
| `races[]` | One entry per published race, in race order. |
| `races[].raceNumber`, `title`, `scheduledTime`, `boatClass`, `flightInfo` | The race, as on the heat sheet. |
| `races[].winningTime` | The winner's time. |
| `races[].approvedAt`, `races[].publishedAt` | When the referee approved it, and when it was published (UTC). |
| `races[].revision` | Changes whenever anything visible in the race changes. |
| `races[].cancelled` | Optional. `{reason, note}` for a race that was cancelled: `reason` is `weather`, `water` (water / course conditions) or `other` (then `note` says why). Its `results` are empty; its `lanes` still list the boats entered. |
| `regatta.cancelled` | Optional. The same shape for a regatta cancelled outright. Its `races` list is empty. |
| `races[].progression` | Optional. A heat's advancement rule, e.g. `Top 3 to Final` or `Top 2 to Final, Next 2 to Petite Final`. The page shows it after the race title. Imported past results carry it; the app doesn't write it yet. |
| `races[].lanes` | Lane number → `{school, additionalInfo, status}`. `status` is `"SCR"` for a scratched boat, otherwise empty. |
| `races[].results[]` | Finish order: `{place, lane, school, additionalInfo, split, time}`. `place` is `1`–`6`, or `DQ`, `DNF` or `DNS`. |
| `additionalInfo` (in lanes and results) | The heat sheet's info for that entry, often empty. For example `Exhibition`, `A` / `B` / `C` for a school's second or third entry, or a boat class when it differs from the race's (`W-Jr-2x`). |

## Getting the file from the PFT laptop to the website

The PFT only ever writes to a folder on the laptop. **For Google Sites,** that folder is a Google Drive for Desktop folder and there is nothing more to do. **For your own web host,** see [Race day in `static/README.md`](static/README.md#race-day-getting-results-from-the-pft-laptop-to-the-web-host). The recommended route is a Drive or OneDrive folder that the host pulls from every minute, which keeps working through venue internet drop-outs.

## Building your own page

[`static/index.html`](static/index.html) is a complete, framework-free reference, one file with no build step. A React, Vue or any other front end needs only two requests:

1. `index.json`: the list of regattas, written by [`static/build-index.py`](static/build-index.py): `{schemaVersion, generatedAt, regattas: [{season, name, date, regattaKey, file, publishedAt, cancelled?}]}`.
2. The chosen regatta's `file`, in the format above.

Fetch both without caching (for example `cache: "no-store"`), so a reload shows a race published a moment ago.
