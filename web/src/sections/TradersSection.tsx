// 交易员编队（指挥中心三栏信息架构）：
// 页头 ACTIVE_NODES + SYSTEM_READY + 编队概览条（TOTAL/RUNNING/PAUSED/STOPPED）
// → 快捷按钮组（MODELS_CONFIG / EXCHANGE_KEYS / Create Trader）
// → 三栏：左资源侧栏（AI MODELS + EXCHANGES）| 中编队网格（选中联动）| 右详情速览（→ 打开终端）
// 数据层：useTraders SWR + 5s 轮询（状态机乐观更新保留）；models/exchanges SWR 自动刷新。
import { useCallback, useEffect, useMemo, useState } from 'react';
import useSWR from 'swr';
import { useTraders } from '../hooks/useTrader';
import { PageHead } from '../components/ui';
import * as modelApi from '../api/v1/modules/models';
import * as exchangeApi from '../api/v1/modules/exchanges';
import type { TraderResponse, AIModel, ExchangeAccount } from '../api/v1/types/contract';
import { ResourceSidebar } from './traders/ResourceSidebar';
import { TraderGrid } from './traders/TraderGrid';
import { InspectorPanel } from './traders/InspectorPanel';
import { CreateTraderPanel } from './traders/CreateTraderPanel';

export default function TradersSection({ onNavigate }: { onNavigate: (tab: 'models' | 'exchanges' | 'dashboard', opts?: { traderId?: string }) => void }) {
  const { data: traders, error, isLoading, runAction, mutate } = useTraders({ size: 50 });
  const [formOpen, setFormOpen] = useState(false);
  const [editing, setEditing] = useState<TraderResponse | null>(null);
  const [selectedId, setSelectedId] = useState<string | null>(null);

  // models/exchanges：SWR + 轮询（创建资源后自动刷新）
  const { data: models = [] } = useSWR<AIModel[]>(['/models', 'fleet'], () => modelApi.listModels(), { refreshInterval: 5000 });
  const { data: exchanges = [] } = useSWR<ExchangeAccount[]>(['/exchanges', 'fleet'], () => exchangeApi.listExchanges(), { refreshInterval: 5000 });

  const items = useMemo(() => traders?.items ?? [], [traders]);
  const activeCount = useMemo(() => items.filter((t) => t.status === 'running').length, [items]);
  const sysReady = activeCount > 0;

  // 默认选中第一个交易员
  useEffect(() => {
    if (!selectedId && items.length > 0) setSelectedId(items[0].id);
  }, [items, selectedId]);
  const selected = useMemo(() => items.find((t) => t.id === selectedId) ?? null, [items, selectedId]);

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

  const pausedCount = items.filter((t) => t.status === 'paused').length;
  const stoppedCount = items.filter((t) => t.status === 'stopped' || t.status === 'idle').length;

  return (
    <section>
      {/* 页头：标题 + ACTIVE_NODES + SYSTEM_READY 终端状态 */}
      <div className="fleet-head">
        <PageHead
          title={`交易员编队 ${activeCount} ACTIVE_NODES`}
          lead={<>每个交易员独立引擎 · 实时轮询</>}
        />
        <div className={`sys-status ${sysReady ? 'ok' : 'standby'}`}>
          <span className="led" />
          {sysReady ? 'SYSTEM_READY' : 'SYSTEM_STANDBY'}
        </div>
      </div>

      {/* 编队概览条：TOTAL / RUNNING / PAUSED / STOPPED */}
      <div className="fleet-summary">
        <SummaryCell label="TOTAL" value={items.length} />
        <SummaryCell label="RUNNING" value={activeCount} tone="ok" />
        <SummaryCell label="PAUSED" value={pausedCount} tone="warn" />
        <SummaryCell label="STOPPED" value={stoppedCount} tone="dim" />
      </div>

      {error && <div className="alert error">错误：{String(error)}</div>}
      {isLoading && <div className="muted mono">· 加载中…</div>}

      {/* 顶部快捷按钮组（MODELS_CONFIG / EXCHANGE_KEYS / Create Trader） */}
      <div className="fleet-actions glass-card">
        <button className="btn ghost" onClick={() => onNavigate('models')}>⬡ MODELS_CONFIG</button>
        <button className="btn ghost" onClick={() => onNavigate('exchanges')}>◐ EXCHANGE_KEYS</button>
        <span className="fleet-actions-spacer" />
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

      {/* 两栏主次：编队网格（全宽 2 列）| 右栏 sticky（资源侧栏 + 详情速览） */}
      <div className="fleet-layout">
        <TraderGrid
          items={items}
          selectedId={selectedId}
          onSelect={setSelectedId}
          modelNameOf={modelNameOf}
          accountOf={accountOf}
          runAction={runAction}
          onEdit={openEdit}
          onView={(t) => onNavigate('dashboard', { traderId: t.id })}
        />
        <aside className="fleet-aside">
          <ResourceSidebar models={models} exchanges={exchanges} onNavigate={onNavigate} />
          <InspectorPanel
            trader={selected}
            modelNameOf={modelNameOf}
            accountOf={accountOf}
            onOpenTerminal={(t) => onNavigate('dashboard', { traderId: t.id })}
          />
        </aside>
      </div>
    </section>
  );
}

function SummaryCell({ label, value, tone }: { label: string; value: number; tone?: 'ok' | 'warn' | 'dim' }) {
  return (
    <div className="fleet-summary-cell">
      <span className="fleet-summary-label">{label}</span>
      <span className={`fleet-summary-value ${tone ?? ''}`}>{value}</span>
    </div>
  );
}
