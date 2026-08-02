# fxcore/mcp — AI 模型访问层

`fxcore/mcp` 是 fxcore 的 LLM 访问层:统一封装多个 LLM 提供商的 REST API(OpenAI 兼容格式 + Anthropic 格式),提供一致的客户端接口、重试、超时、日志、Token 用量统计与上下文截断。

---

## 1. 包结构

```
mcp/                    # 核心:类型、客户端、选项、注册表、SSE、上下文截断
  types.go              # Message/ToolCall/LLMResponse/Request/TokenUsage/Logger
  config.go             # Config/Client/ClientHooks/Provider 常量/默认端点与模型
  options.go            # With* 选项
  client.go             # AIClient 接口、New/NewClient、重试、流式、SetAPIKey
  hooks.go              # 默认 wire format(OpenAI 兼容)+ 响应解析 + 可重试模式
  request_builder.go    # 链式 RequestBuilder + 预设
  context_guard.go      # MaxContext token 估算截断
  sse.go                # ParseSSEStream / ReportStreamUsage
  registry.go           # Provider 注册表(并发安全)
mcp/provider/           # 内置 LLM provider,init() 注册
```

依赖方向:`mcp/provider` 依赖 `mcp` 根包。

---

## 2. 快速开始

```go
import "fxcore/mcp"

// 1. 通过注册表按 provider 创建(自动带默认端点与模型)
client := mcp.NewAIClientByProvider(mcp.ProviderDeepSeek)
if client == nil {
    // provider 未注册(未 import 对应包),回退默认客户端
    client = mcp.New()
}
client.SetAPIKey(os.Getenv("DEEPSEEK_API_KEY"), "", "")

// 2. 双提示词调用
text, err := client.CallWithMessages("你是交易助手", "分析 BTC 当前趋势")

// 3. 或构造完整请求
req, err := mcp.NewRequestBuilder().
    WithSystemPrompt("你是量化分析师").
    WithUserPrompt("给出 ETH 的做多/做空建议").
    WithTemperature(0.7).
    WithMaxTokens(4000).
    Build()
if err != nil {
    return err
}
text, err = client.CallWithRequest(req)
```

环境变量 `AI_MAX_TOKENS` 覆盖默认 token 上限(默认 2000)。

---

## 3. Provider 与默认端点

| Provider | 常量 | BaseURL | 默认模型 |
|---|---|---|---|
| DeepSeek | `ProviderDeepSeek` | `https://api.deepseek.com` | `deepseek-v4-flash` |
| OpenAI | `ProviderOpenAI` | `https://api.openai.com/v1` | `gpt-5.4` |
| Claude | `ProviderClaude` | `https://api.anthropic.com/v1` | `claude-opus-4-6` |
| Qwen | `ProviderQwen` | `https://dashscope.aliyuncs.com/compatible-mode/v1` | `qwen3-max` |
| Gemini | `ProviderGemini` | `https://generativelanguage.googleapis.com/v1beta/openai` | `gemini-3.1-pro` |
| Grok | `ProviderGrok` | `https://api.x.ai/v1` | `grok-3-latest` |
| Kimi | `ProviderKimi` | `https://api.moonshot.ai/v1` | `moonshot-v1-auto` |
| MiniMax | `ProviderMiniMax` | `https://api.minimax.io/v1` | `MiniMax-M2.7` |

- `mcp.New()` 返回 `ProviderCustom` 客户端,默认沿用 deepseek 端点/模型,用于完全自定义端点。
- 端点/模型为**外部契约**,改动需同步上层(store 按模型名前缀推断上下文上限、前端模型下拉等)。

### Provider 差异

- **Claude**:完整 Anthropic Messages wire format(`/v1/messages`、system 顶层、`x-api-key` + `anthropic-version: 2023-06-01`、tool_use/tool_result 块、input_tokens/output_tokens)。
- **Kimi**:请求体强制 `temperature=1.0`(其 K2.5 仅接受该值)。

---

## 4. 客户端配置(Options)

```go
client := mcp.NewClient(
    mcp.WithAPIKey(key),
    mcp.WithBaseURL("https://proxy.example.com/v1"),
    mcp.WithModel("custom-model"),
    mcp.WithProvider(mcp.ProviderCustom),
    mcp.WithMaxTokens(8000),        // 覆盖 AI_MAX_TOKENS
    mcp.WithMaxContext(131072),     // 上下文截断阈值(见 §7)
    mcp.WithTemperature(0.3),
    mcp.WithTimeout(120*time.Second),
    mcp.WithMaxRetries(3),          // 默认 MaxRetryTimes=3
    mcp.WithRetryWaitBase(2*time.Second),
    mcp.WithHTTPClient(hc),
    mcp.WithLogger(myLogger),       // 实现 mcp.Logger 接口
    mcp.WithUseFullURL(true),       // BaseURL 即为完整端点,不拼 /chat/completions
    // With*Config 为 API 兼容预留接口(仅存储,当前不改变请求行为)
    mcp.WithDeepSeekConfig(mcp.DeepSeekConfig{EnableThinking: true}),
    mcp.WithQwenConfig(mcp.QwenConfig{EnableThinking: true}),
    mcp.WithMiniMaxConfig(mcp.MiniMaxConfig{GroupID: "..."}),
)
```

`SetAPIKey(apiKey, customURL, customModel)` 在运行时覆盖凭据与端点:
- `customURL`/`customModel` 非空时覆盖默认;
- `customURL` 以 `#` 结尾表示**完整端点**(不再拼接路径),等价于 `WithUseFullURL(true)`。

---

## 5. RequestBuilder 链式 API

```go
req, err := mcp.NewRequestBuilder().
    AddSystemMessage("...").      // 或 WithSystemPrompt
    AddUserMessage("...").        // 或 WithUserPrompt
    AddAssistantMessage("...").
    AddMessage(mcp.NewMessage("user", "...")).  // 任意 role/content
    AddConversationHistory(prevTurn).   // 追加多轮历史
    AddTool(tool).
    AddFunction(mcp.FunctionDef{Name: "api_request", Description: "...", Parameters: map[string]any{...}}).
    WithToolChoice("auto").             // 或 "none"/"required"
    WithTopP(0.9).
    WithStopSequences([]string{"\n"}).
    WithStream(true).
    Build()                             // 空消息列表报错 "at least one message is required"
```

- 未设置的数值参数保持 `nil`,**零值不会覆盖服务端默认**。
- 预设:`ForChat()`(temp 0.7 / 2000 tokens)、`ForCodeGeneration()`(temp 0.2 / topP 0.1)、`ForCreativeWriting()`(temp 1.2 / 4000 tokens / topP 0.95 / presence 0.6 / freq 0.5)。
- `MustBuild()` 在确定合法时直接返回 `*Request`,否则 panic。

---

## 6. 工具调用(Tool Calling)

`CallWithRequestFull` 返回文本与工具调用;工具循环的标准写法(与 telegram/agent 一致):

```go
resp, err := client.CallWithRequestFull(req)
if err != nil {
    return err
}
if len(resp.ToolCalls) == 0 {
    return resp.Content, nil // 最终文本回复
}
// 把带 tool_calls 的 assistant 消息与 tool 结果消息追加进下一轮
req.Messages = append(req.Messages,
    mcp.Message{Role: "assistant", ToolCalls: resp.ToolCalls},  // wire 上省略 content
    mcp.Message{Role: "tool", ToolCallID: tc.ID, Content: result},
)
```

Wire 行为(自动处理):
- assistant tool_calls 消息**省略 content**;tool 结果消息带 `tool_call_id`;
- `reasoning_content`(thinking 模型)存在时回传,多轮必需;
- tool arguments 统一归一化为可直接 `json.Unmarshal` 的对象文本(OpenAI 字符串形态与内联对象形态均兼容)。

---

## 7. 上下文保护

`WithMaxContext(n)` 启用按 token 估算的截断(启发式:字符数/4 + 消息开销):
- **system 消息永不删除**;优先删除最旧非 system 消息;最新消息始终保留;
- 截断发生在请求发送前,不修改调用方的 `Request`。

```go
// 上限 128k 的模型配 131072 时,超长历史会自动丢弃最旧的轮次
client := mcp.NewClient(mcp.WithMaxContext(131072))
```

---

## 8. 重试、流式与 Token 统计

### 重试(仅 `CallWithMessages` / `CallWithRequest` / `CallWithRequestFull`)

- 最多 `MaxRetries` 次重试(默认 3),退避 = `RetryWaitBase × 尝试次数`(默认 2s/4s/6s),响应上下文取消;
- **仅可重试错误**才重试,匹配错误文本子串:`EOF`、`timeout`、`connection reset`、`connection refused`、`temporary failure`、`no such host`、`stream error`、`INTERNAL_ERROR`、`status 429`、`rate_limit_error`、`upstream_empty_output`、`status 502/503/520/524`;
- API key 为空立即报错,不发起请求。

### 流式

```go
text, err := client.CallWithRequestStream(req, func(cumulative string) {
    // onChunk 收到累计文本
})
```

- SSE `data:` 行解析,`[DONE]` 结束;60s 无数据视为空闲超时并取消连接;
- 结束前解析 usage 并上报;读 goroutine 可随退出路径安全终止。

### Token 统计

```go
mcp.TokenUsageCallback = func(u mcp.TokenUsage) {
    // u.Provider / u.Model / u.PromptTokens / u.CompletionTokens / u.TotalTokens
}
```

`config` 包在启动时挂接此回调,把用量送入 telemetry。

---

## 9. 兼容性契约(上层依赖)

以下导出符号被 api/trader/kernel/telegram/store 直接引用,**签名与语义不可改动**(clean-room 规格 §1.1):

- 接口:`AIClient`、`ClientEmbedder`(`.BaseClient() *Client`)、`ClientHooks`、`Logger`;
- 类型:`Config`、`Client`(上层直读 `Provider`/`Model` 字段)、`Message`、`ToolCall`、`LLMResponse`、`Tool`/`FunctionDef`、`Request`、`TokenUsage`、`RequestBuilder`;
- 函数:`New`、`NewClient`、`NewAIClientByProvider`、`RegisterProvider`、`NewRequestBuilder`、`NewMessage`/`NewSystemMessage`/`NewUserMessage`/`NewAssistantMessage`、`NewNoopLogger`、`ParseSSEStream`、`ReportStreamUsage`;
- 选项:`WithLogger/WithHTTPClient/WithTimeout/WithMaxRetries/WithRetryWaitBase/WithMaxTokens/WithMaxContext/WithTemperature/WithAPIKey/WithBaseURL/WithModel/WithProvider/WithUseFullURL/WithDeepSeekConfig/WithQwenConfig/WithMiniMaxConfig`;
- 常量:全部 `Provider*`(含 `ProviderCustom`)、`Default*BaseURL`/`Default*Model`、`DefaultTimeout`(120s)、`MaxRetryTimes`(3)、`MCPClientTemperature`(0.5);
- 变量:`TokenUsageCallback func(TokenUsage)`。

---

## 10. 测试

```
go test ./mcp/...          # 根包 + provider
go test -race ./mcp/...    # 并发安全(注册表、流式)
```

测试覆盖:8 provider 注册与默认值、选项默认/覆盖、builder 校验与预设、wire format(OpenAI 工具消息/Claude 转换)、SSE 解析、重试次数与退避、上下文截断边界。

---

## 11. 已知边界与运维提示

- **Kimi** 固定 `temperature=1.0`;**Claude** 需要 `x-api-key` 认证,与 Bearer 组不同。
- `CallWithRequestStream` 只做文本路径,不聚合工具调用 delta;工具调用请用 `CallWithRequestFull`。
