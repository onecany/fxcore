// 统一错误码矩阵（API设计.md 第 5 节）
// 与 internal/middleware/error.go 硬编码同步，改动必须两端一致。
export const ERROR_CODES = {
  OK: 0,
  BAD_REQUEST: 1001, // 参数校验失败 -> 高亮字段
  TOKEN_INVALID: 1002, // Token 失效 -> 静默刷新队列
  FORBIDDEN: 1003, // 无权限 -> 隐藏按钮
  NOT_FOUND: 1004, // 资源不存在 / 状态机非法跳转 -> 404 占位
  RATE_LIMITED: 1005, // 限流 -> 读取 Retry-After 倒计时
  REFRESH_INVALID: 1101, // Refresh 无效 -> 强制登出
  INSUFFICIENT_BALANCE: 1201, // 余额不足 -> 跳转充值
  TRADER_RUNNING: 1204, // 交易员运行中 -> 禁用 Start
  AI_FAILED: 1301, // AI 失败 -> 显示重试按钮
  AI_TIMEOUT: 1302, // AI 超时 -> 自动重试 3 次
  INTERNAL: 1500, // 服务器内部错误（错误码矩阵扩展：服务端故障不再复用 1001/1301）
} as const;

interface ErrorMeta {
  http: number;
  label: string;
  action: string;
}

export const ERROR_META: Record<number, ErrorMeta> = {
  [ERROR_CODES.BAD_REQUEST]: { http: 400, label: '参数校验失败', action: 'highlight-fields' },
  [ERROR_CODES.TOKEN_INVALID]: { http: 401, label: 'Token 失效', action: 'silent-refresh-queue' },
  [ERROR_CODES.FORBIDDEN]: { http: 403, label: '无权限', action: 'hide-buttons' },
  [ERROR_CODES.NOT_FOUND]: { http: 404, label: '资源不存在', action: 'not-found-placeholder' },
  [ERROR_CODES.RATE_LIMITED]: { http: 429, label: '请求过于频繁', action: 'retry-after-countdown' },
  [ERROR_CODES.REFRESH_INVALID]: { http: 401, label: '登录已过期', action: 'force-logout' },
  [ERROR_CODES.INSUFFICIENT_BALANCE]: { http: 400, label: '余额不足', action: 'redirect-topup' },
  [ERROR_CODES.TRADER_RUNNING]: { http: 409, label: '交易员运行中', action: 'disable-start' },
  [ERROR_CODES.AI_FAILED]: { http: 503, label: 'AI 服务失败', action: 'show-retry' },
  [ERROR_CODES.AI_TIMEOUT]: { http: 504, label: 'AI 服务超时', action: 'auto-retry-3x' },
  [ERROR_CODES.INTERNAL]: { http: 500, label: '服务器内部错误', action: 'show-retry' },
};

/** 按业务码取可读消息 */
export function getErrorMessage(code: number, fallback = '请求失败'): string {
  return ERROR_META[code]?.label ?? fallback;
}

/** 按业务码取前端动作语义（用于页面统一处理分支） */
export function getErrorAction(code: number): string | undefined {
  return ERROR_META[code]?.action;
}
