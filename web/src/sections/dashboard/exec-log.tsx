// 执行日志组件：决策卡（Cycle 头 + 动作时间线 + AI duration + 可展开 Prompt/COT）。
import { useMemo, useState } from 'react';
import type { DecisionRecord } from '../../api/v1/types/contract';
import { UP, DOWN } from './primitives';

export function fmtTime(raw?: string | number): string {
  if (!raw) return '-';
  const d = typeof raw === 'number' ? new Date(raw * 1000) : new Date(raw);
  return d.toLocaleTimeString('zh-CN', { hour12: false });
}

export function fmtClock(raw?: string): string {
  if (!raw) return '-';
  const d = new Date(raw);
  return d.toLocaleTimeString('zh-CN', { hour12: false });
}

export function pnlColor(v: number | undefined | null): string {
  return (v ?? 0) >= 0 ? UP : DOWN;
}

/** 执行日志卡：Cycle 头 + 动作时间线 + AI duration + 结果行 + 可展开 Prompt / COT */
export function DecisionCard({ d }: { d: DecisionRecord }) {
  const [open, setOpen] = useState(false);
  const decisionsJson = useMemo(() => {
    try { return d.decisionJson ? JSON.parse(d.decisionJson) : []; } catch { return []; }
  }, [d]);
  const coins = [...new Set(d.candidateCoins ?? [])];
  const actions = decisionsJson as Record<string, unknown>[];

  return (
    <div className="decision-card">
      <button className="decision-head" onClick={() => setOpen((o) => !o)}>
        <span className="mono dim" style={{ fontSize: 11 }}>🤖 Cycle #{d.cycleNumber}</span>
        <span className="mono dim" style={{ fontSize: 11 }}>{fmtTime(d.timestamp)}</span>
        <span className="mono dim" style={{ fontSize: 11 }}>{actions.length} actions</span>
        {d.success ? <span className="mono" style={{ fontSize: 11, color: UP }}>✓ ok</span> : <span className="mono" style={{ fontSize: 11, color: DOWN }}>✗ {d.errorMessage ?? 'Failed'}</span>}
        <span className="mono dim" style={{ fontSize: 11, marginLeft: 'auto' }}>{open ? '▾' : '▸'}</span>
      </button>
      {/* 动作时间线（Execution Log 样式：每币一行 动作+符号+conf） */}
      {actions.length > 0 && (
        <div className="exec-timeline">
          {actions.map((a, i) => (
            <div key={i} className="exec-row">
              <span className="exec-t">{fmtTime(d.timestamp)}</span>
              <span className={`exec-act ${a.action === 'wait' || a.action === 'hold' ? 'wait' : 'live'}`}>{String(a.action)}</span>
              {a.symbol ? <span className="exec-sym">{String(a.symbol)}</span> : null}
              {typeof a.confidence === 'number' ? <span className="exec-conf">conf{Math.round((a.confidence as number) * 100)}</span> : null}
            </div>
          ))}
        </div>
      )}
      {coins.length > 0 && (
        <div className="chip-row" style={{ margin: '6px 0 2px' }}>
          {coins.map((c) => <span key={c} className="chip">{c}</span>)}
        </div>
      )}
      {/* AI 调用耗时 + 结果行 */}
      <div className="exec-foot">
        {d.aiRequestDurationMs ? <span className="mono dim" style={{ fontSize: 10.5 }}>AI call duration: {d.aiRequestDurationMs} ms</span> : null}
        <span className="mono" style={{ fontSize: 10.5, color: d.success ? UP : DOWN }}>
          {d.success ? `✓ ${(actions[0]?.symbol ?? 'ALL')} ${(actions[0]?.action ?? 'decision')} succeeded` : '✗ failed'}
        </span>
      </div>
      {open && (
        <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 6 }}>
          {d.systemPrompt && <PromptBlock label="SYSTEM PROMPT" text={d.systemPrompt} />}
          {d.inputPrompt && <PromptBlock label="USER PROMPT" text={d.inputPrompt} />}
          {d.rawResponse && <PromptBlock label="RAW RESPONSE" text={d.rawResponse} />}
          {d.cotTrace && <PromptBlock label="CHAIN OF THOUGHT" text={d.cotTrace} />}
          {!d.systemPrompt && !d.inputPrompt && !d.rawResponse && !d.cotTrace && <div className="muted mono" style={{ fontSize: 11 }}>// 无详细轨迹</div>}
        </div>
      )}
    </div>
  );
}

function PromptBlock({ label, text }: { label: string; text: string }) {
  const [show, setShow] = useState(false);
  return (
    <div style={{ border: '1px solid var(--fxcore-panel-border)', borderRadius: 'var(--fxcore-r-sm)', overflow: 'hidden' }}>
      <button className="decision-head" onClick={() => setShow((s) => !s)} style={{ padding: '4px 10px', width: '100%' }}>
        <span className="mono dim" style={{ fontSize: 10.5, letterSpacing: '0.08em' }}>{label}</span>
        <span className="mono dim" style={{ fontSize: 10.5 }}>{show ? '▾' : '▸'}</span>
      </button>
      {show && (
        <pre style={{ margin: 0, padding: '8px 10px', maxHeight: 220, overflow: 'auto', fontSize: 10.5, lineHeight: 1.6, color: 'var(--fxcore-text-dim)', whiteSpace: 'pre-wrap', wordBreak: 'break-word', borderTop: '1px solid var(--fxcore-panel-border)' }}>{text}</pre>
      )}
    </div>
  );
}

/** Sharpe（年化，按平仓盈亏序列） */
export function fmtSharpe(pnls: number[]): string {
  const vals = pnls.filter((v) => Number.isFinite(v));
  if (vals.length < 2) return '-';
  const mean = vals.reduce((s, v) => s + v, 0) / vals.length;
  const variance = vals.reduce((s, v) => s + (v - mean) ** 2, 0) / (vals.length - 1);
  const sd = Math.sqrt(variance);
  if (sd === 0) return '-';
  return (mean / sd * Math.sqrt(365)).toFixed(2);
}
