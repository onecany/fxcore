package backtest

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/llm"
	"fxcore/internal/provider"
	"fxcore/internal/store"
)

// mockKlines 固定 K 线源。
type mockKlines struct{}

func (m *mockKlines) Klines(ctx context.Context, symbol, interval string, limit int) ([]dto.KlineDTO, error) {
	return []dto.KlineDTO{
		{Timestamp: 100, Open: 100, High: 105, Low: 99, Close: 102, Volume: 10},
		{Timestamp: 200, Open: 102, High: 104, Low: 100, Close: 101, Volume: 8},
	}, nil
}
func (m *mockKlines) Name() string { return "mock" }

// mockAI llm.Client（mock transport 返回决策）。
func mockAI(t *testing.T) *llm.Client {
	rt := &fakeRoundTrip{body: `{"choices":[{"message":{"content":"<decision>[{\"action\":\"open_long\",\"symbol\":\"BTC-USDT\",\"quantity\":0.001,\"leverage\":5,\"confidence\":80}]</decision>"}}]}`}
	return llm.NewWithTransport(rt, 5*time.Second)
}

// fakeRoundTrip mock transport。
type fakeRoundTrip struct {
	body string
}

func (r *fakeRoundTrip) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(r.body)),
	}, nil
}

// testEngine 构造带 mock 依赖的引擎。
func testEngine(t *testing.T) (*Engine, *store.Store) {
	st, err := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"})
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	models := &nilModelProvider{}
	return NewEngine(st, &mockKlines{}, models, mockAI(t)), st
}

// nilModelProvider 空模型提供者（测试用：无模型则回测走 safe wait 路径）。
type nilModelProvider struct{}

func (p *nilModelProvider) GetModel(id string) (*llm.Model, bool) { return nil, false }

func TestStartLockConflict(t *testing.T) {
	e, _ := testEngine(t)
	cfg := dto.BacktestConfig{
		Symbols:        []string{"BTC-USDT"},
		InitialBalance: 10000,
		Leverage:       5,
		StartTime:      time.Now().Add(-time.Hour).Unix(),
		EndTime:        time.Now().Unix(),
		Cadence:        1,
	}
	meta, apiErr := e.Start("u1", cfg)
	if apiErr != nil {
		t.Fatalf("first start: %v", apiErr)
	}
	defer e.Stop(meta.RunID)

	_, apiErr = e.Start("u1", cfg)
	if apiErr == nil || apiErr.Code != 1411 {
		t.Errorf("expected 1411 lock conflict, got %v", apiErr)
	}
}

func TestStartStopTransition(t *testing.T) {
	e, _ := testEngine(t)
	cfg := dto.BacktestConfig{
		Symbols:        []string{"BTC-USDT"},
		InitialBalance: 10000,
		StartTime:      time.Now().Add(-time.Hour).Unix(),
		EndTime:        time.Now().Unix(),
		Cadence:        1,
	}
	meta, apiErr := e.Start("u1", cfg)
	if apiErr != nil {
		t.Fatalf("start: %v", apiErr)
	}
	if meta.State != dto.BacktestRunning {
		t.Errorf("state after start: %s", meta.State)
	}
	// pause → resume → stop
	if m, err := e.Pause(meta.RunID); err != nil || m.State != dto.BacktestPaused {
		t.Errorf("pause: %v %v", m, err)
	}
	if m, err := e.Resume(meta.RunID); err != nil || m.State != dto.BacktestRunning {
		t.Errorf("resume: %v %v", m, err)
	}
	if m, err := e.Stop(meta.RunID); err != nil || m.State != dto.BacktestStopped {
		t.Errorf("stop: %v %v", m, err)
	}
	// 终态再 stop 应报错
	if _, err := e.Stop(meta.RunID); err == nil {
		t.Errorf("expected error on double stop")
	}
}

func TestDeleteRunningRejected(t *testing.T) {
	e, _ := testEngine(t)
	cfg := dto.BacktestConfig{
		Symbols:        []string{"BTC-USDT"},
		InitialBalance: 10000,
		StartTime:      time.Now().Add(-time.Hour).Unix(),
		EndTime:        time.Now().Unix(),
		Cadence:        1,
	}
	meta, apiErr := e.Start("u1", cfg)
	if apiErr != nil {
		t.Fatalf("start: %v", apiErr)
	}
	defer e.Stop(meta.RunID)
	if err := e.Delete(meta.RunID); err == nil || err.Code != 1411 {
		t.Errorf("expected 1411 on delete running, got %v", err)
	}
}

func TestStartValidations(t *testing.T) {
	e, _ := testEngine(t)
	// 无 symbols
	if _, err := e.Start("u1", dto.BacktestConfig{}); err == nil || err.Code != 1001 {
		t.Errorf("expected 1001 for empty symbols, got %v", err)
	}
	// end <= start
	if _, err := e.Start("u1", dto.BacktestConfig{
		Symbols:   []string{"BTC-USDT"},
		StartTime: time.Now().Unix(),
		EndTime:   time.Now().Add(-time.Hour).Unix(),
	}); err == nil || err.Code != 1001 {
		t.Errorf("expected 1001 for inverted window, got %v", err)
	}
}

var _ = provider.KlineProvider(nil)
