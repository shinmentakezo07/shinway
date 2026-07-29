package usagestore

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// PostgresWriter mirrors usage records into a PostgreSQL table.
// It is intended as a durable secondary copy alongside the local SQLite store.
type PostgresWriter struct {
	db     *sql.DB
	table  string
	schema string
}

// NewPostgresWriter creates a writer that persists records to the given
// PostgreSQL connection. The usage_records table is created automatically.
func NewPostgresWriter(db *sql.DB, schema string) (*PostgresWriter, error) {
	if db == nil {
		return nil, fmt.Errorf("postgreswriter: nil db")
	}
	w := &PostgresWriter{
		db:     db,
		schema: strings.TrimSpace(schema),
		table:  "usage_records",
	}
	if w.schema != "" {
		w.table = fmt.Sprintf("%s.usage_records", quotePGIdentifier(w.schema))
	}
	if err := w.ensureTable(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *PostgresWriter) ensureTable() error {
	if w.schema != "" {
		schemaSQL := `CREATE SCHEMA IF NOT EXISTS ` + quotePGIdentifier(w.schema)
		if _, err := w.db.Exec(schemaSQL); err != nil {
			return fmt.Errorf("postgreswriter: create schema: %w", err)
		}
	}
	sql := `CREATE TABLE IF NOT EXISTS ` + w.table + ` (
		id BIGSERIAL PRIMARY KEY,
		ts BIGINT NOT NULL,
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
		latency_ms BIGINT NOT NULL DEFAULT 0,
		ttft_ms BIGINT NOT NULL DEFAULT 0,
		failed SMALLINT NOT NULL DEFAULT 0,
		generate SMALLINT NOT NULL DEFAULT 1,
		fail_status INTEGER NOT NULL DEFAULT 0,
		fail_body TEXT NOT NULL DEFAULT '',
		input_tokens BIGINT NOT NULL DEFAULT 0,
		output_tokens BIGINT NOT NULL DEFAULT 0,
		reasoning_tokens BIGINT NOT NULL DEFAULT 0,
		cached_tokens BIGINT NOT NULL DEFAULT 0,
		cache_read_tokens BIGINT NOT NULL DEFAULT 0,
		cache_creation_tokens BIGINT NOT NULL DEFAULT 0,
		total_tokens BIGINT NOT NULL DEFAULT 0
	)`
	if _, err := w.db.Exec(sql); err != nil {
		return fmt.Errorf("postgreswriter: create table: %w", err)
	}
	indexes := []string{
		`CREATE INDEX IF NOT EXISTS idx_usage_records_ts ON ` + w.table + `(ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_records_model_ts ON ` + w.table + `(model, ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_records_provider_ts ON ` + w.table + `(provider, ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_records_api_key_ts ON ` + w.table + `(api_key, ts DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_usage_records_failed_ts ON ` + w.table + `(failed, ts DESC)`,
	}
	for _, idx := range indexes {
		if _, err := w.db.Exec(idx); err != nil {
			return fmt.Errorf("postgreswriter: create index: %w", err)
		}
	}
	return nil
}

// Write inserts the record into the Postgres usage_records table.
// It is safe for concurrent use; it uses the caller-supplied context directly
// (which already carries a deadline from the caller if needed).
func (w *PostgresWriter) Write(ctx context.Context, r Record) error {
	if w == nil || w.db == nil {
		return fmt.Errorf("postgreswriter: closed")
	}
	failed := 0
	if r.Failed {
		failed = 1
	}
	generate := 0
	if r.Generate {
		generate = 1
	}
	_, err := w.db.ExecContext(ctx,
		`INSERT INTO `+w.table+` (
			ts, request_id, provider, executor_type, model, alias, endpoint,
			auth_type, auth_id, auth_index, api_key, source,
			reasoning_effort, service_tier, response_service_tier,
			client_ip, user_agent,
			latency_ms, ttft_ms, failed, generate, fail_status, fail_body,
			input_tokens, output_tokens, reasoning_tokens, cached_tokens,
			cache_read_tokens, cache_creation_tokens, total_tokens
		) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24,$25,$26,$27,$28,$29,$30)`,
		r.TS.UnixMilli(), r.RequestID, r.Provider, r.ExecutorType, r.Model, r.Alias, r.Endpoint,
		r.AuthType, r.AuthID, r.AuthIndex, r.APIKey, r.Source,
		r.ReasoningEffort, r.ServiceTier, r.ResponseServiceTier,
		r.ClientIP, r.UserAgent,
		r.LatencyMs, r.TTFtMs, failed, generate, r.FailStatus, truncate(r.FailBody, 4096),
		r.InputTokens, r.OutputTokens, r.ReasoningTokens, r.CachedTokens,
		r.CacheReadTokens, r.CacheCreationTokens, r.TotalTokens,
	)
	if err != nil {
		return fmt.Errorf("postgreswriter: insert: %w", err)
	}
	return nil
}

// Close is a no-op for PostgresWriter; the underlying database connection is
// managed by the caller.
func (w *PostgresWriter) Close() error {
	return nil
}

func quotePGIdentifier(id string) string {
	replaced := strings.ReplaceAll(id, `"`, `""`)
	return `"` + replaced + `"`
}
