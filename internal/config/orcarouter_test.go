package config

import (
	"strings"
	"testing"
)

func TestSanitizeOrcaRouterKeysDefaultsBaseURL(t *testing.T) {
	cfg, errParse := ParseConfigBytes([]byte(`orcarouter-api-key:
  - api-key: sk-orca-1
  - api-key: sk-orca-2
    base-url: https://custom.orcarouter.example/v1
`))
	if errParse != nil {
		t.Fatalf("ParseConfigBytes() error = %v", errParse)
	}
	if len(cfg.OrcaRouterKey) != 2 {
		t.Fatalf("len(OrcaRouterKey) = %d, want 2", len(cfg.OrcaRouterKey))
	}
	if got := cfg.OrcaRouterKey[0].BaseURL; got != OrcaRouterProviderBaseURL {
		t.Fatalf("OrcaRouterKey[0].BaseURL = %q, want default %q", got, OrcaRouterProviderBaseURL)
	}
	if got := cfg.OrcaRouterKey[1].BaseURL; got != "https://custom.orcarouter.example/v1" {
		t.Fatalf("OrcaRouterKey[1].BaseURL = %q, want custom base URL", got)
	}
}

func TestSanitizeOrcaRouterKeysTrimsBaseURL(t *testing.T) {
	cfg := &Config{
		OrcaRouterKey: []OrcaRouterKey{
			{APIKey: "sk-orca-1", BaseURL: "  " + OrcaRouterProviderBaseURL + "  "},
		},
	}
	cfg.SanitizeOrcaRouterKeys()
	if len(cfg.OrcaRouterKey) != 1 {
		t.Fatalf("len(OrcaRouterKey) = %d, want 1", len(cfg.OrcaRouterKey))
	}
	if got := cfg.OrcaRouterKey[0].BaseURL; got != OrcaRouterProviderBaseURL {
		t.Fatalf("OrcaRouterKey[0].BaseURL = %q, want trimmed %q", got, OrcaRouterProviderBaseURL)
	}
}

func TestOrcaRouterAPIKeyWeightValidation(t *testing.T) {
	if _, errParse := ParseConfigBytes([]byte(`orcarouter-api-key:
  - api-key: sk-orca-1
    weight: 5
`)); errParse != nil {
		t.Fatalf("ParseConfigBytes(valid weight) error = %v", errParse)
	}
	if _, errParse := ParseConfigBytes([]byte(`orcarouter-api-key:
  - api-key: sk-orca-1
    weight: 1000001
`)); errParse == nil {
		t.Fatal("ParseConfigBytes(weight above max) = nil error, want validation error")
	}
	if _, errParse := ParseConfigBytes([]byte(`orcarouter-api-key:
  - api-key: sk-orca-1
    weight: 1.5
`)); errParse == nil {
		t.Fatal("ParseConfigBytes(fractional weight) = nil error, want validation error")
	}
}

func TestValidateCredentialWeightsIncludesOrcaRouter(t *testing.T) {
	validWeight := 3
	invalidWeight := 1000001
	cfg := &Config{
		OrcaRouterKey: []OrcaRouterKey{
			{APIKey: "sk-orca-1", Weight: &validWeight},
		},
	}
	if errValidate := cfg.ValidateCredentialWeights(); errValidate != nil {
		t.Fatalf("ValidateCredentialWeights() error = %v", errValidate)
	}

	cfg.OrcaRouterKey = append(cfg.OrcaRouterKey, OrcaRouterKey{APIKey: "sk-orca-2", Weight: &invalidWeight})
	errValidate := cfg.ValidateCredentialWeights()
	if errValidate == nil {
		t.Fatal("ValidateCredentialWeights() = nil error, want orcarouter-api-key weight error")
	}
	if !strings.Contains(errValidate.Error(), "orcarouter-api-key") {
		t.Fatalf("ValidateCredentialWeights() error = %q, want mention of orcarouter-api-key", errValidate)
	}
}
