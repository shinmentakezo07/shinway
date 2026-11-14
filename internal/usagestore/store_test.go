package usagestore

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	coreusage "github.com/shinmentakezo07/shinway/v7/sdk/shinway/usage"
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

func TestDefaultOptions(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "usage.jsonl")
	jw, err := NewJSONWriter(jsonPath)
	if err != nil {
		t.Fatalf("NewJSONWriter: %v", err)
	}
	SetDefaultOptions(WithWriter(jw))
	t.Cleanup(func() {
		SetDefaultOptions()
		_ = Close()
	})

	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	ctx := context.Background()
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	if err := store.Insert(ctx, Record{TS: now, Model: "gpt-4", TotalTokens: 100}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	// A second Open call with the same directory should return the existing store.
	store2, err := Open(dir)
	if err != nil {
		t.Fatalf("Open second time: %v", err)
	}
	if store2 != store {
		t.Error("expected same store instance")
	}

	// Verify the JSON file received the record.
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	var decoded Record
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal jsonl: %v", err)
	}
	if decoded.Model != "gpt-4" || decoded.TotalTokens != 100 {
		t.Errorf("decoded record mismatch: %+v", decoded)
	}
}

func TestJSONMirror(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "usage.jsonl")
	jw, err := NewJSONWriter(jsonPath)
	if err != nil {
		t.Fatalf("NewJSONWriter: %v", err)
	}

	ctx := context.Background()
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	rec := Record{
		TS: now, Model: "gpt-4", Provider: "openai", APIKey: "k1",
		InputTokens: 10, OutputTokens: 20, TotalTokens: 30,
		Generate: true, LatencyNs: 750000, UpstreamLatencyNs: 250000,
		TPSBasis: "stale",
	}
	if err := jw.Write(ctx, rec); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := jw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read jsonl: %v", err)
	}
	var decoded Record
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal jsonl line: %v", err)
	}
	if decoded.Model != "gpt-4" || decoded.TotalTokens != 30 {
		t.Errorf("decoded record mismatch: %+v", decoded)
	}
	if decoded.LatencyNs != 750000 || decoded.UpstreamLatencyNs != 250000 || decoded.TPS == nil || *decoded.TPS != 80000 || decoded.TPSBasis != "upstream" {
		t.Errorf("decoded precise throughput mismatch: %+v", decoded)
	}
}

func TestJSONWriterConcurrent(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "usage.jsonl")
	jw, err := NewJSONWriter(jsonPath)
	if err != nil {
		t.Fatalf("NewJSONWriter: %v", err)
	}

	ctx := context.Background()
	const workers = 10
	const perWorker = 20
	errChan := make(chan error, workers)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			for j := 0; j < perWorker; j++ {
				rec := Record{
					TS:          time.Now(),
					Model:       fmt.Sprintf("model-%d", i),
					TotalTokens: int64(j),
				}
				if err := jw.Write(ctx, rec); err != nil {
					errChan <- err
					return
				}
			}
		}(i)
	}
	wg.Wait()
	close(errChan)
	for err := range errChan {
		t.Fatalf("concurrent write failed: %v", err)
	}
	if err := jw.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	f, err := os.Open(jsonPath)
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan jsonl: %v", err)
	}
	expected := workers * perWorker
	if count != expected {
		t.Errorf("jsonl lines=%d, want %d", count, expected)
	}
}

func TestStoreWithJSONWriter(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "usage.jsonl")
	jw, err := NewJSONWriter(jsonPath)
	if err != nil {
		t.Fatalf("NewJSONWriter: %v", err)
	}

	store, err := Open(dir, WithWriter(jw))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = Close() })

	ctx := context.Background()
	now := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	if err := store.Insert(ctx, Record{TS: now, Model: "gpt-4", Provider: "openai", TotalTokens: 100}); err != nil {
		t.Fatalf("Insert: %v", err)
	}
	if err := store.Insert(ctx, Record{TS: now, Model: "claude-3", Provider: "anthropic", TotalTokens: 50}); err != nil {
		t.Fatalf("Insert: %v", err)
	}

	f, err := os.Open(jsonPath)
	if err != nil {
		t.Fatalf("open jsonl: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	count := 0
	for scanner.Scan() {
		var decoded Record
		if err := json.Unmarshal(scanner.Bytes(), &decoded); err != nil {
			t.Fatalf("unmarshal line: %v", err)
		}
		count++
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scan jsonl: %v", err)
	}
	if count != 2 {
		t.Errorf("jsonl lines=%d, want 2", count)
	}
}

func TestStoreTPSValidity(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = Close() })
	ctx := context.Background()
	start := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		output   int64
		latency  int64
		ttft     int64
		failed   bool
		generate bool
		want     float64
		valid    bool
	}{
		{name: "fractional end-to-end", output: 1, latency: 3000, ttft: 2000, generate: true, want: 1.0 / 3, valid: true},
		{name: "subsecond", output: 25, latency: 500, generate: true, want: 50, valid: true},
		{name: "large output without integer overflow", output: 1 << 62, latency: 1000, generate: true, want: float64(1 << 62), valid: true},
		{name: "failed", output: 25, latency: 500, failed: true, generate: true},
		{name: "non-generation", output: 25, latency: 500},
		{name: "zero output", latency: 500, generate: true},
		{name: "negative output", output: -25, latency: 500, generate: true},
		{name: "zero latency", output: 25, generate: true},
		{name: "negative latency", output: 25, latency: -500, generate: true},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from := start.Add(time.Duration(i) * time.Second)
			to := from.Add(time.Second)
			staleTPS := 999.0
			if err := store.Insert(ctx, Record{
				TS: from, OutputTokens: tt.output, LatencyMs: tt.latency, TTFtMs: tt.ttft,
				Failed: tt.failed, Generate: tt.generate, TPS: &staleTPS,
			}); err != nil {
				t.Fatalf("Insert: %v", err)
			}
			records, total, err := store.List(ctx, ListFilter{From: from, To: to})
			if err != nil {
				t.Fatalf("List: %v", err)
			}
			if total != 1 || len(records) != 1 {
				t.Fatalf("List total=%d len=%d, want 1/1", total, len(records))
			}
			summary, err := store.Summary(ctx, from, to)
			if err != nil {
				t.Fatalf("Summary: %v", err)
			}
			for name, got := range map[string]*float64{"TPS": records[0].TPS, "AvgTPS": summary.AvgTPS} {
				if !tt.valid {
					if got != nil {
						t.Errorf("%s=%g, want nil", name, *got)
					}
					continue
				}
				if got == nil {
					t.Errorf("%s=nil, want %g", name, tt.want)
				} else if math.Abs(*got-tt.want) > tt.want*1e-12 {
					t.Errorf("%s=%g, want %g", name, *got, tt.want)
				}
			}
		})
	}
	from := start.Add(time.Duration(len(tests)) * time.Second)
	summary, err := store.Summary(ctx, from, from.Add(time.Second))
	if err != nil {
		t.Fatalf("empty Summary: %v", err)
	}
	if summary.TotalRequests != 0 || summary.AvgTPS != nil {
		t.Errorf("empty Summary=%+v, want zero requests and nil AvgTPS", summary)
	}
}

func TestStoreTPSHistoricalMeanAndTimeWindow(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = Close() })
	ctx := context.Background()
	from := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	// Persist historical rows using only the existing schema. Omitting generate
	// exercises its historical default rather than the current Insert path.
	for _, row := range []struct {
		ts      time.Time
		output  int64
		latency int64
		failed  int
	}{
		{from.Add(-time.Millisecond), 500, 1000, 0},
		{from, 10, 1000, 0},
		{from.Add(time.Minute), 90, 3000, 0},
		{from.Add(2 * time.Minute), 1000, 1000, 1},
		{to, 900, 1000, 0},
	} {
		if _, err := store.db.ExecContext(ctx,
			`INSERT INTO requests (ts, output_tokens, latency_ms, failed) VALUES (?, ?, ?, ?)`,
			row.ts.UnixMilli(), row.output, row.latency, row.failed); err != nil {
			t.Fatalf("insert historical row: %v", err)
		}
	}
	if err := Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	store, err = Open(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	summary, err := store.Summary(ctx, from, to)
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	// Mean of 10 and 30 is 20, not the ratio of totals (100 / 4 = 25).
	if summary.TotalRequests != 3 || summary.AvgTPS == nil || *summary.AvgTPS != 20 {
		t.Errorf("Summary=%+v, want 3 requests and AvgTPS=20", summary)
	}
	records, total, err := store.List(ctx, ListFilter{From: from, To: to, Limit: 1, Offset: 1})
	if err != nil {
		t.Fatalf("paginated List: %v", err)
	}
	if total != 3 || len(records) != 1 {
		t.Fatalf("List total=%d len=%d, want 3/1", total, len(records))
	}
	if records[0].TPS == nil || *records[0].TPS != 30 {
		t.Errorf("historical TPS=%v, want 30", records[0].TPS)
	}
	all, err := store.Summary(ctx, time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("all-time Summary: %v", err)
	}
	if all.AvgTPS == nil || *all.AvgTPS != 360 {
		t.Errorf("all-time AvgTPS=%v, want 360", all.AvgTPS)
	}
}

func TestStoreUpgradeLegacySchema(t *testing.T) {
	dir := t.TempDir()
	db, err := sql.Open("sqlite", filepath.Join(dir, defaultDBFileName))
	if err != nil {
		t.Fatalf("open legacy DB: %v", err)
	}
	defer db.Close()
	_, err = db.Exec(`CREATE TABLE requests (
		id INTEGER PRIMARY KEY AUTOINCREMENT, ts INTEGER NOT NULL,
		request_id TEXT NOT NULL DEFAULT '', provider TEXT NOT NULL DEFAULT '',
		executor_type TEXT NOT NULL DEFAULT '', model TEXT NOT NULL DEFAULT '',
		alias TEXT NOT NULL DEFAULT '', endpoint TEXT NOT NULL DEFAULT '',
		auth_type TEXT NOT NULL DEFAULT '', auth_id TEXT NOT NULL DEFAULT '',
		auth_index TEXT NOT NULL DEFAULT '', api_key TEXT NOT NULL DEFAULT '',
		source TEXT NOT NULL DEFAULT '', reasoning_effort TEXT NOT NULL DEFAULT '',
		service_tier TEXT NOT NULL DEFAULT '', response_service_tier TEXT NOT NULL DEFAULT '',
		client_ip TEXT NOT NULL DEFAULT '', user_agent TEXT NOT NULL DEFAULT '',
		latency_ms INTEGER NOT NULL DEFAULT 0, ttft_ms INTEGER NOT NULL DEFAULT 0,
		failed INTEGER NOT NULL DEFAULT 0, generate INTEGER NOT NULL DEFAULT 1,
		fail_status INTEGER NOT NULL DEFAULT 0, fail_body TEXT NOT NULL DEFAULT '',
		input_tokens INTEGER NOT NULL DEFAULT 0, output_tokens INTEGER NOT NULL DEFAULT 0,
		reasoning_tokens INTEGER NOT NULL DEFAULT 0, cached_tokens INTEGER NOT NULL DEFAULT 0,
		cache_read_tokens INTEGER NOT NULL DEFAULT 0, cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
		total_tokens INTEGER NOT NULL DEFAULT 0
	); PRAGMA user_version = 1;
	INSERT INTO requests (ts, request_id, output_tokens, latency_ms) VALUES (1, 'legacy', 25, 500)`)
	if err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("close legacy DB: %v", err)
	}
	store, err := Open(dir)
	if err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	t.Cleanup(func() { _ = Close() })
	ctx := context.Background()
	if err := store.Insert(ctx, Record{TS: time.UnixMilli(2), RequestID: "precise", Generate: true, OutputTokens: 1, LatencyNs: 250000, UpstreamLatencyNs: 125000}); err != nil {
		t.Fatalf("insert precise row: %v", err)
	}
	if err := Close(); err != nil {
		t.Fatalf("close upgraded store: %v", err)
	}
	store, err = Open(dir)
	if err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	rows, total, err := store.List(ctx, ListFilter{})
	if err != nil || total != 2 || len(rows) != 2 {
		t.Fatalf("List: rows=%+v total=%d err=%v", rows, total, err)
	}
	if rows[0].LatencyNs != 250000 || rows[0].UpstreamLatencyNs != 125000 || rows[0].TPS == nil || *rows[0].TPS != 8000 || rows[0].TPSBasis != "upstream" {
		t.Errorf("precise row=%+v", rows[0])
	}
	if rows[1].RequestID != "legacy" || rows[1].LatencyNs != 0 || rows[1].UpstreamLatencyNs != 0 || rows[1].TPS == nil || *rows[1].TPS != 50 || rows[1].TPSBasis != "end-to-end" {
		t.Errorf("legacy row=%+v", rows[1])
	}
	summary, err := store.Summary(ctx, time.Time{}, time.Time{})
	if err != nil || summary.AvgTPS == nil || *summary.AvgTPS != 4025 {
		t.Errorf("Summary=%+v err=%v, want AvgTPS=4025", summary, err)
	}
}

func TestStorePreciseTPSHierarchy(t *testing.T) {
	store, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = Close() })
	ctx := context.Background()
	start := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name     string
		record   Record
		want     float64
		basis    string
	}{
		{name: "upstream overrides local", record: Record{Generate: true, OutputTokens: 10, LatencyMs: 4000, LatencyNs: 4000000000, UpstreamLatencyNs: 2000000000}, want: 5, basis: "upstream"},
		{name: "upstream without local", record: Record{Generate: true, OutputTokens: 2, UpstreamLatencyNs: 250000}, want: 8000, basis: "upstream"},
		{name: "single nanosecond", record: Record{Generate: true, OutputTokens: 1, LatencyNs: 1}, want: 1e9, basis: "end-to-end"},
		{name: "precise local overrides rounded ms", record: Record{Generate: true, OutputTokens: 3, LatencyMs: 1, LatencyNs: 1500000, UpstreamLatencyNs: -1}, want: 2000, basis: "end-to-end"},
		{name: "legacy fallback", record: Record{Generate: true, OutputTokens: 3, LatencyMs: 1500, LatencyNs: -1}, want: 2, basis: "end-to-end"},
		{name: "large ms avoids overflow", record: Record{Generate: true, OutputTokens: 1, LatencyMs: 1 << 62}, want: 1000.0 / float64(1 << 62), basis: "end-to-end"},
		{name: "failed upstream", record: Record{Generate: true, Failed: true, OutputTokens: 10, UpstreamLatencyNs: 1}},
		{name: "non-generation upstream", record: Record{OutputTokens: 10, UpstreamLatencyNs: 1}},
		{name: "zero output upstream", record: Record{Generate: true, UpstreamLatencyNs: 1}},
		{name: "negative durations", record: Record{Generate: true, OutputTokens: 1, LatencyMs: -1, LatencyNs: -1, UpstreamLatencyNs: -1}},
	}
	var sum float64
	var valid int
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from := start.Add(time.Duration(i) * time.Second)
			tt.record.TS = from
			staleTPS := 999.0
			tt.record.TPS = &staleTPS
			tt.record.TPSBasis = "stale"
			if err := store.Insert(ctx, tt.record); err != nil {
				t.Fatalf("Insert: %v", err)
			}
			rows, _, err := store.List(ctx, ListFilter{From: from, To: from.Add(time.Second)})
			if err != nil || len(rows) != 1 {
				t.Fatalf("List: rows=%+v err=%v", rows, err)
			}
			summary, err := store.Summary(ctx, from, from.Add(time.Second))
			if err != nil {
				t.Fatalf("Summary: %v", err)
			}
			if rows[0].TPSBasis != tt.basis {
				t.Errorf("TPSBasis=%q, want %q", rows[0].TPSBasis, tt.basis)
			}
			for name, got := range map[string]*float64{"TPS": rows[0].TPS, "AvgTPS": summary.AvgTPS} {
				if tt.basis == "" {
					if got != nil {
						t.Errorf("%s=%g, want nil", name, *got)
					}
				} else if got == nil || math.Abs(*got-tt.want) > tt.want*1e-12 {
					t.Errorf("%s=%v, want %g", name, got, tt.want)
				}
			}
		})
		if tt.basis != "" {
			sum += tt.want
			valid++
		}
	}
	summary, err := store.Summary(ctx, time.Time{}, time.Time{})
	want := sum / float64(valid)
	if err != nil || summary.AvgTPS == nil || math.Abs(*summary.AvgTPS-want) > want*1e-12 {
		t.Errorf("mixed Summary=%+v err=%v, want AvgTPS=%g", summary, err, want)
	}
}

func TestRecorderPreciseLatencyPersistenceAndJSON(t *testing.T) {
	dir := t.TempDir()
	jsonPath := filepath.Join(dir, "usage.jsonl")
	writer, err := NewJSONWriter(jsonPath)
	if err != nil {
		t.Fatalf("NewJSONWriter: %v", err)
	}
	store, err := Open(dir, WithWriter(writer))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = Close() })
	(&recorder{}).HandleUsage(context.Background(), coreusage.Record{
		Provider: "openai", Model: "precise", RequestedAt: time.Unix(1, 0),
		Latency: 750001 * time.Nanosecond, UpstreamLatency: 250000 * time.Nanosecond,
		Detail: coreusage.Detail{OutputTokens: 2, TotalTokens: 2},
	})
	rows, total, err := store.List(context.Background(), ListFilter{})
	if err != nil || total != 1 || len(rows) != 1 {
		t.Fatalf("List: rows=%+v total=%d err=%v", rows, total, err)
	}
	data, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read mirror: %v", err)
	}
	var mirrored Record
	if err := json.Unmarshal(data, &mirrored); err != nil {
		t.Fatalf("decode mirror: %v", err)
	}
	for name, record := range map[string]Record{"SQLite": rows[0], "JSON": mirrored} {
		if record.LatencyMs != 0 || record.LatencyNs != 750001 || record.UpstreamLatencyNs != 250000 || record.TPS == nil || *record.TPS != 8000 || record.TPSBasis != "upstream" {
			t.Errorf("%s record=%+v", name, record)
		}
	}
}
