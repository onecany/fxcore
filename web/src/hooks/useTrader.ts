// 交易员 hooks：列表 SWR + 状态机控制（乐观更新，失败回滚，文档 6.4）。
// 注意：乐观更新的 mutate key 必须与 useSWR 的 key 完全一致（数组 key 精确匹配），
// 因此动作与列表共用同一 key，避免打不中缓存。
import { useCallback, useMemo } from 'react';
import useSWR, { useSWRConfig } from 'swr';
import * as traderApi from '../api/v1/modules/traders';
import type { PaginatedData, TraderResponse, TraderStatus } from '../api/v1/types/contract';

export interface TraderListParams {
  page?: number;
  size?: number;
  status?: TraderStatus;
}

export type TraderAction = 'start' | 'pause' | 'resume' | 'stop';

const ACTION_STATUS: Record<TraderAction, TraderStatus> = {
  start: 'running',
  pause: 'paused',
  resume: 'running',
  stop: 'stopped',
};

const ACTION_API: Record<TraderAction, (id: string) => Promise<null>> = {
  start: traderApi.startTrader,
  pause: traderApi.pauseTrader,
  resume: traderApi.resumeTrader,
  stop: traderApi.stopTrader,
};

export function useTraders(params: TraderListParams = {}) {
  const key = useMemo(() => ['/traders', params] as const, [params]);
  const { mutate } = useSWRConfig();
  const { data, error, isLoading, mutate: mutateList } = useSWR<PaginatedData<TraderResponse>>(
    key,
    () => traderApi.listTraders(params),
    { keepPreviousData: true, refreshInterval: 5000 },
  );

  /**
   * 对指定交易员执行生命周期动作（文档 6.4 乐观更新）：
   * 1. 先写乐观值（revalidate:false —— 关键：若先 revalidate 会发出 GET 拿旧状态覆盖乐观值）
   * 2. 再调后端动作 API
   * 3. 成功后 revalidate 拉取权威状态；失败时 revalidate 回滚为服务器真实状态
   */
  const runAction = useCallback(
    async (id: string, action: TraderAction) => {
      const nextStatus = ACTION_STATUS[action];

      // 本地乐观更新（SWR MutatorCallback 必须返回非 undefined 数据）
      const patch = (current?: PaginatedData<TraderResponse>): PaginatedData<TraderResponse> =>
        current
          ? {
              ...current,
              items: current.items.map((t) => (t.id === id ? { ...t, status: nextStatus } : t)),
            }
          : { items: [], pagination: { page: 1, pageSize: 20, total: 0, totalPages: 1 } };

      await mutate(key, patch, {
        optimisticData: patch,
        rollbackOnError: true,
        revalidate: false, // 防止动作 API 前的旧状态 GET 覆盖乐观值（L1）
      });
      try {
        await ACTION_API[action](id);
        await mutate(key, undefined, { revalidate: true }); // 成功后取权威状态
      } catch (err) {
        // 服务器拒绝（如 1004 非法迁移 / 1204 运行中）→ 强制重取还原真实状态
        await mutate(key, undefined, { revalidate: true });
        throw err;
      }
    },
    [key, mutate],
  );

  return { data, error, isLoading, mutate: mutateList, runAction };
}
