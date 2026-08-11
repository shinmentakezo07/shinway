package pluginhost

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/shinmentakezo07/shinway/v7/sdk/api/handlers"
)

const (
	// modelStreamIdleTimeout bounds how long a stream without a callback scope
	// may go unread before the host reaps it. Streams opened under a callback
	// context are owned by that scope's cleanup and are deliberately never
	// backstopped here, so their lifetime is unaffected by this timeout.
	modelStreamIdleTimeout = 10 * time.Minute
	// modelStreamIdleCheckEvery is how often the reaper sweeps idle streams.
	modelStreamIdleCheckEvery = 30 * time.Second
)

type modelStreamBridge struct {
	next       atomic.Uint64
	mu         sync.Mutex
	streams    map[string]modelStreamEntry
	reaperOnce sync.Once
}

type modelStreamEntry struct {
	ownerCallbackID string
	chunks          <-chan handlers.ModelExecutionChunk
	cancel          context.CancelFunc
	lastRead        int64 // unix nanos of the most recent open or read
}

func newModelStreamBridge() *modelStreamBridge {
	return &modelStreamBridge{streams: make(map[string]modelStreamEntry)}
}

func (b *modelStreamBridge) open(ownerCallbackID string, chunks <-chan handlers.ModelExecutionChunk, cancel context.CancelFunc) string {
	if b == nil || chunks == nil {
		if cancel != nil {
			cancel()
		}
		return ""
	}
	b.startReaper()
	id := strconv.FormatUint(b.next.Add(1), 10)
	b.mu.Lock()
	b.streams[id] = modelStreamEntry{
		ownerCallbackID: ownerCallbackID,
		chunks:          chunks,
		cancel:          cancel,
		lastRead:        time.Now().UnixNano(),
	}
	b.mu.Unlock()
	return id
}

// startReaper lazily launches the idle-stream sweeper on first use.
func (b *modelStreamBridge) startReaper() {
	if b == nil {
		return
	}
	b.reaperOnce.Do(func() {
		go b.reapIdleStreams()
	})
}

// reapIdleStreams sweeps unowned streams for the lifetime of the bridge. There
// is intentionally no shutdown hook: the bridge lives for the process lifetime
// alongside the host, so the sweeper goroutine is never expected to exit.
func (b *modelStreamBridge) reapIdleStreams() {
	ticker := time.NewTicker(modelStreamIdleCheckEvery)
	defer ticker.Stop()
	for range ticker.C {
		b.reapIdleStreamsNow(time.Now())
	}
}

// reapIdleStreamsNow closes streams with no callback scope that have not been
// read for modelStreamIdleTimeout, releasing their context regardless of whether
// the owning plugin ever closes or drains them.
func (b *modelStreamBridge) reapIdleStreamsNow(now time.Time) {
	if b == nil {
		return
	}
	deadline := now.Add(-modelStreamIdleTimeout).UnixNano()
	b.mu.Lock()
	defer b.mu.Unlock()
	for id, entry := range b.streams {
		if entry.ownerCallbackID != "" {
			continue
		}
		if entry.lastRead < deadline {
			delete(b.streams, id)
			if entry.cancel != nil {
				entry.cancel()
			}
		}
	}
}

func (b *modelStreamBridge) read(ctx context.Context, id string) (handlers.ModelExecutionChunk, bool, error) {
	if b == nil {
		return handlers.ModelExecutionChunk{}, true, fmt.Errorf("model stream bridge is unavailable")
	}
	if id == "" {
		return handlers.ModelExecutionChunk{}, true, fmt.Errorf("model stream id is required")
	}
	b.mu.Lock()
	entry, ok := b.streams[id]
	if ok {
		entry.lastRead = time.Now().UnixNano()
		b.streams[id] = entry
	}
	b.mu.Unlock()
	if !ok || entry.chunks == nil {
		return handlers.ModelExecutionChunk{}, true, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	select {
	case <-ctx.Done():
		b.close(id)
		return handlers.ModelExecutionChunk{}, true, ctx.Err()
	case chunk, okRead := <-entry.chunks:
		if !okRead {
			b.close(id)
			return handlers.ModelExecutionChunk{}, true, nil
		}
		if chunk.Err != nil {
			b.close(id)
			return chunk, true, nil
		}
		return chunk, false, nil
	}
}

func (b *modelStreamBridge) close(id string) {
	if b == nil || id == "" {
		return
	}
	b.mu.Lock()
	entry := b.streams[id]
	delete(b.streams, id)
	b.mu.Unlock()
	if entry.cancel != nil {
		entry.cancel()
	}
}
