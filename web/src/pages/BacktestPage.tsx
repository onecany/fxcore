// Backtest 页面：回放控制台 + K 线预览 + 运行编队 + 状态轮询。
import { useCallback, useEffect, useMemo, useState } from 'react';
import * as backtestApi from '../api/v1/modules/backtest';
import * as dataApi from '../api/v1/modules/data';
import type { BacktestConfig, RunSummary, KlineDTO, BacktestState } from '../api/v1/types/contract';
import { PageHead, Panel, Badge, Alert, StatCard } from '../components/ui';
import KlineChart from '../components/KlineChart';
import { useT } from '../stores/i18nStore';
import { fmtTime } from '../utils/format';

const FILL_OPTIONS = [
  { value: 'next_open', label: 'next_open · 下一根开盘价' },
  { value: 'bar_vwap', label: 'bar_vwap · K 线均价' },
  { value: 'mid', label: 'mid · 高低中位' },
] as const;

const KLINE_INTERVALS = ['1m', '5m', '15m', '1h', '4h', '1d'] as const;

const VARIANT_OPTIONS = [
  { value: 'balanced', label: '均衡 · 推荐平衡' },
  { value: 'aggressive', label: '激进 · 更高杠杆与更低置信门槛' },
  { value: 'conservative', label: '稳健 · 少交易仅对齐信号' },
  { value: 'scalping', label: '剥头皮 · 快节奏短周期' },
] as const;

// 状态产品文案映射（不暴露 created/running 等内部状态机术语）
const STATE_LABEL: Record<BacktestState, string> = {
  created: '排队中',
  running: '回放中',
  paused: '已暂停',
  stopped: '已停止',
  completed: '已完成',
  failed: '失败',
  liquidated: '爆仓',
};

// datetime-local 字符串 → unix 秒（本地时区）
const toUnix = (v: string): number | null => {
  if (!v) return null;
  const ms = new Date(v).getTime();
  return Number.isFinite(ms) ? Math.floor(ms / 1000) : null;
};

export default function BacktestPage() {
  const t = useT();
  const [runs, setRuns] = useState<RunSummary[]>([]);
  const [form, setForm] = useState({
    symbols: 'BTC-USDT,ETH-USDT',
    startTime: '',
    endTime: '',
    cadence: '15',
    initialBalance: '1000',
    leverage: '5',
    promptVariant: 'balanced',
    fillPolicy: 'next_open',
  });
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [polling, setPolling] = useState(false);

  // ===== K 线预览（提交态分离：输入不触发请求，拉取/回车才生效） =====
  const [klineForm, setKlineForm] = useState({ symbol: 'BTC-USDT', interval: '15m' });
  const [klineQuery, setKlineQuery] = useState({ symbol: 'BTC-USDT', interval: '15m' });
  const [klines, setKlines] = useState<KlineDTO[]>([]);
  const [klineLoading, setKlineLoading] = useState(false);
  const [klineError, setKlineError] = useState<string | null>(null);

  const loadKlines = useCallback(async (q: { symbol: string; interval: string }) => {
    setKlineLoading(true); setKlineError(null);
    try {
      const d = await dataApi.listKlines({ symbol: q.symbol, interval: q.interval, limit: 200 });
      setKlines(d ?? []);
    } catch (e) {
      setKlineError(String(e));
      setKlines([]);
    } finally {
      setKlineLoading(false);
    }
  }, []);

  useEffect(() => { void loadKlines(klineQuery); }, [klineQuery, loadKlines]);

  const submitKline = (e: React.FormEvent) => {
    e.preventDefault();
    const symbol = klineForm.symbol.trim() || 'BTC-USDT';
    if (symbol === klineQuery.symbol && klineForm.interval === klineQuery.interval) {
      void loadKlines(klineQuery); // 相同参数：手动刷新
      return;
    }
    setKlineQuery({ symbol, interval: klineForm.interval });
  };

  // 数据源可能返回降序：范围标注用升序视图（最早 → 最新），图表组件内部已排序
  const sortedKlines = useMemo(() => [...klines].sort((a, b) => a.timestamp - b.timestamp), [klines]);

  const load = useCallback(async () => {
    try { const d = await backtestApi.listBacktestRuns({ size: 20 }); setRuns(d.items); setError(null); } catch (e) { setError(String(e)); }
  }, []);

  useEffect(() => { void load(); }, [load]);

  useEffect(() => {
    if (!polling) return;
    const t = setInterval(() => void load(), 3000);
    return () => clearInterval(t);
  }, [polling, load]);

  useEffect(() => {
    setPolling(runs.some((r) => r.state === 'running' || r.state === 'created'));
  }, [runs]);

  const set = (k: string, v: string) => setForm((f) => ({ ...f, [k]: v }));

  const start = async (e: React.FormEvent) => {
    e.preventDefault();
    setMsg(null); setError(null);
    const now = Math.floor(Date.now() / 1000);
    const startSec = toUnix(form.startTime) ?? now - 86400;
    const endSec = toUnix(form.endTime) ?? now;
    if (endSec <= startSec) {
      setError('结束时间必须晚于开始时间');
      return;
    }
    if (endSec - startSec > 90 * 86400) {
      setError('回放窗口最长 90 天，请缩短时间范围');
      return;
    }
    const cfg: BacktestConfig = {
      symbols: form.symbols.split(',').map((s) => s.trim()).filter(Boolean),
      startTime: startSec,
      endTime: endSec,
      cadence: Number(form.cadence),
      initialBalance: Number(form.initialBalance),
      leverage: Number(form.leverage),
      promptVariant: form.promptVariant as BacktestConfig['promptVariant'],
      fillPolicy: form.fillPolicy as BacktestConfig['fillPolicy'],
    };
    setBusy(true);
    try {
      const meta = await backtestApi.startBacktest(cfg);
      setMsg(`回放启动：${meta.runId}`);
      void load();
    } catch (err) { setError(String(err)); } finally { setBusy(false); }
  };

  const control = async (runId: string, action: backtestApi.BacktestControlAction) => {
    try { await backtestApi.controlBacktest(runId, action); void load(); } catch (err) { setError(String(err)); }
  };
  const remove = async (runId: string) => {
    if (!window.confirm('删除该回测（含关联数据）？')) return;
    try { await backtestApi.deleteBacktest(runId); void load(); } catch (err) { setError(String(err)); }
  };

  const active = runs.filter((r) => r.state === 'running' || r.state === 'created').length;
  const completed = runs.filter((r) => r.state === 'completed').length;

  return (
    <section>
      <PageHead
        title="回放引擎"
        lead={<>历史行情回放 · 实时 K 线 · 进度与权益跟踪</>}
      />
      {error && <Alert kind="error">{error}</Alert>}
      {msg && <Alert kind="ok">{msg}</Alert>}

      <div className="stat-grid">
        <StatCard label="回放中" value={String(active)} tone={active > 0 ? 'up' : 'plain'} hint="3s 自动刷新" />
        <StatCard label="已完成" value={String(completed)} />
        <StatCard label="总运行" value={String(runs.length)} />
      </div>

      <Panel title="新建回放" className="mb">
        <form onSubmit={(e) => void start(e)}>
          <div className="bt-form-3">
            {/* 01 参数卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">01</span>参数</div>
              <div className="bt-field-2">
                <div>
                  <label className="dim" style={{ fontSize: 11 }}>币种（逗号分隔）</label>
                  <input value={form.symbols} onChange={(e) => set('symbols', e.target.value)} placeholder="BTC-USDT,ETH-USDT" />
                </div>
                <div>
                  <label className="dim" style={{ fontSize: 11 }}>周期（分钟）</label>
                  <input type="number" min={3} value={form.cadence} onChange={(e) => set('cadence', e.target.value)} />
                </div>
                <div>
                  <label className="dim" style={{ fontSize: 11 }}>初始资金 USDT</label>
                  <input type="number" min={1} value={form.initialBalance} onChange={(e) => set('initialBalance', e.target.value)} />
                </div>
                <div>
                  <label className="dim" style={{ fontSize: 11 }}>杠杆倍数</label>
                  <input type="number" min={1} max={20} value={form.leverage} onChange={(e) => set('leverage', e.target.value)} />
                </div>
              </div>
            </div>
            {/* 02 时间卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">02</span>时间</div>
              <label className="dim" style={{ fontSize: 11 }}>开始（留空 = 24 小时前）</label>
              <input type="datetime-local" value={form.startTime} onChange={(e) => set('startTime', e.target.value)} />
              <label className="dim" style={{ fontSize: 11, marginTop: 8 }}>结束（留空 = 现在）</label>
              <input type="datetime-local" value={form.endTime} onChange={(e) => set('endTime', e.target.value)} />
            </div>
            {/* 03 执行卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">03</span>执行</div>
              <label className="dim" style={{ fontSize: 11 }}>成交定价</label>
              <select value={form.fillPolicy} onChange={(e) => set('fillPolicy', e.target.value)}>
                {FILL_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
              <label className="dim" style={{ fontSize: 11, marginTop: 8 }}>交易风格</label>
              <select value={form.promptVariant} onChange={(e) => set('promptVariant', e.target.value)}>
                {VARIANT_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
              </select>
              <button className="btn primary" type="submit" disabled={busy} style={{ width: '100%', marginTop: 14 }}>
                {busy ? '启动中…' : '⌁ 启动回放'}
              </button>
            </div>
          </div>
        </form>
      </Panel>

      <Panel title={t.kline.title} className="mb">
          <form className="kline-bar" onSubmit={(e) => void submitKline(e)}>
            <div className="field" style={{ flex: 1, minWidth: 220 }}>
              <label>{t.kline.symbol}</label>
              <input
                value={klineForm.symbol}
                onChange={(e) => setKlineForm((f) => ({ ...f, symbol: e.target.value }))}
                placeholder="BTC-USDT"
              />
            </div>
            <div className="field" style={{ minWidth: 110 }}>
              <label>{t.kline.interval}</label>
              <select
                value={klineForm.interval}
                onChange={(e) => setKlineForm((f) => ({ ...f, interval: e.target.value }))}
              >
                {KLINE_INTERVALS.map((i) => <option key={i} value={i}>{i}</option>)}
              </select>
            </div>
            <div className="row">
              <button className="btn primary" type="submit">⟳ {t.kline.fetch}</button>
            </div>
            {klines.length > 0 && (
              <div className="kline-stats mono dim">
                <span>{sortedKlines.length} {t.kline.bars} · {klineQuery.symbol} · {klineQuery.interval}</span>
                <br />
                <span>{t.kline.range} {fmtTime(sortedKlines[0]?.timestamp)} → {fmtTime(sortedKlines[sortedKlines.length - 1]?.timestamp)} · {t.kline.last} {fmtTime(sortedKlines[sortedKlines.length - 1]?.timestamp)}</span>
              </div>
            )}
          </form>
          {klineError && <Alert kind="error">{t.kline.fetchFailed}：{klineError}</Alert>}
          <div className="kline-panel">
            {klineLoading && <div className="kline-loading">{t.kline.loading}</div>}
            {klines.length > 0 ? (
              <KlineChart data={sortedKlines} height={300} />
            ) : (
              !klineLoading && !klineError && <div className="muted mono" style={{ padding: '28px 0', textAlign: 'center' }}>{t.kline.noData}</div>
            )}
          </div>
        </Panel>

        <Panel title="运行编队" className="mb">
          <div className="row" style={{ marginBottom: 8 }}>
            <span className="dim" style={{ fontSize: 11 }}>3s 自动刷新</span>
            <button className="btn ghost" style={{ marginLeft: 'auto', padding: '2px 10px', fontSize: 11 }} onClick={() => void load()}>⟳ 刷新</button>
          </div>
          <div className="table-wrap"><table className="data-table">
            <thead>
              <tr><th>ID</th><th>状态</th><th>进度</th><th>权益</th><th>回撤</th><th>币种</th><th>操作</th></tr>
            </thead>
            <tbody>
              {runs.map((r) => (
                <tr key={r.runId}>
                  <td className="mono">{r.label || r.runId.slice(0, 10)}</td>
                  <td>
                    <Badge state={r.state} />
                    <span className="dim" style={{ fontSize: 10, marginLeft: 4 }}>{STATE_LABEL[r.state] ?? r.state}</span>
                  </td>
                  <td className="mono">{r.progressPct != null ? `${Math.round(r.progressPct * 100)}%` : '—'}</td>
                  <td className="mono">{r.equityLast != null ? r.equityLast.toFixed(2) : '—'}</td>
                  <td className="mono" style={{ color: (r.maxDrawdownPct ?? 0) > 0 ? 'var(--fxcore-down)' : undefined }}>
                    {r.maxDrawdownPct != null ? `${(r.maxDrawdownPct * 100).toFixed(1)}%` : '—'}
                  </td>
                  <td className="mono">{(r.symbols ?? []).join(', ') || String(r.symbolCount ?? '—')}</td>
                  <td>
                    <span className="row">
                      {r.state === 'running' && <button className="btn" onClick={() => void control(r.runId, 'pause')}>⏸ 暂停</button>}
                      {r.state === 'paused' && <button className="btn primary" onClick={() => void control(r.runId, 'resume')}>▶ 恢复</button>}
                      {(r.state === 'running' || r.state === 'paused') && (
                        <button className="btn" onClick={() => void control(r.runId, 'stop')}>■ 停止</button>
                      )}
                      <button className="btn danger" onClick={() => void remove(r.runId)}>删除</button>
                    </span>
                  </td>
                </tr>
              ))}
              {runs.length === 0 && <tr><td colSpan={7} className="empty">// NO REPLAYS RECORDED</td></tr>}
            </tbody>
          </table></div>
        </Panel>
    </section>
  );
}
