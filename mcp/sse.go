package mcp

import "strings"

// ParseSSEStream parses a Server-Sent Events payload, invoking onEvent once
// per data: line. Parsing stops at the [DONE] sentinel or the end of input.
// Blank lines and comment lines are ignored.
func ParseSSEStream(data []byte, onEvent func(event []byte)) error {
	if onEvent == nil {
		return nil
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if payload == "" {
			continue
		}
		if payload == "[DONE]" {
			return nil
		}
		onEvent([]byte(payload))
	}
	return nil
}

// ReportStreamUsage fills provider/model from the client when missing and
// invokes the global TokenUsageCallback when a total is present.
func ReportStreamUsage(client *Client, usage TokenUsage) {
	if TokenUsageCallback == nil || usage.TotalTokens <= 0 {
		return
	}
	if usage.Provider == "" && client != nil {
		usage.Provider = client.Provider
	}
	if usage.Model == "" && client != nil {
		usage.Model = client.Model
	}
	TokenUsageCallback(usage)
}
