package common

import "testing"

func TestRequestModelNamePrefersOriginalRequest(t *testing.T) {
	original := []byte(`{"model":"original-model"}`)
	translated := []byte(`{"model":"translated-model"}`)

	if got := RequestModelName(original, translated); got != "original-model" {
		t.Fatalf("model = %q, want original-model", got)
	}
}

func TestRequestModelNameSupportsWrappedRequest(t *testing.T) {
	request := []byte(`{"request":{"model":"wrapped-model"}}`)

	if got := RequestModelName(nil, request); got != "wrapped-model" {
		t.Fatalf("model = %q, want wrapped-model", got)
	}
}

func TestRequestModelNameFallsBackToTranslated(t *testing.T) {
	if got := RequestModelName(nil, []byte(`{"model":"translated-model"}`)); got != "translated-model" {
		t.Fatalf("model = %q, want translated-model", got)
	}
	if got := RequestModelName([]byte(`{}`), nil); got != "" {
		t.Fatalf("model = %q, want empty", got)
	}
}
