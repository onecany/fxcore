# Changelog

本项目所有重要变更记录。格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.0.0/)，语义化版本 [SemVer](https://semver.org/lang/zh-CN/)。

项目当前处于开发期（仅 `dev` 分支），尚未发布正式版本；`[0.1.0]` 为首次归档，覆盖仓库全部提交历史。

## [0.1.0] - 2026-08-18

首个归档版本，覆盖截至 2026-08-18 的全部 165 个提交。按主题分组如下。

### 架构与基础设施

- Go 后端 v1 API 分层：`internal/api/v1/`（handler → service → store → router），统一响应信封 `{ code, message, data }`（`code=0` 成功）
- `httpclient` 包（重导出 net/http 类型）；`mcp/` 独立 AI 模型访问层包（尚未接入主链路）
- HTTP/2 支持（TLS_CERT_FILE / TLS_KEY_FILE 同设时启用，ALPN 协商 h2）
- Swagger 注解 + OpenAPI 3.1 规范（docs/ 纯文档目录 + openapi3 embed 包，`GET /api/v1/openapi3.json`）
- 日志双写（文件 + 终端）：应用日志 fxcore.log + API 访问日志 access.log（JSON，lumberjack 轮转）
- 错误码矩阵（0/1001 参数/1002 token 失效/1003 无权限/1004 未找到/1005 限流/1101 refresh 轮换失败/1201 余额不足/1301-1302 AI 失败超时/1401-1420 交易域/1500-1501 内部与数据库错误），前后端硬编码同步

### 引擎（阶段 3）

- `internal/llm`：统一 AI 客户端，5 家 provider（deepseek/qwen/claude/gpt/gemini）+ custom（base_url 可配），超时分类 1301/1302
- `internal/provider`：K 线行情数据源链（hyperliquid → okx → coinank 故障切换），统一 OHLCV
- `internal/kernel`：决策解析容错链（剥 reasoning → fence 提取 → 全角修复 → 六值校验 → safe wait 兜底）+ 系统提示词构建（内置段英文 + 用户段 VERBATIM 透传）+ 技术指标计算（EMA/MACD/RSI/BOLL/ATR/Volume/OI/Funding 按 config 开关注入 K 线上下文，多时间框架支持）
- `internal/backtest`：历史行情回放引擎（状态机 + 执行 → 停止位/爆仓判定 + 权益/回撤/决策明细跟踪）
- `internal/debate`：辩论竞技场引擎（5 人格 + 轮次推进 + 置信加权投票共识 + SSE 事件流 initial/round_start/message/round_end/vote/consensus/error + 15s 心跳）
- `internal/trader/engine`：交易员执行引擎（每交易员独立 goroutine + 周期循环 + 风控守卫 + OrderSync/PositionBuilder），状态机 idle→running→paused→stopped

### 交易所适配器

- 统一 `Adapter` 接口（18 方法）+ 注册表模式（`Register` 可扩展）+ MockAdapter
- 七家真实适配器已注册：binance / bybit / okx / bitget / gate / kucoin / hyperliquid（hyperliquid 为 L1 ed25519 钱包签名）
- 各家签名协议与 SL/TP 差异处理；10 家数量格式纯函数
- 签名修复轮（2026-08，真实 key 实测）：OKX 时间戳改 ISO 8601 UTC 字符串（50102 修复）、OKX/KuCoin 签名串保留 query string、Gate 签名串补 timestamp 行、Bybit 补 api_key 与 GET query（50113 修复），回归测试锁定
- 其余 3 家（aster / lighter / indodax）API 契约待明确，未接入

### 存储与持久化

- GORM 双后端：`store.New(cfg, db)` 的 db 非 nil 走持久化，nil 纯内存（测试零改动）
- SQLite 默认 `data/fxcore.db`（WAL + busy_timeout）；`DB_DSN` 传 `mysql://` 或 `user:pass@tcp(...)` 切 MariaDB/MySQL
- **20 表 AutoMigrate 全量落库**：6 管理实体（users/models/exchanges/strategies/traders/telegram_configs）+ 14 流式运行时表（orders/fills/positions/decisions/equity_snapshots/backtest 5 表/debate 4 表），重启不丢
- 订单/成交幂等去重（`gorm.ErrDuplicatedKey` 翻译 + 既有行重查，含 sqlite busy 超时兜底）
- 嵌套结构体 `gorm:"serializer:json;type:text"` 落库（Trader 配置等 JSON 字段）

### 多用户隔离与安全

- 多用户数据隔离全链：List 按 `user_id` 过滤、Get/Update/Delete/控制端点归属校验（越权 404）、SSE 先校验后订阅、聚合端点子查询过滤、`migrateLegacyOwners` 存量数据迁移
- is_active 唯一激活、回测全局运行锁均按用户作用域化
- 认证：注册（邮箱 unique 双保险）/登录/登出，JWT HttpOnly Cookie（15min access + refresh 原子轮换，防重放 1101）
- 写操作 HMAC-SHA256 签名头（X-Timestamp / X-Nonce / X-Signature，验签后消耗 nonce，防重放与篡改）
- 限流：auth 5/min/IP、refresh 20/min、read 120、write 30（滑动窗口；Redis 可选，缺省内存降级）
- API Key / 交易所密钥 RSA-OAEP 加密落库（`RSA_PRIVATE_KEY` 必须持久化），响应零泄漏（仅脱敏前缀）
- SSRF 防护：探测客户端禁重定向 + 拨号层拒绝私网/环回地址（代理拨号放行）
- `r.SetTrustedProxies(nil)` 防 X-Forwarded-For 伪造绕过限流

### 忘记密码 / 重置密码（2026-08）

- `POST /auth/forgot-password`：防枚举（不存在邮箱同样 200 + 通用 message）；SMTP 配置后发邮件，未配置时 dev 模式写日志 + 响应 dev_link
- `POST /auth/reset-password`：一次性令牌（成功后作废，重放报 1001），bcrypt 哈希更新，密码 ≥8 位
- 独立 token 命名空间 `fx:reset:`（不复用登录 refresh TokenStore，防重置令牌变登录凭据）
- `internal/pkg/mail`：纯标准库 net/smtp 邮件器
- 前端 `/forgot-password` + `/reset-password?token=` 两页

### 前端（阶段 4）

- React 19 + TypeScript + Vite 工程基座：ESLint 9 flat config（--max-warnings 0）、vitest、openapi-typescript 类型生成、MSW
- 三语 i18n（中文 / EN / ID）：`translations.ts` 嵌套键 + zustand 持久化，顶栏 LangSwitch 即时切换
- 7 业务路由（仪表盘 / 编队 / 交易所 / 模型 / 策略 / 回测 / 辩论）+ 落地页 + 登录页 + 法律页（隐私/条款/Cookie）+ 忘记/重置密码页，react-router 8 BrowserRouter + AuthGate
- 设计系统：`--fxcore-*` CSS 设计令牌（Neo-Gold：深空蓝底 #070b14 + 金色主强调 #f0b90b），深色默认 + 浅色主题切换（themeStore + localStorage `fxcore-theme`），绿涨红跌
- 交易终端（TraderDashboard）：SWR 5s 轮询 + 权益曲线 SVG + 决策流/开放持仓/订单/平仓历史 + 分页（8/页决策卡、10/页表格）+ `?trader=` URL 直达 + K 线面板 + 交易所余额横条
- 编队页（Traders）：三栏信息架构（资源侧栏 / 编队网格 / 详情检查器）+ 创建/编辑面板（只选已接入账户、cycle 周期分钟制）
- 策略工作室：提示词双路径编辑（role_definition/trading_frequency/entry_standards/decision_process 四段循环渲染）+ 交易风格 4 档（均衡/激进/稳健/剥头皮）+ 币源 chips（全角逗号兼容）+ K 线参数与技术指标编辑 + AI 试跑面板（test-run 真实拉 K 线 + 指标进提示词）
- K 线图表（lightweight-charts 5，canvas 读 CSS 变量响应主题）+ 交易所图标下拉（自建组件 + createPortal 防覆盖）
- 品牌资产：fxcore-icon.svg（favicon/导航）+ fxcore-logo.svg（金色系，对齐 --fxcore-accent）
- 错误处理体系：全局 ErrorBoundary + 请求层 errMsg 统一错误转用户可读消息（网关/网络错误友好兜底）

### 测试与 CI

- handler 契约测试覆盖 12 个模块（auth/trader/strategy/debate/data/dashboard/model/exchange/telegram/crypto/backtest，8→53 个）
- 多用户隔离越权矩阵测试（backtest 8 端点 + debate 6 + strategy 5 + telegram 引用 + positions 过滤 + 聚合端点）
- 签名回归锁定测试（mock server 捕获请求头重算签名比对 + reduce-only 断言 + 时间戳格式解析）
- 冒烟脚本 `scripts/fxcore-hmac-smoke.sh`（登录取 sign_secret / 签名写 / 防重放 21 项）+ 假数据灌库脚本 `scripts/fxcore-seed-fixtures.py`（20 表全量、uuid、幂等）
- GitHub Actions CI（`.github/workflows/ci.yml`）：后端 go build / vet / test -race / staticcheck + 前端 npm ci / lint / build / test

### 修复与改进（Fix / Refactor）

- 服务重启后恢复 running/paused 交易员的引擎挂载（引擎 runner 纯内存态补齐）
- debate/backtest 存储竞态修复（写方法存副本 + 调用方锁内副本化，`go test -race` 零竞态）
- AI 决策单对象输出归一化为数组（模型不守契约时兜底）
- test-run 解析失败返回空数组而非 null + 前端判空防崩溃（TypeError 实锤修复）
- 合约/DTO 契约修复：pn_l 拼写统一、PositionDTO 8→23 字段、策略激活返回完整对象、debate 控制端点返回数据
- 交易所列表白名单扩到 10 家 + passphrase 类型修正；模型更新留空 key 保留原值
- 币源输入兼容中文全角逗号；全局错误渲染防 Error 对象直渲
- 静态托管缓存策略（/assets immutable + SPA no-cache）+ ExchangeIcon 图标路由

### 已知限制（Known Limitations）

- 三交易所适配器（aster/lighter/indodax）未接入；网格引擎独立迭代
- `/open-orders`、`/symbols`、`/competition`、`/top-traders` 为骨架端点（交易引擎未接 data handler 实时查询）
- `/debates/:id/stream` 等 SSE 端点需登录后访问；`fxcore/mcp` 包尚未接入主链路
- SMTP 未配置时密码重置走 dev 模式（链接写日志），生产必须配置 SMTP_HOST