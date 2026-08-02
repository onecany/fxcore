package mcp

import (
	"strings"
	"testing"
)

func TestBuilderEmptyMessagesRejected(t *testing.T) {
	_, err := NewRequestBuilder().Build()
	if err == nil {
		t.Fatal("Build() with no messages should error")
	}
	if !strings.Contains(err.Error(), "at least one message is required") {
		t.Errorf("error = %q, want mention of at least one message", err)
	}
}

func TestBuilderMustBuildPanicsOnEmpty(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustBuild() should panic on empty messages")
		}
	}()
	NewRequestBuilder().MustBuild()
}

func TestBuilderChainsAllSetters(t *testing.T) {
	temp := 0.3
	req, err := NewRequestBuilder().
		WithModel("test-model").
		WithStream(true).
		WithSystemPrompt("sys").
		WithUserPrompt("user").
		WithTemperature(0.3).
		WithMaxTokens(123).
		WithTopP(0.9).
		WithFrequencyPenalty(0.1).
		WithPresencePenalty(0.2).
		WithStopSequences([]string{"a", "b"}).
		AddStopSequence("c").
		AddTool(Tool{Type: "function", Function: FunctionDef{Name: "fn", Description: "d", Parameters: map[string]any{"type": "object"}}}).
		WithToolChoice("auto").
		Build()
	if err != nil {
		t.Fatalf("Build() failed: %v", err)
	}
	if req.Model != "test-model" {
		t.Errorf("Model = %q", req.Model)
	}
	if !req.Stream {
		t.Error("Stream = false, want true")
	}
	if len(req.Messages) != 2 {
		t.Fatalf("Messages = %d, want 2 (system+user)", len(req.Messages))
	}
	if req.Messages[0].Role != "system" || req.Messages[0].Content != "sys" {
		t.Errorf("message[0] = %+v", req.Messages[0])
	}
	if req.Messages[1].Role != "user" || req.Messages[1].Content != "user" {
		t.Errorf("message[1] = %+v", req.Messages[1])
	}
	if req.Temperature == nil || *req.Temperature != temp {
		t.Errorf("Temperature = %v, want %v", req.Temperature, temp)
	}
	if req.MaxTokens == nil || *req.MaxTokens != 123 {
		t.Errorf("MaxTokens = %v, want 123", req.MaxTokens)
	}
	if req.TopP == nil || *req.TopP != 0.9 {
		t.Errorf("TopP = %v, want 0.9", req.TopP)
	}
	if req.FrequencyPenalty == nil || *req.FrequencyPenalty != 0.1 {
		t.Errorf("FrequencyPenalty = %v", req.FrequencyPenalty)
	}
	if req.PresencePenalty == nil || *req.PresencePenalty != 0.2 {
		t.Errorf("PresencePenalty = %v", req.PresencePenalty)
	}
	if len(req.Stop) != 3 || req.Stop[2] != "c" {
		t.Errorf("Stop = %v", req.Stop)
	}
	if len(req.Tools) != 1 || req.Tools[0].Function.Name != "fn" {
		t.Errorf("Tools = %+v", req.Tools)
	}
	if req.ToolChoice != "auto" {
		t.Errorf("ToolChoice = %q", req.ToolChoice)
	}
}

func TestBuilderUnsetOptionalsStayNil(t *testing.T) {
	req, err := NewRequestBuilder().AddUserMessage("hi").Build()
	if err != nil {
		t.Fatal(err)
	}
	if req.Temperature != nil {
		t.Errorf("Temperature = %v, want nil", req.Temperature)
	}
	if req.MaxTokens != nil {
		t.Errorf("MaxTokens = %v, want nil", req.MaxTokens)
	}
	if req.TopP != nil {
		t.Errorf("TopP = %v, want nil", req.TopP)
	}
	if req.FrequencyPenalty != nil || req.PresencePenalty != nil {
		t.Error("penalties should be nil when unset")
	}
	if len(req.Stop) != 0 || len(req.Tools) != 0 {
		t.Error("Stop/Tools should be empty when unset")
	}
	if req.ToolChoice != "" {
		t.Errorf("ToolChoice = %q, want empty", req.ToolChoice)
	}
}

func TestBuilderMessageHelpers(t *testing.T) {
	req, err := NewRequestBuilder().
		AddSystemMessage("s1").
		AddUserMessage("u1").
		AddAssistantMessage("a1").
		AddMessage(Message{Role: "user", Content: "u2"}).
		AddMessages([]Message{{Role: "user", Content: "u3"}, {Role: "tool", Content: "r", ToolCallID: "t1"}}).
		AddConversationHistory([]Message{{Role: "assistant", Content: "a2"}}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Messages) != 7 {
		t.Fatalf("Messages = %d, want 7", len(req.Messages))
	}
	want := []string{"system", "user", "assistant", "user", "user", "tool", "assistant"}
	for i, w := range want {
		if req.Messages[i].Role != w {
			t.Errorf("message[%d].Role = %q, want %q", i, req.Messages[i].Role, w)
		}
	}
	if req.Messages[5].ToolCallID != "t1" {
		t.Errorf("message[5].ToolCallID = %q", req.Messages[5].ToolCallID)
	}
}

func TestBuilderClearMessages(t *testing.T) {
	b := NewRequestBuilder().AddUserMessage("x").AddUserMessage("y")
	b.ClearMessages()
	if _, err := b.Build(); err == nil {
		t.Error("Build after ClearMessages should fail (no messages)")
	}
	b.AddUserMessage("z")
	if _, err := b.Build(); err != nil {
		t.Errorf("Build after re-adding message failed: %v", err)
	}
}

func TestBuilderAddFunction(t *testing.T) {
	req, err := NewRequestBuilder().
		AddUserMessage("q").
		AddFunction(FunctionDef{Name: "f", Description: "d", Parameters: map[string]any{}}).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Tools) != 1 {
		t.Fatalf("Tools = %d, want 1", len(req.Tools))
	}
	if req.Tools[0].Type != "function" {
		t.Errorf("tool type = %q, want function", req.Tools[0].Type)
	}
}

func TestBuilderPresets(t *testing.T) {
	chat, err := NewRequestBuilder().AddUserMessage("x").ForChat().Build()
	if err != nil {
		t.Fatal(err)
	}
	if *chat.Temperature != 0.7 || *chat.MaxTokens != 2000 {
		t.Errorf("ForChat = temp %v tokens %v, want 0.7/2000", *chat.Temperature, *chat.MaxTokens)
	}

	code, err := NewRequestBuilder().AddUserMessage("x").ForCodeGeneration().Build()
	if err != nil {
		t.Fatal(err)
	}
	if *code.Temperature != 0.2 || *code.TopP != 0.1 {
		t.Errorf("ForCodeGeneration = temp %v topP %v, want 0.2/0.1", *code.Temperature, *code.TopP)
	}

	creative, err := NewRequestBuilder().AddUserMessage("x").ForCreativeWriting().Build()
	if err != nil {
		t.Fatal(err)
	}
	if *creative.Temperature != 1.2 || *creative.MaxTokens != 4000 || *creative.TopP != 0.95 {
		t.Errorf("ForCreativeWriting = temp %v tokens %v topP %v", *creative.Temperature, *creative.MaxTokens, *creative.TopP)
	}
	if *creative.PresencePenalty != 0.6 || *creative.FrequencyPenalty != 0.5 {
		t.Errorf("ForCreativeWriting = presence %v freq %v", *creative.PresencePenalty, *creative.FrequencyPenalty)
	}
}

func TestBuilderMustBuildReturnsRequest(t *testing.T) {
	req := NewRequestBuilder().AddUserMessage("ok").MustBuild()
	if len(req.Messages) != 1 {
		t.Fatalf("Messages = %d", len(req.Messages))
	}
}

func TestBuilderMutatorsAreChainable(t *testing.T) {
	b := NewRequestBuilder()
	if b.WithModel("m") != b || b.WithStream(false) != b || b.AddUserMessage("u") != b {
		t.Error("setters must return the same builder for chaining")
	}
	if b.ClearMessages() != b {
		t.Error("ClearMessages must return the same builder")
	}
}
