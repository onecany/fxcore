// 仪表盘 SWR 轮询 hook（文档 6.4）：页面可见 5s 轮询，后台暂停。
import useSWR from 'swr';
import { getDashboard } from '../api/v1/modules/dashboard';
import type { DashboardSummary } from '../api/v1/types/contract';

function isPageVisible(): boolean {
  return typeof document === 'undefined' ? true : !document.hidden;
}

export function useDashboard() {
  return useSWR<DashboardSummary>('/dashboard', () => getDashboard(), {
    refreshInterval: () => (isPageVisible() ? 5000 : 0), // 可见 5s，后台暂停（文档 6.4）
    revalidateOnFocus: true,
    dedupingInterval: 2000,
  });
}
