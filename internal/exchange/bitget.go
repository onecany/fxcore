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
	"strconv"
	"strings"
	"sync"
	"time"
)

// BitgetAdapter bitget v2 USDT 永续适配器（§14.2）。
// 签名：HMAC-SHA256(timestamp+method+path+body)，头 ACCESS-KEY/SIGN/TIMESTAMP/PASSPHRASE；
// 数量 floor 到 VolumePlace 小数位（instrument info 缓存）。
type BitgetAdapter struct {
	client *http.Client
	base   string
	creds  Credentials

	mu       sync.Mutex
	placeMap map[string]int // symbol → volumePlace
}

// newBitget 构造。
func newBitget(creds Credentials) (Adapter, error) {
	base := creds.BaseURL
	if base == "" {
		base = "https://api.bitget.com"
	}
	return &BitgetAdapter{
		client:   &http.Client{Timeout: 15 * time.Second},
		base:     strings.TrimRight(base, "/"),
		creds:    creds,
		placeMap: make(map[string]int),
	}, nil
}

// Name 适配器名。
func (a *BitgetAdapter) Name() string { return "bitget" }

// init 自注册。
func init() { Register("bitget", newBitget) }

// ========== 开平仓 ==========

// OpenLong 开多。
func (a *BitgetAdapter) OpenLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, "open_long")
}

// OpenShort 开空。
func (a *BitgetAdapter) OpenShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, "open_short")
}

// CloseLong 平多。
func (a *BitgetAdapter) CloseLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, "close_long")
}

// CloseShort 平空。
func (a *BitgetAdapter) CloseShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, "close_short")
}

// placeOrder 统一下单（/api/v2/mix/order/place-order）。
func (a *BitgetAdapter) placeOrder(ctx context.Context, p OrderParams, marginMode string) (*OrderResult, error) {
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

	side := "buy"
	if strings.Contains(marginMode, "short") {
		side = "sell"
	}
	payload := map[string]any{
		"symbol":      p.Symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed", // 简化：crossed（isolated 走配置后续）
		"marginCoin":  "USDT",
		"side":        side,
		"orderType":   "market",
		"size":        strconv.FormatFloat(qty, 'f', -1, 64),
	}
	if strings.Contains(marginMode, "close") {
		payload["reduceOnly"] = true
	}
	if p.ClientOrderID != "" {
		payload["clientOid"] = p.ClientOrderID
	}
	if p.Price > 0 {
		payload["orderType"] = "limit"
		payload["price"] = strconv.FormatFloat(p.Price, 'f', -1, 64)
	}

	var out struct {
		Code string `json:"code"`
		Data struct {
			OrderID string `json:"orderId"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v2/mix/order/place-order", payload, &out); err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("%w: bitget code %s: %s", ErrConn, out.Code, out.Msg)
	}
	return &OrderResult{ExchangeOrderID: out.Data.OrderID, Status: "NEW"}, nil
}

// ========== 条件单 ==========

// SetStopLoss 止损（计划委托）。
func (a *BitgetAdapter) SetStopLoss(ctx context.Context, symbol, side string, quantity, stopPrice float64, reduceOnly bool) (*OrderResult, error) {
	payload := map[string]any{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        side,
		"orderType":   "market",
		"size":        strconv.FormatFloat(quantity, 'f', -1, 64),
		"triggerPrice": strconv.FormatFloat(stopPrice, 'f', -1, 64),
		"triggerType": "le",
	}
	if reduceOnly {
		payload["reduceOnly"] = true
	}
	var out struct {
		Code string `json:"code"`
		Data struct {
			OrderID string `json:"orderId"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v2/mix/order/place-trigger-order", payload, &out); err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("%w: bitget trigger code %s", ErrConn, out.Code)
	}
	return &OrderResult{ExchangeOrderID: out.Data.OrderID}, nil
}

// SetTakeProfit 止盈。
func (a *BitgetAdapter) SetTakeProfit(ctx context.Context, symbol, side string, quantity, takePrice float64, reduceOnly bool) (*OrderResult, error) {
	payload := map[string]any{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginMode":  "crossed",
		"marginCoin":  "USDT",
		"side":        side,
		"orderType":   "market",
		"size":        strconv.FormatFloat(quantity, 'f', -1, 64),
		"triggerPrice": strconv.FormatFloat(takePrice, 'f', -1, 64),
		"triggerType": "ge",
	}
	if reduceOnly {
		payload["reduceOnly"] = true
	}
	var out struct {
		Code string `json:"code"`
		Data struct {
			OrderID string `json:"orderId"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v2/mix/order/place-trigger-order", payload, &out); err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("%w: bitget trigger code %s", ErrConn, out.Code)
	}
	return &OrderResult{ExchangeOrderID: out.Data.OrderID}, nil
}

// ========== 查询 ==========

// GetBalance 可用余额（USDT）。
func (a *BitgetAdapter) GetBalance(ctx context.Context) (float64, error) {
	var out struct {
		Code string `json:"code"`
		Data struct {
			Available string `json:"available"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v2/account/futures/account?productType=USDT-FUTURES", nil, &out); err != nil {
		return 0, err
	}
	if out.Code != "00000" {
		return 0, fmt.Errorf("%w: bitget balance code %s", ErrConn, out.Code)
	}
	return parseF64(out.Data.Available), nil
}

// GetPositions 持仓。
func (a *BitgetAdapter) GetPositions(ctx context.Context) ([]Position, error) {
	var out struct {
		Code string `json:"code"`
		Data []struct {
			Symbol   string `json:"symbol"`
			HoldSide string `json:"holdSide"`
			Total    string `json:"total"`
			AvgPrice string `json:"avgOpenPrice"`
			MarkPrice string `json:"markPrice"`
			Upl      string `json:"upl"`
			Leverage string `json:"leverage"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v2/mix/position/all-position?productType=USDT-FUTURES", nil, &out); err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("%w: bitget positions code %s", ErrConn, out.Code)
	}
	positions := make([]Position, 0, len(out.Data))
	for _, p := range out.Data {
		amt := parseF64(p.Total)
		if amt == 0 {
			continue
		}
		positions = append(positions, Position{
			Symbol:        p.Symbol,
			Side:          p.HoldSide,
			Quantity:      amt,
			EntryPrice:    parseF64(p.AvgPrice),
			MarkPrice:     parseF64(p.MarkPrice),
			UnrealizedPnL: parseF64(p.Upl),
			Leverage:      int(parseF64(p.Leverage)),
		})
	}
	return positions, nil
}

// GetOpenOrders 挂单。
func (a *BitgetAdapter) GetOpenOrders(ctx context.Context) ([]Order, error) {
	var out struct {
		Code string `json:"code"`
		Data struct {
			Enter []struct {
				OrderID   string `json:"orderId"`
				Symbol    string `json:"symbol"`
				Side      string `json:"side"`
				Size      string `json:"size"`
				Price     string `json:"price"`
				Status    string `json:"status"`
				ClientOID string `json:"clientOid"`
			} `json:"entrustedList"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v2/mix/order/unfilled?productType=USDT-FUTURES", nil, &out); err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("%w: bitget orders code %s", ErrConn, out.Code)
	}
	orders := make([]Order, 0, len(out.Data.Enter))
	for _, o := range out.Data.Enter {
		orders = append(orders, Order{
			ExchangeOrderID: o.OrderID,
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
func (a *BitgetAdapter) GetFills(ctx context.Context, sinceMS int64) ([]Fill, error) {
	q := "/api/v2/mix/order/fills?productType=USDT-FUTURES"
	if sinceMS > 0 {
		q += "&startTime=" + strconv.FormatInt(sinceMS, 10)
	}
	var out struct {
		Code string `json:"code"`
		Data struct {
			Fill []struct {
				TradeID string `json:"tradeId"`
				OrderID string `json:"orderId"`
				Symbol  string `json:"symbol"`
				Side    string `json:"side"`
				FillPx  string `json:"fillPrice"`
				FillSz  string `json:"fillQuantity"`
				Fee     string `json:"feeDetail"`
				Ts      string `json:"cTime"`
			} `json:"fillList"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, q, nil, &out); err != nil {
		return nil, err
	}
	if out.Code != "00000" {
		return nil, fmt.Errorf("%w: bitget fills code %s", ErrConn, out.Code)
	}
	fills := make([]Fill, 0, len(out.Data.Fill))
	for _, f := range out.Data.Fill {
		ts, _ := strconv.ParseInt(f.Ts, 10, 64)
		fills = append(fills, Fill{
			ExchangeTradeID: f.TradeID,
			ExchangeOrderID: f.OrderID,
			Symbol:          f.Symbol,
			Side:            f.Side,
			Price:           parseF64(f.FillPx),
			Quantity:        parseF64(f.FillSz),
			Commission:      parseF64(f.Fee),
			TimestampMS:     ts,
		})
	}
	return fills, nil
}

// ========== 工具 ==========

// FormatQuantity floor 到 VolumePlace。
func (a *BitgetAdapter) FormatQuantity(ctx context.Context, symbol string, qty float64) (float64, error) {
	place, err := a.volumePlace(ctx, symbol)
	if err != nil {
		return 0, err
	}
	return FormatBitget(qty, place), nil
}

// GetLeverage 当前杠杆。
func (a *BitgetAdapter) GetLeverage(ctx context.Context, symbol string) (int, error) {
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
func (a *BitgetAdapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	payload := map[string]any{
		"symbol":      symbol,
		"productType": "USDT-FUTURES",
		"marginCoin":  "USDT",
		"leverage":    strconv.Itoa(leverage),
	}
	var out struct {
		Code string `json:"code"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v2/mix/account/set-leverage", payload, &out); err != nil {
		return err
	}
	if out.Code != "00000" {
		return fmt.Errorf("%w: bitget leverage code %s", ErrConn, out.Code)
	}
	return nil
}

// volumePlace 查询合约的 VolumePlace（小数位）并缓存。
func (a *BitgetAdapter) volumePlace(ctx context.Context, symbol string) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if p, ok := a.placeMap[symbol]; ok {
		return p, nil
	}
	var out struct {
		Code string `json:"code"`
		Data []struct {
			Symbol      string `json:"symbol"`
			VolumePlace int    `json:"volumePlace"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v2/mix/market/contracts?productType=USDT-FUTURES", nil, &out); err != nil {
		return 0, err
	}
	if out.Code != "00000" {
		return 0, fmt.Errorf("%w: bitget contracts code %s", ErrConn, out.Code)
	}
	for _, s := range out.Data {
		if s.Symbol == symbol {
			a.placeMap[symbol] = s.VolumePlace
			return s.VolumePlace, nil
		}
	}
	return 0, fmt.Errorf("%w: volumePlace not found for %s", ErrParams, symbol)
}

// ========== 签名与请求 ==========

// sign 签名：HMAC-SHA256(timestamp+method+path+body)，hex。
func (a *BitgetAdapter) sign(ts, method, path, body string) string {
	mac := hmac.New(sha256.New, []byte(a.creds.SecretKey))
	_, _ = mac.Write([]byte(ts + method + path + body))
	return hex.EncodeToString(mac.Sum(nil))
}

// request 统一请求。
func (a *BitgetAdapter) request(ctx context.Context, method, path string, payload any, out any) error {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrParams, err)
		}
	}
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sig := a.sign(ts, method, path, string(body))

	req, err := http.NewRequestWithContext(ctx, method, a.base+path, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConn, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("ACCESS-KEY", a.creds.APIKey)
	req.Header.Set("ACCESS-SIGN", sig)
	req.Header.Set("ACCESS-TIMESTAMP", ts)
	req.Header.Set("ACCESS-PASSPHRASE", a.creds.Passphrase)

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
