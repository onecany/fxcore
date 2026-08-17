package exchange

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// KuCoinAdapter kucoin v3 永续适配器（§14.2）。
// 签名：Base64(HMAC-SHA256(timestamp+method+path+body))，头 KC-API-*；
// 数量 round 到 step（contracts 缓存）。
type KuCoinAdapter struct {
	client *http.Client
	base   string
	creds  Credentials

	mu   sync.Mutex
	step map[string]float64
}

// newKuCoin 构造。
func newKuCoin(creds Credentials) (Adapter, error) {
	base := creds.BaseURL
	if base == "" {
		base = "https://api-futures.kucoin.com"
	}
	return &KuCoinAdapter{
		client: &http.Client{Timeout: 15 * time.Second},
		base:   strings.TrimRight(base, "/"),
		creds:  creds,
		step:   make(map[string]float64),
	}, nil
}

// Name 适配器名。
func (a *KuCoinAdapter) Name() string { return "kucoin" }

// init 自注册。
func init() { Register("kucoin", newKuCoin) }

// ========== 开平仓 ==========

// OpenLong 开多。
func (a *KuCoinAdapter) OpenLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, "buy", "long")
}

// OpenShort 开空。
func (a *KuCoinAdapter) OpenShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, "sell", "short")
}

// CloseLong 平多。
func (a *KuCoinAdapter) CloseLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	p.ReduceOnly = true
	return a.placeOrder(ctx, p, "sell", "long")
}

// CloseShort 平空。
func (a *KuCoinAdapter) CloseShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	p.ReduceOnly = true
	return a.placeOrder(ctx, p, "buy", "short")
}

// placeOrder 统一下单（/api/v1/orders）。
func (a *KuCoinAdapter) placeOrder(ctx context.Context, p OrderParams, side, posSide string) (*OrderResult, error) {
	qty, err := a.FormatQuantity(ctx, p.Symbol, p.Quantity)
	if err != nil {
		return nil, err
	}
	if qty <= 0 {
		return nil, fmt.Errorf("%w: quantity too small: %f", ErrParams, p.Quantity)
	}
	if p.Leverage > 0 {
		_ = a.SetLeverage(ctx, p.Symbol, p.Leverage)
	}

	payload := map[string]any{
		"symbol":   p.Symbol,
		"type":     "market",
		"side":     side,
		"size":     qty,
		"leverage": strconv.Itoa(p.Leverage),
	}
	if p.ClientOrderID != "" {
		payload["clientOid"] = p.ClientOrderID
	}
	if p.ReduceOnly {
		payload["closeOrder"] = true
	}
	if p.Price > 0 {
		payload["type"] = "limit"
		payload["price"] = strconv.FormatFloat(p.Price, 'f', -1, 64)
	}
	// SL/TP（stop 条件）
	if p.StopLoss > 0 {
		payload["stop"] = "down"
		payload["stopPriceType"] = "MP"
		payload["stopPrice"] = strconv.FormatFloat(p.StopLoss, 'f', -1, 64)
	} else if p.TakeProfit > 0 {
		payload["stop"] = "up"
		payload["stopPriceType"] = "MP"
		payload["stopPrice"] = strconv.FormatFloat(p.TakeProfit, 'f', -1, 64)
	}

	var out struct {
		Code string `json:"code"`
		Data struct {
			OrderID string `json:"orderId"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v1/orders", payload, &out); err != nil {
		return nil, err
	}
	if out.Code != "200000" {
		return nil, fmt.Errorf("%w: kucoin code %s", ErrConn, out.Code)
	}
	return &OrderResult{ExchangeOrderID: out.Data.OrderID, Status: "NEW"}, nil
}

// ========== 条件单 ==========

// SetStopLoss 止损。
func (a *KuCoinAdapter) SetStopLoss(ctx context.Context, symbol, side string, quantity, stopPrice float64, reduceOnly bool) (*OrderResult, error) {
	payload := map[string]any{
		"symbol":      symbol,
		"type":        "market",
		"side":        side,
		"size":        quantity,
		"stop":        "down",
		"stopPriceType": "MP",
		"stopPrice":   strconv.FormatFloat(stopPrice, 'f', -1, 64),
	}
	if reduceOnly {
		payload["closeOrder"] = true
	}
	var out struct {
		Code string `json:"code"`
		Data struct {
			OrderID string `json:"orderId"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v1/orders", payload, &out); err != nil {
		return nil, err
	}
	if out.Code != "200000" {
		return nil, fmt.Errorf("%w: kucoin stop code %s", ErrConn, out.Code)
	}
	return &OrderResult{ExchangeOrderID: out.Data.OrderID}, nil
}

// SetTakeProfit 止盈。
func (a *KuCoinAdapter) SetTakeProfit(ctx context.Context, symbol, side string, quantity, takePrice float64, reduceOnly bool) (*OrderResult, error) {
	payload := map[string]any{
		"symbol":      symbol,
		"type":        "market",
		"side":        side,
		"size":        quantity,
		"stop":        "up",
		"stopPriceType": "MP",
		"stopPrice":   strconv.FormatFloat(takePrice, 'f', -1, 64),
	}
	if reduceOnly {
		payload["closeOrder"] = true
	}
	var out struct {
		Code string `json:"code"`
		Data struct {
			OrderID string `json:"orderId"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v1/orders", payload, &out); err != nil {
		return nil, err
	}
	if out.Code != "200000" {
		return nil, fmt.Errorf("%w: kucoin tp code %s", ErrConn, out.Code)
	}
	return &OrderResult{ExchangeOrderID: out.Data.OrderID}, nil
}

// ========== 查询 ==========

// GetBalance 可用余额（USDT）。
func (a *KuCoinAdapter) GetBalance(ctx context.Context) (float64, error) {
	var out struct {
		Code string `json:"code"`
		Data struct {
			AccountEquity string `json:"accountEquity"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v1/account-overview?currency=USDT", nil, &out); err != nil {
		return 0, err
	}
	if out.Code != "200000" {
		return 0, fmt.Errorf("%w: kucoin balance code %s", ErrConn, out.Code)
	}
	return parseF64(out.Data.AccountEquity), nil
}

// GetPositions 持仓。
func (a *KuCoinAdapter) GetPositions(ctx context.Context) ([]Position, error) {
	var out struct {
		Code string `json:"code"`
		Data []struct {
			Symbol     string `json:"symbol"`
			CurrentQty string `json:"currentQty"`
			AvgEntryPrice string `json:"avgEntryPrice"`
			MarkPrice  string `json:"markPrice"`
			UnrealisedPnl string `json:"unrealisedPnl"`
			Leverage   string `json:"leverage"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v1/positions", nil, &out); err != nil {
		return nil, err
	}
	if out.Code != "200000" {
		return nil, fmt.Errorf("%w: kucoin positions code %s", ErrConn, out.Code)
	}
	positions := make([]Position, 0, len(out.Data))
	for _, p := range out.Data {
		amt := parseF64(p.CurrentQty)
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
			EntryPrice:    parseF64(p.AvgEntryPrice),
			MarkPrice:     parseF64(p.MarkPrice),
			UnrealizedPnL: parseF64(p.UnrealisedPnl),
			Leverage:      int(parseF64(p.Leverage)),
		})
	}
	return positions, nil
}

// GetOpenOrders 挂单。
func (a *KuCoinAdapter) GetOpenOrders(ctx context.Context) ([]Order, error) {
	var out struct {
		Code string `json:"code"`
		Data struct {
			Items []struct {
				ID       string `json:"id"`
				Symbol   string `json:"symbol"`
				Side     string `json:"side"`
				Size     string `json:"size"`
				Price    string `json:"price"`
				Status   string `json:"status"`
				ClientOID string `json:"clientOid"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v1/orders?status=active", nil, &out); err != nil {
		return nil, err
	}
	if out.Code != "200000" {
		return nil, fmt.Errorf("%w: kucoin orders code %s", ErrConn, out.Code)
	}
	orders := make([]Order, 0, len(out.Data.Items))
	for _, o := range out.Data.Items {
		orders = append(orders, Order{
			ExchangeOrderID: o.ID,
			Symbol:          o.Symbol,
			Side:            o.Side,
			Quantity:        parseF64(o.Size),
			Price:           parseF64(o.Price),
			Status:          o.Status,
			ClientOrderID:   o.ClientOID,
		})
	}
	return orders, nil
}

// GetFills 成交。
func (a *KuCoinAdapter) GetFills(ctx context.Context, sinceMS int64) ([]Fill, error) {
	q := "/api/v1/fills"
	if sinceMS > 0 {
		q += "?startAt=" + strconv.FormatInt(sinceMS/1000, 10)
	}
	var out struct {
		Code string `json:"code"`
		Data struct {
			Items []struct {
				TradeID string `json:"tradeId"`
				OrderID string `json:"orderId"`
				Symbol  string `json:"symbol"`
				Side    string `json:"side"`
				Price   string `json:"price"`
				Size    string `json:"size"`
				Fee     string `json:"fee"`
				FeeCurrency string `json:"feeCurrency"`
				CreatedAt int64  `json:"createdAt"`
			} `json:"items"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, q, nil, &out); err != nil {
		return nil, err
	}
	if out.Code != "200000" {
		return nil, fmt.Errorf("%w: kucoin fills code %s", ErrConn, out.Code)
	}
	fills := make([]Fill, 0, len(out.Data.Items))
	for _, f := range out.Data.Items {
		fills = append(fills, Fill{
			ExchangeTradeID: f.TradeID,
			ExchangeOrderID: f.OrderID,
			Symbol:          f.Symbol,
			Side:            f.Side,
			Price:           parseF64(f.Price),
			Quantity:        parseF64(f.Size),
			Commission:      parseF64(f.Fee),
			CommissionAsset: f.FeeCurrency,
			TimestampMS:     f.CreatedAt,
		})
	}
	return fills, nil
}

// ========== 工具 ==========

// FormatQuantity round 到 step。
func (a *KuCoinAdapter) FormatQuantity(ctx context.Context, symbol string, qty float64) (float64, error) {
	step, err := a.qtyStep(ctx, symbol)
	if err != nil {
		return 0, err
	}
	return FormatKuCoin(qty, step), nil
}

// GetLeverage 当前杠杆。
func (a *KuCoinAdapter) GetLeverage(ctx context.Context, symbol string) (int, error) {
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
func (a *KuCoinAdapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	payload := map[string]any{
		"symbol":   symbol,
		"leverage": strconv.Itoa(leverage),
	}
	var out struct {
		Code string `json:"code"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v1/position/leverage", payload, &out); err != nil {
		return err
	}
	if out.Code != "200000" {
		return fmt.Errorf("%w: kucoin leverage code %s", ErrConn, out.Code)
	}
	return nil
}

// qtyStep 合约 step 缓存（contracts 列表）。
func (a *KuCoinAdapter) qtyStep(ctx context.Context, symbol string) (float64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s, ok := a.step[symbol]; ok {
		return s, nil
	}
	var out struct {
		Code string `json:"code"`
		Data []struct {
			Symbol      string `json:"symbol"`
			BaseMinSize string `json:"baseMinSize"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v1/contracts/active", nil, &out); err != nil {
		return 0, err
	}
	if out.Code != "200000" {
		return 0, fmt.Errorf("%w: kucoin contracts code %s", ErrConn, out.Code)
	}
	for _, s := range out.Data {
		if s.Symbol == symbol {
			step := parseF64(s.BaseMinSize)
			if step > 0 {
				a.step[symbol] = step
				return step, nil
			}
		}
	}
	return 0, fmt.Errorf("%w: step not found for %s", ErrParams, symbol)
}

// ========== 签名与请求 ==========

// sign 签名：Base64(HMAC-SHA256(timestamp+method+path+body))。
func (a *KuCoinAdapter) sign(ts, method, path, body string) string {
	mac := hmac.New(sha256.New, []byte(a.creds.SecretKey))
	_, _ = mac.Write([]byte(ts + method + path + body))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// request 统一请求。
func (a *KuCoinAdapter) request(ctx context.Context, method, path string, payload any, out any) error {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrParams, err)
		}
	}
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	// 签名串的 requestPath 必须含 query string（KuCoin 官方规范），
	// 砍掉 query 会让带参 GET（orders/fills 等）报签名错误。
	sig := a.sign(ts, method, path, string(body))

	req, err := http.NewRequestWithContext(ctx, method, a.base+path, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConn, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("KC-API-KEY", a.creds.APIKey)
	req.Header.Set("KC-API-SIGN", sig)
	req.Header.Set("KC-API-TIMESTAMP", ts)
	req.Header.Set("KC-API-PASSPHRASE", a.creds.Passphrase)

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
