package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// GateAdapter gate v4 USDT 永续适配器（§14.2）。
// 签名：HMAC-SHA512(method\nurl\nquery\nsha512(body))，头 KEY/SIGN/Timestamp；
// 数量取整（Gate 合约最小 1 张）。
type GateAdapter struct {
	client *http.Client
	base   string
	creds  Credentials
}

// newGate 构造。
func newGate(creds Credentials) (Adapter, error) {
	base := creds.BaseURL
	if base == "" {
		base = "https://api.gateio.ws"
	}
	return &GateAdapter{
		client: &http.Client{Timeout: 15 * time.Second},
		base:   strings.TrimRight(base, "/"),
		creds:  creds,
	}, nil
}

// Name 适配器名。
func (a *GateAdapter) Name() string { return "gate" }

// init 自注册。
func init() { Register("gate", newGate) }

// ========== 开平仓 ==========

// OpenLong 开多。
func (a *GateAdapter) OpenLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, false)
}

// OpenShort 开空。
func (a *GateAdapter) OpenShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, true)
}

// CloseLong 平多。
func (a *GateAdapter) CloseLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	p.ReduceOnly = true
	return a.placeOrder(ctx, p, true)
}

// CloseShort 平空。
func (a *GateAdapter) CloseShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	p.ReduceOnly = true
	return a.placeOrder(ctx, p, false)
}

// placeOrder 统一下单（/api/v4/futures/usdt/orders）。
// gate 单向持仓：开多 buy、开空 sell、平多 sell、平空 buy（reduceOnly 标记）。
func (a *GateAdapter) placeOrder(ctx context.Context, p OrderParams, sell bool) (*OrderResult, error) {
	qty := FormatGate(p.Quantity)
	if qty <= 0 {
		return nil, fmt.Errorf("%w: quantity too small: %f", ErrParams, p.Quantity)
	}
	side := "buy"
	if sell {
		side = "sell"
	}
	payload := map[string]any{
		"contract": p.Symbol,
		"size":     int64(qty), // gate 张数（整数）
		"price":    "0",        // 市价
		"tif":      "ioc",
	}
	if p.ReduceOnly {
		payload["reduce_only"] = true
	}
	if p.Price > 0 {
		payload["price"] = strconv.FormatFloat(p.Price, 'f', -1, 64)
		payload["tif"] = "gtc"
	}
	// gate 无 side 字段，用正负 size 表示方向：正=买、负=卖
	if side == "sell" {
		payload["size"] = -int64(qty)
	}
	if p.ClientOrderID != "" {
		payload["text"] = "t-" + truncateStr(p.ClientOrderID, 32)
	}

	var out struct {
		ID      int64  `json:"id"`
		Status  string `json:"status"`
		Size    int64  `json:"size"`
		Left    int64  `json:"left"`
		FillPx  string `json:"fill_price"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v4/futures/usdt/orders", payload, &out); err != nil {
		return nil, err
	}
	status := out.Status
	if out.Left == 0 {
		status = "FILLED"
	}
	return &OrderResult{
		ExchangeOrderID: strconv.FormatInt(out.ID, 10),
		Status:          status,
		FilledQuantity:  float64(out.Size - out.Left),
		AvgFillPrice:    parseF64(out.FillPx),
	}, nil
}

// ========== 条件单 ==========

// SetStopLoss 止损（计划委托）。
func (a *GateAdapter) SetStopLoss(ctx context.Context, symbol, side string, quantity, stopPrice float64, reduceOnly bool) (*OrderResult, error) {
	payload := map[string]any{
		"contract": symbol,
		"size":     int64(FormatGate(quantity)),
		"price":    "0",
		"stop":     strconv.FormatFloat(stopPrice, 'f', -1, 64),
		"tif":      "ioc",
	}
	if side == "sell" {
		payload["size"] = -int64(FormatGate(quantity))
	}
	var out struct {
		ID int64 `json:"id"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v4/futures/usdt/price_orders", payload, &out); err != nil {
		return nil, err
	}
	return &OrderResult{ExchangeOrderID: strconv.FormatInt(out.ID, 10)}, nil
}

// SetTakeProfit 止盈。
func (a *GateAdapter) SetTakeProfit(ctx context.Context, symbol, side string, quantity, takePrice float64, reduceOnly bool) (*OrderResult, error) {
	payload := map[string]any{
		"contract": symbol,
		"size":     int64(FormatGate(quantity)),
		"price":    "0",
		"stop":     strconv.FormatFloat(takePrice, 'f', -1, 64),
		"tif":      "ioc",
	}
	if side == "sell" {
		payload["size"] = -int64(FormatGate(quantity))
	}
	var out struct {
		ID int64 `json:"id"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v4/futures/usdt/price_orders", payload, &out); err != nil {
		return nil, err
	}
	return &OrderResult{ExchangeOrderID: strconv.FormatInt(out.ID, 10)}, nil
}

// ========== 查询 ==========

// GetBalance 可用余额（USDT）。
func (a *GateAdapter) GetBalance(ctx context.Context) (float64, error) {
	var out struct {
		Available string `json:"available"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v4/futures/usdt/accounts", nil, &out); err != nil {
		return 0, err
	}
	return parseF64(out.Available), nil
}

// GetPositions 持仓。
func (a *GateAdapter) GetPositions(ctx context.Context) ([]Position, error) {
	var out []struct {
		Contract string `json:"contract"`
		Size     int64  `json:"size"`
		EntryPrice string `json:"entry_price"`
		MarkPrice  string `json:"mark_price"`
		UnrealisedPnl string `json:"unrealised_pnl"`
		Leverage string `json:"leverage"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v4/futures/usdt/positions", nil, &out); err != nil {
		return nil, err
	}
	positions := make([]Position, 0, len(out))
	for _, p := range out {
		if p.Size == 0 {
			continue
		}
		side := "long"
		if p.Size < 0 {
			side = "short"
		}
		positions = append(positions, Position{
			Symbol:        p.Contract,
			Side:          side,
			Quantity:      float64(absI64(p.Size)),
			EntryPrice:    parseF64(p.EntryPrice),
			MarkPrice:     parseF64(p.MarkPrice),
			UnrealizedPnL: parseF64(p.UnrealisedPnl),
			Leverage:      int(parseF64(p.Leverage)),
		})
	}
	return positions, nil
}

// GetOpenOrders 挂单。
func (a *GateAdapter) GetOpenOrders(ctx context.Context) ([]Order, error) {
	var out []struct {
		ID       int64  `json:"id"`
		Contract string `json:"contract"`
		Size     int64  `json:"size"`
		Price    string `json:"price"`
		Status   string `json:"status"`
		Text     string `json:"text"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v4/futures/usdt/orders?status=open", nil, &out); err != nil {
		return nil, err
	}
	orders := make([]Order, 0, len(out))
	for _, o := range out {
		side := "buy"
		if o.Size < 0 {
			side = "sell"
		}
		orders = append(orders, Order{
			ExchangeOrderID: strconv.FormatInt(o.ID, 10),
			Symbol:          o.Contract,
			Side:            side,
			Quantity:        float64(absI64(o.Size)),
			Price:           parseF64(o.Price),
			Status:          o.Status,
			ClientOrderID:   o.Text,
		})
	}
	return orders, nil
}

// GetFills 成交。
func (a *GateAdapter) GetFills(ctx context.Context, sinceMS int64) ([]Fill, error) {
	q := "/api/v4/futures/usdt/my_trades"
	if sinceMS > 0 {
		q += "?start=" + strconv.FormatInt(sinceMS, 10)
	}
	var out []struct {
		ID      int64  `json:"id"`
		OrderID string `json:"order_id"`
		Contract string `json:"contract"`
		Size    int64  `json:"size"`
		Price   string `json:"price"`
		Fee     string `json:"fee"`
		Text    string `json:"text"`
		CreateTime float64 `json:"create_time"`
	}
	if err := a.request(ctx, http.MethodGet, q, nil, &out); err != nil {
		return nil, err
	}
	fills := make([]Fill, 0, len(out))
	for _, f := range out {
		side := "buy"
		if f.Size < 0 {
			side = "sell"
		}
		fills = append(fills, Fill{
			ExchangeTradeID: strconv.FormatInt(f.ID, 10),
			ExchangeOrderID: f.OrderID,
			Symbol:          f.Contract,
			Side:            side,
			Price:           parseF64(f.Price),
			Quantity:        float64(absI64(f.Size)),
			Commission:      parseF64(f.Fee),
			TimestampMS:     int64(f.CreateTime),
		})
	}
	return fills, nil
}

// ========== 工具 ==========

// FormatQuantity 取整。
func (a *GateAdapter) FormatQuantity(ctx context.Context, symbol string, qty float64) (float64, error) {
	return FormatGate(qty), nil
}

// GetLeverage 当前杠杆。
func (a *GateAdapter) GetLeverage(ctx context.Context, symbol string) (int, error) {
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
func (a *GateAdapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	payload := map[string]any{
		"leverage": strconv.Itoa(leverage),
	}
	var out struct{}
	return a.request(ctx, http.MethodPost, "/api/v4/futures/usdt/positions/"+symbol+"/leverage", payload, &out)
}

// ========== 签名与请求 ==========

// sign 签名：HMAC-SHA512(method\nurl\nquery\nsha512(body))。
func (a *GateAdapter) sign(ts, method, path, query, body string) string {
	hash := sha512.Sum512([]byte(body))
	mac := hmac.New(sha512.New, []byte(a.creds.SecretKey))
	_, _ = mac.Write([]byte(fmt.Sprintf("%s\n%s\n%s\n%s", method, path, query, hex.EncodeToString(hash[:]))))
	return hex.EncodeToString(mac.Sum(nil))
}

// request 统一请求。
func (a *GateAdapter) request(ctx context.Context, method, path string, payload any, out any) error {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrParams, err)
		}
	}
	ts := strconv.FormatInt(time.Now().Unix(), 10)
	pathOnly := path
	query := ""
	if i := strings.Index(path, "?"); i >= 0 {
		pathOnly = path[:i]
		query = path[i+1:]
	}
	sig := a.sign(ts, method, pathOnly, query, string(body))

	req, err := http.NewRequestWithContext(ctx, method, a.base+path, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConn, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("KEY", a.creds.APIKey)
	req.Header.Set("SIGN", sig)
	req.Header.Set("Timestamp", ts)

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
	return json.Unmarshal(raw, out)
}

func absI64(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
