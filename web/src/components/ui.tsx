// FXcore 科幻 UI 基元：状态徽标 / 数据卡片 / 页头 / 提示 / 面板。
import type { ReactNode } from 'react';

/** 状态徽标（led 呼吸灯 + 语义色） */
export function Badge({ state }: { state: string }) {
  return (
    <span className={`badge ${state.toLowerCase()}`}>
      <span className="led" />
      {state}
    </span>
  );
}

/** 数据卡片（label + value + 可选涨跌色） */
export function StatCard({
  label,
  value,
  tone,
  hint,
}: {
  label: string;
  value: string;
  tone?: 'up' | 'down' | 'plain';
  hint?: string;
}) {
  return (
    <div className="stat-card">
      <div className="label">{label}</div>
      <div className={`value ${tone ?? ''}`}>{value}</div>
      {hint && <div className="hint">{hint}</div>}
    </div>
  );
}

/** 页头（标题 + lead 副标题） */
export function PageHead({ title, lead }: { title: string; lead?: ReactNode }) {
  return (
    <div className="page-head">
      <h2>{title}</h2>
      {lead && <p className="lead">{lead}</p>}
    </div>
  );
}

/** 提示条 */
export function Alert({ kind, children }: { kind: 'error' | 'ok' | 'warn'; children: ReactNode }) {
  if (!children) return null;
  return <div className={`alert ${kind}`}>{children}</div>;
}

/** 玻璃面板 */
export function Panel({ title, children, className }: { title?: string; children: ReactNode; className?: string }) {
  return (
    <div className={`panel ${className ?? ''}`}>
      {title && <h3 className="panel-title">{title}</h3>}
      {children}
    </div>
  );
}

/** 终端窗口（提示词预览 / SSE 事件流） */
export function Terminal({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="terminal">
      <div className="term-bar">
        <span className="t-dot" />
        <span className="t-dot" />
        <span className="t-dot" />
        <span className="t-title">{title}</span>
      </div>
      <div className="term-body">{children}</div>
    </div>
  );
}
