package pluginhost

import (
	"context"
	"testing"
	"time"

	"github.com/shinmentakezo07/shinway/v7/sdk/api/handlers"
)

func TestModelStreamBridgeReapsIdleUnownedStreams(t *testing.T) {
	bridge := newModelStreamBridge()
	cancelled := false
	streamID := bridge.open("", make(chan handlers.ModelExecutionChunk), func() { cancelled = true })
	if streamID == "" {
		t.Fatal("open() returned empty stream id")
	}

	// Force the entry idle so the reaper considers it expired.
	bridge.mu.Lock()
	entry := bridge.streams[streamID]
	entry.lastRead = 0
	bridge.streams[streamID] = entry
	bridge.mu.Unlock()

	bridge.reapIdleStreamsNow(time.Now())

	bridge.mu.Lock()
	_, exists := bridge.streams[streamID]
	bridge.mu.Unlock()
	if exists {
		t.Fatal("idle stream without a callback scope was not reaped")
	}
	if !cancelled {
		t.Fatal("reaped stream did not release its cancel function")
	}
}

func TestModelStreamBridgeReaperKeepsActiveUnownedStreams(t *testing.T) {
	bridge := newModelStreamBridge()
	streamID := bridge.open("", make(chan handlers.ModelExecutionChunk), func() {})
	if streamID == "" {
		t.Fatal("open() returned empty stream id")
	}

	bridge.reapIdleStreamsNow(time.Now())

	bridge.mu.Lock()
	_, exists := bridge.streams[streamID]
	bridge.mu.Unlock()
	if !exists {
		t.Fatal("freshly opened stream was reaped despite being within the idle window")
	}
}

func TestModelStreamBridgeReaperSkipsCallbackOwnedStreams(t *testing.T) {
	bridge := newModelStreamBridge()
	cancelled := false
	streamID := bridge.open("callback-1", make(chan handlers.ModelExecutionChunk), func() { cancelled = true })
	if streamID == "" {
		t.Fatal("open() returned empty stream id")
	}

	// Force the entry idle; callback-scoped streams are owned by the scope
	// cleanup and must survive the reaper.
	bridge.mu.Lock()
	entry := bridge.streams[streamID]
	entry.lastRead = 0
	bridge.streams[streamID] = entry
	bridge.mu.Unlock()

	bridge.reapIdleStreamsNow(time.Now())

	bridge.mu.Lock()
	_, exists := bridge.streams[streamID]
	bridge.mu.Unlock()
	if !exists {
		t.Fatal("callback-scoped stream was reaped by the host idle sweeper")
	}
	if cancelled {
		t.Fatal("callback-scoped stream cancel was invoked by the idle reaper")
	}
}

func TestModelStreamBridgeReadRefreshesIdleWindow(t *testing.T) {
	bridge := newModelStreamBridge()
	chunks := make(chan handlers.ModelExecutionChunk)
	streamID := bridge.open("", chunks, func() {})
	if streamID == "" {
		t.Fatal("open() returned empty stream id")
	}

	// Force the entry idle so a refresh by read() is observable.
	bridge.mu.Lock()
	entry := bridge.streams[streamID]
	entry.lastRead = 0
	bridge.streams[streamID] = entry
	bridge.mu.Unlock()

	readStarted := make(chan struct{})
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		close(readStarted)
		_, _, _ = bridge.read(context.Background(), streamID)
	}()
	<-readStarted

	// read() refreshes lastRead synchronously while looking the entry up, so
	// wait until the timestamp is observed to advance past the forced zero.
	deadline := time.Now().Add(time.Second)
	for {
		bridge.mu.Lock()
		refreshed := bridge.streams[streamID].lastRead
		bridge.mu.Unlock()
		if refreshed != 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("read() did not refresh the stream idle timestamp")
		}
		time.Sleep(time.Millisecond)
	}
	close(chunks)
	<-readDone
}
