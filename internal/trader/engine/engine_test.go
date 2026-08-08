package engine

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/exchange"
	"fxcore/internal/llm"
	"fxcore/internal/model"
	"fxcore/internal/store"
)

func mustMarshal(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestMarginRate(t *testing.T) {
	close := func(a, b float64) bool { return a-b < 1e-9 && b-a < 1e-9 }
	if !close(MarginRate(5), 1.01/5+0.001) {
		t.Errorf("MarginRate(5) = %v", MarginRate(5))
	}
	if !close(MarginRate(10), 1.01/10+0.001) {
		t.Errorf("MarginRate(10) = %v", MarginRate(10))
	}
	if !close(MarginRate(0), 1.011) {
		t.Errorf("MarginRate(0) should default to 1x: %v", MarginRate(0))
	}
}

func TestMinPositionSize(t *testing.T) {
	if minPositionSize("BTC-USDT") != 60 {
		t.Errorf("BTC min should be 60")
	}
	if minPositionSize("ETHUSDT") != 60 {
		t.Errorf("ETH min should be 60")
	}
	if minPositionSize("SOL-USDT") != 12 {
		t.Errorf("alt min should be 12")
	}
}

func TestIsBTCEth(t *testing.T) {
	for _, s := range []string{"BTC-USDT", "BTCUSDT", "ETH-USDT", "ethusdt"} {
		if !isBTCEth(s) {
			t.Errorf("%s should be BTCEth", s)
		}
	}
	for _, s := range []string{"SOL-USDT", "DOGE-USDT"} {
		if isBTCEth(s) {
			t.Errorf("%s should not be BTCEth", s)
		}
	}
}

func TestFilterByRisk(t *testing.T) {
	st, _ := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"})
	// 建交易员（无模型，仅风控过滤用）
	st.CreateTrader(&model.Trader{ID: "t1", Status: model.StatusRunning, StrategyID: ""})
	e := &Engine{store: st}
	adapter := exchange.NewMock(10000)

	// max_positions=3：4 个开仓动作只放行 3 个
	actions := []model.DecisionAction{
		{Action: "open_long", Symbol: "BTC-USDT", Quantity: 0.01, Price: 60000, Leverage: 5},
		{Action: "open_long", Symbol: "ETH-USDT", Quantity: 1, Price: 3000, Leverage: 5},
		{Action: "open_long", Symbol: "SOL-USDT", Quantity: 10, Price: 100, Leverage: 5},
		{Action: "open_long", Symbol: "DOGE-USDT", Quantity: 1000, Price: 0.1, Leverage: 5},
	}
	trader, _ := st.GetTrader("t1")
	filtered := e.filterByRisk(trader, adapter, 10000, actions)
	if len(filtered) != 3 {
		t.Errorf("expected 3 after max_positions, got %d", len(filtered))
	}

	// 仓值比：BTC 0.01*60000=600 ≤ 5×10000 OK；alt 仓值 > 1×equity 拒绝
	actions = []model.DecisionAction{
		{Action: "open_long", Symbol: "SOL-USDT", Quantity: 200, Price: 100, Leverage: 5}, // 20000 > 10000 拒绝
	}
	filtered = e.filterByRisk(trader, adapter, 10000, actions)
	if len(filtered) != 0 {
		t.Errorf("expected 0 (alt value exceeds equity), got %d", len(filtered))
	}

	// 杠杆超限自动降档：alt 限 5，传 20 → 5
	actions = []model.DecisionAction{
		{Action: "open_long", Symbol: "SOL-USDT", Quantity: 1, Price: 100, Leverage: 20},
	}
	filtered = e.filterByRisk(trader, adapter, 10000, actions)
	if len(filtered) != 1 || int(filtered[0].Leverage) != 5 {
		t.Errorf("leverage should downgrade to 5: %+v", filtered)
	}
}

// mockCreds 测试凭据解析器。
type mockCreds struct{}

func (m *mockCreds) Resolve(exchangeType string) (*exchange.Credentials, bool) {
	return &exchange.Credentials{ExchangeType: "mock"}, true
}

// mockModels 测试模型提供者。
type mockModels struct{ m *llm.Model }

func (m *mockModels) GetModel(id string) (*llm.Model, bool) { return m.m, true }

func TestEngineStartStopLifecycle(t *testing.T) {
	st, _ := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"})
	st.CreateTrader(&model.Trader{
		ID: "t1", Exchange: "mock", Status: model.StatusRunning,
		ModelConfig: model.ModelConfig{ModelID: "m1"},
	})
	// 建策略（风控默认）
	cfg := dto.StrategyConfig{RiskControl: dto.RiskControlConfig{MaxPositions: 3, BTCEthMaxLeverage: 5, AltcoinMaxLeverage: 5}}
	st.CreateStrategy(&model.Strategy{ID: "s1", Config: mustMarshal(cfg)})
	st.UpdateTrader(&model.Trader{ID: "t1", Exchange: "mock", Status: model.StatusRunning, StrategyID: "s1", ModelConfig: model.ModelConfig{ModelID: "m1"}})

	e := NewEngine(st, &mockCreds{}, &mockModels{}, mockAI(), nil)
	if err := e.Start("t1"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if !e.IsRunning("t1") {
		t.Errorf("IsRunning should be true")
	}
	// 重复 Start 拒绝
	if err := e.Start("t1"); err == nil {
		t.Errorf("expected error on double start")
	}
	e.Stop("t1")
	if e.IsRunning("t1") {
		t.Errorf("IsRunning should be false after stop")
	}
}

func TestOrderSyncIdempotent(t *testing.T) {
	st, _ := store.New(store.Config{AdminEmail: "a@b.c", AdminPassword: "pw"})
	st.CreateTrader(&model.Trader{ID: "t1", Exchange: "mock", Status: model.StatusRunning})
	_ = NewEngine(st, &mockCreds{}, &mockModels{}, mockAI(), nil)

	// mock 适配器填充一笔成交
	m := exchange.NewMock(10000)
	_, _ = m.OpenLong(t.Context(), exchange.OrderParams{Symbol: "BTC-USDT", Quantity: 1, Price: 100})
	fills, _ := m.GetFills(t.Context(), 0)
	if len(fills) != 1 {
		t.Fatalf("setup fills: %d", len(fills))
	}

	// 直接测 store 幂等：同 exchange_trade_id 写入两次
	f := fills[0]
	first, added1 := st.AddFill(&model.Fill{TraderID: "t1", ExchangeOrderID: f.ExchangeOrderID, ExchangeTradeID: f.ExchangeTradeID, Symbol: f.Symbol, Side: f.Side, Price: f.Price, Quantity: f.Quantity})
	if !added1 || first == nil {
		t.Errorf("first add should succeed")
	}
	_, added2 := st.AddFill(&model.Fill{TraderID: "t1", ExchangeOrderID: f.ExchangeOrderID, ExchangeTradeID: f.ExchangeTradeID, Symbol: f.Symbol, Side: f.Side, Price: f.Price, Quantity: f.Quantity})
	if added2 {
		t.Errorf("second add should be deduped (same exchange_trade_id)")
	}
	if len(st.ListFillsByTrader("t1")) != 1 {
		t.Errorf("fill count should be 1 after dedup")
	}
	// 同 order 幂等
	st.AddOrder(&model.Order{TraderID: "t1", ExchangeOrderID: "o1", Symbol: "BTC-USDT", Quantity: 1, Status: "FILLED"})
	_, orderAdded2 := st.AddOrder(&model.Order{TraderID: "t1", ExchangeOrderID: "o1", Symbol: "BTC-USDT", Quantity: 1, Status: "FILLED"})
	if orderAdded2 {
		t.Errorf("second order add should be deduped")
	}
}

// mockAI 假 AI 客户端（返回 hold 决策，不触发下单）。
func mockAI() *llm.Client {
	return llm.NewWithTransport(&fakeRT{}, 5*time.Second)
}

// fakeRT mock transport（返回 hold 决策）。
type fakeRT struct{}

func (f *fakeRT) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: 200,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"<decision>[{\"action\":\"hold\",\"symbol\":\"BTC-USDT\"}]</decision>"}}]}`)),
	}, nil
}
