package handlers

import (
	"errors"
	"testing"

	"github.com/shinmentakezo07/shinway/v7/internal/interfaces"
)

func TestPendingStreamErrorReadsBufferedError(t *testing.T) {
	errs := make(chan *interfaces.ErrorMessage, 1)
	want := &interfaces.ErrorMessage{StatusCode: 502, Error: errors.New("upstream failed")}
	errs <- want

	got, ok := PendingStreamError(errs)
	if !ok {
		t.Fatal("expected pending error")
	}
	if got != want {
		t.Fatalf("got %p, want %p", got, want)
	}
}

func TestPendingStreamErrorIgnoresEmptyAndClosedChannels(t *testing.T) {
	if _, ok := PendingStreamError(nil); ok {
		t.Fatal("nil channel must not report an error")
	}

	errs := make(chan *interfaces.ErrorMessage)
	close(errs)
	if _, ok := PendingStreamError(errs); ok {
		t.Fatal("closed empty channel must not report an error")
	}
}
