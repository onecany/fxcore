package mcp

// estimateTokens is a heuristic token estimate: characters/4 plus a fixed
// per-message overhead. The exact formula is intentionally simple (spec 3
// leaves it to the implementer); what matters is the relative ordering of
// old vs new messages during truncation.
func estimateTokens(m Message) int {
	n := len(m.Content)/4 + len(m.ReasoningContent)/4
	for _, tc := range m.ToolCalls {
		n += len(tc.Function.Name)/4 + len(tc.Function.Arguments)/4 + 4
	}
	n += len(m.ToolCallID)/4 + 6 // role + framing overhead
	if n < 1 {
		n = 1
	}
	return n
}

func totalTokens(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		total += estimateTokens(m)
	}
	return total
}

// truncateToMaxContext drops the oldest non-system messages until the
// estimated total fits within maxContext. System messages are never removed;
// the newest non-system message is always kept. Returns the original slice
// when no truncation is needed.
func truncateToMaxContext(msgs []Message, maxContext int) []Message {
	if maxContext <= 0 || len(msgs) == 0 {
		return msgs
	}
	var system, others []Message
	for _, m := range msgs {
		if m.Role == "system" {
			system = append(system, m)
		} else {
			others = append(others, m)
		}
	}
	total := totalTokens(system) + totalTokens(others)
	if total <= maxContext {
		return msgs
	}
	// Drop oldest non-system messages until we fit, keeping at least the
	// most recent one.
	for len(others) > 1 && total > maxContext {
		total -= estimateTokens(others[0])
		others = others[1:]
	}
	return append(system, others...)
}

// maybeTruncate applies the client's MaxContext guard to a copy of the
// request so the caller's Request is never mutated.
func maybeTruncate(client *Client, req *Request) *Request {
	if client.Cfg.MaxContext <= 0 || len(req.Messages) == 0 {
		return req
	}
	work := *req
	work.Messages = truncateToMaxContext(req.Messages, client.Cfg.MaxContext)
	return &work
}
