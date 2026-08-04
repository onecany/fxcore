// 全局应用状态（Zustand）。
// 注意：signSecret（请求签名 user_secret）只存内存，绝不落盘；
// refreshToken 存 localStorage（XSS 风险换刷新持久性，见 data/API设计.md 复盘）。
import { create } from 'zustand';
import type { User } from '../api/v1/types/contract';

const REFRESH_TOKEN_KEY = 'fx_refresh_token';

export interface AuthPayload {
  user: User;
  accessToken: string;
  signSecret?: string;
  refreshToken?: string;
}

interface AppState {
  user: User | null;
  accessToken: string | null;
  /** HMAC-SHA256 请求签名密钥，仅登录/刷新时下发，仅存内存 */
  signSecret: string | null;
  refreshToken: string | null;
  setAuth: (payload: AuthPayload) => void;
  setAccessToken: (token: string) => void;
  setRefreshToken: (token: string) => void;
  setSignSecret: (secret: string) => void;
  clearAuth: () => void;
}

export const useAppStore = create<AppState>((set) => ({
  user: null,
  accessToken: null,
  signSecret: null,
  refreshToken: typeof localStorage !== 'undefined' ? localStorage.getItem(REFRESH_TOKEN_KEY) : null,

  setAuth: ({ user, accessToken, signSecret, refreshToken }) => {
    if (refreshToken) {
      localStorage.setItem(REFRESH_TOKEN_KEY, refreshToken);
    } else {
      // 无新 token 时清理旧值，避免 store 与 localStorage 分叉（L4）
      localStorage.removeItem(REFRESH_TOKEN_KEY);
    }
    set({
      user,
      accessToken,
      signSecret: signSecret ?? null,
      refreshToken: refreshToken ?? null,
    });
  },

  setAccessToken: (token) => set({ accessToken: token }),

  setRefreshToken: (token) => {
    localStorage.setItem(REFRESH_TOKEN_KEY, token);
    set({ refreshToken: token });
  },

  setSignSecret: (secret) => set({ signSecret: secret }),

  clearAuth: () => {
    localStorage.removeItem(REFRESH_TOKEN_KEY);
    set({ user: null, accessToken: null, signSecret: null, refreshToken: null });
  },
}));
