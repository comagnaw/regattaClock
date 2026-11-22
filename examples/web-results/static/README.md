# Web results on your own web host

Serve the regatta results page from any web server that can serve files: Apache, nginx, a shared hosting plan, anything. There is no database, no server-side code to run on each request, and no build step.

| File | Where it goes | Purpose |
| --- | --- | --- |
| `index.html` | web root | The results page: season and regatta pickers, a school filter, and race-by-race results. One self-contained file. |
| `build-index.py` | anywhere on the host | Rebuilds `index.json`, the list of regattas the page loads first, from the `results-*.json` files. Python 3 standard library only. |
| `sync-results.sh` | next to `build-index.py` | The race-day job: copies newly published results from the cloud folder into the web root, then rebuilds `index.json`. |

The page reads `index.json` and then the chosen `results-*.json` file, by relative URL. A typical host looks like this:

```text
/var/www/results/            ← web root (https://example.org/results/)
  index.html                 ← copied once
  index.json                 ← written by build-index.py
  results-2026-spring-sprints.json   ← published by the PFT, copied by sync-results.sh
/opt/regatta-results/
  build-index.py
  sync-results.sh
```

## Try it locally

```sh
mkdir /tmp/results-site
cp index.html ../sample-data/*.json /tmp/results-site/
python3 build-index.py /tmp/results-site
cd /tmp/results-site && python3 -m http.server 8000
# open http://localhost:8000/
```

Try `http://localhost:8000/?season=2025&school=coldspring` for a deep link.

## Install on the web host

1. Copy `index.html` into the web root, and `build-index.py` and `sync-results.sh` into a folder outside it, such as `/opt/regatta-results/`.
2. Have the web server send `results-*.json` and `index.json` with **`Cache-Control: no-cache`**. Otherwise a browser or proxy may keep showing an older copy after a new publish. Use one of these:

   **nginx:**

   ```nginx
   location /results/ {
       alias /var/www/results/;
       location ~ \.json$ {
           add_header Cache-Control "no-cache";
       }
   }
   ```

   **Apache** (`.htaccess` in the web root; needs `mod_headers`):

   ```apache
   AddType application/json .json
   <FilesMatch "\.json$">
       Header set Cache-Control "no-cache"
   </FilesMatch>
   ```

3. **Optional: data on another host.** If the JSON files live somewhere other than the page, set `DATA_URL` near the top of the script in `index.html` (it defaults to `./`). The data host must then also send `Access-Control-Allow-Origin` for the page's origin.

## Race day: getting results from the PFT laptop to the web host

The Primary Finish Timer (PFT) only ever writes to a folder **on the laptop**: in the app, **Publish Destinations…** → **Web results (JSON)** → a folder. Something has to move each new file to the web server, unattended, within a minute or so, from a venue whose internet may come and go. Pick one of the three routes below. The first is recommended.

### Recommended: a cloud-folder relay, pulled by the host

```text
PFT laptop                         cloud                        web host (every minute)
Publish ─► Drive/OneDrive folder ─► Google Drive / OneDrive ─► rclone copy ─► build-index.py ─► page
```

- **On the laptop:** install Google Drive for Desktop or OneDrive. In the PFT's **Publish Destinations…**, choose a folder inside it for **Web results (JSON)**. Nothing else runs on the laptop.
- **On the web host:** `sync-results.sh` runs every minute. It pulls `results-*.json` from that cloud folder with [rclone](https://rclone.org/) and rebuilds `index.json`.

**Why this is the default:**

- There are no server passwords or keys on the laptop, and nothing extra for the PFT operator to do. Publish works exactly as usual.
- **If the venue loses internet,** the Drive or OneDrive client queues the file and uploads it when the connection returns, and the host catches up on its next run. Nothing is lost.
- The same folder can feed a [Google Sites page](../google-sites/README.md) at the same time.
- It uses `rclone copy`, not `sync`. A results file deleted from the cloud folder by mistake never removes results the public has already seen.

**Set it up once:**

1. **Install rclone** on the web host (`apt install rclone`, or see [rclone.org/install](https://rclone.org/install/)).
2. **Create a read-only remote for the results folder.** Run `rclone config` as the user that will run the job:
   - **Google Drive:**
     - name the remote `gdrive-results`, with storage type `drive`;
     - set **scope** to `drive.readonly`;
     - set **root_folder_id** to the folder's ID (the last part of its `drive.google.com/drive/folders/<ID>` address).

     The remote is then just `gdrive-results:`.
   - **OneDrive:**
     - name the remote `onedrive-results`, with storage type `onedrive`;
     - pick the account or SharePoint library that holds the folder.

     The remote is then `"onedrive-results:regattaClock Results"` (your folder's path).

   A server has no browser. When rclone asks *"Use web browser to automatically authenticate?"*, answer **No** and follow its instructions: run `rclone authorize` on any computer with a browser and paste the result back. Use an account that can see only this folder where you can, such as a club "results" account the folder is shared with.

   Check it with `rclone ls gdrive-results:`, which should list the `results-*.json` files.
3. **Run the job every minute.** Add a **cron** line for that user (`crontab -e`):

   ```cron
   * * * * * flock -n /tmp/regatta-results.lock /opt/regatta-results/sync-results.sh gdrive-results: /var/www/results >> $HOME/regatta-results.log 2>&1
   ```

   `flock -n` skips a run while the previous one is still going, so a slow network never stacks jobs up. Each run logs one line, for example `2026-05-02T13:36:12Z synced gdrive-results: -> /var/www/results`.

   Or use a **systemd timer** instead of cron:

   ```ini
   # /etc/systemd/system/regatta-results.service
   [Service]
   Type=oneshot
   # the user you ran `rclone config` as
   User=www-data
   ExecStart=/opt/regatta-results/sync-results.sh gdrive-results: /var/www/results

   # /etc/systemd/system/regatta-results.timer
   [Timer]
   OnCalendar=*-*-* *:*:00
   [Install]
   WantedBy=timers.target
   ```

   Enable it with `systemctl enable --now regatta-results.timer`. systemd never starts a second run while one is active, and `journalctl -u regatta-results` shows the log.

**How long it takes:**

- Drive or OneDrive uploads the file in a few seconds.
- The job's next run is up to 60 seconds away.
- The index rebuild takes under a second.

So expect a race to appear about **1 to 1½ minutes after Publish**. The page shows "updated …", so the public can see how current it is.

### Alternative: push directly from the laptop

Choose this only if your organization has no Google Drive or OneDrive.

- A scheduled task **on the laptop** copies the folder to the server over SSH every minute.
- It has the lowest delay.
- But the laptop needs a server login key, and the task has to be set up and checked on every PFT laptop.

- **macOS or Linux:**
  - add a `launchd` agent or a cron job that runs `rsync -t --include='results-*.json' --exclude='*' "<PFT folder>/" user@host:/var/www/results/`;
  - on the server, run `build-index.py /var/www/results` every minute from cron.
- **Windows:**
  - in Task Scheduler, every minute, run a [WinSCP](https://winscp.net/) script that does `synchronize remote` from the PFT folder to `/var/www/results` with the file mask `results-*.json`;
  - on the server, run the same `build-index.py` cron job.

### Alternative: mount the web folder on the laptop

If the laptop and the web server are on the **same network**, share the web root (SMB, or WebDAV over HTTPS) and mount it on the laptop. Then choose that mounted folder in **Publish Destinations…**, and run `build-index.py` every minute on the server.

There's no relay, but at a venue the server is rarely on the same network. If the mount drops, **Publish** says the folder can't be reached. Nothing breaks, and you re-publish once it's back.

### Race-day checklist

- **The week before:**
  - publish a test race (the sample regatta is fine) through the real route;
  - confirm it appears on the page in a private browser window;
  - check that the job's log shows a run every minute.
- **Morning of:**
  - on the PFT laptop, **Publish Destinations…** shows the right folder;
  - the Drive or OneDrive client is signed in and up to date.
- **Keep a fallback:** leave **Results workbook (Excel)** ticked as a second destination. A problem with the web route never blocks publishing.
- **If the page looks stale:**
  1. Check the laptop's Drive or OneDrive icon. Is it still uploading?
  2. Check the last line of the job log on the host.
  3. Press **Reload** on the page.
