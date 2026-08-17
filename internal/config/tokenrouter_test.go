package config

import (
	"strings"
	"testing"
)

func TestSanitizeTokenRouterKeysDefaultsBaseURL(t *testing.T) {
	cfg, errParse := ParseConfigBytes([]byte(`tokenrouter-api-key:
  - api-key: tr-1
  - api-key: tr-2
    base-url: https://custom.tokenrouter.example/v1
`))
	if errParse != nil {
		t.Fatalf("ParseConfigBytes() error = %v", errParse)
	}
	if len(cfg.TokenRouterKey) != 2 {
		t.Fatalf("len(TokenRouterKey) = %d, want 2", len(cfg.TokenRouterKey))
	}
	if got := cfg.TokenRouterKey[0].BaseURL; got != TokenRouterProviderBaseURL {
		t.Fatalf("TokenRouterKey[0].BaseURL = %q, want default %q", got, TokenRouterProviderBaseURL)
	}
	if got := cfg.TokenRouterKey[1].BaseURL; got != "https://custom.tokenrouter.example/v1" {
		t.Fatalf("TokenRouterKey[1].BaseURL = %q, want custom base URL", got)
	}
}

func TestSanitizeTokenRouterKeysTrimsBaseURL(t *testing.T) {
	cfg := &Config{
		TokenRouterKey: []TokenRouterKey{
			{APIKey: "tr-1", BaseURL: "  " + TokenRouterProviderBaseURL + "  "},
		},
	}
	cfg.SanitizeTokenRouterKeys()
	if len(cfg.TokenRouterKey) != 1 {
		t.Fatalf("len(TokenRouterKey) = %d, want 1", len(cfg.TokenRouterKey))
	}
	if got := cfg.TokenRouterKey[0].BaseURL; got != TokenRouterProviderBaseURL {
		t.Fatalf("TokenRouterKey[0].BaseURL = %q, want trimmed %q", got, TokenRouterProviderBaseURL)
	}
}

func TestTokenRouterAPIKeyWeightValidation(t *testing.T) {
	if _, errParse := ParseConfigBytes([]byte(`tokenrouter-api-key:
  - api-key: tr-1
    weight: 5
`)); errParse != nil {
		t.Fatalf("ParseConfigBytes(valid weight) error = %v", errParse)
	}
	if _, errParse := ParseConfigBytes([]byte(`tokenrouter-api-key:
  - api-key: tr-1
    weight: 1000001
`)); errParse == nil {
		t.Fatal("ParseConfigBytes(weight above max) = nil error, want validation error")
	}
	if _, errParse := ParseConfigBytes([]byte(`tokenrouter-api-key:
  - api-key: tr-1
    weight: 1.5
`)); errParse == nil {
		t.Fatal("ParseConfigBytes(fractional weight) = nil error, want validation error")
	}
}

func TestValidateCredentialWeightsIncludesTokenRouter(t *testing.T) {
	validWeight := 3
	invalidWeight := 1000001
	cfg := &Config{
		TokenRouterKey: []TokenRouterKey{
			{APIKey: "tr-1", Weight: &validWeight},
		},
	}
	if errValidate := cfg.ValidateCredentialWeights(); errValidate != nil {
		t.Fatalf("ValidateCredentialWeights() error = %v", errValidate)
	}

	cfg.TokenRouterKey = append(cfg.TokenRouterKey, TokenRouterKey{APIKey: "tr-2", Weight: &invalidWeight})
	errValidate := cfg.ValidateCredentialWeights()
	if errValidate == nil {
		t.Fatal("ValidateCredentialWeights() = nil error, want tokenrouter-api-key weight error")
	}
	if !strings.Contains(errValidate.Error(), "tokenrouter-api-key") {
		t.Fatalf("ValidateCredentialWeights() error = %q, want mention of tokenrouter-api-key", errValidate)
	}
}
