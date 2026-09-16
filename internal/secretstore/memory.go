package secretstore

import "sync"

// MemoryBackend holds secrets in a process-local map. It is for tests and for a
// short-lived process such as cmd/rcprobe that only needs credentials for its
// own run and should leave nothing behind.
type MemoryBackend struct {
	mu sync.Mutex
	m  map[[2]string]string
}

// NewMemoryBackend returns an empty MemoryBackend.
func NewMemoryBackend() *MemoryBackend {
	return &MemoryBackend{m: map[[2]string]string{}}
}

func (b *MemoryBackend) Get(service, key string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if v, ok := b.m[[2]string{service, key}]; ok && v != "" {
		return v, nil
	}
	return "", ErrNotFound
}

func (b *MemoryBackend) Set(service, key, value string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.m[[2]string{service, key}] = value
	return nil
}

func (b *MemoryBackend) Delete(service, key string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.m, [2]string{service, key})
	return nil
}
