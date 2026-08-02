package mcp

import "fmt"

// RequestBuilder is the fluent API for assembling a Request (spec 2.4).
// Optional parameters remain nil until explicitly set, so zero values never
// override server defaults on the wire.
type RequestBuilder struct {
	model            string
	messages         []Message
	stream           bool
	temperature      *float64
	maxTokens        *int
	topP             *float64
	frequencyPenalty *float64
	presencePenalty  *float64
	stop             []string
	tools            []Tool
	toolChoice       string
}

func NewRequestBuilder() *RequestBuilder {
	return &RequestBuilder{}
}

func (b *RequestBuilder) WithModel(m string) *RequestBuilder {
	b.model = m
	return b
}

func (b *RequestBuilder) WithStream(s bool) *RequestBuilder {
	b.stream = s
	return b
}

func (b *RequestBuilder) WithSystemPrompt(p string) *RequestBuilder {
	return b.AddSystemMessage(p)
}

func (b *RequestBuilder) WithUserPrompt(p string) *RequestBuilder {
	return b.AddUserMessage(p)
}

func (b *RequestBuilder) AddSystemMessage(content string) *RequestBuilder {
	return b.AddMessage(Message{Role: "system", Content: content})
}

func (b *RequestBuilder) AddUserMessage(content string) *RequestBuilder {
	return b.AddMessage(Message{Role: "user", Content: content})
}

func (b *RequestBuilder) AddAssistantMessage(content string) *RequestBuilder {
	return b.AddMessage(Message{Role: "assistant", Content: content})
}

func (b *RequestBuilder) AddMessage(m Message) *RequestBuilder {
	b.messages = append(b.messages, m)
	return b
}

func (b *RequestBuilder) AddMessages(ms []Message) *RequestBuilder {
	b.messages = append(b.messages, ms...)
	return b
}

// AddConversationHistory appends a prior turn's messages.
func (b *RequestBuilder) AddConversationHistory(ms []Message) *RequestBuilder {
	return b.AddMessages(ms)
}

func (b *RequestBuilder) ClearMessages() *RequestBuilder {
	b.messages = nil
	return b
}

func (b *RequestBuilder) WithTemperature(t float64) *RequestBuilder {
	b.temperature = &t
	return b
}

func (b *RequestBuilder) WithMaxTokens(n int) *RequestBuilder {
	b.maxTokens = &n
	return b
}

func (b *RequestBuilder) WithTopP(p float64) *RequestBuilder {
	b.topP = &p
	return b
}

func (b *RequestBuilder) WithFrequencyPenalty(p float64) *RequestBuilder {
	b.frequencyPenalty = &p
	return b
}

func (b *RequestBuilder) WithPresencePenalty(p float64) *RequestBuilder {
	b.presencePenalty = &p
	return b
}

func (b *RequestBuilder) WithStopSequences(ss []string) *RequestBuilder {
	b.stop = append([]string(nil), ss...)
	return b
}

func (b *RequestBuilder) AddStopSequence(s string) *RequestBuilder {
	b.stop = append(b.stop, s)
	return b
}

func (b *RequestBuilder) AddTool(t Tool) *RequestBuilder {
	b.tools = append(b.tools, t)
	return b
}

// AddFunction wraps a FunctionDef as a function-type tool.
func (b *RequestBuilder) AddFunction(f FunctionDef) *RequestBuilder {
	return b.AddTool(Tool{Type: "function", Function: f})
}

func (b *RequestBuilder) WithToolChoice(tc string) *RequestBuilder {
	b.toolChoice = tc
	return b
}

// ForChat applies the chat preset: temperature 0.7, 2000 tokens.
func (b *RequestBuilder) ForChat() *RequestBuilder {
	return b.WithTemperature(0.7).WithMaxTokens(2000)
}

// ForCodeGeneration applies the code preset: temperature 0.2, topP 0.1.
func (b *RequestBuilder) ForCodeGeneration() *RequestBuilder {
	return b.WithTemperature(0.2).WithTopP(0.1)
}

// ForCreativeWriting applies the creative preset: temperature 1.2, 4000
// tokens, topP 0.95, presence 0.6, frequency 0.5.
func (b *RequestBuilder) ForCreativeWriting() *RequestBuilder {
	return b.WithTemperature(1.2).WithMaxTokens(4000).WithTopP(0.95).
		WithPresencePenalty(0.6).WithFrequencyPenalty(0.5)
}

// Build assembles the Request, rejecting an empty message list.
func (b *RequestBuilder) Build() (*Request, error) {
	if len(b.messages) == 0 {
		return nil, fmt.Errorf("at least one message is required")
	}
	return &Request{
		Model:            b.model,
		Messages:         b.messages,
		Stream:           b.stream,
		Temperature:      b.temperature,
		MaxTokens:        b.maxTokens,
		TopP:             b.topP,
		FrequencyPenalty: b.frequencyPenalty,
		PresencePenalty:  b.presencePenalty,
		Stop:             b.stop,
		Tools:            b.tools,
		ToolChoice:       b.toolChoice,
	}, nil
}

// MustBuild builds or panics — for call sites where the request is
// guaranteed to be valid.
func (b *RequestBuilder) MustBuild() *Request {
	req, err := b.Build()
	if err != nil {
		panic(err)
	}
	return req
}
