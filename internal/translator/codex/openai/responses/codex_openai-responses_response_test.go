package responses

import (
	"context"
	"testing"

	"github.com/tidwall/gjson"
)

func TestConvertCodexResponseToOpenAIResponsesPreservesCallerModelAlias(t *testing.T) {
	original := []byte(`{"model":"public-alias"}`)
	translated := []byte(`{"model":"upstream-resolved-model"}`)

	for _, event := range []string{
		`{"type":"response.created","sequence_number":1,"response":{"id":"resp_1","object":"response","created_at":0,"status":"in_progress","output":[]}}`,
		`{"type":"response.in_progress","sequence_number":2,"response":{"id":"resp_1","object":"response","created_at":0,"status":"in_progress"}}`,
	} {
		out := ConvertCodexResponseToOpenAIResponses(context.Background(), "fallback-model", original, translated, []byte(event), nil)
		if len(out) != 1 {
			t.Fatalf("event %s: output count = %d, want 1", event, len(out))
		}
		if got := gjson.GetBytes(out[0], "response.model").String(); got != "public-alias" {
			t.Fatalf("event %s: response.model = %q, want caller alias public-alias", event, got)
		}
	}
}

func TestConvertCodexResponseToOpenAIResponsesKeepsExistingModel(t *testing.T) {
	event := `{"type":"response.created","sequence_number":1,"response":{"id":"resp_1","object":"response","created_at":0,"status":"in_progress","model":"already-set","output":[]}}`
	out := ConvertCodexResponseToOpenAIResponses(context.Background(), "fallback-model", []byte(`{"model":"alias"}`), nil, []byte(event), nil)
	if got := gjson.GetBytes(out[0], "response.model").String(); got != "already-set" {
		t.Fatalf("response.model = %q, want existing already-set", got)
	}
}

func TestConvertCodexResponseToOpenAIResponsesFallsBackToModelName(t *testing.T) {
	event := `{"type":"response.created","sequence_number":1,"response":{"id":"resp_1","object":"response","created_at":0,"status":"in_progress","output":[]}}`
	out := ConvertCodexResponseToOpenAIResponses(context.Background(), "fallback-model", nil, nil, []byte(event), nil)
	if got := gjson.GetBytes(out[0], "response.model").String(); got != "fallback-model" {
		t.Fatalf("response.model = %q, want fallback-model", got)
	}
}
