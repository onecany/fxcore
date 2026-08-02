package mcp

import "context"

// Logger is the minimal leveled logging interface used across the package.
// All methods are Printf-style.
type Logger interface {
	Debugf(format string, args ...any)
	Infof(format string, args ...any)
	Warnf(format string, args ...any)
	Errorf(format string, args ...any)
}

// Message is a single chat message exchanged with a model.
// For assistant messages carrying tool calls, Content is omitted on the wire.
type Message struct {
	Role             string
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
	ToolCallID       string
}

// NewMessage builds a message with an explicit role.
func NewMessage(role, content string) Message {
	return Message{Role: role, Content: content}
}

// NewSystemMessage builds a system-role message.
func NewSystemMessage(content string) Message { return NewMessage("system", content) }

// NewUserMessage builds a user-role message.
func NewUserMessage(content string) Message { return NewMessage("user", content) }

// NewAssistantMessage builds an assistant-role message.
func NewAssistantMessage(content string) Message { return NewMessage("assistant", content) }

// ToolCall is a function invocation requested by the model.
type ToolCall struct {
	ID       string
	Type     string
	Function ToolCallFunction
}

// ToolCallFunction names the function and carries its JSON arguments.
type ToolCallFunction struct {
	Name      string
	Arguments string
}

// LLMResponse is the fully parsed result of a model call.
type LLMResponse struct {
	Content          string
	ReasoningContent string
	ToolCalls        []ToolCall
}

// Tool declares a callable function to the model (OpenAI tool format).
type Tool struct {
	Type     string
	Function FunctionDef
}

// FunctionDef describes a function usable as a tool.
type FunctionDef struct {
	Name        string
	Description string
	Parameters  map[string]any
}

// TokenUsage records token consumption for one model call.
type TokenUsage struct {
	Provider         string
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// Channel reports the payment channel for telemetry: "claw402" for
// USDC-paid calls, "native" otherwise.
func (t TokenUsage) Channel() string {
	if t.Provider == ProviderClaw402 {
		return "claw402"
	}
	return "native"
}

// Request is a fully assembled model call. Optional fields use pointers so a
// nil value means "leave the server default"; Ctx is never serialized.
type Request struct {
	Model            string
	Messages         []Message
	Stream           bool
	Temperature      *float64
	MaxTokens        *int
	TopP             *float64
	FrequencyPenalty *float64
	PresencePenalty  *float64
	Stop             []string
	Tools            []Tool
	ToolChoice       string
	Ctx              context.Context
}
