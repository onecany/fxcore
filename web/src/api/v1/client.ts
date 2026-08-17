// FXcore-web API v1 Axios 客户端（API设计.md 第 6 节）。
//
// 核心职责：
// 1. baseURL 由 VITE_API_VERSION 控制 -> /api/v1（版本切换，文档 §8）
// 2. withCredentials 携带 HttpOnly Cookie（Access Token，15min）
// 3. 写操作附加 X-Signature: HMAC-SHA256(timestamp + path + body, user_secret)（文档 6.1）
// 4. 401(1002) 静默刷新队列：多个并发 401 只触发一次 /auth/refresh（文档 §8 防错清单）
// 5. 1101 强制登出；1005 读取 Retry-After
// 6. 键名归一化：后端 JSON Tag 为 snake_case（文档 §9），前端契约为 camelCase（文档 §3），
//    本文件统一转换，业务模块只接触 camelCase。这是文档两处要求的唯一汇合点。
import axios, { AxiosError, AxiosRequestConfig, AxiosResponse, InternalAxiosRequestConfig } from 'axios';
import { useAppStore } from '../../stores/appStore';
import { getErrorMessage } from '../../utils/errorCodes';
import type { ApiResponse } from './types/contract';

const version = import.meta.env.VITE_API_VERSION || 'v1';
export const API_BASE_URL = `/api/${version}`;

const WRITE_METHODS = new Set(['post', 'put', 'patch', 'delete']);
const AUTH_PATHS = ['/auth/login', '/auth/refresh'];

/** 业务错误（携带后端业务码 + HTTP 状态 + 1001 字段高亮 + requestId） */
export class AppError extends Error {
  code: number;
  httpStatus: number;
  retryAfter?: number;
  /** 1001 参数校验失败时的字段级错误映射（后端信封 data） */
  fields?: Record<string, string>;
  requestId?: string;

  constructor(
    code: number,
    httpStatus: number,
    message: string,
    opts?: { retryAfter?: number; fields?: Record<string, string>; requestId?: string },
  ) {
    super(message);
    this.name = 'AppError';
    this.code = code;
    this.httpStatus = httpStatus;
    this.retryAfter = opts?.retryAfter;
    this.fields = opts?.fields;
    this.requestId = opts?.requestId;
  }
}

// ========== 键名归一化（snake_case <-> camelCase） ==========

const toCamel = (s: string): string =>
  s.replace(/_([a-z])/g, (_, ch: string) => ch.toUpperCase());

const toSnake = (s: string): string =>
  s.replace(/[A-Z]/g, (ch: string) => `_${ch.toLowerCase()}`);

function mapKeys(value: unknown, fn: (k: string) => string): unknown {
  if (Array.isArray(value)) return value.map((v) => mapKeys(v, fn));
  if (value !== null && typeof value === 'object') {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      out[fn(k)] = mapKeys(v, fn);
    }
    return out;
  }
  return value;
}

// ========== HMAC-SHA256 签名（Web Crypto） ==========

async function hmacSha256Hex(message: string, secret: string): Promise<string> {
  const enc = new TextEncoder();
  const key = await crypto.subtle.importKey(
    'raw',
    enc.encode(secret),
    { name: 'HMAC', hash: 'SHA-256' },
    false,
    ['sign'],
  );
  const sig = await crypto.subtle.sign('HMAC', key, enc.encode(message));
  return Array.from(new Uint8Array(sig))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

/** 一次性 nonce（S6 防重放）：随机十六进制串 */
function randomHex(bytes: number): string {
  const buf = new Uint8Array(bytes);
  crypto.getRandomValues(buf);
  return Array.from(buf)
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

async function attachSignature(config: InternalAxiosRequestConfig): Promise<void> {
  const method = (config.method || 'get').toLowerCase();
  const url = config.url || '';
  if (!WRITE_METHODS.has(method)) return;
  if (AUTH_PATHS.some((p) => url.startsWith(p))) return; // 登录/刷新无法签名

  const secret = useAppStore.getState().signSecret;
  if (!secret) return; // 未登录时无签名，服务端对 /auth/* 之外会拒绝

  const ts = Math.floor(Date.now() / 1000).toString();
  const nonce = randomHex(16);
  const body = config.data ? JSON.stringify(config.data) : '';
  const path = `${config.baseURL || ''}${url}`;
  // 签名串: timestamp + path + body + nonce（S6：nonce 必须在签名内，否则可换 nonce 重放）
  const signature = await hmacSha256Hex(`${ts}${path}${body}${nonce}`, secret);
  config.headers.set('X-Timestamp', ts);
  config.headers.set('X-Nonce', nonce);
  config.headers.set('X-Signature', signature);
}

// ========== 刷新单飞队列 ==========

let refreshPromise: Promise<boolean> | null = null;

async function tryRefresh(): Promise<boolean> {
  if (refreshPromise) return refreshPromise;
  refreshPromise = (async () => {
    const rt = useAppStore.getState().refreshToken;
    if (!rt) return false;
    try {
      // 注意：此处用裸 axios（绕开拦截器避免递归刷新），后端响应为 snake_case
      const resp = await axios.post<ApiResponse<{ access_token: string; refresh_token?: string; sign_secret?: string }>>(
        `${API_BASE_URL}/auth/refresh`,
        { refresh_token: rt },
        { withCredentials: true, timeout: 10000 },
      );
      if (resp.data.code === 0) {
        const d = resp.data.data;
        useAppStore.getState().setAccessToken(d.access_token);
        if (d.refresh_token) {
          useAppStore.getState().setRefreshToken(d.refresh_token); // 轮换
        }
        if (d.sign_secret) {
          // reload 后 signSecret 只存内存会丢；刷新时后端重新下发，写操作才能继续签名（S3）
          useAppStore.getState().setSignSecret(d.sign_secret);
        }
        return true;
      }
      return false;
    } catch {
      return false;
    } finally {
      refreshPromise = null;
    }
  })();
  return refreshPromise;
}

// 401 重放标记：同一请求最多刷新重放 1 次（S4，防无限刷新轮换循环）
const RETRY_MARK = '_fxRetried';

// ========== 客户端实例 ==========

// ApiClient 自定义类型：拦截器已解信封并返回 data 本体，
// 因此 get/post/put/patch/delete 直接返回 T 而非 AxiosResponse<T>。
export interface ApiClient {
  get<T = unknown>(url: string, config?: AxiosRequestConfig): Promise<T>;
  post<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T>;
  put<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T>;
  patch<T = unknown>(url: string, data?: unknown, config?: AxiosRequestConfig): Promise<T>;
  delete<T = unknown>(url: string, config?: AxiosRequestConfig): Promise<T>;
}

const instance = axios.create({
  baseURL: API_BASE_URL,
  withCredentials: true,
  timeout: 15000,
  headers: {
    'Content-Type': 'application/json',
    'Accept-Encoding': 'gzip, br',
    'X-Requested-With': 'XMLHttpRequest',
  },
});

// 请求：camelCase -> snake_case + 签名
instance.interceptors.request.use(async (config) => {
  if (config.data && typeof config.data === 'object') {
    config.data = mapKeys(config.data, toSnake) as never;
  }
  await attachSignature(config);
  return config;
});

// 响应：解信封 + snake_case -> camelCase + 业务码分支
instance.interceptors.response.use(
  (resp: AxiosResponse<ApiResponse<unknown>>) => {
    const body = resp.data;
    if (body && typeof body.code === 'number') {
      if (body.code === 0) {
        return mapKeys(body.data, toCamel) as never;
      }
      return Promise.reject(new AppError(body.code, resp.status, body.message || getErrorMessage(body.code)));
    }
    return resp.data as never;
  },
  async (error: AxiosError<ApiResponse<unknown>>) => {
    const status = error.response?.status;
    const code = error.response?.data?.code;
    // 网络层兜底（后端服务不可用/断网/超时/网关错误）：
    // 给用户友好中文消息，不把 "Network Error" 等原始英文错误抛到页面。
    // 有后端信封（含 code 字段）的 5xx（如 1301/1302）仍保留后端 message。
    const hasEnvelope =
      error.response?.data && typeof (error.response.data as ApiResponse<unknown> | undefined)?.code === 'number';
    let message: string;
    if (!error.response) {
      message =
        error.code === 'ECONNABORTED'
          ? '请求超时，请检查网络后重试'
          : '无法连接服务器，请检查网络或稍后重试';
    } else if (status && status >= 500 && !hasEnvelope) {
      message = '服务器暂时不可用，请稍后重试';
    } else {
      message = error.response?.data?.message || getErrorMessage(code ?? -1, error.message);
    }
    // L7：错误信封与成功信封统一 request_id（原始 snake_case 字段，绕过类型声明的 requestId）
    const rawBody = error.response?.data as (ApiResponse<unknown> & { request_id?: string }) | undefined;
    const requestId = rawBody?.request_id;
    // 1001 字段高亮映射：后端 WriteError 把 fields 放进信封 data
    const fields = code === 1001 && error.response?.data?.data ? (error.response.data.data as Record<string, string>) : undefined;

    // 1002 Token 失效 -> 单飞刷新后重放原请求（同一请求最多重放 1 次）
    const alreadyRetried = Boolean((error.config as { [RETRY_MARK]?: boolean } | undefined)?.[RETRY_MARK]);
    if (status === 401 && code === 1002 && !(error.config?.url || '').startsWith('/auth/') && !alreadyRetried && error.config) {
      const ok = await tryRefresh();
      if (ok) {
        (error.config as { [RETRY_MARK]?: boolean })[RETRY_MARK] = true;
        return instance(error.config); // 重新走请求拦截器（重签名）
      }
      useAppStore.getState().clearAuth();
      window.dispatchEvent(new CustomEvent('fx:unauthorized'));
      return Promise.reject(new AppError(1101, 401, '登录已过期，请重新登录'));
    }

    // L10：重放后仍 401/1002（刷新轮换了 token 但请求仍被拒，如账号停用）——
    // 不再静默返回 1002 卡死会话，按强制登出处理
    if (status === 401 && code === 1002 && alreadyRetried) {
      useAppStore.getState().clearAuth();
      window.dispatchEvent(new CustomEvent('fx:unauthorized'));
      return Promise.reject(new AppError(1101, 401, '登录已过期，请重新登录'));
    }

    // 1005 限流 -> 带 Retry-After（兼容秒数与 HTTP-date）
    if (code === 1005) {
      const raw = error.response?.headers?.['retry-after'];
      let retryAfter = 0;
      if (raw) {
        const n = Number(raw);
        retryAfter = Number.isFinite(n) ? n : Math.max(0, Math.ceil((Date.parse(raw) - Date.now()) / 1000));
      }
      return Promise.reject(new AppError(code, status ?? 429, message, { retryAfter }));
    }

    return Promise.reject(new AppError(code ?? -1, status ?? 0, message, { fields, requestId }));
  },
);

export default instance as unknown as ApiClient;
