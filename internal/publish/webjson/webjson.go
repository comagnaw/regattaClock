// Package webjson is the web-results publish destination: one JSON file per
// regatta, read by a public results page - a Google Sites page (Apps Script
// listing results-*.json in a Drive-synced folder) or a page on any plain web
// host. See examples/web-results and publish-destinations.md.
//
// Like the results workbook, the file is regenerated whole on every publish
// and carries its own ledger (each race's revision + publishedAt), so it is a
// rendered cache of publish.BuildView, never a source of truth.
package webjson

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona/store"
	"github.com/comagnaw/regattaClock/internal/publish"
)

// SchemaVersion is the file format version the results page understands.
const SchemaVersion = 1

// Generator marks a file as written by RegattaClock; Read treats a file
// without it as foreign and never overwrites it.
const Generator = "regattaClock"

// Destination is the web-results JSON file as a publish.Destination.
type Destination struct{}

func (Destination) FileName(meta publish.Meta) string { return FileName(meta.Name, meta.Date) }
func (Destination) Read(path string) (publish.Published, error) {
	return Read(path)
}
func (Destination) Write(path string, meta publish.Meta, races []publish.PublishableRace, ledger publish.Ledger) error {
	return Write(path, meta, races, ledger)
}

type file struct {
	SchemaVersion int       `json:"schemaVersion"`
	Generator     string    `json:"generator"`
	Regatta       regatta   `json:"regatta"`
	PublishedAt   time.Time `json:"publishedAt"`
	Races         []race    `json:"races"`
}

type regatta struct {
	Name       string `json:"name"`
	Date       string `json:"date"`
	Season     string `json:"season"`
	RegattaKey string `json:"regattaKey"`
}

type race struct {
	RaceNumber    int             `json:"raceNumber"`
	Title         string          `json:"title"`
	ScheduledTime string          `json:"scheduledTime"`
	BoatClass     string          `json:"boatClass"`
	FlightInfo    string          `json:"flightInfo"`
	WinningTime   string          `json:"winningTime"`
	ApprovedAt    *time.Time      `json:"approvedAt,omitempty"`
	Revision      string          `json:"revision"`
	PublishedAt   time.Time       `json:"publishedAt"`
	Lanes         map[string]lane `json:"lanes"`
	Results       []result        `json:"results"`
}

type lane struct {
	School         string `json:"school"`
	AdditionalInfo string `json:"additionalInfo"`
	Status         string `json:"status"`
}

type result struct {
	Place          string `json:"place"`
	Lane           string `json:"lane"`
	School         string `json:"school"`
	AdditionalInfo string `json:"additionalInfo"`
	Split          string `json:"split"`
	Time           string `json:"time"`
}

var (
	nonSlug = regexp.MustCompile(`[^a-z0-9]+`)
	year    = regexp.MustCompile(`\b(19|20)\d{2}\b`)
)

// FileName is results-<season>-<slug>.json: the season is the regatta date's
// year and the slug is the lower-cased name with every run of other
// characters turned into "-". The results page lists results-*.json, so the
// prefix is load-bearing. A date with no recognisable year drops the season.
func FileName(regattaName, regattaDate string) string {
	slug := strings.Trim(nonSlug.ReplaceAllString(strings.ToLower(regattaName), "-"), "-")
	if slug == "" {
		slug = "regatta"
	}
	if season := Season(regattaDate); season != "" {
		return "results-" + season + "-" + slug + ".json"
	}
	return "results-" + slug + ".json"
}

// Season is the regatta date's year, or "" when it has none.
func Season(regattaDate string) string {
	if d, ok := store.ParseRegattaDate(regattaDate); ok {
		return strconv.Itoa(d.Year())
	}
	return year.FindString(regattaDate)
}

// isoDate is regattaDate as YYYY-MM-DD when it parses, so the results page can
// sort regattas by date; otherwise it is passed through as written.
func isoDate(regattaDate string) string {
	if d, ok := store.ParseRegattaDate(regattaDate); ok {
		return d.Format(time.DateOnly)
	}
	return strings.TrimSpace(regattaDate)
}

// Write renders the published races - those in ledger - to path, in place
// (filesystem.SaveBytesFileInPlace) so the file keeps its identity: a
// Drive-synced file keeps its Drive ID, which the results page caches on.
func Write(path string, meta publish.Meta, races []publish.PublishableRace, ledger publish.Ledger) error {
	out := file{
		SchemaVersion: SchemaVersion,
		Generator:     Generator,
		Regatta: regatta{
			Name:       meta.Name,
			Date:       isoDate(meta.Date),
			Season:     Season(meta.Date),
			RegattaKey: meta.RegattaKey,
		},
		Races: []race{},
	}
	for _, pr := range races {
		entry, ok := ledger[pr.RaceNumber]
		if !ok {
			continue
		}
		out.Races = append(out.Races, toRace(pr, entry))
		if entry.PublishedAt.After(out.PublishedAt) {
			out.PublishedAt = entry.PublishedAt
		}
	}
	sort.Slice(out.Races, func(i, j int) bool { return out.Races[i].RaceNumber < out.Races[j].RaceNumber })

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("web results could not be rendered: %w", err)
	}
	if err := filesystem.SaveBytesFileInPlace(append(data, '\n'), path); err != nil {
		if filesystem.IsFileLocked(err) {
			return fmt.Errorf("%w: %w", publish.ErrLocked, err)
		}
		return err
	}
	return nil
}

func toRace(pr publish.PublishableRace, entry publish.LedgerEntry) race {
	r := race{
		RaceNumber:    pr.RaceNumber,
		Title:         pr.Title,
		ScheduledTime: pr.ScheduledTime,
		BoatClass:     pr.BoatClass,
		FlightInfo:    pr.FlightInfo,
		WinningTime:   pr.WinningTime,
		Revision:      entry.Revision,
		PublishedAt:   entry.PublishedAt.UTC(),
		Lanes:         make(map[string]lane, len(pr.Lanes)),
		Results:       make([]result, 0, len(pr.Rows)),
	}
	if !pr.ApprovedAt.IsZero() {
		at := pr.ApprovedAt.UTC()
		r.ApprovedAt = &at
	}
	for n, e := range pr.Lanes {
		r.Lanes[strconv.Itoa(n)] = lane{School: e.SchoolName, AdditionalInfo: e.AdditionalInfo, Status: string(e.Status)}
	}
	for _, row := range pr.Rows {
		r.Results = append(r.Results, result(row))
		// A race carried forward after an un-approve has its published rows
		// but no current winning time; the first-place row still has it.
		if r.WinningTime == "" && row.Place == "1" {
			r.WinningTime = row.Time
		}
	}
	return r
}

// Read loads path's ledger and published rows. A missing file is not an
// error and returns an empty Published; a file that is not a RegattaClock
// web-results file is publish.ErrForeign.
func Read(path string) (publish.Published, error) {
	out := publish.Published{Ledger: publish.Ledger{}, Rows: map[int][]publish.Row{}}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return out, nil
	}
	if err != nil {
		if filesystem.IsFileLocked(err) {
			return out, fmt.Errorf("%w: %w", publish.ErrLocked, err)
		}
		return out, fmt.Errorf("web results %s could not be read: %w", path, err)
	}

	var in file
	if err := json.Unmarshal(data, &in); err != nil || in.Generator != Generator || in.SchemaVersion == 0 {
		return out, fmt.Errorf("%s: %w", path, publish.ErrForeign)
	}
	if in.SchemaVersion > SchemaVersion {
		return out, fmt.Errorf("web results %s is schema version %d, newer than this app's %d", path, in.SchemaVersion, SchemaVersion)
	}

	out.Exists = true
	out.RegattaKey = in.Regatta.RegattaKey
	for _, r := range in.Races {
		out.Ledger[r.RaceNumber] = publish.LedgerEntry{Revision: r.Revision, PublishedAt: r.PublishedAt}
		rows := make([]publish.Row, 0, len(r.Results))
		for _, res := range r.Results {
			rows = append(rows, publish.Row(res))
		}
		out.Rows[r.RaceNumber] = rows
	}
	return out, nil
}
