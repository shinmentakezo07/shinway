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
		{TS: now, Model: "claude-sonnet", Provider: "anthropic", APIKey: "k2", InputTokens: 5, OutputTokens: 10, TotalTokens: 15},
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
