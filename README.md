# FXcore

自主加密货币交易智能体平台 —— Go 后端（行情聚合、交易执行、AI 决策调度）+ React 前端（交易终端与配置界面）。

Autonomous crypto trading agent platform — Go backend (market aggregation, order execution, AI decision orchestration) + React frontend (trading terminal & configuration UI).

变更记录见 [CHANGELOG.md](CHANGELOG.md)。/ See [CHANGELOG.md](CHANGELOG.md) for release history.

## 功能概览 / Features

- **交易员编队**：多交易员独立引擎，状态机（idle / running / paused / stopped），5s 轮询实时监控，重启自动恢复运行中交易员
- **AI 决策**：接入多家 LLM（deepseek / qwen / claude / gpt / gemini / custom），RSA-OAEP 加密 API Key 落库，技术指标（EMA/MACD/RSI/BOLL/ATR 等）注入 K 线上下文
- **策略工作室**：双路径提示词（四段可编辑）、四种交易风格（均衡/激进/稳健/剥头皮）、全局唯一激活策略、AI 试跑预览
- **7 家交易所适配器**：binance / bybit / okx / bitget / gate / kucoin / hyperliquid，注册表模式可扩展；账户余额实时查询
- **回放引擎**：历史行情回放、K 线预览（lightweight-charts）、进度/权益/回撤跟踪
- **辩论竞技场**：多模型观点碰撞、轮次推进、共识投票、SSE 事件流、共识执行下发
- **认证与安全**：注册/登录、HttpOnly Cookie 认证、写操作 HMAC-SHA256 签名头、凭据加密存储、多用户数据隔离、密码找回（SMTP 邮件）
- **前端体验**：三语 i18n（中文 / EN / ID）、深色/浅色主题切换、顶栏语言与主题即时切换、落地页/法律页齐全

## 架构 / Architecture

```
┌─────────────────────────────────────────────┐
│  web/ (React 19 + TS + Vite)                 │
│  pages/ → sections/ → components/            │
│  api/v1/ (axios 客户端 + 类型契约)            │
└──────────────────┬──────────────────────────┘
                   │ REST /api/v1 + WS /ws/dashboard
┌──────────────────▼──────────────────────────┐
│  internal/api/v1  (Gin)                      │
│  handler → service → store (GORM)            │
│  middleware: JWT + HMAC 签名 + 限流          │
├─────────────────────────────────────────────┤
│  internal/trader/engine  交易员执行引擎       │
│  internal/debate         辩论引擎 (SSE)      │
│  internal/backtest       回放引擎            │
│  internal/exchange       7 家交易所适配器    │
│  internal/provider       行情数据源链        │
│  internal/llm            LLM 适配            │
│  internal/kernel         决策解析与提示词构建  │
└─────────────────────────────────────────────┘
```

适配器注册表（`internal/exchange` 的 `Register`）当前已注册 7 家真实适配器：binance / bybit / okx / bitget / gate / kucoin / hyperliquid（另有 `mock` 用于测试）；aster / lighter / indodax 契约待明确，尚未接入。

## 技术栈 / Tech Stack

**后端**：Go 1.26 · Gin · GORM · SQLite（默认）/ MariaDB · JWT · RSA-OAEP · SSE · net/smtp（邮件）

**前端**：React 19 · TypeScript 5.7 · Vite 6 · react-router 8 · SWR（轮询/缓存）· zustand（全局状态）· lightweight-charts 5（K线）· axios · MSW（Mock）· Vitest

**工程化**：ESLint 9 flat config（零警告门禁）· openapi-typescript（契约生成）· GitHub Actions CI · 静态托管（web/dist）· Swagger / OpenAPI 3.1

## 快速开始 / Quick Start

### 1. 构建前端

```bash
npm --prefix web install
npm --prefix web run build      # tsc && vite build → web/dist
```

### 2. 启动后端（托管 web/dist）

```bash
go build -o /tmp/fxcore-server ./cmd/server
PORT=6080 STATIC_DIR=web/dist /tmp/fxcore-server
```

访问 `http://localhost:6080`。

### 3. RSA 私钥（必须持久化）

API Key 用 RSA-OAEP 加密落库。**必须提供 `RSA_PRIVATE_KEY` 环境变量（2048 位），且永远保持不变**——私钥丢失 = 存量密文永久不可解。

```bash
# 生成私钥（PKCS#8）
openssl genpkey -algorithm RSA -pkeyopt rsa_keygen_bits:2048 -out ~/.fxcore/rsa_private.pem

# 启动时加载（两种形态均可：base64(PEM) 或纯 PEM）
export RSA_PRIVATE_KEY="$(base64 < ~/.fxcore/rsa_private.pem)"
PORT=6080 STATIC_DIR=web/dist /tmp/fxcore-server
```

## 环境变量 / Environment

| 变量 | 默认 | 说明 |
|---|---|---|
| `PORT` | `8080` | 监听端口 |
| `STATIC_DIR` | - | 前端构建产物目录（web/dist），提供静态托管 |
| `DATA_DIR` | `data` | SQLite 数据库目录（库文件 `data/fxcore.db`，WAL 模式） |
| `DB_DSN` | SQLite | MariaDB/MySQL 连接串（`mysql://` 前缀或 `user:pass@tcp(...)`） |
| `RSA_PRIVATE_KEY` | 临时生成 | RSA 私钥（PEM 或 base64(PEM)），**必须持久化** |
| `JWT_SECRET` | 自动生成 | JWT 签名密钥（重启失效，需重新登录） |
| `REDIS_URL` | - | 可选，refresh token / 限流 / nonce 存储，缺省内存降级 |
| `CORS_ORIGINS` | - | 额外跨域白名单（逗号分隔） |
| `ADMIN_EMAIL` / `ADMIN_PASSWORD` | `admin@fxcore.local` / `admin123` | 种子管理员账号（生产必须改密码） |
| `ADMIN_SIGN_SECRET` | 自动生成 | 管理员请求签名密钥 |
| `LOG_DIR` | `logs` | 日志目录（fxcore.log + access.log，设空串禁用文件日志） |
| `ENV_FILE` | `.env` | 环境变量文件路径（加载顺序：进程环境 > .env） |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` | - | 同设才启用 HTTPS + HTTP/2 |
| `HYPERLIQUID_BASE_URL` / `OKX_BASE_URL` | 公网 | 行情数据源覆盖（测试用） |
| `SMTP_HOST` | - | 配置后启用密码找回邮件；留空 = dev 模式（链接写日志 + 响应 dev_link） |
| `SMTP_PORT` / `SMTP_USER` / `SMTP_PASS` / `SMTP_FROM` | `587` / - | SMTP 连接参数（留空 = 无认证） |
| `FRONTEND_BASE_URL` | `http://localhost:5173` | 前端地址，拼密码重置链接 |

## API 概览 / API Overview

统一信封：`{ code, message, data }`（`code=0` 成功）。认证：HttpOnly Cookie + 写操作 HMAC 签名头（`X-Timestamp` / `X-Nonce` / `X-Signature`）。

| 模块 | 端点 |
|---|---|
| Auth | `POST /api/v1/auth/login` `/register` `/logout` `/forgot-password` `/reset-password` · `POST /auth/refresh` |
| 模型 | `GET/POST /api/v1/models` `PUT/DELETE /models/:id` `POST /models/:id/test` `GET /models/providers` |
| 交易员 | `GET/POST /api/v1/traders` `GET/PATCH /traders/:id` `POST /traders/:id/start|pause|resume|stop` |
| 仪表盘 | `GET /api/v1/dashboard` · `WS /ws/dashboard`（`?token=`） |
| 持仓 | `GET /api/v1/positions` `DELETE /positions/:id` |
| 交易所 | `GET/POST /api/v1/exchanges` `GET /exchanges/balances` `PUT/DELETE /exchanges/:id` |
| 策略 | `GET/POST /api/v1/strategies` `PUT/DELETE /strategies/:id` `POST /strategies/preview-prompt|test-run` `POST /strategies/:id/activate|duplicate` `GET /strategies/active|default-config|public` |
| 数据 | `GET /api/v1/account` `/decisions` `/orders` `/orders/:id/fills` `/trades` `/statistics` `/equity-history` `/equity-history-batch` `/klines` `/positions/history` `/status` 等 |
| 回测 | `POST /api/v1/backtest/start` `POST /backtest/:action`（pause/resume/stop/label/delete） `GET /backtest/status|runs|equity|trades|metrics|trace|decisions|export|klines` |
| 辩论 | `GET/POST /api/v1/debates` `GET /debates/personalities` `POST /debates/:id/:action` `POST /debates/:id/execute` `GET /debates/:id/messages|votes|stream`(SSE) |
| 加密 | `GET /api/v1/crypto/public-key|config` `POST /crypto/decrypt` |
| Telegram | `GET/POST /api/v1/telegram` `POST /telegram/model` `DELETE /telegram/binding` |

完整契约见 `internal/api/v1/openapi3/swagger.json`（前端类型生成：`npm --prefix web run gen:api`）。

## 前端结构 / Frontend Layout

```
web/src/
├── pages/          # 页面级组件（落地/登录/模型/策略/交易所/回测/辩论/法律/密码重置）
├── sections/       # 业务区块（交易终端 / 编队 + auth|dashboard|strategies|traders|legal 子目录）
├── components/     # 基元与复用组件（ui / KlineChart / exchange-icon / exchange-select / ThemeToggle ...）
├── api/v1/         # client.ts（axios 封装 + snake↔camel 归一化）+ modules/ + types/contract.ts
├── hooks/          # useAuth / useDashboard / useTrader
├── stores/         # appStore / i18nStore（三语）/ themeStore（深色/浅色）
├── i18n/           # translations.ts 嵌套键（中文 / EN / ID）
├── utils/          # format / errorCodes
└── mocks/          # MSW 模拟数据
```

**页面**：业务 7 路由 `/dashboard`（交易终端）· `/traders`（编队）· `/exchanges`（接入）· `/models`（模型）· `/strategies`（策略）· `/backtest`（回放）· `/debate`（辩论）；未登录 `/` 落地页 · `/login` 登录页；另有 `/forgot-password` `/reset-password` `/privacy` `/terms` `/cookie`。

**数据契约**：后端 snake_case ↔ 前端 camelCase，归一化收敛在 `api/v1/client.ts` 单点。页面组件不直接接触原始响应。

## 验证命令 / Verification

前端（改动 `web/` 后必须全过）：

```bash
npm --prefix web run lint    # eslint --max-warnings 0（零警告）
npm --prefix web run build   # tsc && vite build
npm --prefix web run test    # vitest
```

后端（改动 Go 后必须全过）：

```bash
go build ./... && go vet ./...
go test -race -count=1 ./...   # 全量（含竞态检测）
$(go env GOPATH)/bin/staticcheck ./...   # 静态检查
```

端到端：`PORT=6080 STATIC_DIR=web/dist` 启动后端托管前端，浏览器实际打开页面验证；冒烟脚本 `scripts/fxcore-hmac-smoke.sh`（需先 `go build -o /tmp/fxcore-server ./cmd/server` 并起服务）、假数据灌库 `python3 scripts/fxcore-seed-fixtures.py [db路径]`。

## 数据与安全 / Data & Security

- **持久化**：GORM 全量落库（20 表，SQLite 默认 `data/fxcore.db`，WAL + busy_timeout；`DB_DSN` 可切 MariaDB）——管理实体 + 订单/成交/持仓/决策/权益快照/回测/辩论等流式运行时数据，重启不丢
- **多用户隔离**：List 按 `user_id` 过滤，Get/Update/Delete/控制端点归属校验（越权返回 404），流式端点先校验后订阅
- **凭据**：API Key / 交易所密钥 RSA-OAEP 加密落库，响应零泄漏（仅前缀脱敏）；前端不注入 `auth_token` 头
- **认证**：access token 15min HttpOnly Cookie + refresh 原子轮换（防重放）；写操作 HMAC 签名（X-Timestamp / X-Nonce / X-Signature）防篡改
- **限流**：auth 5/min/IP、refresh 20/min、read 120、write 30（滑动窗口，Redis 可选）
- **密码找回**：SMTP 邮件发送一次性重置令牌（防枚举、防重放），未配置 SMTP 时 dev 模式写日志
- **防 XSS**：渲染用户内容转义；`SetTrustedProxies(nil)` 防 X-Forwarded-For 伪造；SSRF 防护（禁重定向 + 私网拒绝）
- **git**：`.env`、`data/`、`*.db`、`web/dist/`、密钥文件均在 `.gitignore`

## 目录 / Directories

```
├── cmd/server/        # 入口
├── internal/
│   ├── api/v1/        # handler / service / dto / router
│   ├── trader/engine  # 交易员执行引擎（周期循环/风控/下单）
│   ├── debate/        # 辩论引擎（SSE 事件流）
│   ├── backtest/      # 回放引擎
│   ├── exchange/      # 7 家交易所适配器（注册表模式）
│   ├── provider/      # 行情数据源链
│   ├── llm/           # LLM 适配
│   ├── kernel/        # 决策解析与提示词构建（含技术指标）
│   ├── store/         # GORM 存储层（SQLite / MariaDB）
│   ├── middleware/    # JWT / HMAC / 限流
│   ├── pkg/           # crypto / jwt / logger / cache / mail
│   └── ...
├── mcp/               # 独立 AI 模型访问层（clean-room，未接入主链路）
├── httpclient/        # HTTP 客户端封装
├── scripts/           # 冒烟脚本 / 假数据灌库脚本
├── docs/              # Swagger / OpenAPI 文档
├── .github/workflows/ # CI 流水线
└── web/               # React 前端
```

## License

Copyright (c) Onecany. All rights reserved.

本项目依赖的第三方 Go 包许可证清单见 [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md)。
