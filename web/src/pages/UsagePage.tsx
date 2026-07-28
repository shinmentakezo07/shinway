/**
 * Usage page: token analytics dashboard + per-request logs.
 */

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useHeaderRefresh } from '@/hooks/useHeaderRefresh';
import { useAuthStore } from '@/stores';
import {
  usageApi,
  type UsageGroupStat,
  type UsagePoint,
  type UsageRange,
  type UsageRecord,
  type UsageStatsResponse,
} from '@/services/api';
import { Card } from '@/components/ui/Card';
import { Button } from '@/components/ui/Button';
import { Input } from '@/components/ui/Input';
import { LoadingSpinner } from '@/components/ui/LoadingSpinner';
import { EmptyState } from '@/components/ui/EmptyState';
import {
  IconAlertTriangle,
  IconRefreshCw,
  IconSearch,
  IconChevronLeft,
  IconChevronRight,
} from '@/components/ui/icons';
import styles from './UsagePage.module.scss';

const RANGES: UsageRange[] = ['1h', '6h', '24h', '7d', '30d', 'all'];
const PAGE_SIZE = 50;

function formatNumber(n: number): string {
  if (!Number.isFinite(n)) return '0';
  if (Math.abs(n) >= 1_000_000_000) return (n / 1_000_000_000).toFixed(2) + 'B';
  if (Math.abs(n) >= 1_000_000) return (n / 1_000_000).toFixed(2) + 'M';
  if (Math.abs(n) >= 1_000) return (n / 1_000).toFixed(2) + 'K';
  return n.toLocaleString();
}

function formatMs(ms: number): string {
  if (!ms) return '0ms';
  if (ms < 1000) return `${Math.round(ms)}ms`;
  return `${(ms / 1000).toFixed(2)}s`;
}

function formatTs(iso: string): string {
  if (!iso) return '-';
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
}

function maskKey(key: string): string {
  if (!key) return '(none)';
  if (key.length <= 8) return key;
  return `${key.slice(0, 4)}…${key.slice(-4)}`;
}

interface BarChartProps {
  data: { label: string; value: number; color?: string }[];
}

function BarChart({ data }: BarChartProps) {
  const max = Math.max(1, ...data.map((d) => d.value));
  return (
    <div className={styles.barChart}>
      {data.map((d, i) => (
        <div key={i} className={styles.barRow}>
          <div className={styles.barLabel} title={d.label}>
            {d.label}
          </div>
          <div className={styles.barTrack}>
            <div
              className={styles.barFill}
              style={{
                width: `${Math.min(100, (d.value / max) * 100)}%`,
                backgroundColor: d.color,
              }}
            />
          </div>
          <div className={styles.barValue}>{formatNumber(d.value)}</div>
        </div>
      ))}
    </div>
  );
}

interface LineChartProps {
  points: UsagePoint[];
  bucketSeconds: number;
  t: (key: string) => string;
}

function LineChart({ points, bucketSeconds, t }: LineChartProps) {
  if (!points.length) return null;

  const W = 800;
  const H = 240;
  const PAD_X = 50;
  const PAD_Y = 30;

  const maxTokens = Math.max(1, ...points.map((p) => p.total_tokens));
  const maxRequests = Math.max(1, ...points.map((p) => p.requests));

  const x = (i: number) => PAD_X + (i / Math.max(1, points.length - 1)) * (W - PAD_X * 2);
  const yTokens = (v: number) => H - PAD_Y - (v / maxTokens) * (H - PAD_Y * 2);
  const yReq = (v: number) => H - PAD_Y - (v / maxRequests) * (H - PAD_Y * 2);

  const path = (values: number[], toY: (v: number) => number) =>
    values
      .map((v, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${toY(v).toFixed(1)}`)
      .join(' ');

  const totalPath = path(
    points.map((p) => p.total_tokens),
    yTokens
  );
  const inputPath = path(
    points.map((p) => p.input_tokens),
    yTokens
  );
  const outputPath = path(
    points.map((p) => p.output_tokens),
    yTokens
  );
  const requestsPath = path(
    points.map((p) => p.requests),
    yReq
  );

  const bucketMs = (iso: string) => {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return '';
    if (bucketSeconds >= 86400) return d.toLocaleDateString();
    return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
  };

  return (
    <div className={styles.lineChartWrap}>
      <svg viewBox={`0 0 ${W} ${H + 30}`} className={styles.lineChart}>
        {/* grid */}
        {[0, 0.25, 0.5, 0.75, 1].map((f) => {
          const y = PAD_Y + f * (H - PAD_Y * 2);
          const value = maxTokens * (1 - f);
          return (
            <g key={f}>
              <line x1={PAD_X} x2={W - PAD_X} y1={y} y2={y} className={styles.gridLine} />
              <text x={8} y={y + 4} className={styles.gridLabel}>
                {formatNumber(value)}
              </text>
            </g>
          );
        })}
        {/* lines */}
        <path d={totalPath} className={styles.lineTotal} fill="none" />
        <path d={inputPath} className={styles.lineInput} fill="none" />
        <path d={outputPath} className={styles.lineOutput} fill="none" />
        <path d={requestsPath} className={styles.lineRequests} fill="none" strokeDasharray="4 3" />
        {/* x labels */}
        {points.map((p, i) =>
          i % Math.ceil(points.length / 8) === 0 ? (
            <text key={i} x={x(i)} y={H + 18} className={styles.gridLabel} textAnchor="middle">
              {bucketMs(p.bucket)}
            </text>
          ) : null
        )}
        {/* dots */}
        {points.map((p, i) => (
          <circle key={i} cx={x(i)} cy={yTokens(p.total_tokens)} r={2.5} className={styles.lineTotal} />
        ))}
      </svg>
      <div className={styles.legend}>
        <span>
          <span className={`${styles.legendDot} ${styles.legendTotal}`} /> {t('usage.legend_total')}
        </span>
        <span>
          <span className={`${styles.legendDot} ${styles.legendInput}`} /> {t('usage.legend_input')}
        </span>
        <span>
          <span className={`${styles.legendDot} ${styles.legendOutput}`} /> {t('usage.legend_output')}
        </span>
        <span>
          <span className={`${styles.legendDot} ${styles.legendRequests}`} /> {t('usage.legend_requests')}
        </span>
      </div>
    </div>
  );
}

interface StatCardProps {
  label: string;
  value: string;
  hint?: string;
  accent?: 'default' | 'success' | 'error' | 'warning';
}

function StatCard({ label, value, hint, accent = 'default' }: StatCardProps) {
  return (
    <div className={`${styles.statCard} ${styles[`accent-${accent}`] || ''}`}>
      <div className={styles.statLabel}>{label}</div>
      <div className={styles.statValue}>{value}</div>
      {hint ? <div className={styles.statHint}>{hint}</div> : null}
    </div>
  );
}

export function UsagePage() {
  const { t } = useTranslation();
  const connectionStatus = useAuthStore((state) => state.connectionStatus);

  const [range, setRange] = useState<UsageRange>('24h');
  const [stats, setStats] = useState<UsageStatsResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const [records, setRecords] = useState<UsageRecord[]>([]);
  const [recordsTotal, setRecordsTotal] = useState(0);
  const [recordsOffset, setRecordsOffset] = useState(0);
  const [recordsLoading, setRecordsLoading] = useState(true);
  const [recordsError, setRecordsError] = useState('');
  const [recordSearch, setRecordSearch] = useState('');
  const [recordFailedOnly, setRecordFailedOnly] = useState(false);
  const [activeDimension, setActiveDimension] = useState<'model' | 'provider' | 'api_key' | 'auth'>('model');

  const disabled = connectionStatus !== 'connected';

  const loadStats = useCallback(async () => {
    setLoading(true);
    setError('');
    try {
      const data = await usageApi.getStats(range);
      setStats(data);
    } catch (err) {
      setError(err instanceof Error ? err.message : t('notification.refresh_failed'));
      setStats(null);
    } finally {
      setLoading(false);
    }
  }, [range, t]);

  const loadRecords = useCallback(
    async (offset: number) => {
      setRecordsLoading(true);
      setRecordsError('');
      try {
        const response = await usageApi.getRecords({
          range,
          search: recordSearch || undefined,
          failed: recordFailedOnly ? true : undefined,
          limit: PAGE_SIZE,
          offset,
        });
        setRecords(response.records || []);
        setRecordsTotal(response.total || 0);
        setRecordsOffset(offset);
      } catch (err) {
        setRecordsError(err instanceof Error ? err.message : t('notification.refresh_failed'));
        setRecords([]);
        setRecordsTotal(0);
      } finally {
        setRecordsLoading(false);
      }
    },
    [range, recordSearch, recordFailedOnly, t]
  );

  useEffect(() => {
    loadStats();
  }, [loadStats]);

  useEffect(() => {
    loadRecords(0);
  }, [loadRecords]);

  useHeaderRefresh(
    useCallback(() => {
      void loadStats();
      void loadRecords(recordsOffset);
    }, [loadStats, loadRecords, recordsOffset])
  );

  const dimensionData = useMemo(() => {
    if (!stats) return [];
    const pick = (data: UsageGroupStat[] | undefined) => data || [];
    switch (activeDimension) {
      case 'provider':
        return pick(stats.by_provider);
      case 'api_key':
        return pick(stats.by_api_key);
      case 'auth':
        return pick(stats.by_auth);
      case 'model':
      default:
        return pick(stats.by_model);
    }
  }, [stats, activeDimension]);

  const successRate =
    stats && stats.summary.total_requests > 0
      ? ((stats.summary.succeeded_requests / stats.summary.total_requests) * 100).toFixed(1)
      : '-';

  return (
    <div className={styles.container}>
      <div className={styles.pageHeaderRow}>
        <div>
          <h1 className={styles.pageTitle}>{t('usage.title')}</h1>
          <p className={styles.description}>{t('usage.description')}</p>
        </div>
        <div className={styles.controls}>
          <div className={styles.rangeButtons}>
            {RANGES.map((r) => (
              <button
                key={r}
                type="button"
                className={`${styles.rangeButton} ${range === r ? styles.rangeButtonActive : ''}`}
                onClick={() => setRange(r)}
                disabled={disabled}
              >
                {t(`usage.range_${r}`)}
              </button>
            ))}
          </div>
          <Button
            variant="secondary"
            size="sm"
            onClick={() => {
              void loadStats();
              void loadRecords(recordsOffset);
            }}
            disabled={disabled || loading}
          >
            <IconRefreshCw size={14} />
          </Button>
        </div>
      </div>

      {error && (
        <div className={styles.errorBox}>
          <IconAlertTriangle size={16} /> {error}
        </div>
      )}

      {loading && !stats ? (
        <div className={styles.loadingWrap}>
          <LoadingSpinner />
        </div>
      ) : stats ? (
        <>
          {/* Top-level metric cards */}
          <div className={styles.statGrid}>
            <StatCard
              label={t('usage.stat_total_requests')}
              value={formatNumber(stats.summary.total_requests)}
              hint={`${formatNumber(stats.summary.succeeded_requests)} ${t('usage.stat_ok')}`}
            />
            <StatCard
              label={t('usage.stat_failed')}
              value={formatNumber(stats.summary.failed_requests)}
              hint={`${successRate}% ${t('usage.stat_success_rate')}`}
              accent={stats.summary.failed_requests > 0 ? 'error' : 'default'}
            />
            <StatCard
              label={t('usage.stat_total_tokens')}
              value={formatNumber(stats.summary.total_tokens)}
              hint={`${formatNumber(stats.summary.total_input_tokens)} in · ${formatNumber(stats.summary.total_output_tokens)} out`}
            />
            <StatCard
              label={t('usage.stat_cached')}
              value={formatNumber(stats.summary.total_cached_tokens)}
              hint={t('usage.stat_cached_hint')}
              accent="success"
            />
            <StatCard
              label={t('usage.stat_reasoning')}
              value={formatNumber(stats.summary.total_reasoning_tokens)}
            />
            <StatCard
              label={t('usage.stat_avg_latency')}
              value={formatMs(stats.summary.avg_latency_ms)}
              hint={`TTFT ${formatMs(stats.summary.avg_ttft_ms)}`}
            />
            <StatCard
              label={t('usage.stat_models')}
              value={String(stats.summary.unique_models)}
            />
            <StatCard
              label={t('usage.stat_api_keys')}
              value={String(stats.summary.unique_api_keys)}
              hint={`${stats.summary.unique_auth_files} ${t('usage.stat_auths')}`}
            />
          </div>

          {/* Time-series chart */}
          <Card title={t('usage.chart_title')}>
            {stats.series && stats.series.length > 0 ? (
              <LineChart points={stats.series} bucketSeconds={stats.bucket_seconds} t={t} />
            ) : (
              <EmptyState title={t('usage.no_data')} />
            )}
          </Card>

          {/* Per-dimension breakdown */}
          <Card
            title={t('usage.breakdown_title')}
            extra={
              <div className={styles.rangeButtons}>
                {(['model', 'provider', 'api_key', 'auth'] as const).map((dim) => (
                  <button
                    key={dim}
                    type="button"
                    className={`${styles.rangeButton} ${activeDimension === dim ? styles.rangeButtonActive : ''}`}
                    onClick={() => setActiveDimension(dim)}
                  >
                    {t(`usage.dim_${dim}`)}
                  </button>
                ))}
              </div>
            }
          >
            {dimensionData.length === 0 ? (
              <EmptyState title={t('usage.no_data')} />
            ) : (
              <>
                <BarChart
                  data={dimensionData.map((d) => ({
                    label: activeDimension === 'api_key' ? maskKey(d.key) : d.key,
                    value: d.total_tokens,
                  }))}
                />
                <div className={styles.dimTableWrap}>
                  <table className={styles.dimTable}>
                    <thead>
                      <tr>
                        <th>{t('usage.col_name')}</th>
                        <th>{t('usage.col_requests')}</th>
                        <th>{t('usage.col_failed')}</th>
                        <th>{t('usage.col_input')}</th>
                        <th>{t('usage.col_output')}</th>
                        <th>{t('usage.col_reasoning')}</th>
                        <th>{t('usage.col_cached')}</th>
                        <th>{t('usage.col_total')}</th>
                        <th>{t('usage.col_avg_latency')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {dimensionData.map((d, i) => (
                        <tr key={i}>
                          <td className={styles.keyCell} title={d.key}>
                            {activeDimension === 'api_key' ? maskKey(d.key) : d.key}
                          </td>
                          <td>{formatNumber(d.requests)}</td>
                          <td className={d.failed > 0 ? styles.failedCell : ''}>
                            {formatNumber(d.failed)}
                          </td>
                          <td>{formatNumber(d.input_tokens)}</td>
                          <td>{formatNumber(d.output_tokens)}</td>
                          <td>{formatNumber(d.reasoning_tokens)}</td>
                          <td>{formatNumber(d.cached_tokens)}</td>
                          <td className={styles.totalCell}>{formatNumber(d.total_tokens)}</td>
                          <td>{formatMs(d.avg_latency_ms)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </>
            )}
          </Card>

          {/* Request log */}
          <Card
            title={t('usage.records_title')}
            extra={
              <div className={styles.recordsControls}>
                <div className={styles.searchWrap}>
                  <Input
                    placeholder={t('usage.search_placeholder')}
                    value={recordSearch}
                    onChange={(e) => setRecordSearch(e.target.value)}
                  />
                  <IconSearch size={14} />
                </div>
                <label className={styles.checkboxLabel}>
                  <input
                    type="checkbox"
                    checked={recordFailedOnly}
                    onChange={(e) => setRecordFailedOnly(e.target.checked)}
                  />
                  {t('usage.failed_only')}
                </label>
              </div>
            }
          >
            {recordsError && <div className={styles.errorBox}>{recordsError}</div>}
            {recordsLoading ? (
              <div className={styles.loadingWrap}>
                <LoadingSpinner />
              </div>
            ) : records.length === 0 ? (
              <EmptyState title={t('usage.no_records')} />
            ) : (
              <>
                <div className={styles.recordsTableWrap}>
                  <table className={styles.recordsTable}>
                    <thead>
                      <tr>
                        <th>{t('usage.col_time')}</th>
                        <th>{t('usage.col_model')}</th>
                        <th>{t('usage.col_provider')}</th>
                        <th>{t('usage.col_key')}</th>
                        <th>{t('usage.col_endpoint')}</th>
                        <th>{t('usage.col_tokens')}</th>
                        <th>{t('usage.col_latency')}</th>
                        <th>{t('usage.col_status')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {records.map((r, i) => (
                        <tr key={`${r.RequestID}-${i}`} className={r.Failed ? styles.failedRow : ''}>
                          <td className={styles.tsCell}>{formatTs(r.TS)}</td>
                          <td className={styles.keyCell} title={r.Alias !== r.Model ? `${r.Alias} → ${r.Model}` : r.Model}>
                            {r.Model}
                          </td>
                          <td>{r.Provider}</td>
                          <td className={styles.keyCell} title={r.APIKey}>
                            {maskKey(r.APIKey)}
                          </td>
                          <td className={styles.keyCell} title={r.Endpoint}>
                            {r.Endpoint || '-'}
                          </td>
                          <td>
                            <span className={styles.tokensCell} title={`in ${r.InputTokens} · out ${r.OutputTokens} · reasoning ${r.ReasoningTokens} · cached ${r.CachedTokens}`}>
                              {formatNumber(r.TotalTokens)}
                            </span>
                          </td>
                          <td>{formatMs(r.LatencyMs)}</td>
                          <td>
                            {r.Failed ? (
                              <span className={styles.statusFail} title={r.FailBody}>
                                {r.FailStatus || 'ERR'}
                              </span>
                            ) : (
                              <span className={styles.statusOk}>{r.FailStatus || 200}</span>
                            )}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
                <div className={styles.pagination}>
                  <Button
                    variant="secondary"
                    size="sm"
                    disabled={recordsOffset === 0 || recordsLoading}
                    onClick={() => loadRecords(Math.max(0, recordsOffset - PAGE_SIZE))}
                  >
                    <IconChevronLeft size={14} /> {t('usage.prev')}
                  </Button>
                  <span className={styles.paginationInfo}>
                    {recordsOffset + 1} - {Math.min(recordsOffset + records.length, recordsTotal)} /{' '}
                    {formatNumber(recordsTotal)}
                  </span>
                  <Button
                    variant="secondary"
                    size="sm"
                    disabled={recordsOffset + records.length >= recordsTotal || recordsLoading}
                    onClick={() => loadRecords(recordsOffset + PAGE_SIZE)}
                  >
                    {t('usage.next')} <IconChevronRight size={14} />
                  </Button>
                </div>
              </>
            )}
          </Card>
        </>
      ) : (
        <div className={styles.loadingWrap}>
          <LoadingSpinner />
        </div>
      )}
    </div>
  );
}
