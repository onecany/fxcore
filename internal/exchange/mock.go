package exchange

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// MockAdapter 内存模拟适配器（测试/冒烟用，非真实下单）。
// 行为：开仓即成交（市价），仓位记账在本适配器，平仓结算 realized PnL。
type MockAdapter struct {
	mu        sync.Mutex
	balance   float64
	positions map[string]*MockPos // symbol → 仓位
	fills     []Fill
	seq       int64
}

// MockPos 模拟持仓。
type MockPos struct {
	Symbol string
	Side   string
	Qty    float64
	Entry  float64
}

// NewMock 构造模拟适配器（初始余额 10000）。
func NewMock(initialBalance float64) *MockAdapter {
	return &MockAdapter{balance: initialBalance, positions: make(map[string]*MockPos)}
}

// Name 适配器名。
func (m *MockAdapter) Name() string { return "mock" }

// register 注册为可构造类型（测试用）。
func init() {
	Register("mock", func(creds Credentials) (Adapter, error) {
		return NewMock(10000), nil
	})
}

// OpenLong 开多：按 Quantity*Price 占用余额（简化 1x）。
func (m *MockAdapter) OpenLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return m.open(p, "long")
}

// OpenShort 开空。
func (m *MockAdapter) OpenShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return m.open(p, "short")
}

// CloseLong 平多。
func (m *MockAdapter) CloseLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return m.close(p, "long")
}

// CloseShort 平空。
func (m *MockAdapter) CloseShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return m.close(p, "short")
}

func (m *MockAdapter) open(p OrderParams, side string) (*OrderResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.positions[p.Symbol]; exists {
		return nil, fmt.Errorf("%w: position already open for %s", ErrParams, p.Symbol)
	}
	price := p.Price
	if price <= 0 {
		price = 100 // 模拟市价 100 USDT
	}
	cost := p.Quantity * price
	if cost > m.balance {
		return nil, fmt.Errorf("%w: need %.2f have %.2f", ErrInsufficient, cost, m.balance)
	}
	m.balance -= cost
	m.seq++
	m.positions[p.Symbol] = &MockPos{Symbol: p.Symbol, Side: side, Qty: p.Quantity, Entry: price}
	m.fills = append(m.fills, Fill{
		ExchangeTradeID: fmt.Sprintf("mock-fill-%d", m.seq),
		ExchangeOrderID: fmt.Sprintf("mock-order-%d", m.seq),
		Symbol: p.Symbol, Side: side, Price: price, Quantity: p.Quantity,
		TimestampMS: time.Now().UnixMilli(),
	})
	return &OrderResult{ExchangeOrderID: fmt.Sprintf("mock-order-%d", m.seq), Status: "FILLED", FilledQuantity: p.Quantity, AvgFillPrice: price}, nil
}

func (m *MockAdapter) close(p OrderParams, side string) (*OrderResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pos, ok := m.positions[p.Symbol]
	if !ok || pos.Side != side {
		return nil, fmt.Errorf("%w: no %s position for %s", ErrParams, side, p.Symbol)
	}
	price := p.Price
	if price <= 0 {
		price = 100
	}
	dir := 1.0
	if side == "short" {
		dir = -1.0
	}
	pnl := (price - pos.Entry) * pos.Qty * dir
	m.balance += pos.Qty*pos.Entry + pnl
	m.seq++
	delete(m.positions, p.Symbol)
	m.fills = append(m.fills, Fill{
		ExchangeTradeID: fmt.Sprintf("mock-fill-%d", m.seq),
		ExchangeOrderID: fmt.Sprintf("mock-order-%d", m.seq),
		Symbol: p.Symbol, Side: side, Price: price, Quantity: pos.Qty,
		RealizedPnL: pnl, TimestampMS: time.Now().UnixMilli(),
	})
	return &OrderResult{ExchangeOrderID: fmt.Sprintf("mock-order-%d", m.seq), Status: "FILLED", FilledQuantity: pos.Qty, AvgFillPrice: price}, nil
}

// SetStopLoss 骨架。
func (m *MockAdapter) SetStopLoss(ctx context.Context, symbol, side string, quantity, stopPrice float64, reduceOnly bool) (*OrderResult, error) {
	return nil, ErrNotImplemented
}

// SetTakeProfit 骨架。
func (m *MockAdapter) SetTakeProfit(ctx context.Context, symbol, side string, quantity, takePrice float64, reduceOnly bool) (*OrderResult, error) {
	return nil, ErrNotImplemented
}

// GetBalance 余额。
func (m *MockAdapter) GetBalance(ctx context.Context) (float64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.balance, nil
}

// GetPositions 持仓。
func (m *MockAdapter) GetPositions(ctx context.Context) ([]Position, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Position, 0, len(m.positions))
	for _, p := range m.positions {
		out = append(out, Position{Symbol: p.Symbol, Side: p.Side, Quantity: p.Qty, EntryPrice: p.Entry, MarkPrice: p.Entry})
	}
	return out, nil
}

// GetOpenOrders 空。
func (m *MockAdapter) GetOpenOrders(ctx context.Context) ([]Order, error) {
	return []Order{}, nil
}

// GetFills 成交。
func (m *MockAdapter) GetFills(ctx context.Context, sinceMS int64) ([]Fill, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Fill, 0, len(m.fills))
	for _, f := range m.fills {
		if f.TimestampMS >= sinceMS {
			out = append(out, f)
		}
	}
	return out, nil
}

// FormatQuantity mock 直接返回原值。
func (m *MockAdapter) FormatQuantity(ctx context.Context, symbol string, qty float64) (float64, error) {
	return qty, nil
}

// GetLeverage 返回 1。
func (m *MockAdapter) GetLeverage(ctx context.Context, symbol string) (int, error) { return 1, nil }

// SetLeverage 无操作。
func (m *MockAdapter) SetLeverage(ctx context.Context, symbol string, leverage int) error { return nil }
