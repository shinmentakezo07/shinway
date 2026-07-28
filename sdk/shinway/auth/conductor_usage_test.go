package auth

import (
	"context"
	"testing"

	shinwayexecutor "github.com/shinmentakezo07/shinway/v7/sdk/shinway/executor"
	coreusage "github.com/shinmentakezo07/shinway/v7/sdk/shinway/usage"
)

func TestContextWithRequestedModelAliasIncludesReasoningEffort(t *testing.T) {
	ctx := contextWithRequestedModelAlias(context.Background(), shinwayexecutor.Options{
		Metadata: map[string]any{
			shinwayexecutor.RequestedModelMetadataKey:  "client-model",
			shinwayexecutor.ReasoningEffortMetadataKey: "medium",
			shinwayexecutor.ServiceTierMetadataKey:     "auto",
			shinwayexecutor.GenerateMetadataKey:        false,
		},
	}, "fallback-model")

	if got := coreusage.RequestedModelAliasFromContext(ctx); got != "client-model" {
		t.Fatalf("requested model alias = %q, want %q", got, "client-model")
	}
	if got := coreusage.ReasoningEffortFromContext(ctx); got != "medium" {
		t.Fatalf("reasoning effort = %q, want %q", got, "medium")
	}
	gotServiceTier := coreusage.ServiceTierFromContext(ctx)
	if gotServiceTier != "auto" {
		t.Fatalf("service tier = %q, want %q", gotServiceTier, "auto")
	}
	if got := coreusage.GenerateFromContext(ctx); got {
		t.Fatalf("generate = %v, want false", got)
	}
}

func TestContextWithRequestedModelAliasDefaultsGenerateTrue(t *testing.T) {
	ctx := contextWithRequestedModelAlias(context.Background(), shinwayexecutor.Options{
		Metadata: map[string]any{
			shinwayexecutor.RequestedModelMetadataKey: "client-model",
		},
	}, "fallback-model")

	if got := coreusage.GenerateFromContext(ctx); !got {
		t.Fatalf("generate = %v, want true", got)
	}
}

func TestContextWithRequestedModelAliasPreservesExistingGenerateFalse(t *testing.T) {
	ctx := coreusage.WithGenerate(context.Background(), false)
	ctx = contextWithRequestedModelAlias(ctx, shinwayexecutor.Options{
		Metadata: map[string]any{
			shinwayexecutor.RequestedModelMetadataKey: "client-model",
		},
	}, "fallback-model")

	if got := coreusage.GenerateFromContext(ctx); got {
		t.Fatalf("generate = %v, want false", got)
	}
}
