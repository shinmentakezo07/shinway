package auth

import (
	"net/http"
	"testing"

	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
)

func TestRequestTerminatedErrorSkipsCreditsFallback(t *testing.T) {
	errTerminated := &shinwayexecutor.RequestTerminatedError{HTTPStatus: http.StatusTooManyRequests}
	if !isRequestTerminatedError(errTerminated) {
		t.Fatal("isRequestTerminatedError() = false")
	}
	if shouldAttemptAntigravityCreditsFallback(&Manager{}, errTerminated, []string{"antigravity"}) {
		t.Fatal("terminated request must not use Antigravity credits fallback")
	}
}
