// 交易员编队（指挥中心布局）：
// 页头 ACTIVE_NODES 计数 + SYSTEM_READY → 快捷按钮组（模型/交易所/创建）
// → 左资源侧栏（AI MODELS + EXCHANGES 卡，状态灯）→ 右主区 Current Traders 卡片行。
// 状态机控制沿用 useTraders 乐观更新；创建/编辑双模式表单。
import { useCallback, useEffect, useState } from 'react';
import { useTraders } from '../hooks/useTrader';
import { PageHead, Panel, Badge } from '../components/ui';
import { ExchangeIcon } from '../components/exchange-icon';
import * as modelApi from '../api/v1/modules/models';
import * as exchangeApi from '../api/v1/modules/exchanges';
import * as traderApi from '../api/v1/modules/traders';
import type { TraderResponse, Exchange, RiskConfig, AIModel, ExchangeAccount } from '../api/v1/types/contract';
import { fmtPnL, fmtPct } from '../utils/format';

export default function TradersSection({ onNavigate }: { onNavigate: (tab: 'models' | 'exchanges' | 'dashboard') => void }) {
  const { data: traders, error, isLoading, runAction, mutate } = useTraders({ size: 50 });
  const [models, setModels] = useState<AIModel[]>([]);
  const [exchanges, setExchanges] = useState<ExchangeAccount[]>([]);
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<TraderResponse | null>(null);

  useEffect(() => {
    void modelApi.listModels().then(setModels).catch(() => setModels([]));
    void exchangeApi.listExchanges().then(setExchanges).catch(() => setExchanges([]));
  }, []);

  const items = traders?.items ?? [];
  const activeCount = items.filter((t) => t.status === 'running').length;
  const sysReady = activeCount > 0;

  // trader.modelConfig.modelId → 模型别名；trader.exchange → 交易所账户名（「模型 · 交易所 - 账户」）
  const modelNameOf = useCallback((t: TraderResponse) => {
    const m = models.find((x) => x.id === t.modelConfig.modelId);
    return m?.name ?? t.modelConfig.modelId.slice(0, 8);
  }, [models]);
  const accountOf = useCallback((t: TraderResponse) => {
    const a = exchanges.find((x) => x.exchangeType === t.exchange);
    return a?.accountName ?? t.exchange;
  }, [exchanges]);

  const openCreate = () => { setEditing(null); setFormOpen(true); };
  const openEdit = (t: TraderResponse) => { setEditing(t); setFormOpen(true); };

  return (
    <section>
      {/* 页头：标题 + ACTIVE_NODES + SYSTEM_READY 终端状态 */}
      <div className="row wrap" style={{ justifyContent: 'space-between', alignItems: 'flex-end', marginBottom: 14 }}>
        <PageHead
          title={`交易员编队 ${activeCount} ACTIVE_NODES`}
          lead={<>每个交易员独立引擎 · 状态机 <code>idle→running→paused→stopped</code></>}
        />
        <div className={`sys-status ${sysReady ? 'ok' : 'standby'}`}>
          <span className="led" />
          {sysReady ? 'SYSTEM_READY' : 'SYSTEM_STANDBY'}
        </div>
      </div>

      {error && <div className="alert error">错误：{String(error)}</div>}
      {isLoading && <div className="muted mono">· 加载中…</div>}

      {/* 顶部快捷按钮组（MODELS_CONFIG / EXCHANGE_KEYS / Create Trader） */}
      <div className="fleet-actions">
        <button className="btn ghost" onClick={() => onNavigate('models')}>⬡ MODELS_CONFIG</button>
        <button className="btn ghost" onClick={() => onNavigate('exchanges')}>◐ EXCHANGE_KEYS</button>
        <button className="btn primary" onClick={openCreate}>＋ Create Trader</button>
      </div>

      {formOpen && (
        <CreateTraderPanel
          key={editing?.id ?? 'new'}
          editing={editing}
          models={models}
          exchanges={exchanges}
          onClose={() => { setFormOpen(false); setEditing(null); }}
          onCreated={() => { setFormOpen(false); setEditing(null); void mutate(); }}
        />
      )}

      <div className="fleet-grid">
        {/* 左资源侧栏：AI MODELS + EXCHANGES 卡（状态灯 STANDBY/ACTIVE） */}
        <aside className="fleet-sidebar">
          <ResourcePanel
            title="AI MODELS"
            count={models.length}
            empty="// NO MODELS"
            onAction={() => onNavigate('models')}
          >
            {models.map((m) => (
              <ResourceCard
                key={m.id}
                name={m.name}
                meta={`${m.provider} / ${m.modelName}`}
                status={m.status === 'active' ? 'ACTIVE' : m.status === 'error' ? 'ERROR' : 'STANDBY'}
                tone={m.status === 'active' ? 'ok' : m.status === 'error' ? 'error' : 'standby'}
              />
            ))}
          </ResourcePanel>
          <ResourcePanel
            title="EXCHANGES"
            count={exchanges.length}
            empty="// NO EXCHANGES"
            onAction={() => onNavigate('exchanges')}
          >
            {exchanges.map((a) => (
              <ResourceCard
                key={a.id}
                icon={<ExchangeIcon type={a.exchangeType} size={16} />}
                name={a.accountName}
                meta={`${a.exchangeType.toUpperCase()} · ${a.testnet ? 'TESTNET' : 'CEX/DEX'}`}
                status={a.enabled ? 'ACTIVE' : 'STANDBY'}
                tone={a.enabled ? 'ok' : 'standby'}
              />
            ))}
          </ResourcePanel>
        </aside>

        {/* 右主区：Current Traders 卡片行 */}
        <main className="fleet-main">
          <Panel title={`Current Traders (${items.length})`}>
            {items.length === 0 ? (
              <div className="muted mono" style={{ fontSize: 12, padding: '18px 0', textAlign: 'center' }}>// NO TRADERS IN FLEET</div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 10 }}>
                {items.map((t) => (
                  <div key={t.id} className="trader-row">
                    <div className="trader-main">
                      <Badge state={t.status} />
                      <div className="trader-info">
                        <span className="trader-name">{t.name}</span>
                        <span className="trader-meta">
                          <ExchangeIcon type={t.exchange} size={14} />
                          {modelNameOf(t)} · {t.exchange.toUpperCase()} - {accountOf(t)}
                        </span>
                      </div>
                    </div>
                    <div className="trader-pnl">
                      <span className="mono" style={{ color: (t.metrics?.totalPnl ?? 0) >= 0 ? 'var(--fxcore-up)' : 'var(--fxcore-down)' }}>
                        {fmtPnL(t.metrics?.totalPnl)}
                      </span>
                      <span className="mono dim" style={{ fontSize: 11 }}>WR {fmtPct(t.metrics?.winRate, 0)}</span>
                    </div>
                    <TraderActions t={t} runAction={runAction} onEdit={openEdit} onView={() => {
                      window.history.replaceState(null, '', `?trader=${t.id}`);
                      onNavigate('dashboard');
                    }} />
                  </div>
                ))}
              </div>
            )}
          </Panel>
        </main>
      </div>
    </section>
  );
}

function ResourcePanel({ title, count, empty, onAction, children }: {
  title: string; count: number; empty: string; onAction: () => void; children: React.ReactNode;
}) {
  return (
    <Panel title={`${title} (${count})`}>
      {count === 0 ? (
        <div className="muted mono" style={{ fontSize: 11, padding: '8px 0' }}>{empty}</div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>{children}</div>
      )}
      <button className="btn ghost" style={{ width: '100%', marginTop: 10, padding: '4px 8px', fontSize: 11 }} onClick={onAction}>
        CONFIG ▸
      </button>
    </Panel>
  );
}

function ResourceCard({ icon, name, meta, status, tone }: {
  icon?: React.ReactNode; name: string; meta: string; status: string; tone: 'ok' | 'standby' | 'error';
}) {
  return (
    <div className="resource-card">
      <div className="resource-head">
        <span className="row" style={{ gap: 6, minWidth: 0 }}>
          {icon}
          <span className="resource-name">{name}</span>
        </span>
        <span className={`sys-status ${tone}`} style={{ fontSize: 10, padding: '1px 8px' }}>
          <span className="led" />{status}
        </span>
      </div>
      <div className="resource-meta">{meta}</div>
    </div>
  );
}

function TraderActions({ t, runAction, onEdit, onView }: {
  t: TraderResponse;
  runAction: (id: string, a: 'start' | 'pause' | 'resume' | 'stop') => Promise<void>;
  onEdit: (t: TraderResponse) => void;
  onView: () => void;
}) {
  const act = (a: 'start' | 'pause' | 'resume' | 'stop') => () => void runAction(t.id, a);
  return (
    <div className="trader-actions">
      <button className="btn ghost" onClick={onView} style={{ padding: '3px 10px', fontSize: 11 }}>View</button>
      <button className="btn ghost" onClick={() => onEdit(t)} disabled={t.status === 'running'} style={{ padding: '3px 10px', fontSize: 11 }}>Edit</button>
      {t.status === 'idle' || t.status === 'stopped' ? (
        <button className="btn primary" onClick={act('start')} style={{ padding: '3px 10px', fontSize: 11 }}>▶ Start</button>
      ) : t.status === 'running' ? (
        <>
          <button className="btn" onClick={act('pause')} style={{ padding: '3px 10px', fontSize: 11 }}>⏸ Pause</button>
          <button className="btn danger" onClick={act('stop')} style={{ padding: '3px 10px', fontSize: 11 }}>■ Stop</button>
        </>
      ) : (
        <>
          <button className="btn primary" onClick={act('resume')} style={{ padding: '3px 10px', fontSize: 11 }}>▶ Resume</button>
          <button className="btn danger" onClick={act('stop')} style={{ padding: '3px 10px', fontSize: 11 }}>■ Stop</button>
        </>
      )}
    </div>
  );
}

// ========== 创建/编辑交易员（双模式：editing 非空 = 编辑回填） ==========
const EXCHANGE_OPTIONS: Exchange[] = ['binance', 'bybit', 'okx', 'bitget', 'gate', 'kucoin', 'hyperliquid', 'aster', 'indodax', 'lighter'];

function CreateTraderPanel({ editing, models, exchanges, onClose, onCreated }: {
  editing: TraderResponse | null;
  models: AIModel[];
  exchanges: ExchangeAccount[];
  onClose: () => void;
  onCreated: () => void;
}) {
  const [name, setName] = useState(editing?.name ?? '');
  const [exchange, setExchange] = useState<Exchange>(editing?.exchange ?? 'binance');
  const [modelId, setModelId] = useState(editing?.modelConfig.modelId ?? '');
  const [strategyId, setStrategyId] = useState(editing?.strategyId ?? '');
  const [strategies, setStrategies] = useState<{ id: string; name: string }[]>([]);
  const [risk, setRisk] = useState<RiskConfig>(editing?.riskConfig ?? { maxPositionSize: 500, stopLoss: 0.05, takeProfit: 0.1, maxDailyLoss: 0.05 });
  const [msg, setMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!editing && models.length > 0) setModelId((cur) => cur || models[0].id);
    void import('../api/v1/modules/strategies').then((s) =>
      s.listStrategies().then((res) => {
        const list = (res.items ?? []).map((x) => ({ id: x.id, name: x.name }));
        setStrategies(list);
        if (!editing && list.length > 0) setStrategyId((cur) => cur || list[0].id);
      }).catch(() => setStrategies([])),
    );
  }, [editing, models]);

  const submit = useCallback(async () => {
    if (!name.trim() || !modelId || !strategyId) {
      setMsg('请填写名称并选择模型与策略');
      return;
    }
    setBusy(true);
    setMsg(null);
    try {
      const model = models.find((m) => m.id === modelId);
      const body = {
        name: name.trim(),
        exchange,
        modelConfig: { provider: (model?.provider ?? 'deepseek') as never, modelId },
        strategyId,
        riskConfig: risk,
      };
      if (editing) {
        await traderApi.updateTrader(editing.id, body);
        setMsg('✓ 交易员已更新');
      } else {
        await traderApi.createTrader(body);
        setMsg('✓ 交易员已创建（idle，可在下方启动）');
      }
      onCreated();
    } catch (e: any) {
      setMsg(`保存失败：${e?.message ?? String(e)}`);
    } finally {
      setBusy(false);
    }
  }, [name, exchange, modelId, strategyId, risk, editing, models, onCreated]);

  return (
    <Panel title={editing ? `编辑交易员 · ${editing.name}` : '创建交易员'} className="mb">
      <div className="form-grid form-grid-4" style={{ gap: 12 }}>
        <label className="mono" style={{ fontSize: 11 }}>
          名称
          <input className="prompt-area" style={{ marginTop: 4 }} value={name} onChange={(e) => setName(e.target.value)} placeholder="如 趋势猎手-1" />
        </label>
        <label className="mono" style={{ fontSize: 11 }}>
          交易所
          <span className="row" style={{ marginTop: 4, gap: 6 }}>
            <ExchangeIcon type={exchange} size={18} />
            <select className="prompt-area" style={{ flex: 1, marginTop: 0 }} value={exchange} onChange={(e) => setExchange(e.target.value as Exchange)}>
              {EXCHANGE_OPTIONS.map((x) => <option key={x} value={x}>{x}</option>)}
            </select>
          </span>
        </label>
        <label className="mono" style={{ fontSize: 11 }}>
          AI 模型
          <select className="prompt-area" style={{ marginTop: 4 }} value={modelId} onChange={(e) => setModelId(e.target.value)}>
            {models.length === 0 && <option value="">（无模型，请先到 ⬡ 模型 页创建）</option>}
            {models.map((m) => <option key={m.id} value={m.id}>{m.name} · {m.provider}</option>)}
          </select>
        </label>
        <label className="mono" style={{ fontSize: 11 }}>
          策略
          <select className="prompt-area" style={{ marginTop: 4 }} value={strategyId} onChange={(e) => setStrategyId(e.target.value)}>
            {strategies.length === 0 && <option value="">（无策略，请先到 ✦ 策略 页创建）</option>}
            {strategies.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
          </select>
        </label>
      </div>
      <div className="form-grid form-grid-4" style={{ gap: 12, marginTop: 12 }}>
        <RiskField label="单笔上限 USDT" value={risk.maxPositionSize} onChange={(v) => setRisk((r) => ({ ...r, maxPositionSize: v }))} />
        <RiskField label="止损" value={risk.stopLoss} onChange={(v) => setRisk((r) => ({ ...r, stopLoss: v }))} pct />
        <RiskField label="止盈" value={risk.takeProfit} onChange={(v) => setRisk((r) => ({ ...r, takeProfit: v }))} pct />
        <RiskField label="日损上限" value={risk.maxDailyLoss} onChange={(v) => setRisk((r) => ({ ...r, maxDailyLoss: v }))} pct />
      </div>
      <div className="row" style={{ marginTop: 12, gap: 10 }}>
        <button className="btn primary" disabled={busy} onClick={() => void submit()}>{busy ? '保存中…' : editing ? '● 保存修改' : '＋ 创建'}</button>
        <button className="btn ghost" onClick={onClose}>取消</button>
        {msg && <span className="mono" style={{ fontSize: 12, color: msg.startsWith('✓') ? 'var(--fxcore-up)' : 'var(--fxcore-down)' }}>{msg}</span>}
      </div>
      {models.length === 0 || strategies.length === 0 ? (
        <div className="muted mono" style={{ marginTop: 8, fontSize: 11 }}>
          提示：新注册账号数据隔离——需要先在 ⬡ 模型 / ✦ 策略 页创建资源，交易员才能引用。
        </div>
      ) : null}
      {exchanges.length === 0 && (
        <div className="muted mono" style={{ marginTop: 4, fontSize: 11 }}>
          提示：尚无交易所账户——启动交易员前请先在 ◐ 交易所 页接入并启用账户。
        </div>
      )}
    </Panel>
  );
}

function RiskField({ label, value, onChange, pct }: { label: string; value: number; onChange: (v: number) => void; pct?: boolean }) {
  return (
    <label className="mono" style={{ fontSize: 11 }}>
      {label}{pct ? ' %' : ''}
      <input
        className="prompt-area"
        style={{ marginTop: 4 }}
        type="number"
        step={pct ? 0.01 : 50}
        min={0}
        value={value}
        onChange={(e) => onChange(Number(e.target.value) || 0)}
      />
    </label>
  );
}
