// 认证 hooks：登录/登出 + 会话状态（Zustand 桥接）。
import { useCallback } from 'react';
import * as authApi from '../api/v1/modules/auth';
import { useAppStore } from '../stores/appStore';
import type { LoginRequest } from '../api/v1/types/contract';

export function useAuth() {
  const { user, accessToken, setAuth, clearAuth } = useAppStore();

  const login = useCallback(
    async (req: LoginRequest) => {
      const resp = await authApi.login(req);
      setAuth({
        user: resp.user,
        accessToken: resp.accessToken,
        signSecret: resp.signSecret,
        refreshToken: resp.refreshToken,
      });
      return resp.user;
    },
    [setAuth],
  );

  const register = useCallback(
    async (req: { email: string; password: string; nickname?: string }) => {
      const resp = await authApi.register(req);
      setAuth({
        user: resp.user,
        accessToken: resp.accessToken,
        signSecret: resp.signSecret,
        refreshToken: resp.refreshToken,
      });
      return resp.user;
    },
    [setAuth],
  );

  const logout = useCallback(async () => {
    try {
      // 携带 refresh token 让服务端吊销（S1：登出后 XSS 窃取的 token 不能继续换新）
      await authApi.logout(useAppStore.getState().refreshToken ?? undefined);
    } finally {
      clearAuth();
    }
  }, [clearAuth]);

  return { user, accessToken, isAuthenticated: !!accessToken, login, register, logout };
}

/** 监听 401 刷新失败（1101 强制登出）事件 */
export function onUnauthorized(handler: () => void): () => void {
  const listener = () => handler();
  window.addEventListener('fx:unauthorized', listener);
  return () => window.removeEventListener('fx:unauthorized', listener);
}
