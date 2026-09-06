package applog

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// awQueueDepth is how many log lines can be in flight before Write starts
	// dropping. At regatta scale a full queue means something downstream has
	// stalled; a few lost lines beat a delayed Start click.
	awQueueDepth = 4096

	// awPendingLines caps the in-memory buffer held before SetOutput provides a
	// file. Startup (persona pick, challenge, directory confirm) produces far
	// fewer than this.
	awPendingLines = 512

	// awFlushInterval is how often the buffered writer is flushed to the file.
	// Per-line Sync would fight cloud-sync and SMB.
	awFlushInterval = time.Second
)

// asyncWriter is the io.Writer behind the slog JSON handler. Write hands a copy
// of each line to a single background goroutine, which appends it to the log
// file (or an in-memory ring until one is set). Write never blocks and never
// returns an error, so a logging call on the timing path cannot stall.
type asyncWriter struct {
	ch   chan []byte
	quit chan struct{}
	done chan struct{}

	dropped atomic.Int64

	mu      sync.Mutex
	file    *os.File
	buf     *bufio.Writer
	pending [][]byte
}

func newAsyncWriter() *asyncWriter {
	w := &asyncWriter{
		ch:   make(chan []byte, awQueueDepth),
		quit: make(chan struct{}),
		done: make(chan struct{}),
	}
	go w.run()
	return w
}

// Write copies p (slog reuses its record buffer) and enqueues it. A full queue
// drops the line and bumps the dropped counter rather than blocking.
func (w *asyncWriter) Write(p []byte) (int, error) {
	b := make([]byte, len(p))
	copy(b, p)
	select {
	case w.ch <- b:
	default:
		w.dropped.Add(1)
	}
	return len(p), nil
}

func (w *asyncWriter) run() {
	defer close(w.done)
	ticker := time.NewTicker(awFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case b := <-w.ch:
			w.consume(b)
		case <-ticker.C:
			w.flush()
		case <-w.quit:
			for {
				select {
				case b := <-w.ch:
					w.consume(b)
				default:
					w.flush()
					return
				}
			}
		}
	}
}

func (w *asyncWriter) consume(b []byte) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf != nil {
		_, _ = w.buf.Write(b)
		return
	}
	w.pending = append(w.pending, b)
	if len(w.pending) > awPendingLines {
		w.pending = w.pending[len(w.pending)-awPendingLines:]
	}
}

func (w *asyncWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf != nil {
		_ = w.buf.Flush()
	}
}

// setFile opens path for appending and replays any buffered lines into it. A
// second call (SetOutput used twice) flushes and closes the previous file
// first.
func (w *asyncWriter) setFile(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("log directory %s could not be created: %w", filepath.Dir(path), err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return fmt.Errorf("log file %s could not be opened: %w", path, err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf != nil {
		_ = w.buf.Flush()
	}
	if w.file != nil {
		_ = w.file.Close()
	}
	w.file = f
	w.buf = bufio.NewWriter(f)
	for _, b := range w.pending {
		_, _ = w.buf.Write(b)
	}
	w.pending = nil
	_ = w.buf.Flush()
	return nil
}

// close stops the goroutine, drains the queue, and flushes and closes the file.
func (w *asyncWriter) close() {
	close(w.quit)
	<-w.done

	w.mu.Lock()
	defer w.mu.Unlock()
	if w.buf != nil {
		_ = w.buf.Flush()
		w.buf = nil
	}
	if w.file != nil {
		_ = w.file.Sync()
		_ = w.file.Close()
		w.file = nil
	}
}
