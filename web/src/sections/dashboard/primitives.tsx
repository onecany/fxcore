// 交易终端展示基元：tm 指标卡 / 风控卡 / 统计卡 / 配置行 / 权益曲线 sparkline。
import { useId, useMemo } from 'react';
import type { EquitySnapshot } from '../../api/v1/types/contract';

export const UP = 'var(--fxcore-up)';
export const DOWN = 'var(--fxcore-down)';

/** tm 指标卡（tm-grid 风格：label 大写 + mono 大数值 + sub 副行） */
export function TmCard({ label, value, sub, color }: { label: string; value: string; sub?: string; color?: string }) {
  return (
    <div className="tm-card">
      <div className="tm-card-label">{label}</div>
      <div className="tm-card-value" style={{ color: color ?? 'var(--fxcore-text)' }}>{value}</div>
      {sub && <div className="tm-card-sub">{sub}</div>}
    </div>
  );
}

export function PhStat({ label, value, color, hint }: { label: string; value: string; color?: string; hint?: string }) {
  return (
    <div className="ph-stat">
      <div className="mono dim" style={{ fontSize: 9.5, letterSpacing: '0.1em' }}>{label}</div>
      <div className="mono" style={{ fontSize: 13.5, fontWeight: 600, color: color ?? 'var(--fxcore-text)', marginTop: 2 }}>{value}</div>
      {hint && <div className="mono dim" style={{ fontSize: 9.5, marginTop: 1 }}>{hint}</div>}
    </div>
  );
}

export function CfgRow({ k, v }: { k: string; v: string }) {
  return (
    <div className="cfg-row">
      <span className="dim">{k}</span>
      <span className="mono">{v}</span>
    </div>
  );
}

/** 权益曲线 SVG（迷你 sparkline，绿涨红跌） */
export function EquityCurve({ points, height = 130 }: { points: EquitySnapshot[]; height?: number }) {
  const uid = useId();
  const gradId = `eqfill-${uid.replace(/[^a-zA-Z0-9-]/g, '')}`;
  const path = useMemo(() => {
    if (points.length < 2) return null;
    const w = 560;
    const h = height;
    const vals = points.map((p) => p.equity);
    const min = Math.min(...vals);
    const max = Math.max(...vals);
    const range = max - min || 1;
    const step = w / (points.length - 1);
    const coords = points.map((p, i) => [i * step, h - ((p.equity - min) / range) * (h - 10) - 5] as const);
    const d = coords.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`).join(' ');
    return { d, min, max };
  }, [points, height]);
  if (!path) return <div className="muted mono" style={{ fontSize: 12 }}>// 暂无权益数据</div>;
  const up = points[points.length - 1].equity >= points[0].equity;
  return (
    <svg viewBox={`0 0 560 ${height}`} style={{ width: '100%', height, display: 'block' }}>
      <defs>
        <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--fxcore-accent)" stopOpacity="0.25" />
          <stop offset="100%" stopColor="var(--fxcore-accent)" stopOpacity="0.02" />
        </linearGradient>
      </defs>
      {[0.25, 0.5, 0.75].map((f) => (
        <line key={f} x1="0" x2="560" y1={height * f} y2={height * f} stroke="var(--fxcore-panel-border)" strokeWidth="0.5" />
      ))}
      <path d={`${path.d} L560,${height} L0,${height} Z`} fill={`url(#${gradId})`} />
      <path d={path.d} fill="none" stroke={up ? 'var(--fxcore-up)' : 'var(--fxcore-down)'} strokeWidth="1.6" />
    </svg>
  );
}
