// 编队页资源侧栏：AI MODELS + EXCHANGES 卡（状态灯 STANDBY/ACTIVE），CONFIG 跳转。
import { Panel } from '../../components/ui';
import { ExchangeIcon } from '../../components/exchange-icon';
import type { AIModel, ExchangeAccount } from '../../api/v1/types/contract';

export function ResourceSidebar({ models, exchanges, onNavigate }: {
  models: AIModel[];
  exchanges: ExchangeAccount[];
  onNavigate: (tab: 'models' | 'exchanges', opts?: { traderId?: string }) => void;
}) {
  return (
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
  );
}

function ResourcePanel({ title, count, empty, onAction, children }: {
  title: string; count: number; empty: string; onAction: () => void; children: React.ReactNode;
}) {
  return (
    <Panel title={`${title} (${count})`}>
      {count === 0 ? (
        <div className="muted mono empty-hint">{empty}</div>
      ) : (
        <div className="resource-stack">{children}</div>
      )}
      <button className="btn ghost btn-sm resource-cta" onClick={onAction}>
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
        <span className="resource-name-wrap">
          {icon}
          <span className="resource-name">{name}</span>
        </span>
        <span className={`sys-status ${tone} resource-status`}>
          <span className="led" />{status}
        </span>
      </div>
      <div className="resource-meta">{meta}</div>
    </div>
  );
}
