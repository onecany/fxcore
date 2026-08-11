// 创建/编辑交易员（双模式：editing 非空 = 编辑回填）。新用户引导：无模型/策略/交易所时提示先去对应页创建。
import { useCallback, useEffect, useState } from 'react';
import { Panel } from '../../components/ui';
import { ExchangeIcon } from '../../components/exchange-icon';
import * as traderApi from '../../api/v1/modules/traders';
import type { TraderResponse, Exchange, RiskConfig, AIModel, ExchangeAccount } from '../../api/v1/types/contract';
const EXCHANGE_OPTIONS: Exchange[] = ['binance', 'bybit', 'okx', 'bitget', 'gate', 'kucoin', 'hyperliquid', 'aster', 'indodax', 'lighter'];

export function CreateTraderPanel({ editing, models, exchanges, onClose, onCreated }: {
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
    void import('../../api/v1/modules/strategies').then((s) =>
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
