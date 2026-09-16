// Command rcprobe is a developer tool for exercising internal/regattacentral
// against the live RegattaCentral v4 API before any UI exists. It is NOT
// shipped: release.yml packages named binaries, not ./..., so rcprobe stays out
// of releases while still being built by `go build ./...` in CI.
//
// It de-risks Phase A of the RegattaCentral integration - confirming the token
// response, the JSON shapes, whether /bulk carries per-seat eligibility, rate
// limits, and the Origin/referer requirement - and its --out captures become
// the goldens that drive the typed read model.
//
// Usage:
//
//	rcprobe [flags] <command> [args]
//
// Commands:
//
//	token                                   acquire and print an access token
//	bulk         [regattaID]                GET /regattas/{id}/bulk
//	events       [regattaID]                GET /regattas/{id}/events
//	entries      <eventID> [regattaID]      GET /regattas/{id}/events/{eventID}/entries
//	lanes        <eventID> [regattaID]      GET /regattas/{id}/events/{eventID}/lanes
//	results      <eventID> [regattaID]      GET /regattas/{id}/events/{eventID}/results
//	active       [regattaID]                GET /regattas/{id}/races?active
//	orgs         [regattaID]                GET /regattas/{id}/organizations
//	search-orgs  <name>                     GET /organizations?name=...
//	search-people <lastname> <yyyy-mm-dd>   GET /participants?lastname=...&birthdate=...
//
// Credentials come from internal/secretstore: an --secrets-file (a 0600 JSON
// file) or, by default, RC_CLIENT_ID / RC_CLIENT_SECRET / RC_USERNAME /
// RC_PASSWORD environment variables. Never pass secrets as flags.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/personacfg"
	"github.com/comagnaw/regattaClock/internal/regattacentral"
	"github.com/comagnaw/regattaClock/internal/secretstore"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "rcprobe:", err)
		os.Exit(1)
	}
}

type options struct {
	secretsFile string
	configFile  string
	baseURL     string
	tokenURL    string
	regattaID   string
	origin      string
	outDir      string
	timeout     time.Duration
}

func run(argv []string) error {
	fs := flag.NewFlagSet("rcprobe", flag.ContinueOnError)
	var o options
	fs.StringVar(&o.secretsFile, "secrets-file", "", "path to a JSON secrets file (default: RC_* environment variables)")
	fs.StringVar(&o.configFile, "config", "", "path to a personacfg deployment file for baseURL/regattaID")
	fs.StringVar(&o.baseURL, "base-url", "", "API base URL (overrides --config and the built-in default)")
	fs.StringVar(&o.tokenURL, "token-url", "", "OAuth2 token endpoint (overrides the built-in default; for pointing at a mock)")
	fs.StringVar(&o.regattaID, "regatta", "", "regatta id (overrides --config; also positional on most commands)")
	fs.StringVar(&o.origin, "origin", "", "Origin header to send")
	fs.StringVar(&o.outDir, "out", "", "directory to also write each raw JSON response into")
	fs.DurationVar(&o.timeout, "timeout", 30*time.Second, "per-request timeout")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: rcprobe [flags] <command> [args]\n\nflags:\n")
		fs.PrintDefaults()
		fmt.Fprint(os.Stderr, "\ncommands: token, bulk, events, entries, lanes, results, active, orgs, search-orgs, search-people\n")
	}
	if err := fs.Parse(argv); err != nil {
		return err
	}
	args := fs.Args()
	if len(args) == 0 {
		fs.Usage()
		return fmt.Errorf("no command given")
	}
	cmd, rest := args[0], args[1:]

	client, defaultRegatta, err := newClient(o)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	pick := func(positional int) string {
		if len(rest) > positional && strings.TrimSpace(rest[positional]) != "" {
			return rest[positional]
		}
		return defaultRegatta
	}
	fetch := func(name string, fn func() (json.RawMessage, error)) error {
		raw, err := fn()
		return dump(o.outDir, name, raw, err)
	}

	switch cmd {
	case "token":
		tok, err := client.Token(ctx)
		if err != nil {
			return err
		}
		return emit(o.outDir, "token", map[string]string{"access_token": tok})

	case "bulk":
		return fetch("bulk", func() (json.RawMessage, error) { return client.Bulk(ctx, pick(0)) })
	case "events":
		return fetch("events", func() (json.RawMessage, error) { return client.Events(ctx, pick(0)) })
	case "active":
		return fetch("active-races", func() (json.RawMessage, error) { return client.ActiveRaces(ctx, pick(0)) })
	case "orgs":
		return fetch("organizations", func() (json.RawMessage, error) { return client.Organizations(ctx, pick(0)) })

	case "entries", "lanes", "results":
		if len(rest) == 0 {
			return fmt.Errorf("%s: need an <eventID>", cmd)
		}
		eventID, regatta := rest[0], pick(1)
		return fetch(cmd+"-"+eventID, func() (json.RawMessage, error) {
			switch cmd {
			case "entries":
				return client.EventEntries(ctx, regatta, eventID)
			case "lanes":
				return client.EventLanes(ctx, regatta, eventID)
			default:
				return client.EventResults(ctx, regatta, eventID)
			}
		})

	case "search-orgs":
		if len(rest) == 0 {
			return fmt.Errorf("search-orgs: need a <name>")
		}
		return fetch("search-orgs-"+rest[0], func() (json.RawMessage, error) {
			return client.SearchOrganizations(ctx, rest[0])
		})

	case "search-people":
		if len(rest) < 2 {
			return fmt.Errorf("search-people: need <lastname> <yyyy-mm-dd>")
		}
		return fetch("search-people-"+rest[0], func() (json.RawMessage, error) {
			return client.SearchParticipants(ctx, rest[0], rest[1])
		})

	default:
		fs.Usage()
		return fmt.Errorf("unknown command %q", cmd)
	}
}

func newClient(o options) (*regattacentral.Client, string, error) {
	if o.secretsFile != "" {
		secretstore.SetBackend(secretstore.NewFileBackend(o.secretsFile))
	} else {
		secretstore.SetBackend(secretstore.NewEnvBackend("RC_"))
	}
	creds, err := regattacentral.LoadCredentials()
	if err != nil {
		return nil, "", fmt.Errorf("%w\n(set RC_CLIENT_ID / RC_CLIENT_SECRET / RC_USERNAME / RC_PASSWORD, or pass --secrets-file)", err)
	}

	baseURL, regatta := o.baseURL, o.regattaID
	if o.configFile != "" {
		cfg, err := personacfg.Load(o.configFile)
		if err != nil {
			return nil, "", err
		}
		if rc, ok := cfg.RegattaCentralConfig(); ok {
			if baseURL == "" {
				baseURL = rc.BaseURL
			}
			if regatta == "" {
				regatta = rc.RegattaID
			}
		}
	}

	client, err := regattacentral.New(regattacentral.Config{
		Credentials: creds,
		RegattaID:   regatta,
		BaseURL:     baseURL,
		TokenURL:    o.tokenURL,
		Origin:      o.origin,
		Timeout:     o.timeout,
	})
	if err != nil {
		return nil, "", err
	}
	return client, regatta, nil
}

// dump prints raw JSON (indented) to stdout and, when outDir is set, writes it
// to <outDir>/<name>.json.
func dump(outDir, name string, raw json.RawMessage, err error) error {
	if err != nil {
		return err
	}
	var pretty any
	if e := json.Unmarshal(raw, &pretty); e != nil {
		// Not JSON (unlikely) - print/save verbatim.
		pretty = string(raw)
	}
	return emit(outDir, name, pretty)
}

func emit(outDir, name string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	if outDir == "" {
		return nil
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	path := filepath.Join(outDir, filesystem.SanitizeForFilename(name)+".json")
	if err := os.WriteFile(path, append(b, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "wrote", path)
	return nil
}
