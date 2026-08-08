package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// BinanceAdapter binance USDT 永续参考实现（§14.2）。
// 签名：HMAC-SHA256(queryString, secret)；SL/TP 走 Algo Order（/fapi/v1/order
// 带 stopPrice+reduceOnly，旧 STOP_ORDER 报 -4120 的规避）。
type BinanceAdapter struct {
	client *http.Client
	base   string
	creds  Credentials

	mu   sync.Mutex
	step map[string]float64 // symbol → LOT_SIZE step（exchangeInfo 缓存）
}

// newBinance 构造 binance 适配器。
func newBinance(creds Credentials) (Adapter, error) {
	base := creds.BaseURL
	if base == "" {
		base = "https://fapi.binance.com"
	}
	if creds.Testnet {
		base = "https://testnet.binancefuture.com"
	}
	return &BinanceAdapter{
		client: &http.Client{Timeout: 15 * time.Second},
		base:   strings.TrimRight(base, "/"),
		creds:  creds,
		step:   make(map[string]float64),
	}, nil
}

// Name 适配器名。
func (a *BinanceAdapter) Name() string { return "binance" }

// init 自注册（依赖序提交：registry 空表 + 各适配器独立注册）。
func init() { Register("binance", newBinance) }

// ========== 开平仓 ==========

// OpenLong 开多（市价或限价）。
func (a *BinanceAdapter) OpenLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	p.Side = "BUY"
	p.PositionSide = "LONG"
	return a.placeOrder(ctx, p)
}

// OpenShort 开空。
func (a *BinanceAdapter) OpenShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	p.Side = "SELL"
	p.PositionSide = "SHORT"
	return a.placeOrder(ctx, p)
}

// CloseLong 平多。
func (a *BinanceAdapter) CloseLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	p.Side = "SELL"
	p.PositionSide = "LONG"
	p.ReduceOnly = true
	return a.placeOrder(ctx, p)
}

// CloseShort 平空。
func (a *BinanceAdapter) CloseShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	p.Side = "BUY"
	p.PositionSide = "SHORT"
	p.ReduceOnly = true
	return a.placeOrder(ctx, p)
}

// placeOrder 统一下单。市价（Price=0）带 reduceOnly 时用 MARKET；限价用 LIMIT+GTC。
func (a *BinanceAdapter) placeOrder(ctx context.Context, p OrderParams) (*OrderResult, error) {
	qty, err := a.FormatQuantity(ctx, p.Symbol, p.Quantity)
	if err != nil {
		return nil, err
	}
	if qty <= 0 {
		return nil, fmt.Errorf("%w: quantity too small after format: %f", ErrParams, p.Quantity)
	}
	// 杠杆
	if p.Leverage > 0 {
		_ = a.SetLeverage(ctx, p.Symbol, p.Leverage)
	}

	q := url.Values{}
	q.Set("symbol", p.Symbol)
	q.Set("side", p.Side)
	q.Set("type", "MARKET")
	q.Set("quantity", strconv.FormatFloat(qty, 'f', -1, 64))
	q.Set("newClientOrderId", p.ClientOrderID)
	if p.PositionSide != "" {
		q.Set("positionSide", p.PositionSide)
	}
	if p.ReduceOnly {
		q.Set("reduceOnly", "true")
	}
	if p.Price > 0 {
		q.Set("type", "LIMIT")
		q.Set("price", strconv.FormatFloat(p.Price, 'f', -1, 64))
		q.Set("timeInForce", "GTC")
	}

	var out struct {
		OrderID     int64  `json:"orderId"`
		ClientOrderID string `json:"clientOrderId"`
		Status      string `json:"status"`
		ExecutedQty string `json:"executedQty"`
		AvgPrice    string `json:"avgPrice"`
		CumQuote    string `json:"cumQuote"`
	}
	if err := a.do(ctx, http.MethodPost, "/fapi/v1/order", q, &out); err != nil {
		return nil, err
	}
	return &OrderResult{
		ExchangeOrderID: strconv.FormatInt(out.OrderID, 10),
		ClientOrderID:   out.ClientOrderID,
		Status:          out.Status,
		FilledQuantity:  parseF64(out.ExecutedQty),
		AvgFillPrice:    parseF64(out.AvgPrice),
	}, nil
}

// ========== 条件单 ==========

// SetStopLoss 止损（Algo Order：stopPrice + reduceOnly 市价）。
func (a *BinanceAdapter) SetStopLoss(ctx context.Context, symbol, side string, quantity, stopPrice float64, reduceOnly bool) (*OrderResult, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("side", side)
	q.Set("type", "STOP_MARKET")
	q.Set("quantity", strconv.FormatFloat(quantity, 'f', -1, 64))
	q.Set("stopPrice", strconv.FormatFloat(stopPrice, 'f', -1, 64))
	if reduceOnly {
		q.Set("reduceOnly", "true")
	}
	var out struct {
		OrderID int64  `json:"orderId"`
		Status  string `json:"status"`
	}
	if err := a.do(ctx, http.MethodPost, "/fapi/v1/order", q, &out); err != nil {
		return nil, err
	}
	return &OrderResult{ExchangeOrderID: strconv.FormatInt(out.OrderID, 10), Status: out.Status}, nil
}

// SetTakeProfit 止盈。
func (a *BinanceAdapter) SetTakeProfit(ctx context.Context, symbol, side string, quantity, takePrice float64, reduceOnly bool) (*OrderResult, error) {
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("side", side)
	q.Set("type", "TAKE_PROFIT_MARKET")
	q.Set("quantity", strconv.FormatFloat(quantity, 'f', -1, 64))
	q.Set("stopPrice", strconv.FormatFloat(takePrice, 'f', -1, 64))
	if reduceOnly {
		q.Set("reduceOnly", "true")
	}
	var out struct {
		OrderID int64  `json:"orderId"`
		Status  string `json:"status"`
	}
	if err := a.do(ctx, http.MethodPost, "/fapi/v1/order", q, &out); err != nil {
		return nil, err
	}
	return &OrderResult{ExchangeOrderID: strconv.FormatInt(out.OrderID, 10), Status: out.Status}, nil
}

// ========== 查询 ==========

// GetBalance 可用余额（USDT）。
func (a *BinanceAdapter) GetBalance(ctx context.Context) (float64, error) {
	var out []struct {
		Asset          string `json:"asset"`
		AvailableBalance string `json:"availableBalance"`
	}
	if err := a.do(ctx, http.MethodGet, "/fapi/v2/balance", nil, &out); err != nil {
		return 0, err
	}
	for _, b := range out {
		if b.Asset == "USDT" {
			return parseF64(b.AvailableBalance), nil
		}
	}
	return 0, fmt.Errorf("%w: USDT balance not found", ErrConn)
}

// GetPositions 当前持仓。
func (a *BinanceAdapter) GetPositions(ctx context.Context) ([]Position, error) {
	var out []struct {
		Symbol        string `json:"symbol"`
		PositionAmt   string `json:"positionAmt"`
		EntryPrice    string `json:"entryPrice"`
		MarkPrice     string `json:"markPrice"`
		UnrealizedPnL string `json:"unRealizedProfit"`
		Leverage      string `json:"leverage"`
	}
	if err := a.do(ctx, http.MethodGet, "/fapi/v2/positionRisk", nil, &out); err != nil {
		return nil, err
	}
	positions := make([]Position, 0, len(out))
	for _, p := range out {
		amt := parseF64(p.PositionAmt)
		if amt == 0 {
			continue
		}
		side := "long"
		if amt < 0 {
			side = "short"
		}
		positions = append(positions, Position{
			Symbol:        p.Symbol,
			Side:          side,
			Quantity:      absF(amt),
			EntryPrice:    parseF64(p.EntryPrice),
			MarkPrice:     parseF64(p.MarkPrice),
			UnrealizedPnL: parseF64(p.UnrealizedPnL),
			Leverage:      int(parseF64(p.Leverage)),
		})
	}
	return positions, nil
}

// GetOpenOrders 挂单。
func (a *BinanceAdapter) GetOpenOrders(ctx context.Context) ([]Order, error) {
	var out []struct {
		OrderID       int64  `json:"orderId"`
		Symbol        string `json:"symbol"`
		Side          string `json:"side"`
		OrigQty       string `json:"origQty"`
		Price         string `json:"price"`
		Status        string `json:"status"`
		ClientOrderID string `json:"clientOrderId"`
	}
	if err := a.do(ctx, http.MethodGet, "/fapi/v1/openOrders", nil, &out); err != nil {
		return nil, err
	}
	orders := make([]Order, 0, len(out))
	for _, o := range out {
		orders = append(orders, Order{
			ExchangeOrderID: strconv.FormatInt(o.OrderID, 10),
			Symbol:          o.Symbol,
			Side:            strings.ToLower(o.Side),
			Quantity:        parseF64(o.OrigQty),
			Price:           parseF64(o.Price),
			Status:          o.Status,
			ClientOrderID:   o.ClientOrderID,
		})
	}
	return orders, nil
}

// GetFills 成交（sinceMS 起；binance 最多 1000 条/次）。
func (a *BinanceAdapter) GetFills(ctx context.Context, sinceMS int64) ([]Fill, error) {
	q := url.Values{}
	if sinceMS > 0 {
		q.Set("startTime", strconv.FormatInt(sinceMS, 10))
	}
	var out []struct {
		ID            int64  `json:"id"`
		OrderID       int64  `json:"orderId"`
		Symbol        string `json:"symbol"`
		Side          string `json:"side"`
		Price         string `json:"price"`
		Qty           string `json:"qty"`
		Commission    string `json:"commission"`
		CommissionAsset string `json:"commissionAsset"`
		RealizedPnL   string `json:"realizedPnl"`
		Maker         bool   `json:"maker"`
		Time          int64  `json:"time"`
	}
	if err := a.do(ctx, http.MethodGet, "/fapi/v1/userTrades", q, &out); err != nil {
		return nil, err
	}
	fills := make([]Fill, 0, len(out))
	for _, f := range out {
		fills = append(fills, Fill{
			ExchangeTradeID: strconv.FormatInt(f.ID, 10),
			ExchangeOrderID: strconv.FormatInt(f.OrderID, 10),
			Symbol:          f.Symbol,
			Side:            strings.ToLower(f.Side),
			Price:           parseF64(f.Price),
			Quantity:        parseF64(f.Qty),
			Commission:      parseF64(f.Commission),
			CommissionAsset: f.CommissionAsset,
			RealizedPnL:     parseF64(f.RealizedPnL),
			IsMaker:         f.Maker,
			TimestampMS:     f.Time,
		})
	}
	return fills, nil
}

// ========== 工具 ==========

// FormatQuantity LOT_SIZE step（exchangeInfo 缓存）。
func (a *BinanceAdapter) FormatQuantity(ctx context.Context, symbol string, qty float64) (float64, error) {
	step, err := a.lotStep(ctx, symbol)
	if err != nil {
		return 0, err
	}
	return FormatBinance(qty, step), nil
}

// GetLeverage 当前杠杆。
func (a *BinanceAdapter) GetLeverage(ctx context.Context, symbol string) (int, error) {
	positions, err := a.GetPositions(ctx)
	if err != nil {
		return 0, err
	}
	for _, p := range positions {
		if p.Symbol == symbol {
			return p.Leverage, nil
		}
	}
	return 0, nil
}

// SetLeverage 设置杠杆。
func (a *BinanceAdapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	q := url.Values{}
	q.Set("symbol", symbol)
	q.Set("leverage", strconv.Itoa(leverage))
	var out struct{}
	return a.do(ctx, http.MethodPost, "/fapi/v1/leverage", q, &out)
}

// lotStep 取 LOT_SIZE step 并缓存。
func (a *BinanceAdapter) lotStep(ctx context.Context, symbol string) (float64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s, ok := a.step[symbol]; ok {
		return s, nil
	}
	var info struct {
		Symbols []struct {
			Symbol string `json:"symbol"`
			Filters []struct {
				FilterType string `json:"filterType"`
				StepSize   string `json:"stepSize"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	if err := a.do(ctx, http.MethodGet, "/fapi/v1/exchangeInfo", nil, &info); err != nil {
		return 0, err
	}
	for _, s := range info.Symbols {
		if s.Symbol != symbol {
			continue
		}
		for _, f := range s.Filters {
			if f.FilterType == "LOT_SIZE" {
				step := parseF64(f.StepSize)
				if step > 0 {
					a.step[symbol] = step
					return step, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("%w: LOT_SIZE not found for %s", ErrParams, symbol)
}

// ========== 签名与请求 ==========

// do 带签名的请求：query 附加 timestamp+recvWindow 后 HMAC-SHA256。
func (a *BinanceAdapter) do(ctx context.Context, method, path string, query url.Values, out any) error {
	if query == nil {
		query = url.Values{}
	}
	query.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
	query.Set("recvWindow", "5000")
	qs := query.Encode()
	mac := hmac.New(sha256.New, []byte(a.creds.SecretKey))
	_, _ = mac.Write([]byte(qs))
	qs += "&signature=" + hex.EncodeToString(mac.Sum(nil))

	req, err := http.NewRequestWithContext(ctx, method, a.base+path+"?"+qs, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConn, err)
	}
	req.Header.Set("X-MBX-APIKEY", a.creds.APIKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConn, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%w: status %d: %s", ErrConn, resp.StatusCode, truncateStr(string(raw), 200))
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("%w: parse response: %v", ErrConn, err)
	}
	return nil
}

// parseF64 安全解析浮点字符串。
func parseF64(s string) float64 {
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func absF(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
