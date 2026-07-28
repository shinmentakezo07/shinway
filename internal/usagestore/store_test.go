package usagestore

import (
	"context"
	"testing"
	"time"
)

func TestStoreLifecycle(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = Close() })

	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	ctx := context.Background()

	records := []Record{
		{TS: now.Add(-3 * time.Hour), Model: "gpt-4", Provider: "openai", APIKey: "k1", InputTokens: 100, OutputTokens: 200, TotalTokens: 300, LatencyMs: 50},
		{TS: now.Add(-2 * time.Hour), Model: "gpt-4", Provider: "openai", APIKey: "k1", InputTokens: 10, OutputTokens: 20, TotalTokens: 30, Failed: true, FailStatus: 500},
		{TS: now.Add(-1 * time.Hour), Model: "claude-3", Provider: "anthropic", APIKey: "k2", InputTokens: 5, OutputTokens: 7, TotalTokens: 12, LatencyMs: 150},
	}
	for _, r := range records {
		if err := store.Insert(ctx, r); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}

	sum, err := store.Summary(ctx, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.TotalRequests != 3 {
		t.Errorf("TotalRequests=%d, want 3", sum.TotalRequests)
	}
	if sum.FailedRequests != 1 {
		t.Errorf("FailedRequests=%d, want 1", sum.FailedRequests)
	}
	if sum.SucceededReqs != 2 {
		t.Errorf("SucceededReqs=%d, want 2", sum.SucceededReqs)
	}
	if sum.TotalTokens != 342 {
		t.Errorf("TotalTokens=%d, want 342", sum.TotalTokens)
	}
	if sum.UniqueModels != 2 {
		t.Errorf("UniqueModels=%d, want 2", sum.UniqueModels)
	}
	if sum.UniqueAPIKeys != 2 {
		t.Errorf("UniqueAPIKeys=%d, want 2", sum.UniqueAPIKeys)
	}

	series, err := store.Series(ctx, now.Add(-4*time.Hour), now, 3600)
	if err != nil {
		t.Fatalf("Series: %v", err)
	}
	if len(series) != 3 {
		t.Fatalf("series len=%d, want 3 (%+v)", len(series), series)
	}

	byModel, err := store.ByDimension(ctx, time.Time{}, time.Time{}, "model", 10)
	if err != nil {
		t.Fatalf("ByDimension: %v", err)
	}
	if len(byModel) != 2 {
		t.Fatalf("byModel len=%d, want 2", len(byModel))
	}
	if byModel[0].Key != "gpt-4" || byModel[0].Total != 330 {
		t.Errorf("byModel[0]=%+v, want gpt-4/330", byModel[0])
	}

	list, total, err := store.List(ctx, ListFilter{Limit: 10})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 || len(list) != 3 {
		t.Errorf("List total=%d len=%d, want 3/3", total, len(list))
	}

	failedOnly := true
	list2, total2, err := store.List(ctx, ListFilter{Limit: 10, Failed: &failedOnly})
	if err != nil {
		t.Fatalf("List failed-only: %v", err)
	}
	if total2 != 1 || len(list2) != 1 || !list2[0].Failed {
		t.Errorf("List failed-only total=%d len=%d, want 1/1", total2, len(list2))
	}

	deleted, err := store.PurgeOlderThan(ctx, now.Add(-90*time.Minute))
	if err != nil {
		t.Fatalf("Purge: %v", err)
	}
	if deleted != 2 {
		t.Errorf("Purge=%d, want 2", deleted)
	}
}

func TestSanitizeDimension(t *testing.T) {
	ok := []string{"model", "provider", "api_key", "auth_id", "endpoint"}
	for _, want := range ok {
		got, err := sanitizeDimension(want)
		if err != nil || got != want {
			t.Errorf("%s: got %q err %v", want, got, err)
		}
	}
	if _, err := sanitizeDimension("malicious; drop table"); err == nil {
		t.Error("expected error for unsafe dimension")
	}
}

func TestEnabledToggle(t *testing.T) {
	if !Enabled() {
		t.Error("default should be enabled")
	}
	SetEnabled(false)
	if Enabled() {
		t.Error("should be disabled after SetEnabled(false)")
	}
	SetEnabled(true)
}
