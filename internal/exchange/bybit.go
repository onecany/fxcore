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

// BybitAdapter bybit USDT 永续适配器（§14.2）。
// v5 API：签名 HMAC-SHA256(timestamp+recvWindow+body)，头 X-BAPI-*；
// 数量 floor(qty/qtyStep)（step 缓存）；SL/TP 走订单内置 stopLoss/takeProfit（conditional）。
type BybitAdapter struct {
	client *http.Client
	base   string
	creds  Credentials

	mu   sync.Mutex
	step map[string]float64
}

// newBybit 构造。
func newBybit(creds Credentials) (Adapter, error) {
	base := creds.BaseURL
	if base == "" {
		base = "https://api.bybit.com"
	}
	if creds.Testnet {
		base = "https://api-testnet.bybit.com"
	}
	return &BybitAdapter{
		client: &http.Client{Timeout: 15 * time.Second},
		base:   strings.TrimRight(base, "/"),
		creds:  creds,
		step:   make(map[string]float64),
	}, nil
}

// Name 适配器名。
func (a *BybitAdapter) Name() string { return "bybit" }

// init 自注册。
func init() { Register("bybit", newBybit) }

// ========== 开平仓 ==========

// OpenLong 开多。
func (a *BybitAdapter) OpenLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, "Buy", "Buy")
}

// OpenShort 开空。
func (a *BybitAdapter) OpenShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, "Sell", "Sell")
}

// CloseLong 平多。
func (a *BybitAdapter) CloseLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	p.ReduceOnly = true
	return a.placeOrder(ctx, p, "Sell", "Buy") // 平多 = 卖平（reduceOnly）
}

// CloseShort 平空。
func (a *BybitAdapter) CloseShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	p.ReduceOnly = true
	return a.placeOrder(ctx, p, "Buy", "Sell")
}

// placeOrder 统一下单（v5 /order/create）。
func (a *BybitAdapter) placeOrder(ctx context.Context, p OrderParams, side, posSide string) (*OrderResult, error) {
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
		"category":   "linear",
		"symbol":     p.Symbol,
		"side":       side,
		"positionIdx": posIdx(posSide),
		"orderType":  "Market",
		"qty":        strconv.FormatFloat(qty, 'f', -1, 64),
	}
	if p.ReduceOnly {
		payload["reduceOnly"] = true
	}
	if p.ClientOrderID != "" {
		payload["orderLinkId"] = p.ClientOrderID
	}
	if p.Price > 0 {
		payload["orderType"] = "Limit"
		payload["price"] = strconv.FormatFloat(p.Price, 'f', -1, 64)
		payload["timeInForce"] = "GTC"
	}
	// SL/TP（conditional 语义：stopLoss/takeProfit 参数）
	if p.StopLoss > 0 {
		payload["stopLoss"] = strconv.FormatFloat(p.StopLoss, 'f', -1, 64)
	}
	if p.TakeProfit > 0 {
		payload["takeProfit"] = strconv.FormatFloat(p.TakeProfit, 'f', -1, 64)
	}

	var out struct {
		RetCode int `json:"retCode"`
		Result  struct {
			OrderID string `json:"orderId"`
			Status  string `json:"orderStatus"`
		} `json:"result"`
		RetMsg string `json:"retMsg"`
	}
	if err := a.post(ctx, "/v5/order/create", payload, &out); err != nil {
		return nil, err
	}
	if out.RetCode != 0 {
		return nil, fmt.Errorf("%w: bybit retCode %d: %s", ErrConn, out.RetCode, out.RetMsg)
	}
	return &OrderResult{ExchangeOrderID: out.Result.OrderID, Status: out.Result.Status}, nil
}

// ========== 条件单 ==========

// SetStopLoss 止损（stopLoss 参数随条件单）。
func (a *BybitAdapter) SetStopLoss(ctx context.Context, symbol, side string, quantity, stopPrice float64, reduceOnly bool) (*OrderResult, error) {
	payload := map[string]any{
		"category":   "linear",
		"symbol":     symbol,
		"side":       side,
		"positionIdx": posIdxFromSide(side),
		"orderType":  "Market",
		"qty":        strconv.FormatFloat(quantity, 'f', -1, 64),
		"stopLoss":   strconv.FormatFloat(stopPrice, 'f', -1, 64),
	}
	if reduceOnly {
		payload["reduceOnly"] = true
	}
	var out struct {
		RetCode int `json:"retCode"`
		Result  struct {
			OrderID string `json:"orderId"`
		} `json:"result"`
	}
	if err := a.post(ctx, "/v5/order/create", payload, &out); err != nil {
		return nil, err
	}
	if out.RetCode != 0 {
		return nil, fmt.Errorf("%w: bybit retCode %d", ErrConn, out.RetCode)
	}
	return &OrderResult{ExchangeOrderID: out.Result.OrderID}, nil
}

// SetTakeProfit 止盈。
func (a *BybitAdapter) SetTakeProfit(ctx context.Context, symbol, side string, quantity, takePrice float64, reduceOnly bool) (*OrderResult, error) {
	payload := map[string]any{
		"category":   "linear",
		"symbol":     symbol,
		"side":       side,
		"positionIdx": posIdxFromSide(side),
		"orderType":  "Market",
		"qty":        strconv.FormatFloat(quantity, 'f', -1, 64),
		"takeProfit": strconv.FormatFloat(takePrice, 'f', -1, 64),
	}
	if reduceOnly {
		payload["reduceOnly"] = true
	}
	var out struct {
		RetCode int `json:"retCode"`
		Result  struct {
			OrderID string `json:"orderId"`
		} `json:"result"`
	}
	if err := a.post(ctx, "/v5/order/create", payload, &out); err != nil {
		return nil, err
	}
	if out.RetCode != 0 {
		return nil, fmt.Errorf("%w: bybit retCode %d", ErrConn, out.RetCode)
	}
	return &OrderResult{ExchangeOrderID: out.Result.OrderID}, nil
}

// ========== 查询 ==========

// GetBalance 可用余额（USDT）。
func (a *BybitAdapter) GetBalance(ctx context.Context) (float64, error) {
	var out struct {
		RetCode int `json:"retCode"`
		Result  struct {
			List []struct {
				TotalEquity string `json:"totalEquity"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := a.get(ctx, "/v5/account/wallet-balance?accountType=UNIFIED", &out); err != nil {
		return 0, err
	}
	if out.RetCode != 0 || len(out.Result.List) == 0 {
		return 0, fmt.Errorf("%w: bybit balance retCode %d", ErrConn, out.RetCode)
	}
	return parseF64(out.Result.List[0].TotalEquity), nil
}

// GetPositions 持仓。
func (a *BybitAdapter) GetPositions(ctx context.Context) ([]Position, error) {
	var out struct {
		RetCode int `json:"retCode"`
		Result  struct {
			List []struct {
				Symbol        string `json:"symbol"`
				Side          string `json:"side"`
				Size          string `json:"size"`
				AvgPrice      string `json:"avgPrice"`
				MarkPrice     string `json:"markPrice"`
				UnrealisedPnl string `json:"unrealisedPnl"`
				Leverage      string `json:"leverage"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := a.get(ctx, "/v5/position/list?category=linear", &out); err != nil {
		return nil, err
	}
	if out.RetCode != 0 {
		return nil, fmt.Errorf("%w: bybit positions retCode %d", ErrConn, out.RetCode)
	}
	positions := make([]Position, 0, len(out.Result.List))
	for _, p := range out.Result.List {
		amt := parseF64(p.Size)
		if amt == 0 {
			continue
		}
		side := "long"
		if p.Side == "Sell" {
			side = "short"
		}
		positions = append(positions, Position{
			Symbol:        p.Symbol,
			Side:          side,
			Quantity:      amt,
			EntryPrice:    parseF64(p.AvgPrice),
			MarkPrice:     parseF64(p.MarkPrice),
			UnrealizedPnL: parseF64(p.UnrealisedPnl),
			Leverage:      int(parseF64(p.Leverage)),
		})
	}
	return positions, nil
}

// GetOpenOrders 挂单。
func (a *BybitAdapter) GetOpenOrders(ctx context.Context) ([]Order, error) {
	var out struct {
		RetCode int `json:"retCode"`
		Result  struct {
			List []struct {
				OrderID string `json:"orderId"`
				Symbol  string `json:"symbol"`
				Side    string `json:"side"`
				Qty     string `json:"qty"`
				Price   string `json:"price"`
				Status  string `json:"orderStatus"`
				LinkID  string `json:"orderLinkId"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := a.get(ctx, "/v5/order/realtime?category=linear", &out); err != nil {
		return nil, err
	}
	if out.RetCode != 0 {
		return nil, fmt.Errorf("%w: bybit open orders retCode %d", ErrConn, out.RetCode)
	}
	orders := make([]Order, 0, len(out.Result.List))
	for _, o := range out.Result.List {
		orders = append(orders, Order{
			ExchangeOrderID: o.OrderID,
			Symbol:          o.Symbol,
			Side:            strings.ToLower(o.Side),
			Quantity:        parseF64(o.Qty),
			Price:           parseF64(o.Price),
			Status:          o.Status,
			ClientOrderID:   o.LinkID,
		})
	}
	return orders, nil
}

// GetFills 成交（startTime ms 起）。
func (a *BybitAdapter) GetFills(ctx context.Context, sinceMS int64) ([]Fill, error) {
	q := "/v5/execution/list?category=linear"
	if sinceMS > 0 {
		q += "&startTime=" + strconv.FormatInt(sinceMS, 10)
	}
	var out struct {
		RetCode int `json:"retCode"`
		Result  struct {
			List []struct {
				ExecID      string `json:"execId"`
				OrderID     string `json:"orderId"`
				Symbol      string `json:"symbol"`
				Side        string `json:"side"`
				ExecPrice   string `json:"execPrice"`
				ExecQty     string `json:"execQty"`
				ExecFee     string `json:"execFee"`
				FeeCurrency string `json:"feeCurrency"`
				ExecTime    string `json:"execTime"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := a.get(ctx, q, &out); err != nil {
		return nil, err
	}
	if out.RetCode != 0 {
		return nil, fmt.Errorf("%w: bybit fills retCode %d", ErrConn, out.RetCode)
	}
	fills := make([]Fill, 0, len(out.Result.List))
	for _, f := range out.Result.List {
		ts, _ := strconv.ParseInt(f.ExecTime, 10, 64)
		fills = append(fills, Fill{
			ExchangeTradeID: f.ExecID,
			ExchangeOrderID: f.OrderID,
			Symbol:          f.Symbol,
			Side:            strings.ToLower(f.Side),
			Price:           parseF64(f.ExecPrice),
			Quantity:        parseF64(f.ExecQty),
			Commission:      parseF64(f.ExecFee),
			CommissionAsset: f.FeeCurrency,
			TimestampMS:     ts,
		})
	}
	return fills, nil
}

// ========== 工具 ==========

// FormatQuantity floor 到 qtyStep。
func (a *BybitAdapter) FormatQuantity(ctx context.Context, symbol string, qty float64) (float64, error) {
	step, err := a.qtyStep(ctx, symbol)
	if err != nil {
		return 0, err
	}
	return FormatBybit(qty, step), nil
}

// GetLeverage 当前杠杆。
func (a *BybitAdapter) GetLeverage(ctx context.Context, symbol string) (int, error) {
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
func (a *BybitAdapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	payload := map[string]any{
		"category":       "linear",
		"symbol":         symbol,
		"buyLeverage":    strconv.Itoa(leverage),
		"sellLeverage":   strconv.Itoa(leverage),
	}
	var out struct {
		RetCode int `json:"retCode"`
	}
	if err := a.post(ctx, "/v5/position/set-leverage", payload, &out); err != nil {
		return err
	}
	if out.RetCode != 0 {
		return fmt.Errorf("%w: bybit leverage retCode %d", ErrConn, out.RetCode)
	}
	return nil
}

// qtyStep 查交易对的 qtyStep 并缓存。
func (a *BybitAdapter) qtyStep(ctx context.Context, symbol string) (float64, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if s, ok := a.step[symbol]; ok {
		return s, nil
	}
	var out struct {
		RetCode int `json:"retCode"`
		Result  struct {
			List []struct {
				Symbol    string `json:"symbol"`
				LotSizeFilter struct {
					QtyStep string `json:"qtyStep"`
				} `json:"lotSizeFilter"`
			} `json:"list"`
		} `json:"result"`
	}
	if err := a.get(ctx, "/v5/market/instruments-info?category=linear", &out); err != nil {
		return 0, err
	}
	if out.RetCode != 0 {
		return 0, fmt.Errorf("%w: bybit instruments retCode %d", ErrConn, out.RetCode)
	}
	for _, s := range out.Result.List {
		if s.Symbol == symbol {
			step := parseF64(s.LotSizeFilter.QtyStep)
			if step > 0 {
				a.step[symbol] = step
				return step, nil
			}
		}
	}
	return 0, fmt.Errorf("%w: qtyStep not found for %s", ErrParams, symbol)
}

// ========== 签名与请求 ==========

// sign 签名：HMAC-SHA256(timestamp+recvWindow+body)。
func (a *BybitAdapter) sign(timestamp, recvWindow, body string) string {
	mac := hmac.New(sha256.New, []byte(a.creds.SecretKey))
	_, _ = mac.Write([]byte(timestamp + recvWindow + body))
	return hex.EncodeToString(mac.Sum(nil))
}

// post 带签名 POST（body JSON）。
func (a *BybitAdapter) post(ctx context.Context, path string, payload map[string]any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrParams, err)
	}
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	recv := "5000"
	sig := a.sign(ts, recv, string(body))

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base+path, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConn, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-BAPI-API-KEY", a.creds.APIKey)
	req.Header.Set("X-BAPI-TIMESTAMP", ts)
	req.Header.Set("X-BAPI-RECV-WINDOW", recv)
	req.Header.Set("X-BAPI-SIGN", sig)

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

// get 带签名 GET。
func (a *BybitAdapter) get(ctx context.Context, path string, out any) error {
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	recv := "5000"
	sig := a.sign(ts, recv, "")

	sep := "?"
	if strings.Contains(path, "?") {
		sep = "&"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.base+path+sep+"recvWindow="+recv, nil)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConn, err)
	}
	req.Header.Set("X-BAPI-API-KEY", a.creds.APIKey)
	req.Header.Set("X-BAPI-TIMESTAMP", ts)
	req.Header.Set("X-BAPI-RECV-WINDOW", recv)
	req.Header.Set("X-BAPI-SIGN", sig)

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

// posIdx bybit 单向持仓索引（0=one-way）。
func posIdx(side string) int {
	switch side {
	case "Buy":
		return 1
	case "Sell":
		return 2
	default:
		return 0
	}
}

func posIdxFromSide(side string) int {
	switch side {
	case "Buy":
		return 1
	case "Sell":
		return 2
	default:
		return 0
	}
}
