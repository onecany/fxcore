// MSW Mock 认证处理器（模拟后端 snake_case 响应，client.ts 归一化逻辑一并被验证）。
import { http, HttpResponse } from 'msw';
import type { ApiResponse } from '../../../api/v1/types/contract';

const ok = <T>(data: T, requestId: string): ApiResponse<T> => ({
  code: 0,
  message: 'success',
  data,
  timestamp: Date.now(),
  requestId,
});

const err = (code: number, message: string, httpStatus: number, requestId = 'mock-error') =>
  new HttpResponse(JSON.stringify({ code, message, data: null, timestamp: Date.now(), requestId }), {
    status: httpStatus,
    headers: { 'Content-Type': 'application/json' },
  });

export const authHandlers = [
  http.post('/api/v1/auth/login', async ({ request }) => {
    const body = (await request.json()) as { email?: string; password?: string };
    if (!body.email || !body.password) {
      return err(1001, 'invalid email or password', 400);
    }
    return HttpResponse.json(
      ok(
        {
          user: { id: 'mock-user-1', email: body.email, nickname: 'Demo Trader' },
          access_token: 'mock-access-token',
          sign_secret: 'mock-sign-secret',
          refresh_token: 'mock-refresh-token',
        },
        'mock-login',
      ),
    );
  }),

  http.post('/api/v1/auth/refresh', async ({ request }) => {
    const body = (await request.json()) as { refresh_token?: string };
    if (!body.refresh_token) {
      return err(1101, 'refresh token invalid or expired', 401);
    }
    return HttpResponse.json(
      ok({ access_token: 'mock-access-token-2', refresh_token: 'mock-refresh-token-2' }, 'mock-refresh'),
    );
  }),

  http.post('/api/v1/auth/logout', () =>
    HttpResponse.json(ok(null, 'mock-logout')),
  ),
];
