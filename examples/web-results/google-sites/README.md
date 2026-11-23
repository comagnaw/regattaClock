# Web results on Google Sites (Apps Script)

Show regatta results to the **general public** on a Google Sites website, with nothing to run or host yourself.

- The PFT laptop writes results JSON into a **private Google Drive folder** that Google Drive for Desktop keeps synced.
- A **Google Apps Script web app** reads that folder and serves the results page.
- The page is **embedded** on a Google Sites page.

This is one of the hosting options in [`../README.md`](../README.md). It uses the same results page and the same JSON files as the [plain web host option](../static/README.md); only the hosting differs.

## How the pieces fit

```text
  PFT laptop                    Google (deploying account)                       Public
  ──────────                    ──────────────────────────                       ──────
  Drive for Desktop             ① Drive folder (PRIVATE, never shared)
   …/My Drive/            ──►     regattaClock Results/
    regattaClock Results/          ├─ results-2026-sample-regatta-day.json
                                   └─ results-2025-spring-sprints.json
                                           │  read by the script, as the owner
                                           ▼
                                ② Apps Script web app                     ◄── direct link
                                   Code.gs, Index.html                         script.google.com/macros/s/<ID>/exec
                                           │                                   ?season=2025&regatta=<key>&school=<text>
                                           ▼  embedded (iframe)
                                ③ Google Sites "Results" page             ◄── site navigation / anchor links
                                                                               sites.google.com/view/<site>/results
```

| File | Purpose |
| --- | --- |
| `Code.gs` | Server side. `doGet` serves the page with the regatta list and the first regatta's results already in it; `getIndex` and `getResults` answer **Reload** and later picks; `checkSetup` is a one-time check, and `installWarmer` an optional race-day helper, both run from the editor. |
| `Index.html` | The page: the same layout, pickers and school filter as [`../static/index.html`](../static/index.html). It asks `Code.gs` for its data with `google.script.run`. |
| `appsscript.json` | The manifest. It turns on the **Drive API** advanced service (lists the folder in one call), sets the Drive read-only and trigger scopes, and the web-app settings (runs as you, anyone can open it). |

**Points about the design:**

- **No `index.json` to maintain.** The script builds the catalog by listing the folder. Any file named `results-*.json` that parses appears under its `regatta.season`. Other files are ignored, and a half-synced file that doesn't parse yet is skipped.
- **The folder stays private.** Visitors never get a Drive link. `getResults` only returns files that are inside the configured folder, so a guessed file ID can't read anything else in the account.
- **Fast, whatever the folder holds.** The regatta list is cached for 5 minutes. Rebuilding it lists the folder in one Drive API call and opens only files it hasn't seen before; each file's regatta name and date are remembered in the script's properties. The page arrives with the list and the newest regatta's results inside it, so the first view makes no further calls.
- **No staleness for results.** Each results file is cached under its ID plus its last-modified time. A new sync changes that time, so the next page load or **Reload** shows the new results immediately. Only a **new** regatta file can take up to 5 minutes to appear in the list.
- **If two files contain the same regatta,** the newer file wins. This can happen with a duplicate web upload.

## Setup

### ① Drive folder

1. In Google Drive (the account that will own the web app), create a folder, for example **`regattaClock Results`**. **Don't share it.**
2. To try it before a real regatta, upload the three sample files from [`../sample-data/`](../sample-data/). On race day the PFT writes its own file here.
3. Open the folder and copy the ID from the address bar: `drive.google.com/drive/folders/`**`<FOLDER_ID>`**.

> **Which account?** The script needs read access to the Drive of the account that deploys it. A dedicated club account (for example `results@<club domain>`) that owns only this folder keeps that access narrow. A Google Workspace account (free for nonprofits) also helps with the banner noted under [Things to check](#things-to-check).

### ② Apps Script web app

1. Go to [script.google.com](https://script.google.com) → **New project**, and name it, for example, *regattaClock Results*.
2. Replace the contents of `Code.gs` with this folder's `Code.gs`, and set `FOLDER_ID` at the top.
3. Choose **+** next to *Files* → **HTML**, name it **`Index`** (the editor adds `.html`), and paste in `Index.html`.
4. Under **Project Settings**, tick *Show "appsscript.json" manifest file*, then replace that file's contents with this folder's `appsscript.json`. Change `timeZone` if needed. This also turns on the Drive API service. Without it, the page still works but rebuilds the regatta list far more slowly (about 0.4 s per file in the folder).
5. In the editor, choose the **`checkSetup`** function and **Run** it. Approve the permission prompts. The log should list the three sample regattas and how long building the list took.
6. **Deploy → New deployment**, gear → **Web app**:
   - *Execute as:* **Me**
   - *Who has access:* **Anyone**

   Choose **Deploy** and copy the **Web app URL**. It ends in `/exec`.
7. Open that URL in a **private / incognito window**. You should see the results with no Google sign-in. Try `…/exec?season=2025&school=coldspring`.

### ③ Google Sites page

1. In the site editor, add a page, for example **Results**.
2. **Insert → Embed → By URL**, paste the `/exec` URL, choose **Whole page**, and **Insert**. Drag the embed's bottom edge to make it tall; a few thousand pixels is fine, because the page scrolls inside the frame.
3. **Publish**. Results now appears in the site navigation. For anchor links elsewhere on the site or on social posts, link to this Sites page: `sites.google.com/view/<site>/results`, or the same path on your custom domain.
4. Check the published page in a private window.

### Updating the code later

Paste the new `Code.gs`, `Index.html` and `appsscript.json`, then use **Deploy → Manage deployments → edit (pencil) → Version: New version → Deploy**. That keeps the same `/exec` URL. Choosing **New deployment** creates a **new URL** and breaks the Sites embed and any shared links.

### Optional: keep it warm on race day

Run **`installWarmer`** once from the editor (approve the prompt). It rebuilds the regatta list every 5 minutes, so no visitor ever waits for a rebuild, and new regatta files appear within 5 minutes even with no visitors. **`removeWarmer`** turns it off. Each run takes about a second and counts as one execution.

## Linking behavior

- **Deep links don't pass through the Sites embed.** The embed URL is fixed when you insert it, so adding `?season=2025` to the Sites page's own URL has no effect. Two ways to link to a specific season or school:
  - Share the `/exec` URL with the selection added, e.g. `…/exec?season=2025&school=coldspring`. It opens the page on its own, outside the site.
  - Or create one Sites page per season, each embedding `/exec?season=<year>`.
- **Links inside the app** open in the top window (`<base target="_top">`) or a new tab. Apps Script's sandbox frame can't navigate itself.

## Race day: connect the PFT

1. Install [Google Drive for Desktop](https://www.google.com/drive/download/) on the PFT laptop and sign in with the folder's account. The folder appears at:
   - **Windows:** `G:\My Drive\regattaClock Results\` (the drive letter can differ)
   - **macOS:** `~/Library/CloudStorage/GoogleDrive-<account>/My Drive/regattaClock Results/`
2. In the PFT session, open **Publish Destinations…** (app menu), tick **Web results (JSON)**, and choose that folder. Keep **Results workbook (Excel)** ticked too if you also publish the workbook; a problem with one never blocks the other.
3. Approve and **Publish** a race. Within a few seconds Drive for Desktop uploads `results-<season>-<regatta>.json`, and the next page load or **Reload** shows it.

**Before the regatta:**

- **The week before:** publish a test race (the sample regatta is fine) and confirm it appears on the Sites page in a private window.
- **Morning of:** confirm Drive for Desktop is signed in and shows "up to date", and that Publish Destinations shows the right folder.
- **If the venue loses internet,** keep publishing. Drive for Desktop uploads the queued file when the connection returns.

## Troubleshooting

| Symptom | Likely cause | Fix |
| --- | --- | --- |
| `/exec` works but the Sites embed is blank or asks for sign-in | The deployment access isn't *Anyone*, or the account's Workspace policy blocks anonymous web apps | Recheck step ② 6, or check the Workspace admin setting for Apps Script web-app access |
| A "This application was created by another user" banner appears | Normal for personal (gmail) accounts | Deploy from a Workspace account on the same domain as the site, or accept the banner |
| "Could not load results" | The script threw an error, or too many visitors loaded at once (see [Capacity](#capacity-and-rate-limits-a-risk-for-large-regattas)) | Run `diagnose` in the editor and read its log, and check **Executions** in the Apps Script sidebar for failures |
| A published race never appears | Drive for Desktop isn't syncing that folder, it's signed in to the wrong account, or the PFT points at another folder | Check the Drive for Desktop status, and the folder in **Publish Destinations…** |

## Things to check

- **Banner:** web apps deployed from a personal Google account show a notice bar. A Workspace account usually avoids it, especially on a Workspace-owned site.
- **Quotas:** the daily totals are not the problem; short bursts are. See [Capacity and rate limits](#capacity-and-rate-limits-a-risk-for-large-regattas).
- **Load time:** the first view after a deploy or a quiet spell may take a second or two while the script starts up. This is normal for Apps Script. The very first run after adding many files opens each one once (several seconds for a few dozen); run `checkSetup` or `installWarmer` so a visitor doesn't.

## Capacity and rate limits: a risk for large regattas

Google limits how many Apps Script executions can run **at the same time** (Google's [Apps Script quotas](https://developers.google.com/apps-script/guides/services/quotas) page, as of November 2026):

- **30 simultaneous executions per user**, for both personal and Workspace accounts;
- **1,000 per script.**

This web app runs **as the account that deployed it**, so every visitor's request counts against that one account's **30**. There is no published daily cap on web-app executions, Drive reads or the cache, so the day's total is not the problem. **The burst is:** everyone pressing Reload in the few minutes after a race they care about is published.

When the limit is hit, the extra requests fail and the page shows "Could not load results". Visitors tend to press Reload again, which adds to the burst.

### Worked example: a large regatta

Take a regatta of 2,000 athletes from 50 teams, with 60 races over 8 hours (one every 8 minutes). The possible audience:

- 2,000 athletes;
- 200 coaches (4 per team);
- 6,000 family members (3 per athlete).

That's **8,200 people** in all.

| Assumption | Low | Typical | High |
| --- | --- | --- | --- |
| Share of the audience that uses the page | 10% (820) | 30% (2,460) | 60% (4,920) |
| Races each viewer follows | 2 | 4 | 8 |
| Reloads per followed race | 2 | 5 | 6 |
| Popularity of the busiest race (vs. average) | 1.5× | 2× | 3× |

Reloads per day = viewers × races followed × reloads per race. Reloads for one race bunch up in about **3 minutes** around its result. Before the speed-up, each reload ran about 2.5 executions of about **0.7 s** (**Reload** ran `getIndex` and `getResults`, and opening the page also ran `doGet`).

So, before the speed-up, **simultaneous executions ≈ reloads per second × 2.5 × 0.7.** With this version, opening the page is one execution that carries its data, and **Reload** two short ones, roughly **1.5 shorter executions** per reload.

| Result | Low | Typical | High |
| --- | --- | --- | --- |
| Reloads over the day | 3,280 | 49,200 | 236,160 |
| Average per hour | 410 | 6,150 | 29,520 |
| Reloads for the busiest race | 82 | 1,640 | 11,808 |
| Peak reloads per second | 0.5 | 9 | 66 |
| Simultaneous executions at the peak, before the speed-up (limit 30) | ≈1 | ≈16 | ≈115 |
| …before the speed-up, with about 20 regattas in the folder | ≈3 | ≈56 | ≈400 |
| **…with this version, however many regattas the folder holds** | **≈0** | **≈7** | **≈49** |

**Reading it:**

- **Low use is comfortable.**
- **Typical use averages about half the limit.** But the averages hide spikes, for example when a result appears and many people reload within the same 30 seconds. Expect occasional failures at the busiest races.
- **High use is well over the limit,** and some visitors will see errors for much of the day.
- **Before the speed-up, the page slowed as the folder filled.** Building the regatta list asked Drive about every file on every visit, about 0.4 s per file: **17–20 s with 43 regattas** (measured). That is the "about 20 regattas" row. This version keeps the time flat however many files the folder holds.

These are estimates from the assumptions above, not measurements. To redo them for your own regatta, change the first table and recompute with the formula.

### Reducing the risk

1. **Use this version** (with the Drive API service on, and `installWarmer` on race day). It brings typical use well under the limit, but not the high case.
2. **For large regattas, use a plain web host instead** ([`../static/`](../static/README.md)). There, visitors fetch static JSON files, which an ordinary web server, or a free CDN in front of it, serves at thousands of requests per second with no execution limit. The same PFT folder can feed both, so you can keep the Google Sites page for small events and point a big regatta at the web host.
3. **Watch it on the day.** Apps Script's **Executions** page lists failed runs. If failures appear around popular races, the limit is being hit.
