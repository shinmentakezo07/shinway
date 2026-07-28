package auth

import (
	"testing"

	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
)

func TestPublishSelectedAuthMetadataIncludesStableIndex(t *testing.T) {
	auth := &Auth{
		ID:       "auth-1",
		Provider: "codex",
		FileName: "auth-1.json",
	}
	selectedAuthID := ""
	selectedAuthIndex := ""
	meta := map[string]any{
		shinwayexecutor.SelectedAuthCallbackMetadataKey: func(authID string) {
			selectedAuthID = authID
		},
		shinwayexecutor.SelectedAuthIndexCallbackMetadataKey: func(authIndex string) {
			selectedAuthIndex = authIndex
		},
	}

	publishSelectedAuthMetadata(meta, auth)

	if selectedAuthID != auth.ID {
		t.Fatalf("selected auth ID = %q, want %q", selectedAuthID, auth.ID)
	}
	if selectedAuthIndex == "" || selectedAuthIndex != auth.Index {
		t.Fatalf("selected auth index = %q, want %q", selectedAuthIndex, auth.Index)
	}
	if got := meta[shinwayexecutor.SelectedAuthMetadataKey]; got != auth.ID {
		t.Fatalf("selected auth metadata = %#v, want %q", got, auth.ID)
	}
	if got := meta[shinwayexecutor.SelectedAuthIndexMetadataKey]; got != auth.Index {
		t.Fatalf("selected auth index metadata = %#v, want %q", got, auth.Index)
	}
}
