// Package usagestore persists per-request usage records into a local SQLite
// database so the management API can serve historical token analytics without
// depending on the volatile in-memory counters or the redis queue.
package usagestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	internallogging "github.com/shinmentakezo07/shinway/v7/internal/logging"
	coreusage "github.com/shinmentakezo07/shinway/v7/sdk/shinway/usage"
	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

const (
	defaultDBFileName = "usage.db"
	schemaVersion     = 1
)

var (
	// enabled mirrors the config `usage-statistics-enabled` toggle so the
	// recorder can be disabled at runtime without unregistering the plugin.
	enabled atomic.Bool

	defaultStoreMu sync.RWMutex
	defaultStore   *Store
	defaultOpts    []OpenOption
)

func init() {
	enabled.Store(true)
	coreusage.RegisterPlugin(&recorder{})
}

// SetEnabled toggles whether incoming records are persisted. It is driven by
// the same config flag that guards the in-memory usage statistics.
func SetEnabled(v bool) { enabled.Store(v) }

// Enabled reports whether persistence is active.
func Enabled() bool { return enabled.Load() }

// OpenOption customizes the default store.
type OpenOption func(*Store)

// SetDefaultOptions registers options that will be applied to every future
// Open call. It is intended to be called once at startup so the store can be
// reopened (e.g. after a config reload) with the same mirror writers.
func SetDefaultOptions(opts ...OpenOption) {
	defaultStoreMu.Lock()
	defer defaultStoreMu.Unlock()
	defaultOpts = opts
}

// WithWriter attaches an extra Writer that receives a copy of every inserted
// usage record. Writers are closed when the store is closed.
func WithWriter(w Writer) OpenOption {
	return func(s *Store) {
		if s == nil || w == nil {
			return
		}
		s.AttachWriter(w)
	}
}

// AttachWriter adds a Writer to the store. It is safe for concurrent use.
func (s *Store) AttachWriter(w Writer) {
	if s == nil || w == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writers = append(s.writers, w)
}

// Open initializes (or reuses) the default store rooted at dir. The returned
// store must be closed by the caller; however the package keeps a reference so
// the usage plugin can reach it from any goroutine.
// Options are remembered so a later Open call without options still uses the
// same backends (JSON mirror, Postgres mirror, etc.).
func Open(dir string, opts ...OpenOption) (*Store, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return nil, errors.New("usagestore: empty directory")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("usagestore: mkdir: %w", err)
	}
	dbPath := filepath.Join(dir, defaultDBFileName)

	defaultStoreMu.Lock()
	defer defaultStoreMu.Unlock()

	if len(opts) == 0 {
		opts = defaultOpts
	}

	if defaultStore != nil {
		if defaultStore.path == dbPath {
			return defaultStore, nil
		}
		// path changed, close previous store
		_ = defaultStore.Close()
		defaultStore = nil
	}

	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)")
	if err != nil {
		return nil, fmt.Errorf("usagestore: open: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)

	store := &Store{db: db, path: dbPath}
	if err := store.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}

	// Attach any configured extra writers (JSON mirror, Postgres mirror, ...).
	for _, opt := range opts {
		opt(store)
	}

	defaultStore = store
	return store, nil
}

// Default returns the initialized store, or nil if Open has not been called.
func Default() *Store {
	defaultStoreMu.RLock()
	defer defaultStoreMu.RUnlock()
	return defaultStore
}

// Close shuts down the default store, if any.
func Close() error {
	defaultStoreMu.Lock()
	defer defaultStoreMu.Unlock()
	if defaultStore == nil {
		return nil
	}
	err := defaultStore.Close()
	defaultStore = nil
	return err
}

// Writer receives a copy of every persisted usage record. Writers are called
// by the primary SQLite store after a successful insert so usage data can be
// mirrored to JSON files, Postgres, or other backends.
type Writer interface {
	Write(ctx context.Context, r Record) error
	Close() error
}

// Store wraps a SQLite database with the schema required by the management
// dashboard. Single-writer via MaxOpenConns(1) plus application-side mutex.
// Additional Writer backends can be attached to mirror records.
type Store struct {
	db      *sql.DB
	path    string
	writers []Writer
	mu      sync.Mutex
}

// Path returns the directory containing the SQLite database.
func (s *Store) Path() string {
	if s == nil {
		return ""
	}
	return filepath.Dir(s.path)
}

// Close closes the underlying database handle and any attached mirror writers.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var err error
	if s.db != nil {
		err = s.db.Close()
		s.db = nil
	}
	for _, w := range s.writers {
		_ = w.Close()
	}
	s.writers = nil
	return err
}

func (s *Store) migrate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, err := s.db.Exec(`PRAGMA user_version`); err != nil {
		return fmt.Errorf("usagestore: pragma: %w", err)
	}

	stmts := []string{
		`CREATE TABLE IF NOT EXISTS requests (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			ts INTEGER NOT NULL,
			request_id TEXT NOT NULL DEFAULT '',
			provider TEXT NOT NULL DEFAULT '',
			executor_type TEXT NOT NULL DEFAULT '',
			model TEXT NOT NULL DEFAULT '',
			alias TEXT NOT NULL DEFAULT '',
			endpoint TEXT NOT NULL DEFAULT '',
			auth_type TEXT NOT NULL DEFAULT '',
			auth_id TEXT NOT NULL DEFAULT '',
			auth_index TEXT NOT NULL DEFAULT '',
			api_key TEXT NOT NULL DEFAULT '',
			source TEXT NOT NULL DEFAULT '',
			reasoning_effort TEXT NOT NULL DEFAULT '',
			service_tier TEXT NOT NULL DEFAULT '',
			response_service_tier TEXT NOT NULL DEFAULT '',
			client_ip TEXT NOT NULL DEFAULT '',
			user_agent TEXT NOT NULL DEFAULT '',
			latency_ms INTEGER NOT NULL DEFAULT 0,
			ttft_ms INTEGER NOT NULL DEFAULT 0,
			failed INTEGER NOT NULL DEFAULT 0,
			generate INTEGER NOT NULL DEFAULT 1,
			fail_status INTEGER NOT NULL DEFAULT 0,
			fail_body TEXT NOT NULL DEFAULT '',
			input_tokens INTEGER NOT NULL DEFAULT 0,
			output_tokens INTEGER NOT NULL DEFAULT 0,
			reasoning_tokens INTEGER NOT NULL DEFAULT 0,
			cached_tokens INTEGER NOT NULL DEFAULT 0,
			cache_read_tokens INTEGER NOT NULL DEFAULT 0,
			cache_creation_tokens INTEGER NOT NULL DEFAULT 0,
			total_tokens INTEGER NOT NULL DEFAULT 0
		)`,
		`CREATE INDEX IF NOT EXISTS idx_requests_ts ON requests(ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_requests_model_ts ON requests(model, ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_requests_provider_ts ON requests(provider, ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_requests_apikey_ts ON requests(api_key, ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_requests_failed_ts ON requests(failed, ts DESC)`,
		fmt.Sprintf(`PRAGMA user_version = %d`, schemaVersion),
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("usagestore: migrate: %w", err)
		}
	}
	return nil
}

// Record is the flattened row persisted per upstream request.
type Record struct {
	TS                  time.Time
	RequestID           string
	Provider            string
	ExecutorType        string
	Model               string
	Alias               string
	Endpoint            string
	AuthType            string
	AuthID              string
	AuthIndex           string
	APIKey              string
	Source              string
	ReasoningEffort     string
	ServiceTier         string
	ResponseServiceTier string
	ClientIP            string
	UserAgent           string
	LatencyMs           int64
	TTFtMs              int64
	Failed              bool
	Generate            bool
	FailStatus          int
	FailBody            string
	InputTokens         int64
	OutputTokens        int64
	ReasoningTokens     int64
	CachedTokens        int64
	CacheReadTokens     int64
	CacheCreationTokens int64
	TotalTokens         int64
}

// Insert appends a usage record. It is safe for concurrent use.
func (s *Store) Insert(ctx context.Context, r Record) error {
	if s == nil {
		return errors.New("usagestore: nil store")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.db == nil {
		return errors.New("usagestore: store closed")
	}

	if r.TS.IsZero() {
		r.TS = time.Now()
	}
	failed := 0
	if r.Failed {
		failed = 1
	}
	generate := 0
	if r.Generate {
		generate = 1
	}

	_, err := s.db.ExecContext(ctx, `INSERT INTO requests (
		ts, request_id, provider, executor_type, model, alias, endpoint,
		auth_type, auth_id, auth_index, api_key, source,
		reasoning_effort, service_tier, response_service_tier,
		client_ip, user_agent,
		latency_ms, ttft_ms, failed, generate, fail_status, fail_body,
		input_tokens, output_tokens, reasoning_tokens, cached_tokens,
		cache_read_tokens, cache_creation_tokens, total_tokens
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		r.TS.UnixMilli(), r.RequestID, r.Provider, r.ExecutorType, r.Model, r.Alias, r.Endpoint,
		r.AuthType, r.AuthID, r.AuthIndex, r.APIKey, r.Source,
		r.ReasoningEffort, r.ServiceTier, r.ResponseServiceTier,
		r.ClientIP, r.UserAgent,
		r.LatencyMs, r.TTFtMs, failed, generate, r.FailStatus, truncate(r.FailBody, 4096),
		r.InputTokens, r.OutputTokens, r.ReasoningTokens, r.CachedTokens,
		r.CacheReadTokens, r.CacheCreationTokens, r.TotalTokens,
	)
	if err != nil {
		return err
	}

	for _, w := range s.writers {
		if wErr := w.Write(ctx, r); wErr != nil {
			log.WithError(wErr).Debug("usagestore: mirror writer failed")
		}
	}
	return nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}

// Summary aggregates totals across an optional time window.
type Summary struct {
	From            time.Time `json:"from"`
	To              time.Time `json:"to"`
	TotalRequests   int64     `json:"total_requests"`
	FailedRequests  int64     `json:"failed_requests"`
	SucceededReqs   int64     `json:"succeeded_requests"`
	TotalInput      int64     `json:"total_input_tokens"`
	TotalOutput     int64     `json:"total_output_tokens"`
	TotalReasoning  int64     `json:"total_reasoning_tokens"`
	TotalCached     int64     `json:"total_cached_tokens"`
	TotalTokens     int64     `json:"total_tokens"`
	AvgLatencyMs    float64   `json:"avg_latency_ms"`
	AvgTTFtMs       float64   `json:"avg_ttft_ms"`
	UniqueModels    int64     `json:"unique_models"`
	UniqueAPIKeys   int64     `json:"unique_api_keys"`
	UniqueAuthFiles int64     `json:"unique_auth_files"`
}

// Summary computes aggregate counters over [from, to). Zero times mean "all".
func (s *Store) Summary(ctx context.Context, from, to time.Time) (Summary, error) {
	out := Summary{From: from, To: to}
	if s == nil {
		return out, errors.New("usagestore: nil store")
	}
	whereSQL, args := timeWindow(from, to)

	row := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*) AS total,
			COALESCE(SUM(failed), 0) AS failed,
			COALESCE(SUM(input_tokens), 0),
			COALESCE(SUM(output_tokens), 0),
			COALESCE(SUM(reasoning_tokens), 0),
			COALESCE(SUM(cache_read_tokens), 0),
			COALESCE(SUM(total_tokens), 0),
			COALESCE(AVG(latency_ms), 0),
			COALESCE(AVG(ttft_ms), 0),
			COUNT(DISTINCT NULLIF(model, '')),
			COUNT(DISTINCT NULLIF(api_key, '')),
			COUNT(DISTINCT NULLIF(auth_id, ''))
		FROM requests `+whereSQL, args...)
	err := row.Scan(
		&out.TotalRequests, &out.FailedRequests,
		&out.TotalInput, &out.TotalOutput, &out.TotalReasoning, &out.TotalCached, &out.TotalTokens,
		&out.AvgLatencyMs, &out.AvgTTFtMs,
		&out.UniqueModels, &out.UniqueAPIKeys, &out.UniqueAuthFiles,
	)
	if err != nil {
		return out, err
	}
	out.SucceededReqs = out.TotalRequests - out.FailedRequests
	return out, nil
}

// Point is a single bucket in a time series.
type Point struct {
	Bucket    time.Time `json:"bucket"`
	Requests  int64     `json:"requests"`
	Failed    int64     `json:"failed"`
	Input     int64     `json:"input_tokens"`
	Output    int64     `json:"output_tokens"`
	Reasoning int64     `json:"reasoning_tokens"`
	Cached    int64     `json:"cached_tokens"`
	Total     int64     `json:"total_tokens"`
}

// Series aggregates requests into buckets of size bucketSeconds between from
// and to. Buckets are aligned to UTC midnight.
func (s *Store) Series(ctx context.Context, from, to time.Time, bucketSeconds int64) ([]Point, error) {
	if bucketSeconds <= 0 {
		bucketSeconds = 3600
	}
	if s == nil {
		return nil, errors.New("usagestore: nil store")
	}
	whereSQL, args := timeWindow(from, to)
	query := fmt.Sprintf(`
		SELECT
			(ts / %d) * %d AS bucket_ms,
			COUNT(*),
			COALESCE(SUM(failed),0),
			COALESCE(SUM(input_tokens),0),
			COALESCE(SUM(output_tokens),0),
			COALESCE(SUM(reasoning_tokens),0),
			COALESCE(SUM(cache_read_tokens),0),
			COALESCE(SUM(total_tokens),0)
		FROM requests %s
		GROUP BY bucket_ms
		ORDER BY bucket_ms ASC`, bucketSeconds*1000, bucketSeconds*1000, whereSQL)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Point
	for rows.Next() {
		var bucketMs int64
		var p Point
		if err := rows.Scan(&bucketMs, &p.Requests, &p.Failed, &p.Input, &p.Output, &p.Reasoning, &p.Cached, &p.Total); err != nil {
			return nil, err
		}
		p.Bucket = time.UnixMilli(bucketMs).UTC()
		out = append(out, p)
	}
	return out, rows.Err()
}

// GroupStat is an aggregate bucket keyed by provider/model/api-key/auth.
type GroupStat struct {
	Key        string  `json:"key"`
	Requests   int64   `json:"requests"`
	Failed     int64   `json:"failed"`
	Input      int64   `json:"input_tokens"`
	Output     int64   `json:"output_tokens"`
	Reasoning  int64   `json:"reasoning_tokens"`
	Cached     int64   `json:"cached_tokens"`
	Total      int64   `json:"total_tokens"`
	AvgLatency float64 `json:"avg_latency_ms"`
}

// ByDimension aggregates per column ('model', 'provider', 'api_key', 'auth_id', 'endpoint').
func (s *Store) ByDimension(ctx context.Context, from, to time.Time, dimension string, limit int) ([]GroupStat, error) {
	col, err := sanitizeDimension(dimension)
	if err != nil {
		return nil, err
	}
	if limit <= 0 || limit > 500 {
		limit = 50
	}
	whereSQL, args := timeWindow(from, to)
	if whereSQL == "" {
		whereSQL = "WHERE 1=1"
	}
	query := fmt.Sprintf(`
		SELECT
			NULLIF(%s, '') AS key,
			COUNT(*) AS requests,
			COALESCE(SUM(failed),0) AS failed,
			COALESCE(SUM(input_tokens),0) AS input_tokens,
			COALESCE(SUM(output_tokens),0) AS output_tokens,
			COALESCE(SUM(reasoning_tokens),0) AS reasoning_tokens,
			COALESCE(SUM(cache_read_tokens),0) AS cached_tokens,
			COALESCE(SUM(total_tokens),0) AS total_tokens,
			COALESCE(AVG(latency_ms),0) AS avg_latency_ms
		FROM requests %s
		GROUP BY key
		ORDER BY total_tokens DESC, requests DESC
		LIMIT %d`, col, whereSQL, limit)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []GroupStat
	for rows.Next() {
		var key sql.NullString
		var g GroupStat
		if err := rows.Scan(&key, &g.Requests, &g.Failed, &g.Input, &g.Output, &g.Reasoning, &g.Cached, &g.Total, &g.AvgLatency); err != nil {
			return nil, err
		}
		if key.Valid {
			g.Key = key.String
		} else {
			g.Key = "(none)"
		}
		out = append(out, g)
	}
	return out, rows.Err()
}

func sanitizeDimension(d string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(d)) {
	case "model":
		return "model", nil
	case "provider":
		return "provider", nil
	case "api_key", "apikey", "key":
		return "api_key", nil
	case "auth_id", "auth":
		return "auth_id", nil
	case "endpoint":
		return "endpoint", nil
	default:
		return "", fmt.Errorf("usagestore: unsupported dimension %q", d)
	}
}

// ListFilter narrows the request-log query.
type ListFilter struct {
	From     time.Time
	To       time.Time
	Model    string
	Provider string
	APIKey   string
	AuthID   string
	Failed   *bool
	Search   string // searches request_id, model, source
	Limit    int
	Offset   int
}

// List returns matching records ordered by newest first along with the total
// count for pagination.
func (s *Store) List(ctx context.Context, f ListFilter) ([]Record, int64, error) {
	if s == nil {
		return nil, 0, errors.New("usagestore: nil store")
	}
	if f.Limit <= 0 || f.Limit > 1000 {
		f.Limit = 100
	}
	where := []string{"1=1"}
	args := []any{}
	if !f.From.IsZero() {
		where = append(where, "ts >= ?")
		args = append(args, f.From.UnixMilli())
	}
	if !f.To.IsZero() {
		where = append(where, "ts < ?")
		args = append(args, f.To.UnixMilli())
	}
	if v := strings.TrimSpace(f.Model); v != "" {
		where = append(where, "model = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(f.Provider); v != "" {
		where = append(where, "provider = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(f.APIKey); v != "" {
		where = append(where, "api_key = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(f.AuthID); v != "" {
		where = append(where, "auth_id = ?")
		args = append(args, v)
	}
	if f.Failed != nil {
		if *f.Failed {
			where = append(where, "failed = 1")
		} else {
			where = append(where, "failed = 0")
		}
	}
	if v := strings.TrimSpace(f.Search); v != "" {
		where = append(where, "(request_id LIKE ? OR model LIKE ? OR source LIKE ? OR alias LIKE ?)")
		like := "%" + v + "%"
		args = append(args, like, like, like, like)
	}
	whereSQL := "WHERE " + strings.Join(where, " AND ")

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM requests `+whereSQL, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	query := `SELECT
		ts, request_id, provider, executor_type, model, alias, endpoint,
		auth_type, auth_id, auth_index, api_key, source,
		reasoning_effort, service_tier, response_service_tier,
		client_ip, user_agent,
		latency_ms, ttft_ms, failed, generate, fail_status, fail_body,
		input_tokens, output_tokens, reasoning_tokens, cached_tokens,
		cache_read_tokens, cache_creation_tokens, total_tokens
	FROM requests ` + whereSQL + ` ORDER BY ts DESC LIMIT ? OFFSET ?`
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var out []Record
	for rows.Next() {
		var r Record
		var ts int64
		var failed, generate int
		err := rows.Scan(
			&ts, &r.RequestID, &r.Provider, &r.ExecutorType, &r.Model, &r.Alias, &r.Endpoint,
			&r.AuthType, &r.AuthID, &r.AuthIndex, &r.APIKey, &r.Source,
			&r.ReasoningEffort, &r.ServiceTier, &r.ResponseServiceTier,
			&r.ClientIP, &r.UserAgent,
			&r.LatencyMs, &r.TTFtMs, &failed, &generate, &r.FailStatus, &r.FailBody,
			&r.InputTokens, &r.OutputTokens, &r.ReasoningTokens, &r.CachedTokens,
			&r.CacheReadTokens, &r.CacheCreationTokens, &r.TotalTokens,
		)
		if err != nil {
			return nil, 0, err
		}
		r.TS = time.UnixMilli(ts).UTC()
		r.Failed = failed == 1
		r.Generate = generate == 1
		out = append(out, r)
	}
	return out, total, rows.Err()
}

// PurgeOlderThan drops records older than the retention cut-off. Returns the
// number of deleted rows.
func (s *Store) PurgeOlderThan(ctx context.Context, cutoff time.Time) (int64, error) {
	if s == nil {
		return 0, errors.New("usagestore: nil store")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	res, err := s.db.ExecContext(ctx, `DELETE FROM requests WHERE ts < ?`, cutoff.UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

func timeWindow(from, to time.Time) (string, []any) {
	clauses := []string{}
	args := []any{}
	if !from.IsZero() {
		clauses = append(clauses, "ts >= ?")
		args = append(args, from.UnixMilli())
	}
	if !to.IsZero() {
		clauses = append(clauses, "ts < ?")
		args = append(args, to.UnixMilli())
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

// recorder is the usage.Plugin that forwards runtime records into the store.
type recorder struct{}

func (r *recorder) HandleUsage(ctx context.Context, rec coreusage.Record) {
	if !Enabled() {
		return
	}
	store := Default()
	if store == nil {
		return
	}

	detail := coreusage.EnsureTokenBreakdownForProvider(rec.Detail, rec.Provider, rec.ExecutorType)

	ts := rec.RequestedAt
	if ts.IsZero() {
		ts = time.Now()
	}
	clientMeta := internallogging.GetClientRequestMetadata(ctx)

	failed := rec.Failed
	status := rec.Fail.StatusCode
	if !failed {
		status = 200
	} else if status <= 0 {
		status = internallogging.GetResponseStatus(ctx)
		if status <= 0 {
			status = 500
		}
	}

	record := Record{
		TS:                  ts,
		RequestID:           strings.TrimSpace(internallogging.GetRequestID(ctx)),
		Provider:            normalizeEmpty(rec.Provider, "unknown"),
		ExecutorType:        strings.TrimSpace(rec.ExecutorType),
		Model:               normalizeEmpty(rec.Model, "unknown"),
		Alias:               normalizeEmpty(rec.Alias, normalizeEmpty(rec.Model, "unknown")),
		Endpoint:            strings.TrimSpace(internallogging.GetEndpoint(ctx)),
		AuthType:            strings.TrimSpace(rec.AuthType),
		AuthID:              strings.TrimSpace(rec.AuthID),
		AuthIndex:           strings.TrimSpace(rec.AuthIndex),
		APIKey:              strings.TrimSpace(rec.APIKey),
		Source:              strings.TrimSpace(rec.Source),
		ReasoningEffort:     strings.TrimSpace(rec.ReasoningEffort),
		ServiceTier:         strings.TrimSpace(rec.ServiceTier),
		ResponseServiceTier: strings.TrimSpace(firstNonEmpty(rec.ResponseServiceTier, detail.ResponseServiceTier)),
		ClientIP:            clientMeta.ClientIP,
		UserAgent:           clientMeta.UserAgent,
		LatencyMs:           rec.Latency.Milliseconds(),
		TTFtMs:              rec.TTFT.Milliseconds(),
		Failed:              failed,
		Generate:            coreusage.GenerateEnabled(rec.Generate),
		FailStatus:          status,
		FailBody:            strings.TrimSpace(rec.Fail.Body),
		InputTokens:         detail.InputTokens,
		OutputTokens:        detail.OutputTokens,
		ReasoningTokens:     detail.ReasoningTokens,
		CachedTokens:        detail.CachedTokens,
		CacheReadTokens:     detail.CacheReadTokens,
		CacheCreationTokens: detail.CacheCreationTokens,
		TotalTokens:         detail.TotalTokens,
	}

	insertCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := store.Insert(insertCtx, record); err != nil {
		log.WithError(err).Debug("usagestore: failed to persist usage record")
	}
}

func normalizeEmpty(v, fallback string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return fallback
	}
	return v
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
