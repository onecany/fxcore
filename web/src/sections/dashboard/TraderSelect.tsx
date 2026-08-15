// 交易员选择器（dashboard 顶栏，组件化便于复用与样式统一）。
import type { TraderResponse } from '../../api/v1/types/contract';

export function TraderSelect({ traders, traderId, onSelect }: {
  traders: TraderResponse[];
  traderId: string;
  onSelect: (id: string) => void;
}) {
  return (
    <div className="ts-wrap">
      <span className="ts-label">交易员</span>
      <select
        className="tm-select"
        value={traderId}
        onChange={(e) => onSelect(e.target.value)}
        aria-label="交易员选择"
      >
        {traders.map((t) => (
          <option key={t.id} value={t.id}>{t.name} · {t.exchange}</option>
        ))}
      </select>
    </div>
  );
}
