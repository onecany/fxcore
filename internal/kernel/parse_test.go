package kernel

import (
	"strings"
	"testing"
)

func TestParseDecisionDecisionTag(t *testing.T) {
	raw := `<reasoning>BTC 强势</reasoning>
<decision>[{"action":"open_long","symbol":"BTC-USDT","leverage":5,"confidence":80}]</decision>`
	actions, err := ParseDecision(raw)
	if err != nil {
		t.Fatalf("ParseDecision: %v", err)
	}
	if len(actions) != 1 || actions[0].Action != "open_long" || actions[0].Symbol != "BTC-USDT" {
		t.Errorf("unexpected: %+v", actions)
	}
}

func TestParseDecisionJSONFence(t *testing.T) {
	raw := "思考...\n```json\n[{\"action\":\"wait\",\"confidence\":50}]\n```\n结束"
	actions, err := ParseDecision(raw)
	if err != nil {
		t.Fatalf("ParseDecision: %v", err)
	}
	if len(actions) != 1 || actions[0].Action != "wait" {
		t.Errorf("unexpected: %+v", actions)
	}
}

func TestParseDecisionFullWidth(t *testing.T) {
	raw := `<decision>[｛"action"："open_short"，"symbol"："ETH-USDT"｝]</decision>`
	actions, err := ParseDecision(raw)
	if err != nil {
		t.Fatalf("ParseDecision fullwidth: %v", err)
	}
	if actions[0].Action != "open_short" {
		t.Errorf("unexpected: %+v", actions)
	}
}

func TestParseDecisionInvalidAction(t *testing.T) {
	raw := `<decision>[{"action":"buy_now","symbol":"BTC-USDT"}]</decision>`
	if _, err := ParseDecision(raw); err == nil {
		t.Errorf("expected error for invalid action")
	} else if !strings.Contains(err.Error(), "invalid action") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestParseDecisionMissingSymbol(t *testing.T) {
	raw := `<decision>[{"action":"open_long","confidence":70}]</decision>`
	if _, err := ParseDecision(raw); err == nil {
		t.Errorf("expected error for missing symbol")
	}
}

func TestParseDecisionNoBlock(t *testing.T) {
	if _, err := ParseDecision("模型拒绝输出"); err == nil {
		t.Errorf("expected error for no decision block")
	}
}

func TestSafeWait(t *testing.T) {
	w := SafeWait()
	if len(w) != 1 || w[0].Action != "wait" || w[0].Confidence != 50 {
		t.Errorf("unexpected safe wait: %+v", w)
	}
}

func TestFixFullWidth(t *testing.T) {
	got := FixFullWidth("{\u201Ca\u201D:1}") // 全角双引号 U+201C/U+201D → ASCII
	want := "{\"a\":1}"
	if got != want {
		t.Errorf("got %q want %q", got, want)
	}
}
