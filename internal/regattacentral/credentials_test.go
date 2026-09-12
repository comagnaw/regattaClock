package regattacentral

import (
	"testing"

	"github.com/comagnaw/regattaClock/internal/secretstore"
)

func setSecrets(t *testing.T, kv map[string]string) {
	t.Helper()
	backend := secretstore.NewMemoryBackend()
	for k, v := range kv {
		if err := backend.Set(SecretService, k, v); err != nil {
			t.Fatal(err)
		}
	}
	secretstore.SetBackend(backend)
	t.Cleanup(func() { secretstore.SetBackend(nil) })
}

func TestLoadCredentialsAPIKeyOptional(t *testing.T) {
	setSecrets(t, map[string]string{
		KeyClientID:     "cid",
		KeyClientSecret: "csec",
		KeyUsername:     "user@example.com",
		KeyPassword:     "pw",
		// KeyAPIKey deliberately absent.
	})

	c, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if c.APIKey != "" {
		t.Errorf("APIKey = %q, want empty when not set", c.APIKey)
	}
	if !c.valid() {
		t.Error("valid() = false, want true - APIKey must not gate validity")
	}
}

func TestLoadCredentialsAPIKeyPresent(t *testing.T) {
	setSecrets(t, map[string]string{
		KeyClientID:     "cid",
		KeyClientSecret: "csec",
		KeyUsername:     "user@example.com",
		KeyPassword:     "pw",
		KeyAPIKey:       "the-api-key",
	})

	c, err := LoadCredentials()
	if err != nil {
		t.Fatalf("LoadCredentials: %v", err)
	}
	if c.APIKey != "the-api-key" {
		t.Errorf("APIKey = %q, want \"the-api-key\"", c.APIKey)
	}
}

func TestLoadCredentialsStillRequiresTheOriginalFour(t *testing.T) {
	setSecrets(t, map[string]string{
		KeyClientID: "cid",
		// ClientSecret/Username/Password deliberately absent.
	})

	if _, err := LoadCredentials(); err == nil {
		t.Error("LoadCredentials() = nil error, want one for the missing required keys")
	}
}
