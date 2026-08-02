package mcp

import "testing"

// mkMsg builds a message of roughly n characters with a distinct fill byte
// so different messages can be told apart by content.
func mkMsg(role string, n int, seed byte) Message {
	content := make([]byte, n)
	for i := range content {
		content[i] = seed
	}
	return Message{Role: role, Content: string(content)}
}

func TestTruncateKeepsSystemAndNewest(t *testing.T) {
	// system ~500 tokens; each user message ~1000 tokens.
	sys := mkMsg("system", 2000, 's')
	old1 := mkMsg("user", 4000, 'a')
	old2 := mkMsg("user", 4000, 'b')
	latest := mkMsg("user", 4000, 'c')

	msgs := []Message{sys, old1, old2, latest}
	out := truncateToMaxContext(msgs, 1600) // system + 1 user fits (1512)

	// system preserved
	if out[0].Role != "system" {
		t.Fatal("system message removed")
	}
	// oldest dropped, newest kept
	foundLatest := false
	for _, m := range out {
		if m.Content == latest.Content {
			foundLatest = true
		}
		if m.Content == old1.Content || m.Content == old2.Content {
			t.Error("oldest non-system messages should be dropped first")
		}
	}
	if !foundLatest {
		t.Error("newest message must be kept")
	}
	if totalTokens(out) > 1600 {
		t.Errorf("truncated total = %d, want <= 1600", totalTokens(out))
	}
	if len(out) != 2 {
		t.Errorf("len = %d, want 2 (system + newest)", len(out))
	}
}

func TestTruncateNoopWhenUnderLimit(t *testing.T) {
	msgs := []Message{mkMsg("system", 100, 's'), mkMsg("user", 100, 'u')}
	out := truncateToMaxContext(msgs, 100000)
	if len(out) != 2 {
		t.Errorf("len = %d, want 2 (no truncation)", len(out))
	}
	if &out[0] != &msgs[0] {
		t.Error("should return original slice when under limit")
	}
}

func TestTruncateKeepsAtLeastOneNonSystem(t *testing.T) {
	sys := mkMsg("system", 100, 's')
	big := mkMsg("user", 100000, 'b') // alone exceeds the limit
	msgs := []Message{sys, big}
	out := truncateToMaxContext(msgs, 1000)
	if len(out) != 2 {
		t.Errorf("len = %d, want 2 (never drop the last message)", len(out))
	}
}

func TestTruncateAllSystemOnly(t *testing.T) {
	sys := mkMsg("system", 100, 's')
	msgs := []Message{sys}
	out := truncateToMaxContext(msgs, 1)
	if len(out) != 1 {
		t.Errorf("len = %d, want 1 (system never removed)", len(out))
	}
}

func TestTruncateZeroMaxContext(t *testing.T) {
	msgs := []Message{mkMsg("user", 10, 'u')}
	out := truncateToMaxContext(msgs, 0)
	if len(out) != 1 {
		t.Errorf("len = %d, want 1 (disabled)", len(out))
	}
}

func TestMaybeTruncateDoesNotMutateRequest(t *testing.T) {
	client := NewClient(WithMaxContext(500)).(*Client)
	orig := []Message{mkMsg("user", 100000, 'u')}
	req := &Request{Messages: orig}
	maybeTruncate(client, req)
	if len(req.Messages) != 1 {
		t.Errorf("original request mutated: %d messages", len(req.Messages))
	}
	if &req.Messages[0] != &orig[0] {
		t.Error("original slice modified")
	}
}

func TestClientAppliesMaxContextEndToEnd(t *testing.T) {
	up := &mockUpstream{status: 200, body: `{"choices":[{"message":{"content":"ok"}}]}`}
	ts := newTestServer(t, up)
	c := NewClient(WithBaseURL(ts.srv.URL), WithAPIKey("k"), WithMaxContext(500))

	msgs := []Message{mkMsg("system", 100, 's'), mkMsg("user", 2000, 'a'), mkMsg("user", 2000, 'b')}
	if _, err := c.CallWithRequest(&Request{Messages: msgs}); err != nil {
		t.Fatal(err)
	}
	if len(up.reqBodies) != 1 {
		t.Fatalf("requests = %d", len(up.reqBodies))
	}
	if len(msgs) != 3 {
		t.Error("caller's message slice mutated")
	}
	// The wire body must contain fewer messages than the original 3.
	if got := countOccurrences(up.reqBodies[0], `"role"`); got >= 3 {
		t.Errorf("wire messages = %d, want < 3 (truncated)", got)
	}
}

func countOccurrences(s, sub string) int {
	n := 0
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			n++
		}
	}
	return n
}
