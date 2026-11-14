package management

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shinmentakezo07/shinway/v7/internal/usagestore"
)

func setupUsageStore(t *testing.T) {
	t.Helper()
	if _, err := usagestore.Open(t.TempDir()); err != nil {
		t.Fatalf("open usage store: %v", err)
	}
	t.Cleanup(func() { _ = usagestore.Close() })
}

func seedUsage(t *testing.T) {
	t.Helper()
	store := usagestore.Default()
	now := time.Now()
	records := []usagestore.Record{
		{TS: now.Add(-2 * time.Hour), Model: "gpt-4o", Provider: "openai", APIKey: "k1", InputTokens: 100, OutputTokens: 200, TotalTokens: 300, LatencyMs: 80},
		{TS: now.Add(-1 * time.Hour), Model: "gpt-4o", Provider: "openai", APIKey: "k1", InputTokens: 50, OutputTokens: 100, TotalTokens: 150, Failed: true, FailStatus: 500},
		// Kept safely inside the 24h window: the stats queries use an exclusive
		// upper bound (ts < to), so a record stamped exactly time.Now() can be
		// excluded when seeding and querying land in the same millisecond.
		{TS: now.Add(-1 * time.Minute), Model: "claude-sonnet", Provider: "anthropic", APIKey: "k2", InputTokens: 5, OutputTokens: 10, TotalTokens: 15},
	}
	for _, r := range records {
		if err := store.Insert(context.Background(), r); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
}

func TestGetUsageStats(t *testing.T) {
	setupUsageStore(t)
	seedUsage(t)

	gin.SetMode(gin.TestMode)
	h := NewHandlerWithoutConfigFilePath(nil, nil)
	r := gin.New()
	r.GET("/stats", h.GetUsageStats)

	req := httptest.NewRequest(http.MethodGet, "/stats?range=24h", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}

	var payload struct {
		Summary struct {
			TotalRequests  int64 `json:"total_requests"`
			FailedRequests int64 `json:"failed_requests"`
			TotalTokens    int64 `json:"total_tokens"`
		} `json:"summary"`
		ByModel []struct {
			Key   string `json:"key"`
			Total int64  `json:"total_tokens"`
		} `json:"by_model"`
		Series []map[string]any `json:"series"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Summary.TotalRequests != 3 {
		t.Errorf("total=%d, want 3", payload.Summary.TotalRequests)
	}
	if payload.Summary.FailedRequests != 1 {
		t.Errorf("failed=%d, want 1", payload.Summary.FailedRequests)
	}
	if payload.Summary.TotalTokens != 465 {
		t.Errorf("tokens=%d, want 465", payload.Summary.TotalTokens)
	}
	if len(payload.ByModel) != 2 {
		t.Errorf("by_model=%d, want 2", len(payload.ByModel))
	}
}

func TestGetUsageRecords(t *testing.T) {
	setupUsageStore(t)
	seedUsage(t)

	gin.SetMode(gin.TestMode)
	h := NewHandlerWithoutConfigFilePath(nil, nil)
	r := gin.New()
	r.GET("/records", h.GetUsageRecords)

	req := httptest.NewRequest(http.MethodGet, "/records?failed=true&limit=10", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}

	var payload struct {
		Total   int64               `json:"total"`
		Records []usagestore.Record `json:"records"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Total != 1 || len(payload.Records) != 1 || !payload.Records[0].Failed {
		t.Errorf("records mismatch %+v", payload)
	}

	req2 := httptest.NewRequest(http.MethodGet, "/records?model=gpt-4o", nil)
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	var payload2 struct {
		Total int64 `json:"total"`
	}
	_ = json.Unmarshal(w2.Body.Bytes(), &payload2)
	if payload2.Total != 2 {
		t.Errorf("model filter total=%d, want 2", payload2.Total)
	}
}

func TestDeleteUsageRecords(t *testing.T) {
	setupUsageStore(t)
	seedUsage(t)

	gin.SetMode(gin.TestMode)
	h := NewHandlerWithoutConfigFilePath(nil, nil)
	r := gin.New()
	r.DELETE("/records", h.DeleteUsageRecords)

	before := strconv.FormatInt(time.Now().Add(-90*time.Minute).UnixMilli(), 10)
	req := httptest.NewRequest(http.MethodDelete, "/records?before="+before, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d", w.Code)
	}

	var payload struct {
		Deleted int64 `json:"deleted"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &payload)
	if payload.Deleted != 1 {
		t.Errorf("deleted=%d, want 1", payload.Deleted)
	}
}

func TestGetUsageStatsNoStore(t *testing.T) {
	_ = usagestore.Close() // ensure no store
	gin.SetMode(gin.TestMode)
	h := NewHandlerWithoutConfigFilePath(nil, nil)
	r := gin.New()
	r.GET("/stats", h.GetUsageStats)
	req := httptest.NewRequest(http.MethodGet, "/stats", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusServiceUnavailable {
		t.Errorf("status=%d, want 503", w.Code)
	}
}

func TestUsageTPSJSON(t *testing.T) {
	setupUsageStore(t)
	from := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	to := from.Add(time.Hour)
	for i, record := range []usagestore.Record{
		{RequestID: "slow", Generate: true, OutputTokens: 10, LatencyMs: 1000, TTFtMs: 900},
		{RequestID: "fast", Generate: true, OutputTokens: 90, LatencyMs: 3000},
		{RequestID: "failed", Generate: true, Failed: true, OutputTokens: 1000, LatencyMs: 1000},
		{RequestID: "non-generation", OutputTokens: 1000, LatencyMs: 1000},
		{RequestID: "zero-output", Generate: true, LatencyMs: 1000},
		{RequestID: "zero-latency", Generate: true, OutputTokens: 1000},
	} {
		record.TS = from.Add(time.Duration(i) * time.Minute)
		if err := usagestore.Default().Insert(context.Background(), record); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	gin.SetMode(gin.TestMode)
	h := NewHandlerWithoutConfigFilePath(nil, nil)
	r := gin.New()
	r.GET("/stats", h.GetUsageStats)
	r.GET("/records", h.GetUsageRecords)
	query := "?from=" + strconv.FormatInt(from.UnixMilli(), 10) + "&to=" + strconv.FormatInt(to.UnixMilli(), 10)

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/records"+query, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("records status=%d body=%s", w.Code, w.Body.String())
	}
	var payload struct {
		Records []map[string]json.RawMessage `json:"records"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode records: %v", err)
	}
	if len(payload.Records) != 6 {
		t.Fatalf("records len=%d, want 6", len(payload.Records))
	}
	for _, record := range payload.Records {
		var requestID string
		if err := json.Unmarshal(record["RequestID"], &requestID); err != nil {
			t.Fatalf("decode request ID: %v", err)
		}
		tps, exists := record["TPS"]
		if !exists {
			t.Errorf("%s missing TPS key", requestID)
			continue
		}
		want := "null"
		switch requestID {
		case "slow":
			want = "10"
		case "fast":
			want = "30"
		}
		if string(tps) != want {
			t.Errorf("%s TPS=%s, want %s", requestID, tps, want)
		}
	}
	for _, tt := range []struct {
		name  string
		query string
		want  string
	}{
		{name: "full range", query: query + "&limit=1", want: "20"},
		{name: "invalid-only range", query: "?from=" + strconv.FormatInt(from.Add(2*time.Minute).UnixMilli(), 10) + "&to=" + strconv.FormatInt(to.UnixMilli(), 10), want: "null"},
		{name: "empty range", query: "?from=" + strconv.FormatInt(to.UnixMilli(), 10), want: "null"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stats"+tt.query, nil))
			if w.Code != http.StatusOK {
				t.Fatalf("stats status=%d body=%s", w.Code, w.Body.String())
			}
			var payload struct {
				Summary map[string]json.RawMessage `json:"summary"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
				t.Fatalf("decode summary: %v", err)
			}
			avgTPS, exists := payload.Summary["avg_tps"]
			if !exists || string(avgTPS) != tt.want {
				t.Errorf("avg_tps=%s exists=%v, want %s", avgTPS, exists, tt.want)
			}
		})
	}
}

func TestUsagePreciseTPSJSON(t *testing.T) {
	setupUsageStore(t)
	from := time.Date(2026, 7, 28, 12, 0, 0, 0, time.UTC)
	for i, record := range []usagestore.Record{
		{RequestID: "upstream", Generate: true, OutputTokens: 2, LatencyMs: 4000, LatencyNs: 4000000000, UpstreamLatencyNs: 250000},
		{RequestID: "precise-local", Generate: true, OutputTokens: 1, LatencyNs: 500000},
		{RequestID: "legacy-local", Generate: true, OutputTokens: 6, LatencyMs: 3000},
		{RequestID: "unavailable", Generate: true, Failed: true, OutputTokens: 2, UpstreamLatencyNs: 250000},
	} {
		record.TS = from.Add(time.Duration(i) * time.Minute)
		if err := usagestore.Default().Insert(context.Background(), record); err != nil {
			t.Fatalf("Insert: %v", err)
		}
	}
	gin.SetMode(gin.TestMode)
	h := NewHandlerWithoutConfigFilePath(nil, nil)
	r := gin.New()
	r.GET("/stats", h.GetUsageStats)
	r.GET("/records", h.GetUsageRecords)
	query := "?from=" + strconv.FormatInt(from.UnixMilli(), 10) + "&to=" + strconv.FormatInt(from.Add(time.Hour).UnixMilli(), 10)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/records"+query, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("records status=%d body=%s", w.Code, w.Body.String())
	}
	var payload struct {
		Records []map[string]json.RawMessage `json:"records"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode records: %v", err)
	}
	if len(payload.Records) != 4 {
		t.Fatalf("records len=%d, want 4", len(payload.Records))
	}
	for _, record := range payload.Records {
		var requestID, basis string
		if err := json.Unmarshal(record["RequestID"], &requestID); err != nil {
			t.Fatalf("decode request ID: %v", err)
		}
		if err := json.Unmarshal(record["TPSBasis"], &basis); err != nil {
			t.Fatalf("decode TPS basis: %v", err)
		}
		wantTPS, wantBasis := "null", ""
		switch requestID {
		case "upstream":
			wantTPS, wantBasis = "8000", "upstream"
			if string(record["LatencyNs"]) != "4000000000" || string(record["UpstreamLatencyNs"]) != "250000" || string(record["LatencyMs"]) != "4000" {
				t.Errorf("upstream latency JSON=%v", record)
			}
		case "precise-local":
			wantTPS, wantBasis = "2000", "end-to-end"
			if string(record["LatencyNs"]) != "500000" || string(record["LatencyMs"]) != "0" {
				t.Errorf("precise latency JSON=%v", record)
			}
		case "legacy-local":
			wantTPS, wantBasis = "2", "end-to-end"
		}
		if string(record["TPS"]) != wantTPS || basis != wantBasis {
			t.Errorf("%s TPS=%s basis=%q, want %s/%q", requestID, record["TPS"], basis, wantTPS, wantBasis)
		}
	}
	w = httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/stats"+query, nil))
	if w.Code != http.StatusOK {
		t.Fatalf("stats status=%d body=%s", w.Code, w.Body.String())
	}
	var stats struct {
		Summary struct {
			AvgTPS *float64 `json:"avg_tps"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &stats); err != nil {
		t.Fatalf("decode stats: %v", err)
	}
	if stats.Summary.AvgTPS == nil || *stats.Summary.AvgTPS != 3334 {
		t.Errorf("avg_tps=%v, want 3334", stats.Summary.AvgTPS)
	}
}
