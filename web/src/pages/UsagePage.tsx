/**
 * Usage page: token analytics dashboard + per-request logs.
 */

import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ComponentType,
  type ReactNode,
} from 'react';
import { useTranslation } from 'react-i18next';
import { useHeaderRefresh } from '@/hooks/useHeaderRefresh';
import { useAuthStore, useUsagePrefsStore } from '@/stores';
import { estimateRequestCost, formatUsd } from '@/utils/modelPricing';
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
import { ToggleSwitch } from '@/components/ui/ToggleSwitch';
import {
  IconAlertTriangle,
  IconRefreshCw,
  IconSearch,
  IconChevronLeft,
  IconChevronRight,
  IconCheckCircle2,
  IconChart,
  IconBot,
  IconTimer,
  IconModelCluster,
  IconKey,
  IconNetwork,
  IconPieChart,
  IconFlame,
  IconActivity,
  IconScrollText,
  IconFilterAll,
} from '@/components/ui/icons';
import styles from './UsagePage.module.scss';

const RANGES: UsageRange[] = ['1h', '6h', '24h', '7d', '30d', 'all'];
const PAGE_SIZE = 50;
const BAR_COLORS = ['#3b82f6', '#10b981', '#a855f7', '#f59e0b', '#ef4444', '#06b6d4'];

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

function formatShortTs(iso: string): string {
  if (!iso) return '-';
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
}

/** True when the backend returned a zero-value (unbounded) timestamp. */
function isZeroTs(iso: string): boolean {
  if (!iso) return true;
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) || d.getFullYear() < 1970;
}

/** Classify latency into a tone for the pill in the request log. */
function latencyTone(ms: number): 'fast' | 'mid' | 'slow' {
  if (ms < 2000) return 'fast';
  if (ms < 8000) return 'mid';
  return 'slow';
}

function maskKey(key: string): string {
  if (!key) return '(none)';
  if (key.length <= 8) return key;
  return `${key.slice(0, 4)}…${key.slice(-4)}`;
}

function prefersReducedMotion(): boolean {
  return typeof window !== 'undefined' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
}

// easeOutCubic for a snappy deceleration on count-ups.
const easeOutCubic = (x: number): number => 1 - Math.pow(1 - x, 3);

/**
 * Smoothly counts a numeric stat towards its target so metric cards feel
 * alive when data loads or the range changes. Falls back to an instant
 * render when the user prefers reduced motion.
 */
function useCountUp(target: number, format: (n: number) => string): string {
  const [display, setDisplay] = useState(() => format(0));
  const currentRef = useRef(0);

  useEffect(() => {
    if (prefersReducedMotion() || !Number.isFinite(target)) {
      currentRef.current = target;
      setDisplay(format(target));
      return;
    }
    const from = currentRef.current;
    if (from === target) {
      setDisplay(format(target));
      return;
    }
    let raf = 0;
    const duration = 700;
    const start = performance.now();
    const tick = (now: number) => {
      const t = Math.min(1, (now - start) / duration);
      const value = from + (target - from) * easeOutCubic(t);
      currentRef.current = value;
      setDisplay(format(value));
      if (t < 1) {
        raf = requestAnimationFrame(tick);
      } else {
        currentRef.current = target;
        setDisplay(format(target));
      }
    };
    raf = requestAnimationFrame(tick);
    return () => cancelAnimationFrame(raf);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [target]);

  return display;
}

interface BarChartProps {
  data: { label: string; value: number; color?: string }[];
}

function BarChart({ data }: BarChartProps) {
  const max = Math.max(1, ...data.map((d) => d.value));
  const total = Math.max(
    1,
    data.reduce((sum, d) => sum + d.value, 0)
  );
  return (
    <div className={styles.barChart}>
      {data.map((d, i) => {
        const color = BAR_COLORS[i % BAR_COLORS.length];
        const pct = (d.value / total) * 100;
        return (
          <div key={i} className={styles.barRow}>
            <div className={styles.barLabel} title={d.label}>
              <span className={styles.barDot} style={{ backgroundColor: color }} />
              {d.label}
            </div>
            <div className={styles.barTrack}>
              <div
                className={styles.barFill}
                style={{
                  width: `${Math.min(100, (d.value / max) * 100)}%`,
                  background: `linear-gradient(90deg, ${color}, ${color}99)`,
                }}
              />
            </div>
            <div className={styles.barValue}>{formatNumber(d.value)}</div>
            <div className={styles.barPct}>{pct.toFixed(1)}%</div>
          </div>
        );
      })}
    </div>
  );
}

interface LineChartProps {
  points: UsagePoint[];
  bucketSeconds: number;
  t: (key: string) => string;
}

function LineChart({ points, bucketSeconds, t }: LineChartProps) {
  const [hoverIdx, setHoverIdx] = useState<number | null>(null);

  if (!points.length) return null;

  const W = 900;
  const H = 260;
  const PAD_X = 56;
  const PAD_TOP = 24;
  const PAD_BOTTOM = 34;

  const maxTokens = Math.max(1, ...points.map((p) => p.total_tokens));
  const maxRequests = Math.max(1, ...points.map((p) => p.requests));

  const x = (i: number) => PAD_X + (i / Math.max(1, points.length - 1)) * (W - PAD_X * 2);
  const yTokens = (v: number) => H - PAD_BOTTOM - (v / maxTokens) * (H - PAD_TOP - PAD_BOTTOM);
  const yReq = (v: number) => H - PAD_BOTTOM - (v / maxRequests) * (H - PAD_TOP - PAD_BOTTOM);

  const line = (values: number[], toY: (v: number) => number) =>
    values.map((v, i) => `${i === 0 ? 'M' : 'L'}${x(i).toFixed(1)},${toY(v).toFixed(1)}`).join(' ');

  // Catmull-Rom -> cubic bezier smoothing for a softer, more organic curve.
  const smoothLine = (values: number[], toY: (v: number) => number) => {
    if (values.length < 3) return line(values, toY);
    const pts = values.map((v, i) => [x(i), toY(v)] as const);
    let d = `M${pts[0][0].toFixed(1)},${pts[0][1].toFixed(1)}`;
    for (let i = 0; i < pts.length - 1; i++) {
      const p0 = pts[Math.max(0, i - 1)];
      const p1 = pts[i];
      const p2 = pts[i + 1];
      const p3 = pts[Math.min(pts.length - 1, i + 2)];
      const c1x = p1[0] + (p2[0] - p0[0]) / 6;
      const c1y = p1[1] + (p2[1] - p0[1]) / 6;
      const c2x = p2[0] - (p3[0] - p1[0]) / 6;
      const c2y = p2[1] - (p3[1] - p1[1]) / 6;
      d += ` C${c1x.toFixed(1)},${c1y.toFixed(1)} ${c2x.toFixed(1)},${c2y.toFixed(1)} ${p2[0].toFixed(1)},${p2[1].toFixed(1)}`;
    }
    return d;
  };

  const area = (values: number[], toY: (v: number) => number) =>
    `${smoothLine(values, toY)} L${x(values.length - 1).toFixed(1)},${H - PAD_BOTTOM} L${x(0).toFixed(1)},${H - PAD_BOTTOM} Z`;

  const totals = points.map((p) => p.total_tokens);
  const inputs = points.map((p) => p.input_tokens);
  const outputs = points.map((p) => p.output_tokens);
  const requests = points.map((p) => p.requests);

  const bucketLabel = (iso: string) => {
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return '';
    if (bucketSeconds >= 86400) return d.toLocaleDateString();
    return d.toLocaleTimeString(undefined, { hour: '2-digit', minute: '2-digit' });
  };

  const bucketFull = (iso: string) => {
    const d = new Date(iso);
    return Number.isNaN(d.getTime()) ? iso : d.toLocaleString();
  };

  const hovered = hoverIdx !== null ? points[hoverIdx] : null;

  return (
    <div className={styles.lineChartWrap}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        className={styles.lineChart}
        onMouseLeave={() => setHoverIdx(null)}
        onMouseMove={(e) => {
          const rect = e.currentTarget.getBoundingClientRect();
          const relX = ((e.clientX - rect.left) / rect.width) * W;
          const usable = W - PAD_X * 2;
          const frac = (relX - PAD_X) / Math.max(1, usable);
          const idx = Math.round(frac * (points.length - 1));
          setHoverIdx(Math.max(0, Math.min(points.length - 1, idx)));
        }}
      >
        <defs>
          <linearGradient id="usageAreaGradient" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" className={styles.areaStopTop} />
            <stop offset="100%" className={styles.areaStopBottom} />
          </linearGradient>
        </defs>
        {/* grid */}
        {[0, 0.25, 0.5, 0.75, 1].map((f) => {
          const y = PAD_TOP + f * (H - PAD_TOP - PAD_BOTTOM);
          const value = maxTokens * (1 - f);
          return (
            <g key={f}>
              <line x1={PAD_X} x2={W - PAD_X} y1={y} y2={y} className={styles.gridLine} />
              <text x={PAD_X - 8} y={y + 4} textAnchor="end" className={styles.gridLabel}>
                {formatNumber(value)}
              </text>
            </g>
          );
        })}
        {/* area + lines */}
        <path d={area(totals, yTokens)} fill="url(#usageAreaGradient)" stroke="none" />
        <path d={smoothLine(totals, yTokens)} className={styles.lineTotal} fill="none" />
        <path d={smoothLine(inputs, yTokens)} className={styles.lineInput} fill="none" />
        <path d={smoothLine(outputs, yTokens)} className={styles.lineOutput} fill="none" />
        <path
          d={smoothLine(requests, yReq)}
          className={styles.lineRequests}
          fill="none"
          strokeDasharray="4 3"
        />
        {/* x labels */}
        {points.map((p, i) =>
          i % Math.ceil(points.length / 9) === 0 ? (
            <text key={i} x={x(i)} y={H - 12} className={styles.gridLabel} textAnchor="middle">
              {bucketLabel(p.bucket)}
            </text>
          ) : null
        )}
        {/* hover crosshair */}
        {hovered && hoverIdx !== null && (
          <g className={styles.crosshair}>
            <line
              x1={x(hoverIdx)}
              x2={x(hoverIdx)}
              y1={PAD_TOP}
              y2={H - PAD_BOTTOM}
              className={styles.crosshairLine}
            />
            <circle
              cx={x(hoverIdx)}
              cy={yTokens(hovered.total_tokens)}
              r={4.5}
              className={styles.dotTotal}
            />
            <circle
              cx={x(hoverIdx)}
              cy={yTokens(hovered.input_tokens)}
              r={3.5}
              className={styles.dotInput}
            />
            <circle
              cx={x(hoverIdx)}
              cy={yTokens(hovered.output_tokens)}
              r={3.5}
              className={styles.dotOutput}
            />
            <circle
              cx={x(hoverIdx)}
              cy={yReq(hovered.requests)}
              r={3.5}
              className={styles.dotRequests}
            />
          </g>
        )}
      </svg>
      <div className={styles.chartFooter}>
        <div className={styles.legend}>
          <span className={styles.legendItem}>
            <span className={`${styles.legendDot} ${styles.legendTotal}`} />{' '}
            {t('usage.legend_total')}
          </span>
          <span className={styles.legendItem}>
            <span className={`${styles.legendDot} ${styles.legendInput}`} />{' '}
            {t('usage.legend_input')}
          </span>
          <span className={styles.legendItem}>
            <span className={`${styles.legendDot} ${styles.legendOutput}`} />{' '}
            {t('usage.legend_output')}
          </span>
          <span className={styles.legendItem}>
            <span className={`${styles.legendDot} ${styles.legendRequests}`} />{' '}
            {t('usage.legend_requests')}
          </span>
        </div>
        <div className={`${styles.tooltip} ${hovered ? styles.tooltipVisible : ''}`}>
          {hovered ? (
            <>
              <span className={styles.tooltipTime}>{bucketFull(hovered.bucket)}</span>
              <span className={styles.tooltipRow}>
                <span className={`${styles.legendDot} ${styles.legendTotal}`} />
                {formatNumber(hovered.total_tokens)}
              </span>
              <span className={styles.tooltipRow}>
                <span className={`${styles.legendDot} ${styles.legendInput}`} />
                {formatNumber(hovered.input_tokens)}
              </span>
              <span className={styles.tooltipRow}>
                <span className={`${styles.legendDot} ${styles.legendOutput}`} />
                {formatNumber(hovered.output_tokens)}
              </span>
              <span className={styles.tooltipRow}>
                <span className={`${styles.legendDot} ${styles.legendRequests}`} />
                {formatNumber(hovered.requests)}
              </span>
            </>
          ) : (
            <span className={styles.tooltipHint}>&nbsp;</span>
          )}
        </div>
      </div>
    </div>
  );
}

interface StatCardProps {
  label: string;
  value: ReactNode;
  hint?: ReactNode;
  tone?: 'default' | 'success' | 'error' | 'info' | 'accent' | 'amber';
  icon?: ComponentType<{ size?: number }>;
}

function StatCard({ label, value, hint, tone = 'default', icon: Icon }: StatCardProps) {
  return (
    <div className={`${styles.statCard} ${styles[`tone-${tone}`] || ''}`}>
      <div className={styles.statCardBg} />
      <div className={styles.statTop}>
        <span className={styles.statLabel}>{label}</span>
        {Icon && (
          <span className={styles.statIcon}>
            <Icon size={15} />
          </span>
        )}
      </div>
      <div className={styles.statValue}>{value}</div>
      {hint ? <div className={styles.statHint}>{hint}</div> : null}
    </div>
  );
}

/** Animated numeric stat value that counts up towards its target. */
function AnimatedNumber({
  value,
  format,
}: {
  value: number;
  format: (n: number) => string;
}) {
  const display = useCountUp(value, format);
  return <>{display}</>;
}

const formatInt = (n: number): string => Math.round(n).toLocaleString();

/** Card heading with a leading icon for visual rhythm across sections. */
function SectionTitle({
  icon: Icon,
  children,
}: {
  icon: ComponentType<{ size?: number }>;
  children: ReactNode;
}) {
  return (
    <span className={styles.sectionTitle}>
      <span className={styles.sectionTitleIcon}>
        <Icon size={15} />
      </span>
      {children}
    </span>
  );
}

/** Numeric token breakdown (input/output/reasoning/cached) for one request. */
function TokenNumbers({
  input,
  output,
  reasoning,
  cached,
}: {
  input: number;
  output: number;
  reasoning: number;
  cached: number;
}) {
  const { t } = useTranslation();
  const items = [
    { key: 'input', label: t('usage.legend_input'), value: input, cls: styles.tnInput },
    { key: 'output', label: t('usage.legend_output'), value: output, cls: styles.tnOutput },
    { key: 'reasoning', label: t('usage.legend_reasoning'), value: reasoning, cls: styles.tnReasoning },
    { key: 'cached', label: t('usage.legend_cached'), value: cached, cls: styles.tnCached },
  ];
  const sum = items.reduce((s, it) => s + Math.max(0, it.value), 0);
  return (
    <span className={styles.tokenNums}>
      {sum > 0 && (
        <span className={styles.tokenStack} aria-hidden="true">
          {items.map((it) =>
            it.value > 0 ? (
              <span
                key={it.key}
                className={`${styles.tokenStackSeg} ${it.cls}`}
                style={{ width: `${(Math.max(0, it.value) / sum) * 100}%` }}
              />
            ) : null
          )}
        </span>
      )}
      <span className={styles.tokenChips}>
        {items.map((it) => (
          <span
            key={it.key}
            className={`${styles.tokenChip} ${it.cls} ${it.value <= 0 ? styles.tokenChipZero : ''}`}
            title={`${it.label}: ${Math.max(0, it.value).toLocaleString()}`}
          >
            <span className={styles.tokenChipDot} aria-hidden="true" />
            {formatNumber(it.value)}
          </span>
        ))}
      </span>
    </span>
  );
}

/** Estimated USD cost for one request, flagged when the rate is a fallback. */
function CostCell({ record }: { record: UsageRecord }) {
  const { t } = useTranslation();
  const estimate = estimateRequestCost(
    record.Model,
    record.InputTokens,
    record.OutputTokens,
    record.ReasoningTokens
  );
  if (estimate.free) {
    return <span className={styles.costFree}>{t('usage.cost_free')}</span>;
  }
  const label = `${estimate.estimated ? '~' : ''}${formatUsd(estimate.cost)}`;
  const title = estimate.estimated
    ? t('usage.cost_estimated_hint')
    : t('usage.cost_hint');
  return (
    <span
      className={`${styles.costValue} ${estimate.estimated ? styles.costEstimated : ''}`}
      title={title}
    >
      {label}
    </span>
  );
}

interface DonutProps {
  segments: { label: string; value: number; color: string }[];
  centerLabel: string;
  centerValue: string;
}

function DonutChart({ segments, centerLabel, centerValue }: DonutProps) {
  const total = Math.max(
    1,
    segments.reduce((s, x) => s + x.value, 0)
  );
  const R = 54;
  const CIRC = 2 * Math.PI * R;

  // Precompute cumulative fractions to avoid mutation during render
  const slices = segments.map((s, i) => ({
    frac: s.value / total,
    offset: (-segments.slice(0, i).reduce((a, x) => a + x.value, 0) / total) * CIRC,
    color: s.color,
    label: s.label,
  }));

  return (
    <div className={styles.donutWrap}>
      <div className={styles.donutSvgWrap}>
        <svg viewBox="0 0 140 140" className={styles.donutSvg}>
          <circle cx="70" cy="70" r={R} className={styles.donutTrack} />
          {slices.map((sl, i) =>
            sl.frac <= 0 ? null : (
              <circle
                key={i}
                cx="70"
                cy="70"
                r={R}
                className={styles.donutSegment}
                stroke={sl.color}
                strokeDasharray={`${Math.max(0, sl.frac * CIRC - 2)} ${CIRC - sl.frac * CIRC + 2}`}
                strokeDashoffset={sl.offset}
              />
            )
          )}
        </svg>
        <div className={styles.donutCenter}>
          <div className={styles.donutValue}>{centerValue}</div>
          <div className={styles.donutLabel}>{centerLabel}</div>
        </div>
      </div>
      <div className={styles.donutLegend}>
        {segments.map((s, i) => {
          const pct = (s.value / total) * 100;
          return (
            <div key={i} className={styles.donutLegendRow}>
              <span className={styles.barDot} style={{ backgroundColor: s.color }} />
              <span className={styles.donutLegendLabel}>{s.label}</span>
              <span className={styles.donutLegendPct}>{pct.toFixed(1)}%</span>
              <span className={styles.donutLegendVal}>{formatNumber(s.value)}</span>
            </div>
          );
        })}
      </div>
    </div>
  );
}

interface ScatterProps {
  records: UsageRecord[];
}

function LatencyScatter({ records }: ScatterProps) {
  if (!records.length) return null;
  const W = 560;
  const H = 260;
  const PAD_X = 46;
  const PAD_TOP = 18;
  const PAD_BOTTOM = 34;
  const lat = records.map((r) => r.LatencyMs || 0).filter((v) => v >= 0);
  if (!lat.length) return null;
  const maxLat = Math.max(1, ...lat);
  const maxTok = Math.max(1, ...records.map((r) => r.TotalTokens || 1));
  const tMin = Math.min(...records.map((r) => new Date(r.TS).getTime()));
  const tMax = Math.max(tMin + 1, ...records.map((r) => new Date(r.TS).getTime()));

  const x = (ts: string) => {
    const t = new Date(ts).getTime();
    return PAD_X + ((t - tMin) / Math.max(1, tMax - tMin)) * (W - PAD_X - 18);
  };
  const y = (ms: number) => H - PAD_BOTTOM - (ms / maxLat) * (H - PAD_TOP - PAD_BOTTOM);
  const r = (tok: number) => 3 + 5 * Math.sqrt((tok || 1) / maxTok);

  return (
    <svg viewBox={`0 0 ${W} ${H}`} className={styles.scatterSvg}>
      {[0, 0.5, 1].map((f) => {
        const yy = PAD_TOP + f * (H - PAD_TOP - PAD_BOTTOM);
        const val = maxLat * (1 - f);
        return (
          <g key={f}>
            <line x1={PAD_X} x2={W - 14} y1={yy} y2={yy} className={styles.gridLine} />
            <text x={PAD_X - 7} y={yy + 3.5} textAnchor="end" className={styles.gridLabel}>
              {formatMs(val)}
            </text>
          </g>
        );
      })}
      {[0.25, 0.75].map((f) => {
        const t = tMin + (tMax - tMin) * f;
        const str = new Date(t).toLocaleTimeString(undefined, {
          hour: '2-digit',
          minute: '2-digit',
        });
        const xx = PAD_X + f * (W - PAD_X - 18);
        return (
          <text key={f} x={xx} y={H - 10} textAnchor="middle" className={styles.gridLabel}>
            {str}
          </text>
        );
      })}
      {records.map((rec, i) => (
        <circle
          key={i}
          cx={x(rec.TS)}
          cy={y(rec.LatencyMs || 0)}
          r={r(rec.TotalTokens)}
          className={rec.Failed ? styles.scatterDotFail : styles.scatterDot}
        >
          <title>{`${rec.Model} · ${formatMs(rec.LatencyMs)} · ${formatNumber(rec.TotalTokens)} tok`}</title>
        </circle>
      ))}
    </svg>
  );
}

interface HeatmapProps {
  series: UsagePoint[];
  t: (key: string) => string;
}

function ActivityHeatmap({ series, t }: HeatmapProps) {
  if (!series.length) return null;
  const maxRequests = Math.max(1, ...series.map((p) => p.requests));
  const gap = 3;

  const cells = series.map((p) => {
    const intensity = Math.min(1, (p.requests / maxRequests) ** 0.45);
    const failed = p.failed > 0;
    return { p, intensity, failed };
  });

  return (
    <div className={styles.heatmapWrap}>
      <div
        className={styles.heatmapGrid}
        style={{
          gridTemplateColumns: `repeat(${cells.length}, 1fr)`,
          gap: `${gap}px`,
        }}
      >
        {cells.map((c, i) => (
          <div
            key={i}
            className={`${styles.heatCell} ${c.failed ? styles.heatCellFailed : ''}`}
            style={{
              opacity: 0.15 + 0.85 * c.intensity,
            }}
            title={`${new Date(c.p.bucket).toLocaleString()} · ${c.p.requests} req · ${formatNumber(c.p.total_tokens)} tok`}
          />
        ))}
      </div>
      <div className={styles.heatmapLegend}>
        <span className={styles.heatLabel}>{t('usage.heatmap_quiet')}</span>
        {[0.2, 0.45, 0.7, 0.9].map((v) => (
          <div key={v} className={styles.heatLegendCell} style={{ opacity: 0.15 + 0.85 * v }} />
        ))}
        <span className={styles.heatLabel}>{t('usage.heatmap_busy')}</span>
      </div>
    </div>
  );
}

interface SuccessRingProps {
  rate: number;
}

function SuccessRing({ rate }: SuccessRingProps) {
  const clamped = Math.max(0, Math.min(100, rate));
  const R = 15.5;
  const C = 2 * Math.PI * R;
  const color =
    clamped >= 99 ? '#10b981' : clamped >= 95 ? '#3b82f6' : clamped >= 90 ? '#f59e0b' : '#ef4444';
  return (
    <span className={styles.ring} title={`${clamped.toFixed(1)}%`}>
      <svg viewBox="0 0 40 40" className={styles.ringSvg}>
        <circle cx="20" cy="20" r={R} className={styles.ringTrack} />
        <circle
          cx="20"
          cy="20"
          r={R}
          className={styles.ringFill}
          stroke={color}
          strokeDasharray={`${(clamped / 100) * C} ${C}`}
        />
      </svg>
      <span className={styles.ringText} style={{ color }}>
        {Math.round(clamped)}
      </span>
    </span>
  );
}

export function UsagePage() {
  const { t } = useTranslation();
  const connectionStatus = useAuthStore((state) => state.connectionStatus);
  const showLogCost = useUsagePrefsStore((state) => state.showLogCost);
  const showLogTokenPie = useUsagePrefsStore((state) => state.showLogTokenPie);
  const setShowLogCost = useUsagePrefsStore((state) => state.setShowLogCost);
  const setShowLogTokenPie = useUsagePrefsStore((state) => state.setShowLogTokenPie);

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
  const [activeDimension, setActiveDimension] = useState<'model' | 'provider' | 'api_key' | 'auth'>(
    'model'
  );

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
      ? (stats.summary.succeeded_requests / stats.summary.total_requests) * 100
      : -1;

  return (
    <div className={styles.container}>
      {/* Decorative background orbs */}
      <div className={styles.orbs} aria-hidden="true">
        <div className={styles.orb1} />
        <div className={styles.orb2} />
      </div>

      <div className={styles.pageHeaderRow}>
        <span className={styles.pageWatermark} aria-hidden="true">
          USAGE
        </span>
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
        <div className={styles.skeletonGrid}>
          {Array.from({ length: 8 }).map((_, i) => (
            <div key={i} className={styles.skeletonCard} />
          ))}
          <div className={`${styles.skeletonCard} ${styles.skeletonChart}`} />
        </div>
      ) : stats ? (
        <>
          {/* Top-level metric cards */}
          <div className={styles.statGrid}>
            <StatCard
              label={t('usage.stat_total_requests')}
              value={<AnimatedNumber value={stats.summary.total_requests} format={formatNumber} />}
              hint={`${formatNumber(stats.summary.succeeded_requests)} ${t('usage.stat_ok')}`}
              icon={IconNetwork}
              tone="info"
            />
            <StatCard
              label={t('usage.stat_failed')}
              value={<AnimatedNumber value={stats.summary.failed_requests} format={formatInt} />}
              hint={
                successRate >= 0 ? (
                  <span className={styles.successHint}>
                    <SuccessRing rate={successRate} />
                    {successRate.toFixed(1)}% {t('usage.stat_success_rate')}
                  </span>
                ) : undefined
              }
              icon={IconAlertTriangle}
              tone={stats.summary.failed_requests > 0 ? 'error' : 'default'}
            />
            <StatCard
              label={t('usage.stat_total_tokens')}
              value={<AnimatedNumber value={stats.summary.total_tokens} format={formatNumber} />}
              hint={
                <span className={styles.tokBreakdown}>
                  <span className={styles.tokIn}>
                    {formatNumber(stats.summary.total_input_tokens)} in
                  </span>
                  <span className={styles.tokSep}>/</span>
                  <span className={styles.tokOut}>
                    {formatNumber(stats.summary.total_output_tokens)} out
                  </span>
                </span>
              }
              icon={IconChart}
              tone="accent"
            />
            <StatCard
              label={t('usage.stat_cached')}
              value={<AnimatedNumber value={stats.summary.total_cached_tokens} format={formatNumber} />}
              hint={t('usage.stat_cached_hint')}
              icon={IconCheckCircle2}
              tone="success"
            />
            <StatCard
              label={t('usage.stat_reasoning')}
              value={
                <AnimatedNumber value={stats.summary.total_reasoning_tokens} format={formatNumber} />
              }
              icon={IconBot}
            />
            <StatCard
              label={t('usage.stat_avg_latency')}
              value={<AnimatedNumber value={stats.summary.avg_latency_ms} format={formatMs} />}
              hint={
                <span className={styles.ttftPill}>TTFT {formatMs(stats.summary.avg_ttft_ms)}</span>
              }
              icon={IconTimer}
              tone="amber"
            />
            <StatCard
              label={t('usage.stat_models')}
              value={<AnimatedNumber value={stats.summary.unique_models} format={formatInt} />}
              icon={IconModelCluster}
            />
            <StatCard
              label={t('usage.stat_api_keys')}
              value={<AnimatedNumber value={stats.summary.unique_api_keys} format={formatInt} />}
              hint={`${stats.summary.unique_auth_files} ${t('usage.stat_auths')}`}
              icon={IconKey}
            />
          </div>

          {/* Time-series chart */}
          <Card
            title={<SectionTitle icon={IconChart}>{t('usage.chart_title')}</SectionTitle>}
            className={styles.glassCard}
            extra={
              <span className={styles.windowNote}>
                {isZeroTs(stats.summary.from) || isZeroTs(stats.summary.to)
                  ? t('usage.range_all')
                  : `${formatShortTs(stats.summary.from)} → ${formatShortTs(stats.summary.to)}`}
              </span>
            }
          >
            {stats.series && stats.series.length > 0 ? (
              <LineChart points={stats.series} bucketSeconds={stats.bucket_seconds} t={t} />
            ) : (
              <EmptyState title={t('usage.no_data')} />
            )}
          </Card>

          {/* Diagram wall: composition donut, request activity heatmap, latency scatter */}
          <div className={styles.vizGrid}>
            <Card
              title={<SectionTitle icon={IconPieChart}>{t('usage.donut_title')}</SectionTitle>}
              className={styles.glassCard}
            >
              <DonutChart
                segments={[
                  {
                    label: t('usage.legend_input'),
                    value: stats.summary.total_input_tokens,
                    color: '#10b981',
                  },
                  {
                    label: t('usage.legend_output'),
                    value: stats.summary.total_output_tokens,
                    color: '#a855f7',
                  },
                  {
                    label: t('usage.legend_reasoning'),
                    value: stats.summary.total_reasoning_tokens,
                    color: '#f59e0b',
                  },
                  {
                    label: t('usage.legend_cached'),
                    value: stats.summary.total_cached_tokens,
                    color: '#06b6d4',
                  },
                ]}
                centerLabel={t('usage.stat_total_tokens')}
                centerValue={formatNumber(stats.summary.total_tokens)}
              />
            </Card>
            <Card
              title={<SectionTitle icon={IconFlame}>{t('usage.heatmap_title')}</SectionTitle>}
              className={styles.glassCard}
            >
              {stats.series && stats.series.length > 0 ? (
                <ActivityHeatmap series={stats.series} t={t} />
              ) : (
                <EmptyState title={t('usage.no_data')} />
              )}
            </Card>
            <Card
              title={<SectionTitle icon={IconActivity}>{t('usage.scatter_title')}</SectionTitle>}
              className={`${styles.glassCard} ${styles.vizGridWide}`}
            >
              {records.length > 0 ? (
                <LatencyScatter records={records} />
              ) : (
                <EmptyState title={t('usage.no_records')} />
              )}
            </Card>
          </div>

          {/* Per-dimension breakdown */}
          <Card
            title={<SectionTitle icon={IconFilterAll}>{t('usage.breakdown_title')}</SectionTitle>}
            className={styles.glassCard}
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
            title={
              <SectionTitle icon={IconScrollText}>
                {t('usage.records_title')}
                {recordsTotal > 0 && (
                  <span className={styles.countBadge}>{formatNumber(recordsTotal)}</span>
                )}
              </SectionTitle>
            }
            className={styles.glassCard}
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
                <span className={styles.metricsDivider} aria-hidden="true" />
                <ToggleSwitch
                  checked={showLogCost}
                  onChange={setShowLogCost}
                  label={t('usage.show_cost')}
                  ariaLabel={t('usage.show_cost')}
                />
                <ToggleSwitch
                  checked={showLogTokenPie}
                  onChange={setShowLogTokenPie}
                  label={t('usage.show_token_pie')}
                  ariaLabel={t('usage.show_token_pie')}
                />
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
                        {showLogCost && <th>{t('usage.col_cost')}</th>}
                        <th>{t('usage.col_latency')}</th>
                        <th>{t('usage.col_status')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {records.map((r, i) => (
                        <tr
                          key={`${r.RequestID}-${i}`}
                          className={r.Failed ? styles.failedRow : ''}
                        >
                          <td className={styles.tsCell}>{formatTs(r.TS)}</td>
                          <td
                            className={styles.keyCell}
                            title={r.Alias !== r.Model ? `${r.Alias} → ${r.Model}` : r.Model}
                          >
                            <span className={styles.modelCell}>{r.Model}</span>
                          </td>
                          <td>
                            <span className={styles.providerPill}>{r.Provider || '-'}</span>
                          </td>
                          <td className={styles.keyCell} title={r.APIKey}>
                            {maskKey(r.APIKey)}
                          </td>
                          <td className={styles.keyCell} title={r.Endpoint}>
                            {r.Endpoint || '-'}
                          </td>
                          <td>
                            <span
                              className={styles.tokensCell}
                              title={`in ${r.InputTokens} · out ${r.OutputTokens} · reasoning ${r.ReasoningTokens} · cached ${r.CachedTokens}`}
                            >
                              <span className={styles.tokensValue}>{formatNumber(r.TotalTokens)}</span>
                              {showLogTokenPie && (
                                <TokenNumbers
                                  input={r.InputTokens}
                                  output={r.OutputTokens}
                                  reasoning={r.ReasoningTokens}
                                  cached={r.CachedTokens}
                                />
                              )}
                            </span>
                          </td>
                          {showLogCost && (
                            <td>
                              <CostCell record={r} />
                            </td>
                          )}
                          <td>
                            <span
                              className={`${styles.latencyPill} ${styles[`latency-${latencyTone(r.LatencyMs)}`]}`}
                            >
                              {formatMs(r.LatencyMs)}
                            </span>
                          </td>
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
