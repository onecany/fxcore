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
	"time"
)

// OKXAdapter OKX v5 永续适配器（§14.2）。
// 签名：Base64(HMAC-SHA256(timestamp+method+path+body, secret))，头 OK-ACCESS-*；
// tdMode 取配置（cross/isolated，默认 cross）；SL/TP 走 algo 参数；
// 数量为合约张数（整数）。
type OKXAdapter struct {
	client *http.Client
	base   string
	creds  Credentials
	tdMode string
}

// newOKX 构造。
func newOKX(creds Credentials) (Adapter, error) {
	base := creds.BaseURL
	if base == "" {
		base = "https://www.okx.com"
	}
	if creds.Testnet {
		base = "https://www.okx.com" // OKX 模拟盘域名（demo）
	}
	tdMode := "cross"
	if creds.TDMode == "isolated" {
		tdMode = "isolated"
	}
	return &OKXAdapter{
		client: &http.Client{Timeout: 15 * time.Second},
		base:   strings.TrimRight(base, "/"),
		creds:  creds,
		tdMode: tdMode,
	}, nil
}

// Name 适配器名。
func (a *OKXAdapter) Name() string { return "okx" }

// init 自注册。
func init() { Register("okx", newOKX) }

// ========== 开平仓 ==========

// OpenLong 开多。
func (a *OKXAdapter) OpenLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, "buy", "long")
}

// OpenShort 开空。
func (a *OKXAdapter) OpenShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, "sell", "short")
}

// CloseLong 平多。
func (a *OKXAdapter) CloseLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, "sell", "long")
}

// CloseShort 平空。
func (a *OKXAdapter) CloseShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, "buy", "short")
}

// placeOrder 统一下单（/api/v5/trade/order）。
func (a *OKXAdapter) placeOrder(ctx context.Context, p OrderParams, side, posSide string) (*OrderResult, error) {
	sz, err := a.FormatQuantity(ctx, p.Symbol, p.Quantity)
	if err != nil {
		return nil, err
	}
	if sz <= 0 {
		return nil, fmt.Errorf("%w: quantity too small: %f", ErrParams, p.Quantity)
	}
	if p.Leverage > 0 {
		_ = a.SetLeverage(ctx, p.Symbol, p.Leverage)
	}

	payload := map[string]any{
		"instId":  p.Symbol,
		"tdMode":  a.tdMode,
		"side":    side,
		"posSide": posSide,
		"ordType": "market",
		"sz":      strconv.FormatFloat(sz, 'f', -1, 64),
	}
	if p.ClientOrderID != "" {
		payload["clOrdId"] = p.ClientOrderID
	}
	if p.Price > 0 {
		payload["ordType"] = "limit"
		payload["px"] = strconv.FormatFloat(p.Price, 'f', -1, 64)
	}
	// algo SL/TP（§14.2：okx algo，tdMode 取配置）
	if p.StopLoss > 0 || p.TakeProfit > 0 {
		algo := map[string]any{}
		if p.StopLoss > 0 {
			algo["slTriggerPx"] = strconv.FormatFloat(p.StopLoss, 'f', -1, 64)
			algo["slOrdPx"] = "-1"
		}
		if p.TakeProfit > 0 {
			algo["tpTriggerPx"] = strconv.FormatFloat(p.TakeProfit, 'f', -1, 64)
			algo["tpOrdPx"] = "-1"
		}
		payload["attachAlgoOrds"] = []any{algo}
	}

	var out struct {
		Code string `json:"code"`
		Data []struct {
			OrdID   string `json:"ordId"`
			ClOrdID string `json:"clOrdId"`
			State   string `json:"sCode"`
			Msg     string `json:"sMsg"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v5/trade/order", payload, &out); err != nil {
		return nil, err
	}
	if out.Code != "0" || len(out.Data) == 0 || out.Data[0].State != "0" {
		msg := ""
		if len(out.Data) > 0 {
			msg = out.Data[0].Msg
		}
		return nil, fmt.Errorf("%w: okx code %s: %s", ErrConn, out.Code, msg)
	}
	return &OrderResult{ExchangeOrderID: out.Data[0].OrdID, ClientOrderID: out.Data[0].ClOrdID, Status: "NEW"}, nil
}

// ========== 条件单 ==========

// SetStopLoss 止损（algo order）。
func (a *OKXAdapter) SetStopLoss(ctx context.Context, symbol, side string, quantity, stopPrice float64, reduceOnly bool) (*OrderResult, error) {
	return a.placeAlgo(ctx, symbol, side, quantity, stopPrice, 0, "conditional")
}

// SetTakeProfit 止盈。
func (a *OKXAdapter) SetTakeProfit(ctx context.Context, symbol, side string, quantity, takePrice float64, reduceOnly bool) (*OrderResult, error) {
	return a.placeAlgo(ctx, symbol, side, quantity, 0, takePrice, "conditional")
}

// placeAlgo 条件单（/api/v5/trade/order-algo）。
func (a *OKXAdapter) placeAlgo(ctx context.Context, symbol, side string, qty, sl, tp float64, kind string) (*OrderResult, error) {
	payload := map[string]any{
		"instId":  symbol,
		"tdMode":  a.tdMode,
		"side":    side,
		"ordType": "market",
		"sz":      strconv.FormatFloat(qty, 'f', -1, 64),
	}
	if sl > 0 {
		payload["slTriggerPx"] = strconv.FormatFloat(sl, 'f', -1, 64)
		payload["slOrdPx"] = "-1"
	}
	if tp > 0 {
		payload["tpTriggerPx"] = strconv.FormatFloat(tp, 'f', -1, 64)
		payload["tpOrdPx"] = "-1"
	}
	var out struct {
		Code string `json:"code"`
		Data []struct {
			AlgoID string `json:"algoId"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v5/trade/order-algo", payload, &out); err != nil {
		return nil, err
	}
	if out.Code != "0" || len(out.Data) == 0 {
		return nil, fmt.Errorf("%w: okx algo code %s", ErrConn, out.Code)
	}
	return &OrderResult{ExchangeOrderID: out.Data[0].AlgoID}, nil
}

// ========== 查询 ==========

// GetBalance 可用余额（USDT）。
func (a *OKXAdapter) GetBalance(ctx context.Context) (float64, error) {
	var out struct {
		Code string `json:"code"`
		Data []struct {
			Details []struct {
				Ccy  string `json:"ccy"`
				Avail string `json:"availEq"`
			} `json:"details"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v5/account/balance", nil, &out); err != nil {
		return 0, err
	}
	if out.Code != "0" || len(out.Data) == 0 {
		return 0, fmt.Errorf("%w: okx balance code %s", ErrConn, out.Code)
	}
	for _, d := range out.Data[0].Details {
		if d.Ccy == "USDT" {
			return parseF64(d.Avail), nil
		}
	}
	return 0, fmt.Errorf("%w: USDT not found", ErrConn)
}

// GetPositions 持仓。
func (a *OKXAdapter) GetPositions(ctx context.Context) ([]Position, error) {
	var out struct {
		Code string `json:"code"`
		Data []struct {
			InstID    string `json:"instId"`
			PosSide   string `json:"posSide"`
			Pos       string `json:"pos"`
			AvgPx     string `json:"avgPx"`
			MarkPx    string `json:"markPx"`
			Upl       string `json:"upl"`
			Lever     string `json:"lever"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v5/account/positions", nil, &out); err != nil {
		return nil, err
	}
	if out.Code != "0" {
		return nil, fmt.Errorf("%w: okx positions code %s", ErrConn, out.Code)
	}
	positions := make([]Position, 0, len(out.Data))
	for _, p := range out.Data {
		amt := parseF64(p.Pos)
		if amt == 0 {
			continue
		}
		side := "long"
		if p.PosSide == "short" {
			side = "short"
		}
		positions = append(positions, Position{
			Symbol:        p.InstID,
			Side:          side,
			Quantity:      amt,
			EntryPrice:    parseF64(p.AvgPx),
			MarkPrice:     parseF64(p.MarkPx),
			UnrealizedPnL: parseF64(p.Upl),
			Leverage:      int(parseF64(p.Lever)),
		})
	}
	return positions, nil
}

// GetOpenOrders 挂单。
func (a *OKXAdapter) GetOpenOrders(ctx context.Context) ([]Order, error) {
	var out struct {
		Code string `json:"code"`
		Data []struct {
			InstID string `json:"instId"`
			OrdID  string `json:"ordId"`
			Side   string `json:"side"`
			Sz     string `json:"sz"`
			Px     string `json:"px"`
			State  string `json:"state"`
			ClOrdID string `json:"clOrdId"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, "/api/v5/trade/orders-pending?instType=SWAP", nil, &out); err != nil {
		return nil, err
	}
	if out.Code != "0" {
		return nil, fmt.Errorf("%w: okx open orders code %s", ErrConn, out.Code)
	}
	orders := make([]Order, 0, len(out.Data))
	for _, o := range out.Data {
		orders = append(orders, Order{
			ExchangeOrderID: o.OrdID,
			Symbol:          o.InstID,
			Side:            o.Side,
			Quantity:        parseF64(o.Sz),
			Price:           parseF64(o.Px),
			Status:          o.State,
			ClientOrderID:   o.ClOrdID,
		})
	}
	return orders, nil
}

// GetFills 成交（sinceMS 起）。
func (a *OKXAdapter) GetFills(ctx context.Context, sinceMS int64) ([]Fill, error) {
	q := "/api/v5/trade/fills-history?instType=SWAP"
	if sinceMS > 0 {
		q += "&begin=" + strconv.FormatInt(sinceMS, 10)
	}
	var out struct {
		Code string `json:"code"`
		Data []struct {
			InstID string `json:"instId"`
			BillID string `json:"billId"`
			OrdID  string `json:"ordId"`
			Side   string `json:"side"`
			Px     string `json:"px"`
			Sz     string `json:"sz"`
			Fee    string `json:"fee"`
			FeeCcy string `json:"feeCcy"`
			PnL    string `json:"pnl"`
			ExecType string `json:"execType"` // M=maker（§14.2 IsMaker 判定）
			Ts     string `json:"ts"`
		} `json:"data"`
	}
	if err := a.request(ctx, http.MethodGet, q, nil, &out); err != nil {
		return nil, err
	}
	if out.Code != "0" {
		return nil, fmt.Errorf("%w: okx fills code %s", ErrConn, out.Code)
	}
	fills := make([]Fill, 0, len(out.Data))
	for _, f := range out.Data {
		ts, _ := strconv.ParseInt(f.Ts, 10, 64)
		fills = append(fills, Fill{
			ExchangeTradeID: f.BillID,
			ExchangeOrderID: f.OrdID,
			Symbol:          f.InstID,
			Side:            f.Side,
			Price:           parseF64(f.Px),
			Quantity:        parseF64(f.Sz),
			Commission:      parseF64(f.Fee),
			CommissionAsset: f.FeeCcy,
			RealizedPnL:     parseF64(f.PnL),
			IsMaker:         f.ExecType == "M", // §14.2 okx ExecType==M
			TimestampMS:     ts,
		})
	}
	return fills, nil
}

// ========== 工具 ==========

// FormatQuantity 合约张数（整数）。
func (a *OKXAdapter) FormatQuantity(ctx context.Context, symbol string, qty float64) (float64, error) {
	return FormatOKX(qty), nil
}

// GetLeverage 当前杠杆。
func (a *OKXAdapter) GetLeverage(ctx context.Context, symbol string) (int, error) {
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
func (a *OKXAdapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	payload := map[string]any{
		"instId":  symbol,
		"lever":   strconv.Itoa(leverage),
		"mgnMode": a.tdMode,
	}
	var out struct {
		Code string `json:"code"`
	}
	if err := a.request(ctx, http.MethodPost, "/api/v5/account/set-leverage", payload, &out); err != nil {
		return err
	}
	if out.Code != "0" {
		return fmt.Errorf("%w: okx leverage code %s", ErrConn, out.Code)
	}
	return nil
}

// ========== 签名与请求 ==========

// sign 签名：Base64(HMAC-SHA256(timestamp+method+path+body))。
func (a *OKXAdapter) sign(ts, method, path, body string) string {
	mac := hmac.New(sha256.New, []byte(a.creds.SecretKey))
	_, _ = mac.Write([]byte(ts + method + path + body))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// request 统一请求（GET 无 body，POST JSON body）。
func (a *OKXAdapter) request(ctx context.Context, method, path string, payload any, out any) error {
	var body []byte
	var err error
	if payload != nil {
		body, err = json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("%w: %v", ErrParams, err)
		}
	}
	ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
	pathOnly := path
	if i := strings.Index(path, "?"); i >= 0 {
		pathOnly = path[:i]
	}
	sig := a.sign(ts, method, pathOnly, string(body))

	req, err := http.NewRequestWithContext(ctx, method, a.base+path, strings.NewReader(string(body)))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConn, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("OK-ACCESS-KEY", a.creds.APIKey)
	req.Header.Set("OK-ACCESS-SIGN", sig)
	req.Header.Set("OK-ACCESS-TIMESTAMP", ts)
	req.Header.Set("OK-ACCESS-PASSPHRASE", a.creds.Passphrase)

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
