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
  // ---- §12 交易域错误码扩展 ----
  EXCHANGE_FAILED: 1401, // 交易所连接失败 -> 显示重试 + 检查密钥
  RISK_REJECTED: 1402, // 风控拒绝 -> 展示风控原因
  POSITION_EXISTS: 1403, // 同方向持仓已存在 -> 提示当前持仓
  CLOSE_POSITION_FAILED: 1404, // 平仓失败/无持仓 -> 刷新持仓
  DECISION_PARSE_FAILED: 1405, // AI 决策解析失败 -> 显示降级状态
  GRID_PAUSED: 1410, // 网格策略暂停 -> 显示 regime 状态
  BACKTEST_LOCK: 1411, // 回测运行锁冲突 -> 等待或停止
  BACKTEST_NOT_FOUND: 1412, // 回测 run 不存在 -> 404 占位
  DEBATE_NOT_EXECUTABLE: 1420, // 辩论不可执行 -> 禁用执行按钮
  INTERNAL: 1500, // 服务器内部错误（错误码矩阵扩展：服务端故障不再复用 1001/1301）
  DB_ERROR: 1501, // 数据库错误 -> 通用错误
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
  [ERROR_CODES.EXCHANGE_FAILED]: { http: 400, label: '交易所连接失败', action: 'show-retry-check-key' },
  [ERROR_CODES.RISK_REJECTED]: { http: 400, label: '风控拒绝', action: 'show-risk-reason' },
  [ERROR_CODES.POSITION_EXISTS]: { http: 409, label: '同方向持仓已存在', action: 'show-current-position' },
  [ERROR_CODES.CLOSE_POSITION_FAILED]: { http: 400, label: '平仓失败或无可平持仓', action: 'refresh-positions' },
  [ERROR_CODES.DECISION_PARSE_FAILED]: { http: 503, label: 'AI 决策解析失败，已降级', action: 'show-degraded' },
  [ERROR_CODES.GRID_PAUSED]: { http: 409, label: '网格策略已暂停（趋势行情）', action: 'show-regime-status' },
  [ERROR_CODES.BACKTEST_LOCK]: { http: 400, label: '回测运行中，无法重复启动', action: 'wait-or-stop' },
  [ERROR_CODES.BACKTEST_NOT_FOUND]: { http: 404, label: '回测任务不存在', action: 'not-found-placeholder' },
  [ERROR_CODES.DEBATE_NOT_EXECUTABLE]: { http: 409, label: '辩论未完成或无开仓共识', action: 'disable-execute' },
  [ERROR_CODES.INTERNAL]: { http: 500, label: '服务器内部错误', action: 'show-retry' },
  [ERROR_CODES.DB_ERROR]: { http: 500, label: '数据服务异常', action: 'show-retry' },
};

/** 按业务码取可读消息 */
export function getErrorMessage(code: number, fallback = '请求失败'): string {
  return ERROR_META[code]?.label ?? fallback;
}

/** 按业务码取前端动作语义（用于页面统一处理分支） */
export function getErrorAction(code: number): string | undefined {
  return ERROR_META[code]?.action;
}
