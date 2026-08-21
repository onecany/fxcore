// Backtest 页面：回放控制台（四卡表单）+ K 线预览 + 运行编队卡片网格。
// 数据层 SWR：runs 有活跃运行 3s 自动轮询（refreshInterval 动态），写操作后 mutate。
import { useCallback, useEffect, useMemo, useState } from 'react';
import useSWR from 'swr';
import * as backtestApi from '../api/v1/modules/backtest';
import * as dataApi from '../api/v1/modules/data';
import type { BacktestConfig, RunSummary, KlineDTO, BacktestState } from '../api/v1/types/contract';
import { PageHead, Panel, Badge, Alert } from '../components/ui';
import KlineChart from '../components/KlineChart';
import RunDetail from '../sections/backtest/run-detail';
import { useT } from '../stores/i18nStore';
import { fmtTime } from '../utils/format';
import { errMsg } from '../utils/errorCodes';

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

  // ===== 运行编队：SWR + 有活跃运行 3s 轮询（无活跃自动停） =====
  const { data, mutate } = useSWR<{ items: RunSummary[] }>(
    ['/backtest/runs', 'page'],
    () => backtestApi.listBacktestRuns({ size: 20 }),
    { refreshInterval: (d) => (d?.items ?? []).some((r) => r.state === 'running' || r.state === 'created') ? 3000 : 0 },
  );
  const runs = useMemo(() => data?.items ?? [], [data]);
  const hasActive = useMemo(() => runs.some((r) => r.state === 'running' || r.state === 'created'), [runs]);

  // ===== 详情视图：选中 run 展开详情面板 =====
  const [selectedRunId, setSelectedRunId] = useState<string | null>(null);
  const selectedRun = useMemo(() => runs.find((r) => r.runId === selectedRunId) ?? null, [runs, selectedRunId]);

  const toggleSelect = useCallback((runId: string) => {
    setSelectedRunId((cur) => (cur === runId ? null : runId));
  }, []);

  const removeSelected = useCallback(() => {
    if (!selectedRunId) return;
    if (!window.confirm('删除该回测（含关联数据）？')) return;
    void (async () => {
      try {
        await backtestApi.deleteBacktest(selectedRunId);
        setSelectedRunId(null);
        void mutate();
      } catch (err) { setError(String(err)); }
    })();
  }, [selectedRunId, mutate]);

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
      void mutate();
    } catch (err) { setError(String(err)); } finally { setBusy(false); }
  };

  const control = async (runId: string, action: backtestApi.BacktestControlAction) => {
    try { await backtestApi.controlBacktest(runId, action); void mutate(); } catch (err) { setError(String(err)); }
  };
  const remove = async (runId: string) => {
    if (!window.confirm('删除该回测（含关联数据）？')) return;
    try { await backtestApi.deleteBacktest(runId); void mutate(); } catch (err) { setError(String(err)); }
  };

  const active = runs.filter((r) => r.state === 'running' || r.state === 'created').length;
  const completed = runs.filter((r) => r.state === 'completed').length;

  return (
    <section>
      <PageHead
        title="回放引擎"
        lead={<>历史行情回放 · 实时 K 线 · 进度与权益跟踪</>}
      />
      {error && <Alert kind="error">{errMsg(error)}</Alert>}
      {msg && <Alert kind="ok">{msg}</Alert>}

      {/* KPI 概览（毛玻璃） */}
      <div className="stat-grid">
        <div className="stat-card">
          <div className="label">回放中</div>
          <div className={`value ${active > 0 ? 'up' : ''}`}>{active}</div>
          <div className="hint">3s 自动刷新</div>
        </div>
        <div className="stat-card">
          <div className="label">已完成</div>
          <div className="value">{completed}</div>
          <div className="hint">历史回放</div>
        </div>
        <div className="stat-card">
          <div className="label">总运行</div>
          <div className="value">{runs.length}</div>
          <div className="hint">本次会话</div>
        </div>
      </div>

      <Panel title="新建回放" className="mb">
        <form onSubmit={(e) => void start(e)}>
          <div className="bt-form-4">
            {/* 01 标的卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">01</span>标的</div>
              <div className="field">
                <label className="bt-label">币种（逗号分隔）</label>
                <input value={form.symbols} onChange={(e) => set('symbols', e.target.value)} placeholder="BTC-USDT,ETH-USDT" />
              </div>
              <div className="field">
                <label className="bt-label">周期（分钟）</label>
                <input type="number" min={3} value={form.cadence} onChange={(e) => set('cadence', e.target.value)} />
              </div>
            </div>
            {/* 02 资金卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">02</span>资金</div>
              <div className="field">
                <label className="bt-label">初始资金 USDT</label>
                <input type="number" min={1} value={form.initialBalance} onChange={(e) => set('initialBalance', e.target.value)} />
              </div>
              <div className="field">
                <label className="bt-label">杠杆倍数</label>
                <input type="number" min={1} max={20} value={form.leverage} onChange={(e) => set('leverage', e.target.value)} />
              </div>
            </div>
            {/* 03 时间卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">03</span>时间</div>
              <div className="field">
                <label className="bt-label">开始（留空 = 24 小时前）</label>
                <input type="datetime-local" value={form.startTime} onChange={(e) => set('startTime', e.target.value)} />
              </div>
              <div className="field">
                <label className="bt-label">结束（留空 = 现在）</label>
                <input type="datetime-local" value={form.endTime} onChange={(e) => set('endTime', e.target.value)} />
              </div>
            </div>
            {/* 04 执行卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">04</span>执行</div>
              <div className="field">
                <label className="bt-label">成交定价</label>
                <select value={form.fillPolicy} onChange={(e) => set('fillPolicy', e.target.value)}>
                  {FILL_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                </select>
              </div>
              <div className="field">
                <label className="bt-label">交易风格</label>
                <select value={form.promptVariant} onChange={(e) => set('promptVariant', e.target.value)}>
                  {VARIANT_OPTIONS.map((o) => <option key={o.value} value={o.value}>{o.label}</option>)}
                </select>
              </div>
              <button className="btn primary bt-submit" type="submit" disabled={busy}>
                {busy ? '启动中…' : '⌁ 启动回放'}
              </button>
            </div>
          </div>
        </form>
      </Panel>

      <Panel title={t.kline.title} className="mb">
        <form className="kline-bar" onSubmit={(e) => void submitKline(e)}>
          <div className="field bt-kline-field">
            <label>{t.kline.symbol}</label>
            <input
              value={klineForm.symbol}
              onChange={(e) => setKlineForm((f) => ({ ...f, symbol: e.target.value }))}
              placeholder="BTC-USDT"
            />
          </div>
          <div className="field bt-kline-int">
            <label>{t.kline.interval}</label>
            <select
              value={klineForm.interval}
              onChange={(e) => setKlineForm((f) => ({ ...f, interval: e.target.value }))}
            >
              {KLINE_INTERVALS.map((i) => <option key={i} value={i}>{i}</option>)}
            </select>
          </div>
          <div className="row bt-kline-go">
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
        {klineError && <Alert kind="error">{t.kline.fetchFailed}：{errMsg(klineError)}</Alert>}
        <div className="kline-panel">
          {klineLoading && <div className="kline-loading">{t.kline.loading}</div>}
          {klines.length > 0 ? (
            <KlineChart data={sortedKlines} height={320} />
          ) : (
            !klineLoading && !klineError && <div className="muted mono bt-kline-empty">{t.kline.noData}</div>
          )}
        </div>
      </Panel>

      <Panel title={`运行编队 (${runs.length})`}>
        <div className="bt-table-head">
          <span className="dim bt-auto-hint">{hasActive ? '● 有活跃回放，3s 自动刷新' : '已就绪 · 无活跃回放'}</span>
          <button className="btn ghost btn-sm" onClick={() => void mutate()}>⟳ 刷新</button>
        </div>
        {runs.length === 0 ? (
          <div className="muted mono bt-empty">// NO REPLAYS RECORDED</div>
        ) : (
          <div className="bt-run-grid">
            {runs.map((r) => {
              const pct = r.progressPct != null ? Math.round(r.progressPct * 100) : null;
              const sel = r.runId === selectedRunId;
              return (
                <div
                  className={`bt-run-card${sel ? ' selected' : ''}`}
                  key={r.runId}
                  onClick={() => toggleSelect(r.runId)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); toggleSelect(r.runId); } }}
                  title={sel ? '收起详情' : '展开详情'}
                >
                  <div className="bt-run-head">
                    <span className="bt-run-id mono">{r.label || r.runId.slice(0, 10)}</span>
                    <span className="bt-run-state">
                      <Badge state={r.state} />
                      <span className="dim bt-run-state-label">{STATE_LABEL[r.state] ?? r.state}</span>
                    </span>
                  </div>
                  {pct != null && (
                    <div className="bt-progress">
                      <div className="bt-progress-bar" style={{ width: `${pct}%` }} />
                    </div>
                  )}
                  <div className="bt-run-metrics">
                    <div className="bt-metric">
                      <span className="bt-metric-label">进度</span>
                      <span className="bt-metric-value mono">{pct != null ? `${pct}%` : '—'}</span>
                    </div>
                    <div className="bt-metric">
                      <span className="bt-metric-label">权益</span>
                      <span className="bt-metric-value mono">{r.equityLast != null ? r.equityLast.toFixed(2) : '—'}</span>
                    </div>
                    <div className="bt-metric">
                      <span className="bt-metric-label">回撤</span>
                      <span className="bt-metric-value mono" style={{ color: (r.maxDrawdownPct ?? 0) > 0 ? 'var(--fxcore-down)' : undefined }}>
                        {r.maxDrawdownPct != null ? `${(r.maxDrawdownPct * 100).toFixed(1)}%` : '—'}
                      </span>
                    </div>
                    <div className="bt-metric">
                      <span className="bt-metric-label">币种</span>
                      <span className="bt-metric-value mono bt-metric-syms">{(r.symbols ?? []).join(', ') || String(r.symbolCount ?? '—')}</span>
                    </div>
                  </div>
                  <div className="bt-run-actions" onClick={(e) => e.stopPropagation()}>
                    {r.state === 'running' && <button className="btn btn-sm" onClick={() => void control(r.runId, 'pause')}>⏸ 暂停</button>}
                    {r.state === 'paused' && <button className="btn primary btn-sm" onClick={() => void control(r.runId, 'resume')}>▶ 恢复</button>}
                    {(r.state === 'running' || r.state === 'paused') && (
                      <button className="btn btn-sm" onClick={() => void control(r.runId, 'stop')}>■ 停止</button>
                    )}
                    <button className="btn danger btn-sm" onClick={() => void remove(r.runId)}>删除</button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </Panel>
      {selectedRun && (
        <RunDetail
          run={selectedRun}
          onClose={() => setSelectedRunId(null)}
          onRemoved={() => void removeSelected()}
        />
      )}
    </section>
  );
}
