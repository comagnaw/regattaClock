package regattacentral

import (
	"errors"
	"fmt"

	"github.com/comagnaw/regattaClock/internal/secretstore"
)

// Secret store coordinates. The service is the app; the keys are namespaced to
// this integration so a later one can share the store.
const (
	SecretService   = "regattaClock"
	KeyClientID     = "regattacentral/client_id"
	KeyClientSecret = "regattacentral/client_secret"
	KeyUsername     = "regattacentral/username"
	KeyPassword     = "regattacentral/password"
)

// SecretKeys lists the keys LoadCredentials reads, in credential order. Useful
// for a setup command or a --help listing.
var SecretKeys = []string{KeyClientID, KeyClientSecret, KeyUsername, KeyPassword}

// LoadCredentials reads the four credential strings from the process-wide
// secretstore (whatever backend the caller installed via secretstore.SetBackend).
// A missing or unreachable secret is returned as an error naming the key; the
// caller decides whether to prompt or abort. It does no environment fallback of
// its own - that belongs to an EnvBackend the caller chose to install.
func LoadCredentials() (Credentials, error) {
	get := func(key string) (string, error) {
		v, err := secretstore.Get(SecretService, key)
		if err != nil {
			if errors.Is(err, secretstore.ErrNotFound) {
				return "", fmt.Errorf("regattacentral: credential %q not set", key)
			}
			return "", fmt.Errorf("regattacentral: read credential %q: %w", key, err)
		}
		return v, nil
	}

	var (
		c   Credentials
		err error
	)
	if c.ClientID, err = get(KeyClientID); err != nil {
		return Credentials{}, err
	}
	if c.ClientSecret, err = get(KeyClientSecret); err != nil {
		return Credentials{}, err
	}
	if c.Username, err = get(KeyUsername); err != nil {
		return Credentials{}, err
	}
	if c.Password, err = get(KeyPassword); err != nil {
		return Credentials{}, err
	}
	return c, nil
}
