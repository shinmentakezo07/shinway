package config

import (
	"encoding/json"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestModelCapabilityConfigDecoding(t *testing.T) {
	const yamlConfig = `codex-api-key:
  - models:
      - name: codex-upstream
        alias: codex-alias
        max-context-length: 1048576
        is-compat: true
claude-api-key:
  - models:
      - name: claude-upstream
        alias: claude-alias
        max-context-length: 1048576
        is-compat: true
gemini-api-key:
  - models:
      - name: gemini-upstream
        alias: gemini-alias
        max-context-length: 1048576
        is-compat: true
interactions-api-key:
  - models:
      - name: interactions-upstream
        alias: interactions-alias
        max-context-length: 1048576
        is-compat: true
xai-api-key:
  - models:
      - name: xai-upstream
        alias: xai-alias
        max-context-length: 1048576
        is-compat: true
openai-compatibility:
  - models:
      - name: compat-upstream
        alias: compat-alias
        max-context-length: 1048576
        is-compat: true
vertex-api-key:
  - models:
      - name: vertex-upstream
        alias: vertex-alias
        max-context-length: 1048576
        is-compat: true
`
	const jsonConfig = `{"codex-api-key":[{"models":[{"name":"codex-upstream","alias":"codex-alias","max-context-length":1048576,"is-compat":true}]}],"claude-api-key":[{"models":[{"name":"claude-upstream","alias":"claude-alias","max-context-length":1048576,"is-compat":true}]}],"gemini-api-key":[{"models":[{"name":"gemini-upstream","alias":"gemini-alias","max-context-length":1048576,"is-compat":true}]}],"interactions-api-key":[{"models":[{"name":"interactions-upstream","alias":"interactions-alias","max-context-length":1048576,"is-compat":true}]}],"xai-api-key":[{"models":[{"name":"xai-upstream","alias":"xai-alias","max-context-length":1048576,"is-compat":true}]}],"openai-compatibility":[{"models":[{"name":"compat-upstream","alias":"compat-alias","max-context-length":1048576,"is-compat":true}]}],"vertex-api-key":[{"models":[{"name":"vertex-upstream","alias":"vertex-alias","max-context-length":1048576,"is-compat":true}]}]}`

	for _, testCase := range []struct {
		name   string
		decode func(*Config) error
	}{
		{"YAML", func(cfg *Config) error { return yaml.Unmarshal([]byte(yamlConfig), cfg) }},
		{"JSON", func(cfg *Config) error { return json.Unmarshal([]byte(jsonConfig), cfg) }},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			var cfg Config
			if err := testCase.decode(&cfg); err != nil {
				t.Fatalf("decode config: %v", err)
			}
			models := []struct {
				name string
				got  struct {
					max    int
					compat bool
				}
			}{
				{"codex", struct {
					max    int
					compat bool
				}{cfg.CodexKey[0].Models[0].MaxContextLength, cfg.CodexKey[0].Models[0].IsCompat}},
				{"claude", struct {
					max    int
					compat bool
				}{cfg.ClaudeKey[0].Models[0].MaxContextLength, cfg.ClaudeKey[0].Models[0].IsCompat}},
				{"gemini", struct {
					max    int
					compat bool
				}{cfg.GeminiKey[0].Models[0].MaxContextLength, cfg.GeminiKey[0].Models[0].IsCompat}},
				{"interactions", struct {
					max    int
					compat bool
				}{cfg.InteractionsKey[0].Models[0].MaxContextLength, cfg.InteractionsKey[0].Models[0].IsCompat}},
				{"xai", struct {
					max    int
					compat bool
				}{cfg.XAIKey[0].Models[0].MaxContextLength, cfg.XAIKey[0].Models[0].IsCompat}},
				{"openai compatibility", struct {
					max    int
					compat bool
				}{cfg.OpenAICompatibility[0].Models[0].MaxContextLength, cfg.OpenAICompatibility[0].Models[0].IsCompat}},
				{"vertex", struct {
					max    int
					compat bool
				}{cfg.VertexCompatAPIKey[0].Models[0].MaxContextLength, cfg.VertexCompatAPIKey[0].Models[0].IsCompat}},
			}
			for _, model := range models {
				if model.got.max != 1048576 || !model.got.compat {
					t.Errorf("%s capabilities = %+v, want max-context-length=1048576 and is-compat=true", model.name, model.got)
				}
			}
		})
	}
}
