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
| `Code.gs` | Server side. `doGet` serves the page, `getIndex` lists `results-*.json` in the folder, `getResults` returns one file, and `checkSetup` is a one-time check run from the editor. |
| `Index.html` | The page: the same layout, pickers and school filter as [`../static/index.html`](../static/index.html). It asks `Code.gs` for its data with `google.script.run`. |
| `appsscript.json` | Optional manifest. It sets the Drive read-only scope and the web-app settings (runs as you, anyone can open it). |

**Points about the design:**

- **No `index.json` to maintain.** The script builds the catalog by listing the folder. Any file named `results-*.json` that parses appears under its `regatta.season`. Other files are ignored, and a half-synced file that doesn't parse yet is skipped.
- **The folder stays private.** Visitors never get a Drive link. `getResults` only returns files that are inside the configured folder, so a guessed file ID can't read anything else in the account.
- **No staleness.** Each file is cached under its ID plus its last-modified time. A new sync changes that time, so the next page load or **Reload** shows the new results immediately.
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
4. Optional: under **Project Settings**, tick *Show "appsscript.json" manifest file*, then replace that file's contents with this folder's `appsscript.json`. Change `timeZone` if needed.
5. In the editor, choose the **`checkSetup`** function and **Run** it. Approve the Drive permission prompt. The log should list the three sample regattas.
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

Use **Deploy → Manage deployments → edit (pencil) → Version: New version → Deploy**. That keeps the same `/exec` URL. Choosing **New deployment** creates a **new URL** and breaks the Sites embed and any shared links.

## Linking behavior

- **Deep links don't pass through the Sites embed.** The embed URL is fixed when you insert it, so adding `?season=2025` to the Sites page's own URL has no effect. Two ways to link to a specific season or school:
  - The page's **Open full page** link opens the `/exec` URL for the current selection (season, regatta and school filter) in a new tab. That is the shareable deep link.
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
| "Could not load results" | The script threw an error | Run `checkSetup` in the editor, and check **Executions** in the Apps Script sidebar |
| A published race never appears | Drive for Desktop isn't syncing that folder, it's signed in to the wrong account, or the PFT points at another folder | Check the Drive for Desktop status, and the folder in **Publish Destinations…** |

## Things to check

- **Banner:** web apps deployed from a personal Google account show a notice bar. A Workspace account usually avoids it, especially on a Workspace-owned site.
- **Quotas:** Apps Script's free daily limits are far above a regatta audience. Each page view makes a few Drive metadata calls, and file contents come from cache.
- **Load time:** the first view after a deploy or a quiet spell may take a second or two while the script starts up. This is normal for Apps Script.
