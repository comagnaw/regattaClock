// Package personacfg parses the optional deployment file that lets an
// established organisation pin a computer to a persona (skipping the startup
// picker) and/or replace the built-in challenge codes with its own.
//
// It is a leaf: it imports internal/persona for the persona registry and
// internal/filesystem to read the file, and nothing else. internal/regatta owns
// the preference key, the Configuration screen row, and the startup wiring that
// acts on a parsed Config.
package personacfg

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/comagnaw/regattaClock/internal/filesystem"
	"github.com/comagnaw/regattaClock/internal/persona"
)

// Config is the parsed deployment file. Every section is optional: a file may
// carry host assignments, challenge overrides, RegattaCentral config, any
// combination, or none.
type Config struct {
	// Hosts maps a hostname to the persona ID that computer operates as. A
	// match skips the picker entirely - no challenge is asked.
	Hosts map[string]string `json:"hosts"`

	// Challenges maps a persona ID to the challenge code that replaces its
	// built-in rc-* code. Only consulted when the picker is shown.
	Challenges map[string]string `json:"challenges"`

	// RegattaCentral is the non-secret RegattaCentral API configuration for
	// this deployment. Absent unless the section is present in the file.
	RegattaCentral *RegattaCentral `json:"regattacentral,omitempty"`
}

// RegattaCentral is the non-secret half of the RegattaCentral API configuration:
// which API to talk to and which regatta this deployment is timing. The secrets
// (OAuth2 client id / secret, the operator's RegattaCentral login) never appear
// in this file - they live in internal/secretstore.
type RegattaCentral struct {
	// BaseURL is the API root, e.g. "https://api.regattacentral.com/v4.0/".
	// Optional: when empty the internal/regattacentral client uses its own
	// default. When set it must be an absolute http(s) URL.
	BaseURL string `json:"baseURL,omitempty"`

	// RegattaID identifies the regatta on RegattaCentral. Required when the
	// section is present - a RegattaCentral block that does not say which
	// regatta is meaningless.
	RegattaID string `json:"regattaID"`
}

// Load reads and validates the file at path. A missing file surfaces as an
// fs.ErrNotExist-wrapped error (errors.Is still works through the wrap), which
// the caller treats the same as any other failure - fall back to the picker -
// but can log differently. An unknown persona ID or a blank challenge code is a
// hard error: a deployment file that does not say what its author meant is not
// silently half-applied.
func Load(path string) (*Config, error) {
	var c Config
	if err := filesystem.ReadJSONFile(&c, path); err != nil {
		return nil, fmt.Errorf("persona config %q: %w", path, err)
	}

	for id, code := range c.Challenges {
		if _, ok := persona.ByID(id); !ok {
			return nil, fmt.Errorf("persona config: unknown persona ID %q in \"challenges\"", id)
		}
		if strings.TrimSpace(code) == "" {
			return nil, fmt.Errorf("persona config: empty challenge code for persona %q", id)
		}
	}

	for host, id := range c.Hosts {
		if strings.TrimSpace(host) == "" {
			return nil, fmt.Errorf("persona config: empty hostname key in \"hosts\"")
		}
		if strings.TrimSpace(id) == "" {
			return nil, fmt.Errorf("persona config: no persona ID for host %q", host)
		}
		if _, ok := persona.ByID(strings.TrimSpace(id)); !ok {
			return nil, fmt.Errorf("persona config: unknown persona ID %q for host %q", id, host)
		}
	}

	if rc := c.RegattaCentral; rc != nil {
		if strings.TrimSpace(rc.RegattaID) == "" {
			return nil, fmt.Errorf("persona config: \"regattacentral\" is present but has no \"regattaID\"")
		}
		if base := strings.TrimSpace(rc.BaseURL); base != "" {
			u, err := url.Parse(base)
			if err != nil {
				return nil, fmt.Errorf("persona config: \"regattacentral.baseURL\" %q is not a valid URL: %w", base, err)
			}
			if !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
				return nil, fmt.Errorf("persona config: \"regattacentral.baseURL\" %q must be an absolute http(s) URL", base)
			}
		}
	}

	return &c, nil
}

// AssignmentFor returns the persona this hostname is pinned to. Matching is
// case-insensitive and whitespace-trimmed; an FQDN also matches an entry for
// its first label ("timer-1.regatta.local" matches "timer-1"), since os.Hostname
// reports the short name on some platforms and the FQDN on others.
func (c *Config) AssignmentFor(hostname string) (persona.Definition, bool) {
	if c == nil || len(c.Hosts) == 0 {
		return persona.Definition{}, false
	}
	h := strings.ToLower(strings.TrimSpace(hostname))
	if h == "" {
		return persona.Definition{}, false
	}

	lookup := func(key string) (persona.Definition, bool) {
		for k, v := range c.Hosts {
			if strings.ToLower(strings.TrimSpace(k)) == key {
				return persona.ByID(strings.TrimSpace(v))
			}
		}
		return persona.Definition{}, false
	}

	if d, ok := lookup(h); ok {
		return d, true
	}
	if i := strings.IndexByte(h, '.'); i > 0 {
		return lookup(h[:i])
	}
	return persona.Definition{}, false
}

// MatchesChallenge reports whether input is the accepted challenge for def. When
// the config overrides def's code, the override is the only accepted value - the
// built-in rc-* no longer works. Personas with no override fall through to their
// built-in code. Comparison mirrors persona.MatchesChallenge (lower-case,
// trimmed, empty input always fails).
func (c *Config) MatchesChallenge(def persona.Definition, input string) bool {
	if c != nil {
		if code, ok := c.Challenges[def.ID]; ok && strings.TrimSpace(code) != "" {
			got := strings.ToLower(strings.TrimSpace(input))
			return got != "" && got == strings.ToLower(strings.TrimSpace(code))
		}
	}
	return def.MatchesChallenge(input)
}

// RegattaCentralConfig returns the deployment's RegattaCentral configuration and
// whether it was supplied. Load has already validated a non-nil result:
// RegattaID is non-empty and BaseURL, if set, is an absolute http(s) URL.
func (c *Config) RegattaCentralConfig() (RegattaCentral, bool) {
	if c == nil || c.RegattaCentral == nil {
		return RegattaCentral{}, false
	}
	rc := *c.RegattaCentral
	rc.BaseURL = strings.TrimSpace(rc.BaseURL)
	rc.RegattaID = strings.TrimSpace(rc.RegattaID)
	return rc, true
}
