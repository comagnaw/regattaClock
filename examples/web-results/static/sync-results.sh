#!/bin/sh
# Copy newly published regattaClock results from a cloud folder to the web
# root, then rebuild index.json. Meant to run every minute from cron or a
# systemd timer, on the web host - see README.md, "Race day".
#
# Usage: sync-results.sh REMOTE WEB_ROOT
#   REMOTE    an rclone remote (and path) for the PFT's Web results folder,
#             e.g. "gdrive-results:" or "onedrive-results:regattaClock Results"
#   WEB_ROOT  the folder index.html is served from, e.g. /var/www/results
#
# "copy", not "sync": a results file deleted from the cloud folder by mistake
# never removes results the public has already seen.
set -eu

if [ "$#" -ne 2 ]; then
	echo "usage: $0 REMOTE WEB_ROOT" >&2
	exit 2
fi
remote=$1
web_root=$2
here=$(cd "$(dirname "$0")" && pwd)

rclone copy "$remote" "$web_root" --include 'results-*.json'
python3 "$here/build-index.py" "$web_root"
echo "$(date -u +%Y-%m-%dT%H:%M:%SZ) synced $remote -> $web_root"
