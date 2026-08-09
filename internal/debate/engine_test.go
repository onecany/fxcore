package debate

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/llm"
	"fxcore/internal/model"
	"fxcore/internal/store"
)

// fakeModels 测试模型提供者。
type fakeModels struct {
	models map[string]*llm.Model
}

func (f *fakeModels) GetModel(id string) (*llm.Model, bool) {
	m, ok := f.models[id]
	return m, ok
}

// fakeAI mock transport。
type fakeAI struct {
	body string
}

func (f *fakeAI) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(f.body)),
	}, nil
}

func testEngine(t *testing.T) (*Engine, *store.Store) {
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"}, nil)
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	models := &fakeModels{models: map[string]*llm.Model{
		"m1": {Provider: "gpt", ModelName: "gpt-4o", APIKey: "k1"},
		"m2": {Provider: "gpt", ModelName: "gpt-4o", APIKey: "k2"},
		"m3": {Provider: "gpt", ModelName: "gpt-4o", APIKey: "k3"},
	}}
	ai := llm.NewWithTransport(&fakeAI{body: `{"choices":[{"message":{"content":"test view"}}]}`}, 5*time.Second)
	return NewEngine(st, models, ai), st
}

func TestCreateValidation(t *testing.T) {
	e, _ := testEngine(t)
	base := dto.CreateDebateRequest{StrategyID: "s1", Symbol: "BTC-USDT", MaxRounds: 3}
	// participants < 2
	if _, err := e.Create(base); err == nil || err.Code != 1001 {
		t.Errorf("expected 1001 for <2 participants, got %v", err)
	}
	// participants > 5
	base.Participants = []string{"m1", "m2", "m3", "m1", "m2", "m3"}
	if _, err := e.Create(base); err == nil || err.Code != 1001 {
		t.Errorf("expected 1001 for >5 participants, got %v", err)
	}
	// max_rounds > 5
	base.Participants = []string{"m1", "m2"}
	base.MaxRounds = 6
	if _, err := e.Create(base); err == nil || err.Code != 1001 {
		t.Errorf("expected 1001 for max_rounds>5, got %v", err)
	}
	// 重复模型
	base.MaxRounds = 3
	base.Participants = []string{"m1", "m1"}
	if _, err := e.Create(base); err == nil || err.Code != 1001 {
		t.Errorf("expected 1001 for duplicate participants, got %v", err)
	}
	// 模型不存在
	base.Participants = []string{"m1", "nope"}
	if _, err := e.Create(base); err == nil || err.Code != 1004 {
		t.Errorf("expected 1004 for missing model, got %v", err)
	}
}

func TestCreateAndGet(t *testing.T) {
	e, _ := testEngine(t)
	sess, apiErr := e.Create(dto.CreateDebateRequest{
		Name:         "test debate",
		StrategyID:   "s1",
		Symbol:       "BTC-USDT",
		Participants: []string{"m1", "m2", "m3"},
		MaxRounds:    2,
	})
	if apiErr != nil {
		t.Fatalf("create: %v", apiErr)
	}
	if sess.Status != dto.DebatePending {
		t.Errorf("status: %s", sess.Status)
	}
	detail, apiErr := e.Get(sess.ID)
	if apiErr != nil {
		t.Fatalf("get: %v", apiErr)
	}
	if len(detail.Participants) != 3 {
		t.Errorf("participants: %d", len(detail.Participants))
	}
	// 人格按序分配
	if detail.Participants[0].Personality != dto.PersonalityBull {
		t.Errorf("first personality: %s", detail.Participants[0].Personality)
	}
}

func TestStartCancelTransitions(t *testing.T) {
	e, _ := testEngine(t)
	sess, _ := e.Create(dto.CreateDebateRequest{
		Name:         "t", StrategyID: "s1", Symbol: "BTC-USDT",
		Participants: []string{"m1", "m2"}, MaxRounds: 1,
	})
	// 未 start 先 cancel 应报错
	if err := e.Cancel(sess.ID); err == nil {
		t.Errorf("expected error cancelling pending")
	}
	if err := e.Start(sess.ID); err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := e.Cancel(sess.ID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	// 二次 start 应报错（已 cancelled）
	if err := e.Start(sess.ID); err == nil {
		t.Errorf("expected error restarting cancelled")
	}
}

func TestAggregateConsensus(t *testing.T) {
	e, _ := testEngine(t)
	sess, _ := e.Create(dto.CreateDebateRequest{
		Name: "t", StrategyID: "s1", Symbol: "BTC-USDT",
		Participants: []string{"m1", "m2", "m3"}, MaxRounds: 1,
	})
	// 3 票：两个 open_long（置信 80/70），一个 open_short（置信 60）
	for _, v := range []struct {
		mid    string
		action string
		conf   float64
	}{
		{"m1", "open_long", 80},
		{"m2", "open_long", 70},
		{"m3", "open_short", 60},
	} {
		e.store.AddDebateVote(&model.DebateVote{
			SessionID: sess.ID, AIModelID: v.mid, Action: v.action,
			Symbol: "BTC-USDT", Confidence: v.conf, Leverage: 5,
		})
	}
	c := e.aggregate(sess.ID)
	if c == nil || c.Action != "open_long" {
		t.Errorf("consensus: %+v", c)
	}
}

func TestAggregateNoConsensus(t *testing.T) {
	e, _ := testEngine(t)
	sess, _ := e.Create(dto.CreateDebateRequest{
		Name: "t", StrategyID: "s1", Symbol: "BTC-USDT",
		Participants: []string{"m1", "m2"}, MaxRounds: 1,
	})
	e.store.AddDebateVote(&model.DebateVote{SessionID: sess.ID, AIModelID: "m1", Action: "wait", Confidence: 50})
	e.store.AddDebateVote(&model.DebateVote{SessionID: sess.ID, AIModelID: "m2", Action: "wait", Confidence: 60})
	c := e.aggregate(sess.ID)
	if c == nil || c.Action != "wait" {
		t.Errorf("expected wait consensus, got %+v", c)
	}
}

func TestExtractJSONObject(t *testing.T) {
	raw := "here is my vote:\n```json\n{\"action\":\"open_long\"}\n```\nthanks"
	body := extractJSONObject(raw)
	if !strings.Contains(body, "open_long") {
		t.Errorf("extract failed: %q", body)
	}
	if body != `{"action":"open_long"}` {
		t.Errorf("got %q", body)
	}
	// 无对象
	if extractJSONObject("no json here") != "" {
		t.Errorf("expected empty for no object")
	}
}

func TestCastVoteParse(t *testing.T) {
	e, _ := testEngine(t)
	e.ai = llm.NewWithTransport(&fakeAI{body: `{"choices":[{"message":{"content":"{\"action\":\"open_long\",\"symbol\":\"BTC-USDT\",\"confidence\":85,\"leverage\":5,\"position_pct\":20}"}}]}`}, 5*time.Second)
	p := &model.DebateParticipant{
		SessionID: "s1", AIModelID: "m1", AIModelName: "gpt-4o",
		Personality: dto.PersonalityBull, SpeakOrder: 0,
	}
	vote, err := e.castVote(t.Context(), p, "BTC-USDT")
	if err != nil {
		t.Fatalf("castVote: %v", err)
	}
	if vote.Action != "open_long" || vote.Confidence != 85 || vote.Leverage != 5 {
		t.Errorf("vote: %+v", vote)
	}
}
