package regattacentral

import (
	"errors"
	"fmt"
	"net/http"
)

// Sentinel errors. Callers test with errors.Is.
var (
	// ErrNoCredentials means New was called without a complete credential set.
	ErrNoCredentials = errors.New("regattacentral: incomplete credentials")

	// ErrNoRegattaID means a method needing a regatta id was called with "" and
	// Config.RegattaID is also empty.
	ErrNoRegattaID = errors.New("regattacentral: no regatta id")

	// ErrAuth means the token endpoint rejected the credentials (or the refresh
	// token). It is not retried.
	ErrAuth = errors.New("regattacentral: authentication failed")
)

// APIError is a non-2xx response from an API endpoint. The body is captured
// (capped) for logging; RegattaCentral's error shape is not yet modelled.
type APIError struct {
	Method     string
	URL        string
	StatusCode int
	Body       string
}

func (e *APIError) Error() string {
	b := e.Body
	if len(b) > 300 {
		b = b[:300] + "…"
	}
	return fmt.Sprintf("regattacentral: %s %s: HTTP %d %s: %s",
		e.Method, e.URL, e.StatusCode, http.StatusText(e.StatusCode), b)
}

// Temporary reports whether retrying the request might succeed.
func (e *APIError) Temporary() bool {
	return e.StatusCode == http.StatusTooManyRequests || e.StatusCode >= 500
}
