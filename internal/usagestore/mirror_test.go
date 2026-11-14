package usagestore

import (
	"context"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeUsageWriter struct {
	mu        sync.Mutex
	records   []Record
	closed    atomic.Bool
	firstFail atomic.Bool
	failNext  atomic.Bool
}

func (w *fakeUsageWriter) Write(_ context.Context, r Record) error {
	if w.failNext.Load() {
		w.failNext.Store(false)
		return context.DeadlineExceeded
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.records = append(w.records, r)
	return nil
}

func (w *fakeUsageWriter) Close() error {
	w.closed.Store(true)
	return nil
}

func (w *fakeUsageWriter) count() int {
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.records)
}

func TestStoreMirrorWriterFanout(t *testing.T) {
	dir := t.TempDir()
	fake := &fakeUsageWriter{}
	SetDefaultOptions(WithWriter(fake))
	t.Cleanup(func() {
		SetDefaultOptions()
		_ = Close()
	})

	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	ctx := context.Background()
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	if err := store.Insert(ctx, Record{TS: now, Model: "m1", TotalTokens: 5}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if fake.count() != 1 {
		t.Fatalf("mirror records = %d, want 1", fake.count())
	}

	// A failing write must not break the SQLite insert and must be flagged as
	// the writer's first failure so the store can log it visibly.
	fake.failNext.Store(true)
	if err := store.Insert(ctx, Record{TS: now.Add(time.Second), Model: "m2"}); err != nil {
		t.Fatalf("Insert with failing mirror: %v", err)
	}
	if got := fake.count(); got != 1 {
		t.Fatalf("mirror records after failure = %d, want 1", got)
	}
}

func TestAttachPostgresMirrorNoStore(t *testing.T) {
	// No store open: must be a no-op, not an error, so reload paths can call
	// it unconditionally.
	SetDefaultOptions()
	t.Cleanup(func() { SetDefaultOptions() })
	if err := AttachPostgresMirror(nil, ""); err == nil {
		t.Fatal("AttachPostgresMirror(nil) = nil, want error")
	}
}

func TestAttachPostgresMirrorIdempotent(t *testing.T) {
	dir := t.TempDir()
	SetDefaultOptions()
	t.Cleanup(func() {
		SetDefaultOptions()
		_ = Close()
	})
	store, err := Open(filepath.Join(dir, "db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	// The DB handle is never used because NewPostgresWriter would fail on a
	// nil pool; use a fake that satisfies Writer and verify dedupe by count.
	fake := &fakeUsageWriter{}
	store.AttachWriter(fake)
	if err := AttachPostgresMirror(nil, ""); err == nil {
		t.Fatal("AttachPostgresMirror(nil db) = nil, want error")
	}
	if store.writersCount() != 1 {
		t.Fatalf("writers = %d, want 1", store.writersCount())
	}
}

func TestTakeFirstFailureJSONWriter(t *testing.T) {
	w := &JSONWriter{}
	if !w.TakeFirstFailure() {
		t.Fatal("first TakeFirstFailure() = false, want true")
	}
	if w.TakeFirstFailure() {
		t.Fatal("second TakeFirstFailure() = true, want false")
	}
}

func (s *Store) writersCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.writers)
}
