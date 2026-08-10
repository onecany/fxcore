// 通用格式化纯函数（交易面板展示逻辑，可测试）。
export function fmtUsd(v: number | undefined | null, signed = false): string {
  if (typeof v !== 'number' || Number.isNaN(v)) return '-';
  const abs = Math.abs(v).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
  return signed ? `${v >= 0 ? '+' : '-'}${abs}` : abs;
}

/** 带符号金额（P&L 展示）：--356.20 双负号 bug 的修复版，负数必须 abs 再拼符号 */
export function fmtPnL(v: number | undefined | null): string {
  return fmtUsd(v, true);
}

export function fmtPct(v: number | undefined | null, precision = 1): string {
  if (typeof v !== 'number' || Number.isNaN(v)) return '-';
  return `${(v * 100).toFixed(precision)}%`;
}

/** 持仓时长（openedAt→closedAt），<1h 显示分钟，<24h 显示小时，否则天数 */
export function fmtDur(start?: string, end?: string): string {
  if (!start || !end) return '-';
  const ms = new Date(end).getTime() - new Date(start).getTime();
  if (Number.isNaN(ms) || ms < 0) return '-';
  const h = ms / 3600000;
  if (h < 1) return `${Math.max(1, Math.round(ms / 60000))}m`;
  if (h < 24) return `${h.toFixed(1)}h`;
  return `${(h / 24).toFixed(1)}d`;
}

/** Unix 秒（number）或 ISO 字符串 → 本地时间 HH:MM:SS */
export function fmtTime(raw?: string | number): string {
  if (!raw) return '-';
  const d = typeof raw === 'number' ? new Date(raw * 1000) : new Date(raw);
  if (Number.isNaN(d.getTime())) return '-';
  return d.toLocaleTimeString('zh-CN', { hour12: false });
}

/** 非 number 入参返回 '-'（渲染兜底，字段再错也不白屏） */
export function safeNum(v: unknown): number | undefined {
  return typeof v === 'number' && !Number.isNaN(v) ? v : undefined;
}
