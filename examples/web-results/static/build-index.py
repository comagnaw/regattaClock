#!/usr/bin/env python3
"""Rebuild index.json for the regattaClock web results page.

Usage:  python3 build-index.py RESULTS_DIR

Reads every results-*.json in RESULTS_DIR (the files the Primary Finish Timer
publishes) and writes RESULTS_DIR/index.json, the regatta list that index.html
loads first. Python 3 standard library only; safe to run every minute.

- Files not written by regattaClock, files in a newer format, and files that
  do not parse yet (caught mid-sync) are skipped, never fatal.
- When two files hold the same regatta (same regattaKey), the most recently
  published one wins.
- index.json is written to a temporary file and renamed into place, so a
  visitor never loads half of it.
"""

import glob
import json
import os
import sys
import tempfile
from datetime import datetime, timezone

GENERATOR = "regattaClock"
SCHEMA_VERSION = 1


def read_entry(path):
    """Return the index entry for one results file, or None to skip it."""
    try:
        with open(path, encoding="utf-8") as f:
            data = json.load(f)
    except (OSError, ValueError) as err:
        print(f"skip {os.path.basename(path)}: {err}", file=sys.stderr)
        return None
    if not isinstance(data, dict) or data.get("generator") != GENERATOR:
        print(f"skip {os.path.basename(path)}: not a regattaClock results file", file=sys.stderr)
        return None
    if not isinstance(data.get("schemaVersion"), int) or data["schemaVersion"] > SCHEMA_VERSION:
        print(f"skip {os.path.basename(path)}: unsupported schemaVersion {data.get('schemaVersion')}", file=sys.stderr)
        return None
    regatta = data.get("regatta") or {}
    return {
        "season": str(regatta.get("season", "")),
        "name": regatta.get("name", ""),
        "date": regatta.get("date", ""),
        "regattaKey": regatta.get("regattaKey", ""),
        "file": os.path.basename(path),
        "publishedAt": data.get("publishedAt", ""),
    }


def build(results_dir):
    by_key = {}
    for path in sorted(glob.glob(os.path.join(results_dir, "results-*.json"))):
        entry = read_entry(path)
        if entry is None:
            continue
        seen = by_key.get(entry["regattaKey"])
        if seen is None or seen["publishedAt"] < entry["publishedAt"]:  # RFC 3339 UTC sorts as text
            by_key[entry["regattaKey"]] = entry
    regattas = sorted(by_key.values(), key=lambda e: (e["date"], e["name"]), reverse=True)
    return {
        "schemaVersion": SCHEMA_VERSION,
        "generatedAt": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
        "regattas": regattas,
    }


def write_atomically(path, data):
    fd, tmp = tempfile.mkstemp(prefix=".index-", suffix=".json", dir=os.path.dirname(path))
    try:
        with os.fdopen(fd, "w", encoding="utf-8") as f:
            json.dump(data, f, indent=2)
            f.write("\n")
        os.chmod(tmp, 0o644)  # mkstemp creates 0600; the web server must read it
        os.replace(tmp, path)
    except BaseException:
        os.unlink(tmp)
        raise


def main(argv):
    if len(argv) != 2 or not os.path.isdir(argv[1]):
        print(__doc__.strip().splitlines()[2], file=sys.stderr)
        return 2
    index = build(argv[1])
    write_atomically(os.path.join(argv[1], "index.json"), index)
    print(f"index.json: {len(index['regattas'])} regatta(s)")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
