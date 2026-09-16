package secretstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

// FileBackend stores secrets as one JSON object on disk, shaped for an operator
// to hand-edit:
//
//	{
//	  "regattaClock": {
//	    "regattacentral/client_id": "…",
//	    "regattacentral/client_secret": "…"
//	  }
//	}
//
// It is the fallback when no OS keyring is available (headless Linux) and the
// operator would rather not export environment variables. The file is written
// 0600 via a temp-file-and-rename so a concurrent reader never sees a truncated
// document; it is the caller's job to place it outside the synced regattaData/
// tree.
type FileBackend struct {
	path string
	mu   sync.Mutex
}

// NewFileBackend returns a FileBackend backed by the file at path. The file need
// not exist yet; the first Set creates it (and any missing parent directories).
func NewFileBackend(path string) *FileBackend { return &FileBackend{path: path} }

type secretsFile map[string]map[string]string

func (f *FileBackend) load() (secretsFile, error) {
	b, err := os.ReadFile(f.path)
	if errors.Is(err, fs.ErrNotExist) {
		return secretsFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("secretstore file %q: %w", f.path, err)
	}
	m := secretsFile{}
	if len(b) == 0 {
		return m, nil
	}
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("secretstore file %q: %w", f.path, err)
	}
	return m, nil
}

func (f *FileBackend) save(m secretsFile) error {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("secretstore file %q: %w", f.path, err)
	}
	if dir := filepath.Dir(f.path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return fmt.Errorf("secretstore file %q: %w", f.path, err)
		}
	}
	tmp := f.path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return fmt.Errorf("secretstore file %q: %w", f.path, err)
	}
	if err := os.Rename(tmp, f.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("secretstore file %q: %w", f.path, err)
	}
	return nil
}

func (f *FileBackend) Get(service, key string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, err := f.load()
	if err != nil {
		return "", err
	}
	if kv, ok := m[service]; ok {
		if v, ok := kv[key]; ok && v != "" {
			return v, nil
		}
	}
	return "", ErrNotFound
}

func (f *FileBackend) Set(service, key, value string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, err := f.load()
	if err != nil {
		return err
	}
	if m[service] == nil {
		m[service] = map[string]string{}
	}
	m[service][key] = value
	return f.save(m)
}

func (f *FileBackend) Delete(service, key string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	m, err := f.load()
	if err != nil {
		return err
	}
	kv, ok := m[service]
	if !ok {
		return nil
	}
	delete(kv, key)
	if len(kv) == 0 {
		delete(m, service)
	}
	return f.save(m)
}
