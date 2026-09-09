/*
Package version exposes the build-time attributes compiled into the binary.

The exported vars below are overridden at link time by scripts/compile (and the
release workflow) via `go build -ldflags -X`. A plain `go build` / `go run` leaves
them at "dev".
*/
package version

import (
	"encoding/json"
	"strings"
)

// dev is the value every attribute has when the binary was not built through
// scripts/compile (a `go run`, a bare `go build`, or `go test`).
const dev = "dev"

// Overridden at link time (-ldflags -X github.com/comagnaw/regattaClock/internal/version.<name>=...).
//
// RepoURL is the canonical source location (the "View on GitHub" link in the
// Version window and the "source" field of `regattaClock -v`). It is assembled
// from GH_HOST/ORG/REPO in the Makefile - not hard-coded here - so a repo move
// is a one-line edit there.
var (
	Project     = dev
	ExeName     = dev
	BuildBranch = dev
	BuildDate   = dev
	BuildCommit = dev
	BuildUser   = dev
	Version     = dev
	RepoURL     = dev
)

// Current is a snapshot of the link-time build attributes.
type Current struct {
	Project string `json:"project"`
	ExeName string `json:"exeName"`
	Branch  string `json:"branch"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	User    string `json:"user"`
	Version string `json:"version"`
	Source  string `json:"source"`
}

// Get returns the current build attributes.
func Get() Current {
	c := Current{
		Project: Project,
		ExeName: ExeName,
		Branch:  BuildBranch,
		Commit:  BuildCommit,
		Date:    BuildDate,
		User:    BuildUser,
		Version: Version,
	}
	c.Source = RepoURL
	if RepoURL != dev && RepoURL != "" && c.Commit != dev && c.Commit != "" {
		c.Source = RepoURL + "/commit/" + c.Commit
	}
	return c
}

// JSON renders the build attributes as indented JSON - the `regattaClock -v`
// output.
func (c Current) JSON() string {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

// Fields returns the attributes as ordered Key/Value pairs for a human-friendly
// display (the Version menu window). Source is excluded - the window renders it
// as a hyperlink.
func (c Current) Fields() [][2]string {
	return [][2]string{
		{"Version", c.Version},
		{"Project", c.Project},
		{"Executable", c.ExeName},
		{"Branch", c.Branch},
		{"Commit", c.Commit},
		{"Built", c.Date},
		{"By", c.User},
	}
}

// String is a plain multi-line "Label: value" rendering, for logs.
func (c Current) String() string {
	var b strings.Builder
	for _, kv := range c.Fields() {
		b.WriteString(kv[0])
		b.WriteString(": ")
		b.WriteString(kv[1])
		b.WriteByte('\n')
	}
	b.WriteString("Source: ")
	b.WriteString(c.Source)
	return b.String()
}
