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
// Version window and the "source" field of `regattaClock -v`). It is derived in
// the Makefile from the go.mod module path and the origin remote - not
// hard-coded here - so a repo move needs no edit.
//
// BuildCount / BuildDirty describe where the tree sits relative to the last
// release tag. They default to "" (not "dev") because empty is the honest state
// for a build that did not go through scripts/compile: unknown, so no suffix.
var (
	Project     = dev
	ExeName     = dev
	BuildBranch = dev
	BuildDate   = dev
	BuildCommit = dev
	BuildUser   = dev
	Version     = dev
	RepoURL     = dev
	BuildCount  = "" // commits between the most recent tag and HEAD; "" or "0" => no +build suffix
	BuildDirty  = "" // "true" when the working tree had uncommitted changes at build time
)

// Current is a snapshot of the link-time build attributes.
type Current struct {
	// Full is the headline version string: the release line plus, for a build
	// past its tag, semver +build metadata - "0.4.1-alpha+127.gc0ffee", or
	// "...+127.gc0ffee.dirty", or just "0.4.1-alpha" off an exact clean tag.
	Full    string `json:"full"`
	Version string `json:"version"` // the version-file value, verbatim - the release line this build sits on
	Build   string `json:"build"`   // "<count>.g<sha>" metadata; "" off an exact clean tag
	Dirty   bool   `json:"dirty"`
	Project string `json:"project"`
	ExeName string `json:"exeName"`
	Branch  string `json:"branch"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
	User    string `json:"user"`
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

	c.Dirty = BuildDirty == "true"
	if BuildCount != "" && BuildCount != "0" && BuildCommit != dev && BuildCommit != "" {
		c.Build = BuildCount + ".g" + BuildCommit
	}

	c.Full = c.Version
	switch {
	case c.Build != "" && c.Dirty:
		c.Full += "+" + c.Build + ".dirty"
	case c.Build != "":
		c.Full += "+" + c.Build
	case c.Dirty:
		c.Full += "+dirty"
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
	f := [][2]string{{"Version", c.Full}}
	if c.Build != "" || c.Dirty {
		// The decorated Full string is not itself a release; show the line it
		// sits on so the window is not misread as "this is release c.Full".
		f = append(f, [2]string{"Release", c.Version})
	}
	return append(f,
		[2]string{"Project", c.Project},
		[2]string{"Executable", c.ExeName},
		[2]string{"Branch", c.Branch},
		[2]string{"Commit", c.Commit},
		[2]string{"Built", c.Date},
		[2]string{"By", c.User},
	)
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
