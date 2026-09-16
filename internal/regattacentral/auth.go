package regattacentral

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// expirySkew is how far before the stated expiry a token is treated as stale,
// so a request never goes out with a token about to be rejected.
const expirySkew = 60 * time.Second

// tokenResponse is the OAuth2 token endpoint payload.
// PROVISIONAL: field names assumed standard; confirm with cmd/rcprobe.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"` // seconds
	TokenType    string `json:"token_type"`
}

// tokenSource acquires and refreshes the OAuth2 access token for one Client.
// All state is guarded by mu; token() is safe for concurrent callers.
type tokenSource struct {
	cfg *Config

	mu           sync.Mutex
	accessToken  string
	refreshToken string
	expiry       time.Time
}

func newTokenSource(cfg *Config) *tokenSource { return &tokenSource{cfg: cfg} }

// token returns a currently-valid access token, doing a refresh-token refresh
// or a full password grant as needed.
func (t *tokenSource) token(ctx context.Context) (string, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.accessToken != "" && t.cfg.now().Before(t.expiry.Add(-expirySkew)) {
		return t.accessToken, nil
	}
	if t.refreshToken != "" {
		if err := t.grantLocked(ctx, url.Values{
			"grant_type":    {"refresh_token"},
			"refresh_token": {t.refreshToken},
		}); err == nil {
			return t.accessToken, nil
		}
		// Refresh failed (expired / revoked): fall back to a password grant.
	}
	if err := t.grantLocked(ctx, url.Values{
		"grant_type": {"password"},
		"username":   {t.cfg.Credentials.Username},
		"password":   {t.cfg.Credentials.Password},
	}); err != nil {
		return "", err
	}
	return t.accessToken, nil
}

// invalidate drops the cached access token so the next token() re-acquires. The
// transport calls this once after a 401.
func (t *tokenSource) invalidate() {
	t.mu.Lock()
	t.accessToken = ""
	t.expiry = time.Time{}
	t.mu.Unlock()
}

// grantLocked posts a token request, merging in the client credentials, and
// stores the result. Caller holds t.mu.
func (t *tokenSource) grantLocked(ctx context.Context, form url.Values) error {
	form.Set("client_id", t.cfg.Credentials.ClientID)
	form.Set("client_secret", t.cfg.Credentials.ClientSecret)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.cfg.TokenURL,
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("regattacentral: build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := t.cfg.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("regattacentral: token request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	if resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("%w: HTTP %d: %s", ErrAuth, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if resp.StatusCode/100 != 2 {
		return &APIError{Method: http.MethodPost, URL: t.cfg.TokenURL, StatusCode: resp.StatusCode, Body: string(body)}
	}

	var tr tokenResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return fmt.Errorf("regattacentral: decode token response: %w", err)
	}
	if tr.AccessToken == "" {
		return fmt.Errorf("%w: token response carried no access_token", ErrAuth)
	}

	t.accessToken = tr.AccessToken
	if tr.RefreshToken != "" {
		t.refreshToken = tr.RefreshToken
	}
	ttl := time.Duration(tr.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour // server omitted expires_in; re-check conservatively
	}
	t.expiry = t.cfg.now().Add(ttl)
	return nil
}
