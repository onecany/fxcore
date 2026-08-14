# FXcore

自主加密货币交易智能体平台 —— Go 后端（行情聚合、交易执行、AI 决策调度）+ React 前端（交易终端与配置界面）。

Autonomous crypto trading agent platform — Go backend (market aggregation, order execution, AI decision orchestration) + React frontend (trading terminal & configuration UI).

## 功能概览 / Features

- **交易员编队**：多交易员独立引擎，状态机（idle / running / paused / stopped），5s 轮询实时监控
- **AI 决策**：接入多家 LLM（deepseek / qwen / claude / gpt / gemini / custom），RSA-OAEP 加密 API Key 落库
- **策略工作室**：双路径提示词、四种交易风格（均衡/激进/稳健/剥头皮）、全局唯一激活策略
- **10 家交易所适配层**：binance / bybit / okx / bitget / gate / kucoin / indodax / hyperliquid / aster / lighter；账户余额实时查询
- **回放引擎**：历史行情回放、K 线预览（lightweight-charts）、进度/权益/回撤跟踪
- **辩论竞技场**：多模型观点碰撞、轮次推进、共识投票、SSE 事件流、共识执行下发
- **安全**：HttpOnly Cookie 认证、写操作 HMAC-SHA256 签名头、凭据加密存储、多用户数据隔离

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
│  internal/exchange       10 家交易所适配器   │
│  internal/provider       行情数据源链        │
│  internal/llm            LLM 适配            │
│  internal/kernel         决策解析与提示词构建  │
└─────────────────────────────────────────────┘
```

## 技术栈 / Tech Stack

**后端**：Go 1.26 · Gin · GORM · SQLite（默认）/ MariaDB · JWT · RSA-OAEP · SSE

**前端**：React 19 · TypeScript 5.8 · Vite 6 · react-router 8 · SWR（服务端缓存/轮询）· zustand（全局状态）· lightweight-charts 5（K线）· axios · MSW（Mock）

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
| `DB_DSN` | SQLite `data/fxcore.db` | MariaDB/MySQL 连接串（含 `parseTime=True`） |
| `RSA_PRIVATE_KEY` | 临时生成 | RSA 私钥（PEM 或 base64(PEM)），**必须持久化** |
| `JWT_SECRET` | 自动生成 | JWT 签名密钥（重启失效，需重新登录） |
| `REDIS_URL` | - | 可选，令牌缓存 |
| `CORS_ORIGINS` | - | 额外跨域白名单（逗号分隔） |
| `ADMIN_SIGN_SECRET` | 自动生成 | 管理签名密钥 |
| `TLS_CERT_FILE` / `TLS_KEY_FILE` | - | 可选 HTTPS |
| `HYPERLIQUID_BASE_URL` / `OKX_BASE_URL` | 公网 | 行情数据源覆盖（测试用） |

## API 概览 / API Overview

统一信封：`{ code, message, data }`（`code=0` 成功）。认证：HttpOnly Cookie + 写操作 HMAC 签名头（`X-Timestamp` / `X-Nonce` / `X-Signature`）。

| 模块 | 端点 |
|---|---|
| Auth | `POST /api/v1/auth/login` `/register` `/logout` |
| 模型 | `GET/POST /api/v1/models` `PUT/DELETE /api/v1/models/:id` `POST /models/:id/test` `GET /models/providers` |
| 交易员 | `GET/POST /api/v1/traders` `PATCH /traders/:id` `POST /traders/:id/start|pause|resume|stop` |
| 仪表盘 | `GET /api/v1/dashboard` · `WS /ws/dashboard` |
| 持仓 | `GET /api/v1/positions` `DELETE /positions/:id` |
| 交易所 | `GET/POST /api/v1/exchanges` `GET /exchanges/balances` `PUT/DELETE /exchanges/:id` |
| 策略 | `GET/POST /api/v1/strategies` `PUT/DELETE /strategies/:id` `POST /strategies/:id/activate|duplicate` `GET /strategies/active|default-config` |
| 数据 | `GET /api/v1/account` `/decisions` `/orders` `/trades` `/statistics` `/equity-history-batch` 等 |
| 回测 | `POST /api/v1/backtest/start` `POST /backtest/:action` `GET /backtest/status|runs|equity|trades|metrics|trace|klines` 等 |
| 辩论 | `GET/POST /api/v1/debates` `POST /debates/:id/:action` `POST /debates/:id/execute` `GET /debates/:id/stream`(SSE) |
| 加密 | `GET /api/v1/crypto/public-key` `POST /crypto/decrypt` |
| Telegram | `GET/POST /api/v1/telegram` `POST /telegram/model` `DELETE /telegram/binding` |

完整契约见 `internal/api/v1/openapi3/swagger.json`（前端类型生成：`npm --prefix web run gen:api`）。

## 前端结构 / Frontend Layout

```
web/src/
├── pages/          # 页面级组件（模型/策略/交易所/回测/辩论）
├── sections/       # 业务区块（交易终端 / 交易员编队 / 仪表盘 + dashboard|traders 子组件）
├── components/     # 基元与复用组件（ui.tsx / KlineChart / exchange-icon / exchange-select）
├── api/v1/         # client.ts（axios 封装 + snake↔camel 归一化）+ modules/ + types/contract.ts
├── hooks/          # useAuth / useDashboard / useTrader
├── stores/         # appStore / i18nStore（三语：中文 / EN / ID）
├── i18n/           # translations.ts 嵌套键
├── utils/          # format / errorCodes
└── mocks/          # MSW 模拟数据
```

**页面**（7 路由）：`/dashboard`（交易终端）· `/traders`（编队）· `/exchanges`（接入）· `/models`（模型）· `/strategies`（策略）· `/backtest`（回放）· `/debate`（辩论）

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
go test ./internal/...       # 全量（含 -race 并发测试）
```

验收：`PORT=6080 STATIC_DIR=web/dist` 启动后端托管前端，浏览器实际打开页面验证。

## 数据与安全 / Data & Security

- **持久化**：GORM 全量落库（20 实体，SQLite 默认 `data/fxcore.db`，WAL + busy_timeout；`DB_DSN` 可切 MariaDB）
- **多用户隔离**：List 按 `user_id` 过滤，Get/Update/Delete/控制端点归属校验（越权返回 404）
- **凭据**：API Key / 交易所密钥 RSA-OAEP 加密落库，响应零泄漏（仅前缀脱敏）；前端不注入 `auth_token` 头
- **防 XSS**：渲染用户内容转义；写操作 HMAC 签名防重放
- **git**：`.env`、`data/`、`*.db`、`web/dist/`、密钥文件均在 `.gitignore`

## 目录 / Directories

```
├── cmd/server/        # 入口
├── internal/
│   ├── api/v1/        # handler / service / dto / router
│   ├── trader/engine  # 交易员执行引擎（周期循环/风控/下单）
│   ├── debate/        # 辩论引擎（SSE 事件流）
│   ├── backtest/      # 回放引擎
│   ├── exchange/      # 10 家交易所适配器（注册表模式）
│   ├── provider/      # 行情数据源链
│   ├── llm/           # LLM 适配
│   ├── kernel/        # 决策解析与提示词构建
│   ├── store/         # GORM 存储层
│   ├── middleware/    # JWT / HMAC / 限流
│   └── pkg/           # crypto / jwt / logger / cache
└── web/               # React 前端
```

## License

Copyright (c) Onecany. All rights reserved.

本项目依赖的第三方 Go 包许可证清单见 [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md)。
