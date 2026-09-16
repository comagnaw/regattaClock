// Package secretstore keeps small credential strings out of the synced
// regattaData/ tree and out of Fyne Preferences. Today the only secrets are the
// RegattaCentral OAuth2 client id / secret and the operator's RegattaCentral
// login (see internal/regattacentral); this is the codebase's first stored
// secret.
//
// The intended long-term backend is the OS keyring (macOS Keychain, Windows
// Credential Manager, freedesktop Secret Service). That library is deliberately
// not wired yet: a secret is addressed through the Backend interface, and until
// the keyring lands a caller configures an EnvBackend and/or a FileBackend via
// SetBackend. An unconfigured store - or a backend that cannot reach its
// underlying store, e.g. no Secret Service on a headless Linux box - returns
// ErrUnavailable, and the caller decides how to degrade (prompt, read a config
// file, run read-only).
package secretstore

import (
	"errors"
	"sync"
)

// Sentinel errors. Callers test with errors.Is; backends must wrap these rather
// than return bespoke strings.
var (
	// ErrUnavailable means there is no usable backend: none configured, or the
	// configured one cannot reach its underlying store.
	ErrUnavailable = errors.New("secretstore: no secret backend available")

	// ErrNotFound means the backend works but holds no value for that key.
	ErrNotFound = errors.New("secretstore: secret not found")

	// ErrReadOnly is returned by Set / Delete on a backend that cannot write,
	// such as EnvBackend.
	ErrReadOnly = errors.New("secretstore: backend is read-only")
)

// Backend is somewhere secrets are read from and, optionally, written to. A
// secret is addressed by (service, key): service is a stable namespace for the
// application, key names one secret within it. Implementations return ErrNotFound
// for an absent key and ErrUnavailable when the store itself is unreachable.
type Backend interface {
	Get(service, key string) (string, error)
	Set(service, key, value string) error
	Delete(service, key string) error
}

var (
	mu     sync.RWMutex
	active Backend = unavailable{}
)

// SetBackend installs the process-wide backend. Pass nil to reset to the
// unconfigured (ErrUnavailable) state. Call once during startup.
func SetBackend(b Backend) {
	mu.Lock()
	defer mu.Unlock()
	if b == nil {
		active = unavailable{}
		return
	}
	active = b
}

// Get returns the secret for (service, key) from the process-wide backend.
func Get(service, key string) (string, error) { return current().Get(service, key) }

// Set stores value for (service, key) on the process-wide backend.
func Set(service, key, value string) error { return current().Set(service, key, value) }

// Delete removes (service, key) from the process-wide backend. Removing a key
// that is absent is not an error.
func Delete(service, key string) error { return current().Delete(service, key) }

func current() Backend {
	mu.RLock()
	defer mu.RUnlock()
	return active
}

// unavailable is the default backend: every operation fails with ErrUnavailable.
type unavailable struct{}

func (unavailable) Get(_, _ string) (string, error) { return "", ErrUnavailable }
func (unavailable) Set(_, _, _ string) error        { return ErrUnavailable }
func (unavailable) Delete(_, _ string) error        { return ErrUnavailable }
