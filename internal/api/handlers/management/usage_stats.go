package management

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/shinmentakezo07/shinway/v7/internal/usagestore"
)

// UsageStats handles the aggregated dashboard payload. It returns summary
// counters, a time series ready for charting, and per-dimension rollups
// (model/provider/api key/auth) in a single response so the UI needs one
// round trip.
func (h *Handler) GetUsageStats(c *gin.Context) {
	store := usagestore.Default()
	if store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "usage store not initialized"})
		return
	}

	from, to, err := parseTimeRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	bucketSeconds := parseInt64Query(c, "bucket_seconds", 0)
	if bucketSeconds <= 0 {
		bucketSeconds = pickBucketSeconds(from, to)
	}

	summary, err := store.Summary(c.Request.Context(), from, to)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "summary failed: " + err.Error()})
		return
	}
	series, err := store.Series(c.Request.Context(), from, to, bucketSeconds)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "series failed: " + err.Error()})
		return
	}
	byModel, err := store.ByDimension(c.Request.Context(), from, to, "model", 25)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "by-model failed: " + err.Error()})
		return
	}
	byProvider, err := store.ByDimension(c.Request.Context(), from, to, "provider", 25)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "by-provider failed: " + err.Error()})
		return
	}
	byAPIKey, err := store.ByDimension(c.Request.Context(), from, to, "api_key", 25)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "by-api-key failed: " + err.Error()})
		return
	}
	byAuth, err := store.ByDimension(c.Request.Context(), from, to, "auth_id", 25)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "by-auth failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"summary":        summary,
		"bucket_seconds": bucketSeconds,
		"series":         series,
		"by_model":       byModel,
		"by_provider":    byProvider,
		"by_api_key":     byAPIKey,
		"by_auth":        byAuth,
	})
}

// GetUsageRecords returns individual request rows for the logs pane.
// Supports pagination, filtering and free-text search.
func (h *Handler) GetUsageRecords(c *gin.Context) {
	store := usagestore.Default()
	if store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "usage store not initialized"})
		return
	}

	from, to, err := parseTimeRange(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	filter := usagestore.ListFilter{
		From:     from,
		To:       to,
		Model:    c.Query("model"),
		Provider: c.Query("provider"),
		APIKey:   c.Query("api_key"),
		AuthID:   c.Query("auth_id"),
		Search:   c.Query("search"),
		Limit:    int(parseInt64Query(c, "limit", 100)),
		Offset:   int(parseInt64Query(c, "offset", 0)),
	}
	if v := strings.TrimSpace(c.Query("failed")); v != "" {
		parsed, errBool := strconv.ParseBool(v)
		if errBool != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "failed must be boolean"})
			return
		}
		filter.Failed = &parsed
	}

	records, total, err := store.List(c.Request.Context(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "list failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"total":   total,
		"limit":   filter.Limit,
		"offset":  filter.Offset,
		"records": records,
	})
}

// DeleteUsageRecords purges records older than the given cutoff timestamp.
// Requires `before` (RFC3339 or unix ms). Use DeleteUsageStats for full wipe.
func (h *Handler) DeleteUsageRecords(c *gin.Context) {
	store := usagestore.Default()
	if store == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "usage store not initialized"})
		return
	}

	beforeStr := strings.TrimSpace(c.Query("before"))
	if beforeStr == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "before query parameter is required"})
		return
	}

	cutoff, err := parseFlexibleTime(beforeStr, time.Time{})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid before: " + err.Error()})
		return
	}

	deleted, err := store.PurgeOlderThan(c.Request.Context(), cutoff)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "purge failed: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": deleted})
}

// parseTimeRange reads from/to query params. Accepts `from` and `to` as
// RFC3339, unix seconds/millis, or a shorthand `range` like 1h, 24h, 7d, 30d.
// Zero times mean "all time".
func parseTimeRange(c *gin.Context) (time.Time, time.Time, error) {
	if shorthand := strings.TrimSpace(c.Query("range")); shorthand != "" {
		d, err := parseShorthandDuration(shorthand)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
		to := time.Now()
		return to.Add(-d), to, nil
	}

	var from, to time.Time
	var err error
	if v := strings.TrimSpace(c.Query("from")); v != "" {
		from, err = parseFlexibleTime(v, time.Time{})
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid from: " + err.Error())
		}
	}
	if v := strings.TrimSpace(c.Query("to")); v != "" {
		to, err = parseFlexibleTime(v, time.Time{})
		if err != nil {
			return time.Time{}, time.Time{}, errors.New("invalid to: " + err.Error())
		}
	}
	return from, to, nil
}

// parseFlexibleTime accepts RFC3339, unix seconds or unix milliseconds.
func parseFlexibleTime(value string, fallback time.Time) (time.Time, error) {
	if value == "" {
		return fallback, nil
	}
	if t, err := time.Parse(time.RFC3339, value); err == nil {
		return t, nil
	}
	if n, err := strconv.ParseInt(value, 10, 64); err == nil {
		// heuristic: ms if the number is in the 10^12 range
		if n > 1e12 {
			return time.UnixMilli(n), nil
		}
		return time.Unix(n, 0), nil
	}
	return time.Time{}, errors.New("unsupported time format")
}

// parseShorthandDuration accepts 1h, 24h, 7d, 30d etc. Days are converted.
func parseShorthandDuration(value string) (time.Duration, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return 0, errors.New("empty duration")
	}
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil || days <= 0 {
			return 0, errors.New("invalid day duration: " + value)
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	d, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if d <= 0 {
		return 0, errors.New("duration must be positive")
	}
	return d, nil
}

// pickBucketSeconds selects a sensible bucket width based on the requested
// window. Aims for roughly 24-90 buckets per chart.
func pickBucketSeconds(from, to time.Time) int64 {
	if from.IsZero() {
		return 3600 // hourly default when range isn't set
	}
	span := to.Sub(from)
	switch {
	case span <= 2*time.Hour:
		return 60 // 1 minute
	case span <= 12*time.Hour:
		return 300 // 5 minutes
	case span <= 48*time.Hour:
		return 900 // 15 minutes
	case span <= 14*24*time.Hour:
		return 3600 // hourly
	default:
		return 86400 // daily
	}
}

func parseInt64Query(c *gin.Context, name string, fallback int64) int64 {
	value := strings.TrimSpace(c.Query(name))
	if value == "" {
		return fallback
	}
	n, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}
