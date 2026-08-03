# httpclient

`httpclient` 是一个代理感知（proxy-aware）的 HTTP/WebSocket 客户端包。它为 FXcore 的统一传输层提供三块能力：

1. **HTTP 客户端工厂** —— 统一管理超时、连接池与代理路由，供所有 REST 调用复用。
2. **请求构建与执行** —— 提供 JSON 请求的构造与发送的便捷封装，覆盖请求头合并与 Content-Type 约定。
3. **WebSocket 拨号** —— 在 `golang.org/x/net/websocket` 之上封装，复用同一份代理路由逻辑。

代理通过标准环境变量（`ALL_PROXY` / `HTTPS_PROXY` / `HTTP_PROXY`）配置，支持 `socks5`、`socks`、`socks5h`、`http`、`https` 多种 scheme。

---

## 设计动机

大型语言模型（LLM）供应商、行情与交易交易所（如 Hyperliquid）等上游服务通常位于境外，直连不稳定。包内把「代理优先」的拨号逻辑收敛到一处，使所有调用方——无论 REST 还是 WebSocket——都能自动享受代理，无需各自实现。

这一决策消除了分散的复制粘贴代理逻辑，以少量代码换取全包一致的传输行为。

---

## 目录结构

```
httpclient/
├── httpclient.go      # 客户端工厂、Transport 构造、代理解析与 SOCKS5 拨号
├── request.go         # 请求构建（NewRequest/NewJSONRequest）与执行（Do/DoJSON）
├── websocket.go       # WebSocket 拨号、HTTP CONNECT 隧道、TLS 包裹
├── request_test.go    # 请求构建与 JSON 发送测试
└── websocket_test.go  # WebSocket 直连与 HTTP 代理隧道测试
```

---

## 快速开始

### 创建 HTTP 客户端

```go
import (
    "time"
    "github.com/onecany/fxcore/httpclient"
)

client := httpclient.New(30 * time.Second)
// 若设置了 ALL_PROXY 等环境变量，client 自动走代理
```

### 发送 JSON 请求

```go
resp, err := httpclient.DoJSON(ctx, client, http.MethodPost, url, payload, headers)
```

### 拨号 WebSocket

```go
conn, err := httpclient.DialWebsocketContext(ctx, "wss://example.com/socket", origin)
```

---

## API 参考

### HTTP 客户端

#### `func New(timeout time.Duration) *http.Client`

以给定的超时时间创建一个代理感知的 `*http.Client`。默认 transport 提供如下配置：

| 参数 | 值 |
| --- | --- |
| `Proxy` | 代理感知解析器（见下文） |
| `TLSHandshakeTimeout` | 10s |
| `IdleConnTimeout` | 90s |
| `MaxIdleConns` | 100 |
| `MaxIdleConnsPerHost` | 10 |

#### `func NewWithTransport(timeout time.Duration, transport http.RoundTripper) *http.Client`

在自定义 transport 之上集中处理超时。适用于调用方需要包装共享的代理感知 transport 的情形。

#### `func DefaultTransport() *http.Transport`

返回代理感知的 `*http.Transport`，供调用方二次包装（例如 Bybit 的 header round-trip 装饰器）。

### 请求构建与执行

#### `func NewRequest(ctx context.Context, method, requestURL string, body io.Reader, headers http.Header) (*http.Request, error)`

构造带可选请求头的请求。`ctx` 为 `nil` 时回退到 `context.Background()`。传入的 `headers` 通过追加（`Add`）合并到请求头。

#### `func NewJSONRequest(ctx context.Context, method, requestURL string, payload any, headers http.Header) (*http.Request, error)`

将 `payload` 序列化为 JSON 并构造请求。若调用方未显式设置 `Content-Type`，则设为 `application/json`。

#### `func Do(ctx context.Context, client *http.Client, method, requestURL string, body io.Reader, headers http.Header) (*http.Response, error)`

构造并执行请求。`client` 为 `nil` 时返回错误。

#### `func DoJSON(ctx context.Context, client *http.Client, method, requestURL string, payload any, headers http.Header) (*http.Response, error)`

构造 JSON 请求并执行。

### WebSocket

#### `func DialWebsocketContext(ctx context.Context, serverURL, origin string, protocols ...string) (*websocket.Conn, error)`

打开一个 WebSocket 连接，基于 URL 与 origin 生成配置，并可选传入子协议列表，全程走本包统一的代理拨号逻辑。

#### `func DialWebsocketConfigContext(ctx context.Context, config *websocket.Config) (*websocket.Conn, error)`

从预构建的配置打开连接，同样遵守代理配置。会校验 `config` 非空、`Location` 与 `Origin` 合法；支持通过 context 取消拨号。

### 辅助函数

#### `func SetProxyEnv()`

早早在 `main()` 中调用，把当前代理配置打印到日志。对依赖 Go 标准库 `http.ProxyFromEnvironment` 的 SDK（如 Hyperliquid）尤其重要——该函数让它们也能感知代理，同时便于运维确认代理是否生效。

---

## 代理解析规则

包内两处代理取值逻辑：`resolveProxy`（HTTP 层面）与 `findProxyURL`（拨号层面），规则一致并互为补充。

### 优先级

1. `ALL_PROXY` / `all_proxy` 优先，覆盖一切 per-scheme 设置；
2. 其次按请求 scheme 选择 `HTTPS_PROXY` / `HTTP_PROXY`（均为大小写不敏感）。

### scheme 处理

| 代理 scheme | 处理方式 |
| --- | --- |
| `http` / `https` | 走标准库 HTTP CONNECT 机制（`resolveProxy`），或 WebSocket 的自建 CONNECT 隧道 |
| `socks5` / `socks` / `socks5h` | 自定义 SOCKS5 拨号器（Go 标准库不支持 SOCKS5，需自行实现） |
| 其他 | 记录警告，按直连处理 |

### 关键实现说明

- **`resolveProxy`** 通过 `http.Transport.Proxy` 返回代理 URL，让标准库处理 HTTP/HTTPS CONNECT 代理。
- **`dialContext`** 是自定义 `DialContext`，处理 SOCKS5：非 SOCKS 代理直接透传到底层拨号器，由 `resolveProxy` 兜底。
- SOCKS5 未显式指定端口时默认 `:1080`；支持 proxy URL 中的用户名/密码认证。
- WebSocket 在 `wss` 场景下会对连接做 TLS 包裹，自动填充 `ServerName`（若配置未指定则取主机名）。
- 底层拨号器 `Timeout: 30s`、`KeepAlive: 30s`。

### 直连回退

未配置任何代理时，`resolveProxy` 返回 `nil`、`dialContext` 走 `baseNetDialer()`，连接行为与普通 Go HTTP 客户端一致。

---

## 环境变量一览

| 变量 | 说明 | 优先级 |
| --- | --- | --- |
| `ALL_PROXY` / `all_proxy` | 全局代理，覆盖 per-scheme | 最高 |
| `HTTPS_PROXY` / `https_proxy` | HTTPS/WSS 请求代理 | 中 |
| `HTTP_PROXY` / `http_proxy` | HTTP/WS 请求代理 | 低 |

---

## 测试

```bash
cd httpclient
go test ./...
```

覆盖场景：

- `NewJSONRequest` 是否正确设置 `Content-Type` 与自定义请求头（`request_test.go`）。
- `DoJSON` 是否正确编码 JSON body、透传请求头并返回服务端状态码（`request_test.go`）。
- WebSocket 直连拨号与消息收发（`websocket_test.go`）。
- 经 HTTP CONNECT 代理的 WebSocket 隧道拨号与消息收发（`websocket_test.go`）。

> 注意：代理路径测试通过 `t.Setenv` 注入代理环境变量。测试依赖代理实现支持 CONNECT 劫持；若需验证 SOCKS5 场景，建议补齐基于 SOCKS5 测试代理的用例。

---

## 常见问题

### 为什么需要自定义 `dialContext` 而不直接用 `http.ProxyFromEnvironment`？

Go 标准库的 `http.ProxyFromEnvironment` **不支持 SOCKS5**，只处理 HTTP/HTTPS CONNECT 代理。因此本包拆分为两层：标准库处理 HTTP/HTTPS 代理，自定义 `dialContext` 处理 SOCKS5，二者通过环境变量协同。

### 我的外部 SDK 不走这个包的拨号逻辑，代理失效怎么办？

部分 SDK 内部自己构造 `http.Client`。此时在 `main()` 早期调用 `SetProxyEnv()`，通过 `http.ProxyFromEnvironment` 让标准库代理自动生效——前提是该 SDK 使用默认 transport 继承进程环境变量。

### 代理返回非 200 状态会怎样？

WebSocket 的 HTTP CONNECT 隧道会校验 `resp.StatusCode == 200`，否则读取响应体（上限 8KB）并返回带状态与 body 的错误信息，便于排查中间代理故障。

---

## 维护注意

- 修改代理解析规则时，`resolveProxy` 与 `findProxyURL` 两处需保持语义一致，避免 HTTP 与 WebSocket 行为分叉。
- 所有 key 均应使大小写不敏感（同时探测 `ALL_PROXY` 与 `all_proxy` 等），以兼容不同部署习惯。
- 新增连接级参数（如超时、KeepAlive）时同步更新 `baseNetDialer`，确保直连与代理路径参数一致。
- WebSocket 测试中的 CONNECT 代理为内联实现，改动 `dialHTTPProxyTunnel` 时应同步验证该用例。
