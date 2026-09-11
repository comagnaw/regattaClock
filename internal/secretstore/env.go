package secretstore

import (
	"os"
	"strings"
)

// EnvBackend reads secrets from environment variables. It is read-only: Set and
// Delete return ErrReadOnly.
//
// The variable name is Prefix followed by the last "/"-separated segment of key,
// upper-cased with every run of non-alphanumeric characters collapsed to a
// single "_". With Prefix "RC_", key "regattacentral/client_id" reads
// RC_CLIENT_ID. service is not part of the name, so two services that share a
// final key segment collide under one EnvBackend - give them separate prefixes
// if that ever matters.
type EnvBackend struct {
	Prefix string

	// lookup defaults to os.LookupEnv; tests override it.
	lookup func(string) (string, bool)
}

// NewEnvBackend returns an EnvBackend whose variable names start with prefix.
func NewEnvBackend(prefix string) *EnvBackend {
	return &EnvBackend{Prefix: prefix, lookup: os.LookupEnv}
}

// VarName reports the environment variable EnvBackend reads for key. Exposed so
// a caller (or its --help text) can tell the operator exactly what to export.
func (e *EnvBackend) VarName(key string) string {
	seg := key
	if i := strings.LastIndexByte(seg, '/'); i >= 0 {
		seg = seg[i+1:]
	}

	var b strings.Builder
	b.WriteString(e.Prefix)
	pendingUnderscore := false
	written := false
	for _, r := range strings.ToUpper(seg) {
		if (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			if pendingUnderscore && written {
				b.WriteByte('_')
			}
			b.WriteRune(r)
			written = true
			pendingUnderscore = false
			continue
		}
		pendingUnderscore = true
	}
	return b.String()
}

func (e *EnvBackend) Get(_, key string) (string, error) {
	lookup := e.lookup
	if lookup == nil {
		lookup = os.LookupEnv
	}
	if v, ok := lookup(e.VarName(key)); ok && v != "" {
		return v, nil
	}
	return "", ErrNotFound
}

func (e *EnvBackend) Set(_, _, _ string) error { return ErrReadOnly }
func (e *EnvBackend) Delete(_, _ string) error { return ErrReadOnly }
