package regattacentral

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/comagnaw/regattaClock/internal/applog"
)

// retryBaseBackoff is the first wait after a 429 / 5xx; it doubles each attempt.
// A Retry-After header, when present and shorter-or-longer, wins.
const retryBaseBackoff = 500 * time.Millisecond

// maxResponseBytes caps a decoded response body. /bulk for a large regatta is
// still well under this; it exists so a misbehaving endpoint cannot exhaust
// memory.
const maxResponseBytes = 32 << 20 // 32 MiB

// do performs one API call: method + path (relative to BaseURL) + optional raw
// query string. body, if non-nil, is JSON-encoded as the request payload; out,
// if non-nil, receives the JSON-decoded response. 429 / 5xx are retried with
// backoff; a single 401 triggers a token refresh and one more attempt.
func (c *Client) do(ctx context.Context, method, path, rawQuery string, body, out any) error {
	url := c.cfg.BaseURL + strings.TrimPrefix(path, "/")
	if rawQuery != "" {
		url += "?" + rawQuery
	}

	var payload []byte
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("regattacentral: marshal request body: %w", err)
		}
		payload = b
	}

	attempts := c.cfg.MaxRetries + 1
	refreshed := false
	var lastErr error

	for attempt := 1; attempt <= attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}

		token, err := c.token.token(ctx)
		if err != nil {
			return err
		}

		var reqBody io.Reader
		if payload != nil {
			reqBody = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
		if err != nil {
			return fmt.Errorf("regattacentral: build request: %w", err)
		}
		// PROVISIONAL: the Cookbook says the token is sent "as a HTTP header
		// Authorization" without stating a scheme. Sent verbatim for now; if a
		// live test shows "Bearer " is required this is the one line to change.
		req.Header.Set("Authorization", token)
		req.Header.Set("Accept", "application/json")
		if payload != nil {
			req.Header.Set("Content-Type", "application/json; charset=UTF-8")
		}
		if c.cfg.Origin != "" {
			req.Header.Set("Origin", c.cfg.Origin)
		}

		resp, err := c.cfg.HTTPClient.Do(req)
		if err != nil {
			// Transport error (DNS, connection reset, timeout): retry within budget.
			lastErr = fmt.Errorf("regattacentral: %s %s: %w", method, url, err)
			if attempt < attempts {
				if werr := c.cfg.sleep(ctx, backoffFor(attempt, "")); werr != nil {
					return werr
				}
				continue
			}
			return lastErr
		}

		respBody, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
		resp.Body.Close()

		switch {
		case resp.StatusCode/100 == 2:
			if readErr != nil {
				return fmt.Errorf("regattacentral: read response: %w", readErr)
			}
			if out == nil || len(respBody) == 0 {
				return nil
			}
			if err := json.Unmarshal(respBody, out); err != nil {
				return fmt.Errorf("regattacentral: decode %s %s: %w", method, url, err)
			}
			return nil

		case resp.StatusCode == http.StatusUnauthorized && !refreshed:
			refreshed = true
			c.token.invalidate()
			applog.Info("rc token refresh on 401", "component", "regattacentral", "url", url)
			attempt-- // this attempt does not count against the retry budget
			continue

		case resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500:
			lastErr = &APIError{Method: method, URL: url, StatusCode: resp.StatusCode, Body: string(respBody)}
			if attempt < attempts {
				wait := backoffFor(attempt, resp.Header.Get("Retry-After"))
				applog.Warn("rc retry", "component", "regattacentral", "url", url,
					"status", resp.StatusCode, "attempt", attempt, "wait_ms", wait.Milliseconds())
				if werr := c.cfg.sleep(ctx, wait); werr != nil {
					return werr
				}
				continue
			}
			return lastErr

		default:
			return &APIError{Method: method, URL: url, StatusCode: resp.StatusCode, Body: string(respBody)}
		}
	}

	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("regattacentral: %s %s: exhausted %d attempts", method, url, attempts)
}

// backoffFor returns the wait before the next attempt: the Retry-After header if
// it parses as a delay, otherwise exponential backoff from retryBaseBackoff.
func backoffFor(attempt int, retryAfter string) time.Duration {
	if retryAfter = strings.TrimSpace(retryAfter); retryAfter != "" {
		if secs, err := strconv.Atoi(retryAfter); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
		if when, err := http.ParseTime(retryAfter); err == nil {
			if d := time.Until(when); d > 0 {
				return d
			}
			return 0
		}
	}
	d := retryBaseBackoff
	for i := 1; i < attempt; i++ {
		d *= 2
	}
	if d > 30*time.Second {
		d = 30 * time.Second
	}
	return d
}
