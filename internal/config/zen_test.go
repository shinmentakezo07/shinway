package config

import (
	"strings"
	"testing"
)

func TestSanitizeZenKeysDefaultsBaseURL(t *testing.T) {
	cfg, errParse := ParseConfigBytes([]byte(`zen-api-key:
  - api-key: oc-1
  - api-key: oc-2
    base-url: https://custom.zen.example/v1
`))
	if errParse != nil {
		t.Fatalf("ParseConfigBytes() error = %v", errParse)
	}
	if len(cfg.ZenKey) != 2 {
		t.Fatalf("len(ZenKey) = %d, want 2", len(cfg.ZenKey))
	}
	if got := cfg.ZenKey[0].BaseURL; got != ZenProviderBaseURL {
		t.Fatalf("ZenKey[0].BaseURL = %q, want default %q", got, ZenProviderBaseURL)
	}
	if got := cfg.ZenKey[1].BaseURL; got != "https://custom.zen.example/v1" {
		t.Fatalf("ZenKey[1].BaseURL = %q, want custom base URL", got)
	}
}

func TestSanitizeZenKeysTrimsBaseURL(t *testing.T) {
	cfg := &Config{
		ZenKey: []ZenKey{
			{APIKey: "oc-1", BaseURL: "  " + ZenProviderBaseURL + "  "},
		},
	}
	cfg.SanitizeZenKeys()
	if len(cfg.ZenKey) != 1 {
		t.Fatalf("len(ZenKey) = %d, want 1", len(cfg.ZenKey))
	}
	if got := cfg.ZenKey[0].BaseURL; got != ZenProviderBaseURL {
		t.Fatalf("ZenKey[0].BaseURL = %q, want trimmed %q", got, ZenProviderBaseURL)
	}
}

func TestZenAPIKeyWeightValidation(t *testing.T) {
	if _, errParse := ParseConfigBytes([]byte(`zen-api-key:
  - api-key: oc-1
    weight: 5
`)); errParse != nil {
		t.Fatalf("ParseConfigBytes(valid weight) error = %v", errParse)
	}
	if _, errParse := ParseConfigBytes([]byte(`zen-api-key:
  - api-key: oc-1
    weight: 1000001
`)); errParse == nil {
		t.Fatal("ParseConfigBytes(weight above max) = nil error, want validation error")
	}
	if _, errParse := ParseConfigBytes([]byte(`zen-api-key:
  - api-key: oc-1
    weight: 1.5
`)); errParse == nil {
		t.Fatal("ParseConfigBytes(fractional weight) = nil error, want validation error")
	}
}

func TestValidateCredentialWeightsIncludesZen(t *testing.T) {
	validWeight := 3
	invalidWeight := 1000001
	cfg := &Config{
		ZenKey: []ZenKey{
			{APIKey: "oc-1", Weight: &validWeight},
		},
	}
	if errValidate := cfg.ValidateCredentialWeights(); errValidate != nil {
		t.Fatalf("ValidateCredentialWeights() error = %v", errValidate)
	}

	cfg.ZenKey = append(cfg.ZenKey, ZenKey{APIKey: "oc-2", Weight: &invalidWeight})
	errValidate := cfg.ValidateCredentialWeights()
	if errValidate == nil {
		t.Fatal("ValidateCredentialWeights() = nil error, want zen-api-key weight error")
	}
	if !strings.Contains(errValidate.Error(), "zen-api-key") {
		t.Fatalf("ValidateCredentialWeights() error = %q, want mention of zen-api-key", errValidate)
	}
}

func TestZenHeaderDefaultsParsing(t *testing.T) {
	cfg, errParse := ParseConfigBytes([]byte(`zen-header-defaults:
  user-agent: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
`))
	if errParse != nil {
		t.Fatalf("ParseConfigBytes() error = %v", errParse)
	}
	wantUA := "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/143.0.0.0 Safari/537.36"
	if got := cfg.ZenHeaderDefaults.UserAgent; got != wantUA {
		t.Fatalf("ZenHeaderDefaults.UserAgent = %q, want %q", got, wantUA)
	}
}

func TestZenHeaderDefaultsEmpty(t *testing.T) {
	cfg, errParse := ParseConfigBytes([]byte(`zen-header-defaults: {}`))
	if errParse != nil {
		t.Fatalf("ParseConfigBytes() error = %v", errParse)
	}
	if got := cfg.ZenHeaderDefaults.UserAgent; got != "" {
		t.Fatalf("ZenHeaderDefaults.UserAgent = %q, want empty", got)
	}
}
