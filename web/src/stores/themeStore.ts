// 主题状态（深色默认 / 浅色切换）：localStorage 持久化 + documentElement data-theme + colorScheme。
import { create } from 'zustand';

export type Theme = 'dark' | 'light';

const KEY = 'fxcore-theme';

const getInitial = (): Theme => {
  try {
    return localStorage.getItem(KEY) === 'light' ? 'light' : 'dark';
  } catch {
    return 'dark';
  }
};

const apply = (t: Theme) => {
  document.documentElement.dataset.theme = t;
  document.documentElement.style.colorScheme = t;
};

export const useTheme = create<{ theme: Theme; init: () => void; toggle: () => void }>((set, get) => ({
  theme: getInitial(),
  init: () => {
    const t = getInitial();
    apply(t);
    set({ theme: t });
  },
  toggle: () => {
    const next = get().theme === 'dark' ? 'light' : 'dark';
    try { localStorage.setItem(KEY, next); } catch { /* 忽略存储失败 */ }
    apply(next);
    set({ theme: next });
  },
}));
