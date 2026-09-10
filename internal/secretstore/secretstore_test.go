package secretstore

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

const svc = "regattaClock"

func TestDefaultBackendUnavailable(t *testing.T) {
	SetBackend(nil)
	t.Cleanup(func() { SetBackend(nil) })

	if _, err := Get(svc, "regattacentral/client_id"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Get on unconfigured store: got %v, want ErrUnavailable", err)
	}
	if err := Set(svc, "k", "v"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Set on unconfigured store: got %v, want ErrUnavailable", err)
	}
	if err := Delete(svc, "k"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Delete on unconfigured store: got %v, want ErrUnavailable", err)
	}
}

func TestSetBackendDelegatesAndResets(t *testing.T) {
	t.Cleanup(func() { SetBackend(nil) })

	mem := NewMemoryBackend()
	SetBackend(mem)

	if err := Set(svc, "regattacentral/username", "cox@example.com"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	got, err := Get(svc, "regattacentral/username")
	if err != nil || got != "cox@example.com" {
		t.Fatalf("Get after Set: got %q, %v", got, err)
	}

	SetBackend(nil)
	if _, err := Get(svc, "regattacentral/username"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("Get after reset: got %v, want ErrUnavailable", err)
	}
}

func TestEnvBackendVarName(t *testing.T) {
	e := NewEnvBackend("RC_")
	cases := map[string]string{
		"regattacentral/client_id":     "RC_CLIENT_ID",
		"regattacentral/client_secret": "RC_CLIENT_SECRET",
		"username":                     "RC_USERNAME",
		"a/b/c-d":                      "RC_C_D",
		"trailing/":                    "RC_",
	}
	for key, want := range cases {
		if got := e.VarName(key); got != want {
			t.Errorf("VarName(%q) = %q, want %q", key, got, want)
		}
	}
	if got := (&EnvBackend{}).VarName("client_id"); got != "CLIENT_ID" {
		t.Errorf("empty prefix VarName = %q, want CLIENT_ID", got)
	}
}

func TestEnvBackend(t *testing.T) {
	t.Cleanup(func() { SetBackend(nil) })

	vars := map[string]string{"RC_CLIENT_ID": "abc123", "RC_CLIENT_SECRET": ""}
	e := &EnvBackend{Prefix: "RC_", lookup: func(k string) (string, bool) {
		v, ok := vars[k]
		return v, ok
	}}
	SetBackend(e)

	if got, err := Get(svc, "regattacentral/client_id"); err != nil || got != "abc123" {
		t.Fatalf("Get present var: got %q, %v", got, err)
	}
	if _, err := Get(svc, "regattacentral/client_secret"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get empty var: got %v, want ErrNotFound", err)
	}
	if _, err := Get(svc, "regattacentral/password"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get missing var: got %v, want ErrNotFound", err)
	}
	if err := Set(svc, "regattacentral/password", "x"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Set on EnvBackend: got %v, want ErrReadOnly", err)
	}
	if err := Delete(svc, "regattacentral/password"); !errors.Is(err, ErrReadOnly) {
		t.Fatalf("Delete on EnvBackend: got %v, want ErrReadOnly", err)
	}
}

func TestFileBackendRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "secrets.json")
	f := NewFileBackend(path)

	if _, err := f.Get(svc, "regattacentral/client_id"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get before file exists: got %v, want ErrNotFound", err)
	}

	if err := f.Set(svc, "regattacentral/client_id", "id-1"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if err := f.Set(svc, "regattacentral/client_secret", "sec-1"); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// A fresh instance reads what the first wrote.
	f2 := NewFileBackend(path)
	if got, err := f2.Get(svc, "regattacentral/client_secret"); err != nil || got != "sec-1" {
		t.Fatalf("Get from second instance: got %q, %v", got, err)
	}

	if err := f2.Delete(svc, "regattacentral/client_id"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := f2.Get(svc, "regattacentral/client_id"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: got %v, want ErrNotFound", err)
	}
	// Deleting an absent key is a no-op, not an error.
	if err := f2.Delete(svc, "regattacentral/client_id"); err != nil {
		t.Fatalf("Delete absent key: %v", err)
	}

	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("secrets file perm = %o, want 600", perm)
		}
	}
}

func TestFileBackendEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "secrets.json")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("seed empty file: %v", err)
	}
	f := NewFileBackend(path)
	if _, err := f.Get(svc, "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get from empty file: got %v, want ErrNotFound", err)
	}
	if err := f.Set(svc, "k", "v"); err != nil {
		t.Fatalf("Set into empty file: %v", err)
	}
	if got, _ := f.Get(svc, "k"); got != "v" {
		t.Fatalf("Get after Set: %q", got)
	}
}

func TestMemoryBackend(t *testing.T) {
	b := NewMemoryBackend()
	if _, err := b.Get(svc, "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get empty: got %v, want ErrNotFound", err)
	}
	if err := b.Set(svc, "k", "v"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got, err := b.Get(svc, "k"); err != nil || got != "v" {
		t.Fatalf("Get: got %q, %v", got, err)
	}
	// Distinct services do not collide on the same key.
	if _, err := b.Get("other", "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-service leak: got %v, want ErrNotFound", err)
	}
	if err := b.Delete(svc, "k"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := b.Get(svc, "k"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get after Delete: got %v, want ErrNotFound", err)
	}
}
