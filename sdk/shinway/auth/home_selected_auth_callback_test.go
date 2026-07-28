package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"testing"

	internalconfig "github.com/shinmentakezo07/shinway/v7/internal/config"
	"github.com/shinmentakezo07/shinway/v7/sdk/shinway/executionregistry"
	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
)

type selectedAuthCallbackDispatcher struct {
	calls atomic.Int32
}

func (*selectedAuthCallbackDispatcher) HeartbeatOK() bool { return true }
func (d *selectedAuthCallbackDispatcher) RPopAuth(context.Context, string, string, http.Header, int) ([]byte, error) {
	if d.calls.Add(1) > 2 {
		return json.Marshal(homeErrorEnvelope{Error: &homeErrorDetail{Code: homeRequestRetryExceededErrorCode, Message: "no more auths"}})
	}
	return json.Marshal(homeAuthDispatchResponse{Auth: Auth{ID: "home-auth", Provider: "home-execution", Status: StatusActive, Attributes: map[string]string{"websockets": "true"}}})
}
func (*selectedAuthCallbackDispatcher) AbortAmbiguousDispatch() {}

type callbackPinHomeExecutor struct {
	manager *Manager
	session string
	calls   atomic.Int32
}

func (*callbackPinHomeExecutor) Identifier() string { return "home-execution" }
func (e *callbackPinHomeExecutor) Execute(context.Context, *Auth, shinwayexecutor.Request, shinwayexecutor.Options) (shinwayexecutor.Response, error) {
	return shinwayexecutor.Response{}, nil
}
func (e *callbackPinHomeExecutor) ExecuteStream(_ context.Context, _ *Auth, _ shinwayexecutor.Request, opts shinwayexecutor.Options) (*shinwayexecutor.StreamResult, error) {
	if e.calls.Add(1) == 2 {
		return nil, errSelectedAuthCallbackFailure
	}
	if lifecycle, ok := opts.ExecutionLifecycle.(interface{ Retain() }); ok {
		lifecycle.Retain()
	}
	chunks := make(chan shinwayexecutor.StreamChunk, 1)
	chunks <- shinwayexecutor.StreamChunk{Payload: []byte(`{"type":"response.completed"}`)}
	close(chunks)
	return &shinwayexecutor.StreamResult{Chunks: chunks}, nil
}
func (*callbackPinHomeExecutor) Refresh(context.Context, *Auth) (*Auth, error) { return nil, nil }
func (*callbackPinHomeExecutor) CountTokens(context.Context, *Auth, shinwayexecutor.Request, shinwayexecutor.Options) (shinwayexecutor.Response, error) {
	return shinwayexecutor.Response{}, nil
}
func (*callbackPinHomeExecutor) HttpRequest(context.Context, *Auth, *http.Request) (*http.Response, error) {
	return nil, nil
}

var errSelectedAuthCallbackFailure = &Error{HTTPStatus: 502, Message: "selected auth failed"}

func TestHomeSelectedAuthCallbackPinsFirstHandlerSelectionAndCleansFailure(t *testing.T) {
	manager := NewManager(nil, nil, nil)
	manager.SetConfig(&internalconfig.Config{Home: internalconfig.HomeConfig{Enabled: true}})
	manager.PublishHomeDispatch(&selectedAuthCallbackDispatcher{}, executionregistry.New(), 1)
	executor := &callbackPinHomeExecutor{manager: manager, session: "callback-session"}
	manager.RegisterExecutor(executor)

	ctx := shinwayexecutor.WithDownstreamWebsocket(context.Background())
	callbackSawRuntimeAuth := false
	opts := shinwayexecutor.Options{Stream: true, Metadata: map[string]any{
		shinwayexecutor.ExecutionSessionMetadataKey: executor.session,
		shinwayexecutor.SelectedAuthCallbackMetadataKey: func(authID string) {
			_, callbackSawRuntimeAuth = manager.GetExecutionSessionAuthByID(executor.session, authID)
		},
	}}
	result, errExecute := manager.ExecuteStream(ctx, []string{"home-execution"}, shinwayexecutor.Request{Model: "model-a"}, opts)
	if errExecute != nil {
		t.Fatalf("first ExecuteStream() error = %v", errExecute)
	}
	for range result.Chunks {
	}
	if !callbackSawRuntimeAuth {
		t.Fatal("first selected-auth callback could not resolve the Home runtime auth")
	}

	manager.CloseExecutionSession(executor.session)
	callbackSawRuntimeAuth = false
	_, errExecute = manager.ExecuteStream(ctx, []string{"home-execution"}, shinwayexecutor.Request{Model: "model-b"}, opts)
	if errExecute == nil {
		t.Fatal("failed ExecuteStream() error = nil")
	}
	if !callbackSawRuntimeAuth {
		t.Fatal("failed selected-auth callback could not resolve the Home runtime auth")
	}
	if _, ok := manager.GetExecutionSessionAuthByID(executor.session, "home-auth"); ok {
		t.Fatal("failed selection retained Home runtime auth")
	}
}
