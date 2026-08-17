// FXcore-web API v1 认证模块。
import client from '../client';
import type { ForgotPasswordResponse, LoginRequest, LoginResponse, ResetPasswordResponse } from '../types/contract';

/** 登录：后端设置 HttpOnly Cookie，响应返回 accessToken + signSecret */
export function login(req: LoginRequest): Promise<LoginResponse> {
  return client.post<LoginResponse>('/auth/login', req);
}

/** 注册：创建账号并直接登录（响应结构同 login） */
export function register(req: { email: string; password: string; nickname?: string }): Promise<LoginResponse> {
  return client.post<LoginResponse>('/auth/register', req);
}

/** 刷新（Refresh 轮换）：由 client.ts 刷新队列内部调用，一般不直接使用 */
export function refresh(refreshToken: string): Promise<{ accessToken: string; refreshToken?: string; signSecret?: string }> {
  return client.post('/auth/refresh', { refresh_token: refreshToken });
}

/**
 * 登出：清除 Cookie + 服务端吊销 refresh token（S1）。
 * 不吊销的话，XSS 窃取的 refresh token 在登出后仍可换新 access token（持久接管）。
 */
export function logout(refreshToken?: string): Promise<null> {
  return client.post<null>('/auth/logout', refreshToken ? { refresh_token: refreshToken } : {});
}

/** 忘记密码：向注册邮箱发送重置邮件（邮箱不存在时同样返回成功，防枚举） */
export function forgotPassword(email: string): Promise<ForgotPasswordResponse> {
  return client.post<ForgotPasswordResponse>('/auth/forgot-password', { email });
}

/** 重置密码：携带邮件链接中的一次性令牌设置新密码（成功后令牌作废） */
export function resetPassword(token: string, password: string): Promise<ResetPasswordResponse> {
  return client.post<ResetPasswordResponse>('/auth/reset-password', { token, password });
}
