/**
 * regattaClock results - Google Apps Script web app (PoC).
 *
 * Serves Index.html to the public and reads results-*.json from ONE private
 * Google Drive folder (synced from the PFT laptop by Google Drive for
 * Desktop). The folder is never shared: the script runs as the deploying
 * account, and only hands out files that sit in that folder.
 *
 * Setup: see README.md in this folder.
 */

// The ID at the end of the folder's URL: drive.google.com/drive/folders/<ID>
var FOLDER_ID = 'PASTE_FOLDER_ID_HERE';

var RESULTS_NAME = /^results-.*\.json$/i; // anything else in the folder is ignored
var CACHE_TTL = 21600;                    // 6 h, CacheService's maximum
var CACHE_MAX = 100 * 1024;               // CacheService's per-value limit

/** Web app entry point: renders Index.html with the URL's query parameters. */
function doGet(e) {
  var p = (e && e.parameter) || {};
  var page = HtmlService.createTemplateFromFile('Index');
  page.initial = safeJSON({
    season: p.season || '',
    regatta: p.regatta || '',
    school: p.school || ''
  });
  return page.evaluate()
    .setTitle('Regatta Results')
    .addMetaTag('viewport', 'width=device-width, initial-scale=1')
    .setXFrameOptionsMode(HtmlService.XFrameOptionsMode.ALLOWALL); // lets Google Sites embed it
}

/**
 * Called from the page. Lists every results file in the folder and returns
 * an index.json-shaped catalog, as text. `file` is the Drive file ID.
 * Unreadable files (e.g. one caught mid-sync) are skipped, not fatal.
 */
function getIndex() {
  var files = folder_().getFiles();
  var byKey = {};
  while (files.hasNext()) {
    var f = files.next();
    if (!RESULTS_NAME.test(f.getName())) { continue; }
    try {
      var r = JSON.parse(readCached_(f)).regatta;
      var entry = { season: String(r.season), name: r.name, date: r.date,
                    regattaKey: r.regattaKey, file: f.getId(),
                    updated: f.getLastUpdated().getTime() };
      // Two files for one regatta (e.g. a duplicate web upload): newest wins.
      var seen = byKey[entry.regattaKey];
      if (!seen || seen.updated < entry.updated) { byKey[entry.regattaKey] = entry; }
    } catch (err) {
      console.warn('Skipping ' + f.getName() + ': ' + err.message);
    }
  }
  var regattas = Object.keys(byKey).map(function (k) { return byKey[k]; });
  return JSON.stringify({ schemaVersion: 1, regattas: regattas });
}

/** Called from the page. Returns one results file's JSON text. */
function getResults(fileId) {
  var f;
  try {
    f = DriveApp.getFileById(String(fileId));
  } catch (err) {
    throw new Error('Results file not found.');
  }
  // Never serve an arbitrary Drive file the deploying account can read:
  // only results files that live in the configured folder.
  if (!RESULTS_NAME.test(f.getName()) || !inFolder_(f)) {
    throw new Error('Results file not found.');
  }
  return readCached_(f);
}

/** Run once from the editor: authorizes Drive access and logs what it sees. */
function checkSetup() {
  var idx = JSON.parse(getIndex());
  console.log('Folder "' + folder_().getName() + '": ' + idx.regattas.length + ' regatta(s)');
  idx.regattas.forEach(function (r) {
    console.log('  ' + r.season + '  ' + r.date + '  ' + r.name + '  (' + r.file + ')');
  });
}

// --- helpers ---------------------------------------------------------------

function folder_() {
  if (FOLDER_ID === 'PASTE_FOLDER_ID_HERE') {
    throw new Error('Set FOLDER_ID in Code.gs to the results folder ID.');
  }
  return DriveApp.getFolderById(FOLDER_ID);
}

function inFolder_(f) {
  var parents = f.getParents();
  while (parents.hasNext()) {
    if (parents.next().getId() === FOLDER_ID) { return true; }
  }
  return false;
}

/**
 * File text, cached under its ID + last-modified time. A re-sync changes the
 * time, so the next request reads the new content: no stale window.
 */
function readCached_(f) {
  var cache = CacheService.getScriptCache();
  var key = 'r:' + f.getId() + ':' + f.getLastUpdated().getTime();
  var text = cache.get(key);
  if (text === null) {
    text = JSON.stringify(JSON.parse(f.getBlob().getDataAsString('UTF-8'))); // validate + minify
    if (text.length < CACHE_MAX) { cache.put(key, text, CACHE_TTL); }
  }
  return text;
}

/** JSON safe to print inside a <script> block. */
function safeJSON(v) {
  return JSON.stringify(v)
    .replace(/</g, '\\u003c')
    .replace(/\u2028/g, '\\u2028')
    .replace(/\u2029/g, '\\u2029');
}
