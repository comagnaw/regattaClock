// Package regattacentral is the client for the RegattaCentral v4 API - the
// regatta registration service that also acts as the public results board.
//
// It is a leaf: standard library plus internal/secretstore (for the
// credentials helper in credentials.go) and internal/applog (structured
// logging). No Fyne, no internal/regatta. It owns OAuth2 auth (resource-owner
// password grant with refresh), an net/http transport with timeouts and
// retry/backoff, a typed JSON model, and read plus upload methods. Thin
// adapters in internal/regatta consume it.
//
// See docs/features/personas/regattacentral-integration.md and its vendored
// reference/RegattaCentral_APIV4_Cookbook.pdf. Several wire-format details
// (exact Authorization header form, model field names) are marked PROVISIONAL
// until confirmed against a live staff account with cmd/rcprobe.
package regattacentral

import (
	"context"
	"net/http"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the v4 API root used when Config.BaseURL is empty.
	DefaultBaseURL = "https://api.regattacentral.com/v4.0/"

	// DefaultTokenURL is the OAuth2 token endpoint. It sits outside the
	// versioned API root, so it is configured separately from BaseURL.
	DefaultTokenURL = "https://api.regattacentral.com/oauth2/api/token"

	// DefaultTimeout bounds a single HTTP request (each retry attempt).
	DefaultTimeout = 30 * time.Second

	// DefaultMaxRetries is how many times a 429 / 5xx response is retried
	// before the error is returned.
	DefaultMaxRetries = 3
)

// Credentials are the four secrets the password grant needs. They come from
// internal/secretstore (see LoadCredentials) or, for cmd/rcprobe, from the
// environment - never from a flag or the synced regattaData/ tree.
type Credentials struct {
	ClientID     string
	ClientSecret string
	Username     string
	Password     string
}

func (c Credentials) valid() bool {
	return c.ClientID != "" && c.ClientSecret != "" && c.Username != "" && c.Password != ""
}

// Config configures a Client. The zero value is not usable - Credentials must
// be set - but every other field falls back to a default.
type Config struct {
	// Credentials for the OAuth2 password grant. Required.
	Credentials Credentials

	// RegattaID this client operates on. Most methods take it explicitly too;
	// this is the default when they are called with "".
	RegattaID string

	// BaseURL overrides DefaultBaseURL. Must end in "/".
	BaseURL string

	// TokenURL overrides DefaultTokenURL.
	TokenURL string

	// Origin, when set, is sent as the Origin header. RegattaCentral requires
	// it for a client id that has a registered referer. PROVISIONAL: whether a
	// desktop client needs it at all is an open item.
	Origin string

	// Timeout bounds one HTTP attempt. <= 0 uses DefaultTimeout.
	Timeout time.Duration

	// MaxRetries for 429 / 5xx responses. <= 0 uses DefaultMaxRetries. A 401 is
	// handled separately (one token refresh + retry) and never counts here.
	MaxRetries int

	// HTTPClient is the underlying client. nil uses a new http.Client with
	// Timeout. Tests inject one pointed at an httptest server.
	HTTPClient *http.Client

	// now and sleep are indirection seams for tests. nil uses time.Now and a
	// context-aware sleep.
	now   func() time.Time
	sleep func(context.Context, time.Duration) error
}

func (c *Config) applyDefaults() {
	if c.BaseURL == "" {
		c.BaseURL = DefaultBaseURL
	}
	if !strings.HasSuffix(c.BaseURL, "/") {
		c.BaseURL += "/"
	}
	if c.TokenURL == "" {
		c.TokenURL = DefaultTokenURL
	}
	if c.Timeout <= 0 {
		c.Timeout = DefaultTimeout
	}
	if c.MaxRetries <= 0 {
		c.MaxRetries = DefaultMaxRetries
	}
	if c.HTTPClient == nil {
		c.HTTPClient = &http.Client{Timeout: c.Timeout}
	}
	if c.now == nil {
		c.now = time.Now
	}
	if c.sleep == nil {
		c.sleep = sleepCtx
	}
}

// Client talks to the RegattaCentral v4 API. It is safe for concurrent use; the
// token is guarded by its own mutex.
type Client struct {
	cfg   Config
	token *tokenSource
}

// New returns a Client. It performs no I/O; the first API call acquires a
// token. ErrNoCredentials is returned if the config has no complete credential
// set.
func New(cfg Config) (*Client, error) {
	if !cfg.Credentials.valid() {
		return nil, ErrNoCredentials
	}
	cfg.applyDefaults()
	c := &Client{cfg: cfg}
	c.token = newTokenSource(&c.cfg)
	return c, nil
}

// RegattaID resolves an explicit id against the configured default.
func (c *Client) regattaID(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	if c.cfg.RegattaID != "" {
		return c.cfg.RegattaID, nil
	}
	return "", ErrNoRegattaID
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return ctx.Err()
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
