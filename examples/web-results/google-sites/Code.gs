/**
 * regattaClock results - Google Apps Script web app.
 *
 * Serves Index.html to the public and reads results-*.json from ONE private
 * Google Drive folder (synced from the PFT laptop by Google Drive for
 * Desktop). The folder is never shared: the script runs as the deploying
 * account, and only hands out files that sit in that folder.
 *
 * Speed: the page arrives with the regatta list and the default regatta's
 * results already inside it, so the first view makes no further calls. The
 * list is cached for INDEX_TTL; rebuilding it lists the folder in one Drive
 * API call and opens only files it hasn't seen (each file's regatta header is
 * kept in Script Properties). Results are cached compressed, in chunks, so
 * files of any size stay cached.
 *
 * Setup: see README.md in this folder.
 */

// The ID at the end of the folder's URL: drive.google.com/drive/folders/<ID>
var FOLDER_ID = 'PASTE_FOLDER_ID_HERE';

var RESULTS_NAME = /^results-.*\.json$/i; // anything else in the folder is ignored
var INDEX_TTL = 300;                      // 5 min: how soon a NEW regatta file appears
var CACHE_TTL = 21600;                    // 6 h, CacheService's maximum
var CHUNK = 90 * 1024;                    // under CacheService's 100 KB per-value limit
var INDEX_KEY = 'idx:v2';

/** Web app entry point: renders Index.html with the first view's data in it. */
function doGet(e) {
  var p = (e && e.parameter) || {};
  var initial = { season: p.season || '', regatta: p.regatta || '', school: p.school || '' };
  try {
    var index = JSON.parse(indexJSON_());
    var pick = defaultRegatta_(index, initial.season, initial.regatta);
    initial.index = index;
    if (pick) {
      initial.results = { file: pick.file, data: JSON.parse(resultsText_(pick.file)) };
    }
  } catch (err) {
    // The page falls back to fetching on its own, and shows any error there.
    console.warn('doGet prefetch: ' + err.message);
  }
  var page = HtmlService.createTemplateFromFile('Index');
  page.initial = safeJSON(initial);
  return page.evaluate()
    .setTitle('Regatta Results')
    .addMetaTag('viewport', 'width=device-width, initial-scale=1')
    .setXFrameOptionsMode(HtmlService.XFrameOptionsMode.ALLOWALL); // lets Google Sites embed it
}

/**
 * Called from the page (Reload). Returns an index.json-shaped catalog, as
 * text. `file` is the Drive file ID.
 */
function getIndex() {
  return indexJSON_();
}

/** Called from the page. Returns one results file's JSON text. */
function getResults(fileId) {
  fileId = String(fileId);
  // Never serve an arbitrary Drive file the deploying account can read: only
  // results files in the configured folder. The regatta list answers that
  // cheaply; if the file isn't in it (the list may be a few minutes old),
  // rebuild the list once, then fall back to asking Drive directly.
  if (!listed_(indexJSON_(), fileId) && !listed_(buildIndex_(), fileId)) {
    console.warn('getResults: ' + fileId + ' not in the regatta list; checking the folder directly');
    if (!inResultsFolder_(fileId)) { throw new Error('Results file not found.'); }
  }
  return resultsText_(fileId);
}

function listed_(indexText, fileId) {
  return JSON.parse(indexText).regattas.some(function (r) { return r.file === fileId; });
}

function inResultsFolder_(fileId) {
  try {
    var f = DriveApp.getFileById(fileId);
    if (!RESULTS_NAME.test(f.getName())) { return false; }
    var parents = f.getParents();
    while (parents.hasNext()) {
      if (parents.next().getId() === FOLDER_ID) { return true; }
    }
  } catch (err) {
    console.warn('inResultsFolder_: ' + err.message);
  }
  return false;
}

/**
 * Run from the editor when something looks wrong, and share the log. It
 * compares the folder listing both ways, shows the cache, and fetches every
 * regatta the way the page does.
 */
function diagnose() {
  var t = Date.now(), lap = function () { var d = Date.now() - t; t = Date.now(); return d + ' ms'; };
  console.log('Drive API service: ' + (typeof Drive === 'undefined' ? 'OFF' : 'on'));
  if (typeof Drive !== 'undefined') {
    var meta = Drive.Files.get(FOLDER_ID, { supportsAllDrives: true, fields: 'id, name, driveId' });
    console.log('folder "' + meta.name + '" is in ' + (meta.driveId ? 'a Shared Drive (' + meta.driveId + ')' : 'My Drive'));
  }
  var api = typeof Drive === 'undefined' ? [] : listFolderApi_();
  console.log('Drive API listing: ' + api.length + ' results files (' + lap() + ')');
  var direct = [], it = folder_().getFiles();
  while (it.hasNext()) { var f = it.next(); if (RESULTS_NAME.test(f.getName())) { direct.push(f.getId()); } }
  console.log('DriveApp listing: ' + direct.length + ' results files (' + lap() + ')');
  var apiIds = api.map(function (x) { return x.id; });
  direct.filter(function (id) { return apiIds.indexOf(id) < 0; }).forEach(function (id) {
    console.log('  only in DriveApp listing: ' + id);
  });
  apiIds.filter(function (id) { return direct.indexOf(id) < 0; }).forEach(function (id) {
    console.log('  only in Drive API listing: ' + id);
  });
  var cached = CacheService.getScriptCache().get(INDEX_KEY);
  console.log('cached regatta list: ' + (cached === null ? 'none' : JSON.parse(cached).regattas.length + ' regattas'));
  var idx = JSON.parse(buildIndex_());
  console.log('rebuilt regatta list: ' + idx.regattas.length + ' regattas (' + lap() + ')');
  var props = PropertiesService.getScriptProperties().getProperties();
  console.log('remembered file headers: ' + Object.keys(props).filter(function (k) { return k.indexOf('h:') === 0; }).length);
  idx.regattas.forEach(function (r) {
    try {
      getResults(r.file);
      console.log('  ok   ' + r.season + ' ' + r.name + ' (' + lap() + ')');
    } catch (err) {
      console.log('  FAIL ' + r.season + ' ' + r.name + ' ' + r.file + ': ' + err.message);
    }
  });
}

/** Run once from the editor: authorizes Drive access and logs what it sees. */
function checkSetup() {
  var t0 = Date.now();
  var idx = JSON.parse(buildIndex_());
  console.log('Folder "' + folder_().getName() + '": ' + idx.regattas.length + ' regatta(s), list built in ' +
              (Date.now() - t0) + ' ms' + (typeof Drive === 'undefined' ? ' (Drive API service off: slower listing)' : ''));
  idx.regattas.forEach(function (r) {
    console.log('  ' + r.season + '  ' + r.date + '  ' + r.name + (r.cancelled ? '  (cancelled)' : '') + '  (' + r.file + ')');
  });
}

/**
 * Optional, for race day: run once from the editor to rebuild the regatta
 * list every 5 minutes, so no visitor ever waits for a rebuild.
 * removeWarmer() undoes it.
 */
function installWarmer() {
  removeWarmer();
  ScriptApp.newTrigger('warm').timeBased().everyMinutes(5).create();
  console.log('Warmer installed: the regatta list is rebuilt every 5 minutes.');
}

function removeWarmer() {
  ScriptApp.getProjectTriggers().forEach(function (t) {
    if (t.getHandlerFunction() === 'warm') { ScriptApp.deleteTrigger(t); }
  });
}

/** The trigger target: rebuild and cache the regatta list now. */
function warm() {
  buildIndex_();
}

// --- regatta list ------------------------------------------------------------

function indexJSON_() {
  var cached = CacheService.getScriptCache().get(INDEX_KEY);
  return cached !== null ? cached : buildIndex_();
}

/**
 * Lists the folder, reads the regatta header of any file not seen before (or
 * changed since), and caches the catalog. Unreadable files (e.g. one caught
 * mid-sync) are skipped, not fatal.
 */
function buildIndex_() {
  var props = PropertiesService.getScriptProperties();
  var known = props.getProperties(); // 'h:<fileId>' -> {m: modifiedMs, h: header}
  var fresh = {}, byKey = {}, listed = {};
  listFolder_().forEach(function (f) {
    var k = 'h:' + f.id, rec = null;
    listed[k] = true;
    try { rec = known[k] ? JSON.parse(known[k]) : null; } catch (err) { rec = null; }
    if (!rec || rec.m !== f.modified) {
      try {
        var r = JSON.parse(resultsText_(f.id, f.modified)).regatta;
        rec = { m: f.modified, h: { season: String(r.season), name: r.name, date: r.date,
                                    regattaKey: r.regattaKey, cancelled: r.cancelled || null } };
        fresh[k] = JSON.stringify(rec);
      } catch (err) {
        console.warn('Skipping ' + f.name + ': ' + err.message);
        return;
      }
    }
    var entry = { season: rec.h.season, name: rec.h.name, date: rec.h.date,
                  regattaKey: rec.h.regattaKey, file: f.id, updated: f.modified };
    if (rec.h.cancelled) { entry.cancelled = rec.h.cancelled; } // a regatta cancelled outright
    // Two files for one regatta (e.g. a duplicate web upload): newest wins.
    var seen = byKey[entry.regattaKey];
    if (!seen || seen.updated < entry.updated) { byKey[entry.regattaKey] = entry; }
  });
  if (Object.keys(fresh).length) { props.setProperties(fresh); }
  // Forget files that have left the folder.
  Object.keys(known).forEach(function (k) {
    if (k.indexOf('h:') === 0 && !listed[k]) { props.deleteProperty(k); }
  });
  var text = JSON.stringify({ schemaVersion: 1,
                              regattas: Object.keys(byKey).map(function (k) { return byKey[k]; }) });
  CacheService.getScriptCache().put(INDEX_KEY, text, INDEX_TTL);
  return text;
}

/**
 * [{id, name, modified}] for every results file in the folder. One Drive API
 * request per 1000 files when the Drive advanced service is on (see
 * appsscript.json); otherwise DriveApp, one call per file per field.
 */
function listFolder_() {
  if (FOLDER_ID === 'PASTE_FOLDER_ID_HERE') { folder_(); } // throws the setup message
  if (typeof Drive !== 'undefined') {
    var out = listFolderApi_();
    if (out.length) { return out; }
    // Never build an empty list on the API's word alone: an empty answer is
    // also what a misconfigured listing looks like. Ask DriveApp.
    console.warn('Drive API listed no results files; checking with DriveApp');
  }
  return listFolderDriveApp_();
}

function listFolderApi_() {
  var out = [], token;
  do {
    var page = Drive.Files.list({
      q: "'" + FOLDER_ID + "' in parents and trashed = false",
      fields: 'nextPageToken, files(id, name, modifiedTime)',
      pageSize: 1000, pageToken: token,
      // A folder in a Shared Drive is invisible to the listing without these.
      supportsAllDrives: true, includeItemsFromAllDrives: true, corpora: 'allDrives'
    });
    (page.files || []).forEach(function (f) {
      if (RESULTS_NAME.test(f.name)) {
        out.push({ id: f.id, name: f.name, modified: new Date(f.modifiedTime).getTime() });
      }
    });
    token = page.nextPageToken;
  } while (token);
  return out;
}

function listFolderDriveApp_() {
  var out = [];
  var files = folder_().getFiles();
  while (files.hasNext()) {
    var f = files.next();
    if (RESULTS_NAME.test(f.getName())) {
      out.push({ id: f.getId(), name: f.getName(), modified: f.getLastUpdated().getTime() });
    }
  }
  return out;
}

/** The regatta the page shows first: as asked for, else the newest. */
function defaultRegatta_(index, season, regattaKey) {
  var seasons = index.regattas.map(function (r) { return r.season; }).sort().reverse();
  if (seasons.indexOf(season) < 0) { season = seasons[0]; }
  var inSeason = index.regattas.filter(function (r) { return r.season === season; })
    .sort(function (a, b) { return b.date.localeCompare(a.date); });
  return inSeason.filter(function (r) { return r.regattaKey === regattaKey; })[0] || inSeason[0] || null;
}

// --- results files -------------------------------------------------------------

/**
 * A results file's JSON text (validated, minified), cached under its ID and
 * last-modified time: a re-sync changes the time, so the next request reads
 * the new content - no stale window. Stored gzipped and split into chunks,
 * so a file of any size fits CacheService's per-value limit.
 */
function resultsText_(fileId, modified) {
  var f = null;
  if (modified === undefined) {
    f = DriveApp.getFileById(fileId);
    modified = f.getLastUpdated().getTime();
  }
  var cache = CacheService.getScriptCache();
  var key = 'r3:' + fileId + ':' + modified;
  var head = cache.get(key);
  if (head !== null) {
    var n = Number(head), keys = [];
    for (var i = 0; i < n; i++) { keys.push(key + ':' + i); }
    var parts = cache.getAll(keys), b64 = '';
    for (var j = 0; j < n && parts[keys[j]] !== undefined; j++) { b64 += parts[keys[j]]; }
    if (j === n) {
      var blob = Utilities.newBlob(Utilities.base64Decode(b64), 'application/x-gzip');
      return Utilities.ungzip(blob).getDataAsString('UTF-8');
    }
  }
  if (!f) { f = DriveApp.getFileById(fileId); }
  var text = JSON.stringify(JSON.parse(f.getBlob().getDataAsString('UTF-8'))); // validate + minify
  var gz = Utilities.base64Encode(Utilities.gzip(Utilities.newBlob(text, 'application/json')).getBytes());
  var put = {}, count = Math.ceil(gz.length / CHUNK);
  for (var c = 0; c < count; c++) { put[key + ':' + c] = gz.substr(c * CHUNK, CHUNK); }
  put[key] = String(count);
  try { cache.putAll(put, CACHE_TTL); } catch (err) { console.warn('cache: ' + err.message); }
  return text;
}

// --- helpers ---------------------------------------------------------------

function folder_() {
  if (FOLDER_ID === 'PASTE_FOLDER_ID_HERE') {
    throw new Error('Set FOLDER_ID in Code.gs to the results folder ID.');
  }
  return DriveApp.getFolderById(FOLDER_ID);
}

/** JSON safe to print inside a <script> block. */
function safeJSON(v) {
  return JSON.stringify(v)
    .replace(/</g, '\\u003c')
    .replace(/\u2028/g, '\\u2028')
    .replace(/\u2029/g, '\\u2029');
}
