package main

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// reportData feeds the HTML template. Every field is plain text meant for a
// non-technical reader - no JSON, no RC ids, no jargon.
type reportData struct {
	RegattaName  string
	Generated    string
	Total        int
	MatchedCount int
	Races        []reportRace
	Unused       []string // organization names, for display only
}

type reportRace struct {
	RaceNumber int
	BoatClass  string
	FlightInfo string
	Lanes      []reportLane
}

type reportLane struct {
	Lane        int
	Boat        string // "SchoolName (AdditionalInfo)"
	StatusLabel string
	StatusClass string // css class: ok / warn / bad
	MatchName   string // best-guess RC organization name(s), for ambiguous/matched
}

func buildReport(regattaName string, matches []laneMatch, unused []rcEntry) reportData {
	data := reportData{
		RegattaName: strings.TrimSpace(regattaName),
		Generated:   time.Now().Format("January 2, 2006 3:04 PM"),
	}
	if data.RegattaName == "" {
		data.RegattaName = "This regatta"
	}

	byRace := map[int]*reportRace{}
	var order []int
	for _, m := range matches {
		data.Total++
		rr, ok := byRace[m.RaceNumber]
		if !ok {
			rr = &reportRace{RaceNumber: m.RaceNumber, BoatClass: m.BoatClass, FlightInfo: m.FlightInfo}
			byRace[m.RaceNumber] = rr
			order = append(order, m.RaceNumber)
		}

		boat := m.SchoolName
		if strings.TrimSpace(m.AdditionalInfo) != "" {
			boat = fmt.Sprintf("%s (%s)", m.SchoolName, m.AdditionalInfo)
		}

		lane := reportLane{Lane: m.Lane, Boat: boat}
		switch m.Status {
		case statusMatched:
			data.MatchedCount++
			lane.StatusLabel, lane.StatusClass = "Matches RegattaCentral", "ok"
			lane.MatchName = m.Candidates[0].OrgName
		case statusGuessed:
			// Candidates[0] is the pick (see matchRaces/lastResortPick);
			// the rest are the untaken, equally-plausible alternative(s) -
			// shown so a human can still see what this was chosen over.
			lane.StatusLabel, lane.StatusClass = "Best guess (RegattaCentral doesn't distinguish these)", "warn"
			lane.MatchName = m.Candidates[0].OrgName
			if len(m.Candidates) > 1 {
				alts := make([]string, len(m.Candidates)-1)
				for i, c := range m.Candidates[1:] {
					alts[i] = c.OrgName
				}
				lane.MatchName += " (picked over " + strings.Join(alts, " / ") + ")"
			}
		case statusAmbiguous:
			lane.StatusLabel, lane.StatusClass = "Needs a quick check (more than one possible match)", "warn"
			names := make([]string, len(m.Candidates))
			for i, c := range m.Candidates {
				names[i] = c.OrgName
			}
			lane.MatchName = strings.Join(names, " / ")
		default:
			lane.StatusLabel, lane.StatusClass = "Not found on RegattaCentral", "bad"
		}
		rr.Lanes = append(rr.Lanes, lane)
	}

	for _, n := range order {
		data.Races = append(data.Races, *byRace[n])
	}
	for _, e := range unused {
		data.Unused = append(data.Unused, displayOrgName(e))
	}
	return data
}

// displayOrgName is what the report shows for an rcEntry's organization. An
// entry whose OrgID never resolved against the organizations index (see
// entriesFromDir) has a blank OrgName; show its RegattaCentral id instead of a
// blank bullet, so the report stays legible rather than silently dropping it.
func displayOrgName(e rcEntry) string {
	if e.OrgName != "" {
		return e.OrgName
	}
	if e.OrgID != "" {
		return fmt.Sprintf("Unknown organization (RegattaCentral id %s)", e.OrgID)
	}
	return "Unknown organization"
}

func writeReport(path string, data reportData) error {
	tmpl, err := template.New("report").Parse(reportTemplate)
	if err != nil {
		return fmt.Errorf("parse report template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("render report: %w", err)
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create report directory: %w", err)
		}
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	fmt.Fprintln(os.Stderr, "wrote", path)
	return nil
}

// reportTemplate is self-contained: inline CSS, no external assets, no
// scripts, safe to open straight from disk. Written for the regatta
// executives, not developers.
const reportTemplate = `<!doctype html>
<html>
<head>
<meta charset="utf-8">
<title>{{.RegattaName}} — RegattaCentral comparison</title>
<style>
  body { font-family: -apple-system, "Segoe UI", Helvetica, Arial, sans-serif; margin: 2rem auto; max-width: 900px; color: #1f2328; }
  h1 { font-size: 1.4rem; }
  h2 { font-size: 1.1rem; margin-top: 2.2rem; }
  p.note { color: #57606a; }
  .summary { background: #f6f8fa; border: 1px solid #d0d7de; border-radius: 6px; padding: 1rem 1.25rem; margin: 1.25rem 0 2rem; }
  table { border-collapse: collapse; width: 100%; margin-top: .5rem; }
  th, td { border: 1px solid #d0d7de; padding: .45rem .7rem; text-align: left; font-size: .95rem; }
  th { background: #f6f8fa; }
  .ok { color: #1a7f37; font-weight: 600; }
  .warn { color: #9a6700; font-weight: 600; }
  .bad { color: #cf222e; font-weight: 600; }
  ul.unused { color: #57606a; }
</style>
</head>
<body>
  <h1>{{.RegattaName}} — comparing the lineup sheet with RegattaCentral</h1>
  <p class="note">Generated {{.Generated}}. This is a side-by-side check only — nothing was changed on RegattaCentral, and nothing here has been sent anywhere.</p>
  <div class="summary"><strong>{{.MatchedCount}} of {{.Total}} boats matched RegattaCentral automatically.</strong></div>

  {{range .Races}}
  <h2>Race {{.RaceNumber}}{{if .BoatClass}} — {{.BoatClass}}{{end}}{{if .FlightInfo}} ({{.FlightInfo}}){{end}}</h2>
  <table>
    <tr><th>Lane</th><th>Boat (from the lineup sheet)</th><th>RegattaCentral</th></tr>
    {{range .Lanes}}
    <tr>
      <td>{{.Lane}}</td>
      <td>{{.Boat}}</td>
      <td class="{{.StatusClass}}">{{.StatusLabel}}{{if .MatchName}} — {{.MatchName}}{{end}}</td>
    </tr>
    {{end}}
  </table>
  {{end}}

  {{if .Unused}}
  <h2>Registered on RegattaCentral but not seen in the lineup sheet</h2>
  <p class="note">These may be scratches, or boats the lineup sheet never included.</p>
  <ul class="unused">{{range .Unused}}<li>{{.}}</li>{{end}}</ul>
  {{end}}
</body>
</html>
`
