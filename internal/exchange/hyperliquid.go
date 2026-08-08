package exchange

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/sha3"
)

// HyperliquidAdapter hyperliquid DEX 永续适配器（§14.2）。
// 查询侧（/info，无签名）：clearinghouseState / userFills / openOrders / meta。
// 下单侧（/exchange）：L1 签名（主钱包模式 ed25519）。
//   actionStr = JSON.stringify(action)（无空格）
//   actionHash = keccak256(actionStr) 前 16 字节
//   message    = 0x00 + actionHash + nonce(8B big-endian) + vault(32B，无 vault 全 0)
//   signature  = ed25519.sign(privateKey, message)
// 注意：keccak256（LegacyKeccak256）≠ SHA3-256；签名规范以实盘验证为准。
type HyperliquidAdapter struct {
	client *http.Client
	base   string // 默认 https://api.hyperliquid.xyz
	creds  Credentials

	mu        sync.Mutex
	metaCache []string // universe 资产索引缓存（symbol → index）
	metaAt    time.Time
}

// newHyperliquid 构造。
func newHyperliquid(creds Credentials) (Adapter, error) {
	if creds.WalletAddr == "" {
		return nil, fmt.Errorf("%w: hyperliquid requires wallet address", ErrParams)
	}
	base := creds.BaseURL
	if base == "" {
		base = "https://api.hyperliquid.xyz"
	}
	return &HyperliquidAdapter{
		client: &http.Client{Timeout: 15 * time.Second},
		base:   strings.TrimRight(base, "/"),
		creds:  creds,
	}, nil
}

// Name 适配器名。
func (a *HyperliquidAdapter) Name() string { return "hyperliquid" }

// init 自注册。
func init() { Register("hyperliquid", newHyperliquid) }

// ========== 查询（/info 无签名） ==========

// GetBalance 账户权益（crossMarginSummary.accountValue）。
func (a *HyperliquidAdapter) GetBalance(ctx context.Context) (float64, error) {
	var out struct {
		CrossMarginSummary struct {
			AccountValue string `json:"accountValue"`
		} `json:"crossMarginSummary"`
	}
	if err := a.info(ctx, map[string]any{
		"type": "clearinghouseState",
		"user": a.creds.WalletAddr,
	}, &out); err != nil {
		return 0, err
	}
	return parseF64(out.CrossMarginSummary.AccountValue), nil
}

// GetPositions 持仓（assetPositions）。
func (a *HyperliquidAdapter) GetPositions(ctx context.Context) ([]Position, error) {
	var out struct {
		AssetPositions []struct {
			Position struct {
				Coin          string `json:"coin"`
				Szi           string `json:"szi"`
				EntryPx       string `json:"entryPx"`
				PositionValue string `json:"positionValue"`
				UnrealizedPnl string `json:"unrealizedPnl"`
				Leverage      struct {
					Value int `json:"value"`
				} `json:"leverage"`
			} `json:"position"`
		} `json:"assetPositions"`
	}
	if err := a.info(ctx, map[string]any{
		"type": "clearinghouseState",
		"user": a.creds.WalletAddr,
	}, &out); err != nil {
		return nil, err
	}
	positions := make([]Position, 0, len(out.AssetPositions))
	for _, ap := range out.AssetPositions {
		szi := parseF64(ap.Position.Szi)
		if szi == 0 {
			continue
		}
		side := "long"
		if szi < 0 {
			side = "short"
		}
		positions = append(positions, Position{
			Symbol:        ap.Position.Coin + "-USDT", // 归一化
			Side:          side,
			Quantity:      absF(szi),
			EntryPrice:    parseF64(ap.Position.EntryPx),
			MarkPrice:     parseF64(ap.Position.PositionValue) / absF(szi),
			UnrealizedPnL: parseF64(ap.Position.UnrealizedPnl),
			Leverage:      ap.Position.Leverage.Value,
		})
	}
	return positions, nil
}

// GetOpenOrders 挂单。
func (a *HyperliquidAdapter) GetOpenOrders(ctx context.Context) ([]Order, error) {
	var out []struct {
		Coin     string `json:"coin"`
		Side     string `json:"side"`
		LimitPx  string `json:"limitPx"`
		Sz       string `json:"sz"`
		OID      int64  `json:"oid"`
		ReduceOnly bool `json:"reduceOnly"`
	}
	if err := a.info(ctx, map[string]any{
		"type": "openOrders",
		"user": a.creds.WalletAddr,
	}, &out); err != nil {
		return nil, err
	}
	orders := make([]Order, 0, len(out))
	for _, o := range out {
		side := "buy"
		if o.Side == "A" {
			side = "sell"
		}
		orders = append(orders, Order{
			ExchangeOrderID: strconv.FormatInt(o.OID, 10),
			Symbol:          o.Coin + "-USDT",
			Side:            side,
			Quantity:        parseF64(o.Sz),
			Price:           parseF64(o.LimitPx),
			Status:          "NEW",
		})
	}
	return orders, nil
}

// GetFills 成交（userFills）。
func (a *HyperliquidAdapter) GetFills(ctx context.Context, sinceMS int64) ([]Fill, error) {
	payload := map[string]any{
		"type": "userFills",
		"user": a.creds.WalletAddr,
	}
	if sinceMS > 0 {
		payload["startTime"] = sinceMS
	}
	var out []struct {
		Coin     string `json:"coin"`
		Px       string `json:"px"`
		Sz       string `json:"sz"`
		Side     string `json:"side"`
		Time     int64  `json:"time"`
		Dir      string `json:"dir"`
		ClosedPnl string `json:"closedPnl"`
		Hash     string `json:"hash"`
		OID      int64  `json:"oid"`
		TID      int64  `json:"tid"`
		Fee      string `json:"fee"`
		FeeToken string `json:"feeToken"`
	}
	if err := a.info(ctx, payload, &out); err != nil {
		return nil, err
	}
	fills := make([]Fill, 0, len(out))
	for _, f := range out {
		side := "buy"
		if f.Side == "A" {
			side = "sell"
		}
		fills = append(fills, Fill{
			ExchangeTradeID: strconv.FormatInt(f.TID, 10),
			ExchangeOrderID: strconv.FormatInt(f.OID, 10),
			Symbol:          f.Coin + "-USDT",
			Side:            side,
			Price:           parseF64(f.Px),
			Quantity:        parseF64(f.Sz),
			Commission:      parseF64(f.Fee),
			CommissionAsset: f.FeeToken,
			RealizedPnL:     parseF64(f.ClosedPnl),
			TimestampMS:     f.Time,
		})
	}
	return fills, nil
}

// ========== 工具 ==========

// FormatQuantity 5 位有效数字（§14.2）。
func (a *HyperliquidAdapter) FormatQuantity(ctx context.Context, symbol string, qty float64) (float64, error) {
	return FormatHyperliquid(qty), nil
}

// GetLeverage 当前杠杆（从持仓）。
func (a *HyperliquidAdapter) GetLeverage(ctx context.Context, symbol string) (int, error) {
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

// SetLeverage hyperliquid 杠杆随下单 leverage 字段（isolated 模式），无独立接口。
func (a *HyperliquidAdapter) SetLeverage(ctx context.Context, symbol string, leverage int) error {
	return nil // 下单时按 leverage 参数设置
}

// ========== 下单（/exchange，L1 签名） ==========

// OpenLong 开多。
func (a *HyperliquidAdapter) OpenLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, true, false)
}

// OpenShort 开空。
func (a *HyperliquidAdapter) OpenShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, false, false)
}

// CloseLong 平多。
func (a *HyperliquidAdapter) CloseLong(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, false, true)
}

// CloseShort 平空。
func (a *HyperliquidAdapter) CloseShort(ctx context.Context, p OrderParams) (*OrderResult, error) {
	return a.placeOrder(ctx, p, true, true)
}

// placeOrder 下单（L1 签名）。市价（Price=0）按 ±1% IOC 限价（§14.2 hyperliquid 契约）。
func (a *HyperliquidAdapter) placeOrder(ctx context.Context, p OrderParams, buy, reduceOnly bool) (*OrderResult, error) {
	if a.creds.PrivateKey == "" {
		return nil, fmt.Errorf("%w: hyperliquid requires private key for trading", ErrParams)
	}
	qty, err := a.FormatQuantity(ctx, p.Symbol, p.Quantity)
	if err != nil {
		return nil, err
	}
	if qty <= 0 {
		return nil, fmt.Errorf("%w: quantity too small: %f", ErrParams, p.Quantity)
	}
	// 资产索引
	idx, err := a.assetIndex(ctx, p.Symbol)
	if err != nil {
		return nil, err
	}
	// 价格：市价 → 需 mark price（metaAndAssetCtxs）
	price := p.Price
	if price <= 0 {
		mark, err := a.markPrice(ctx, p.Symbol)
		if err != nil {
			return nil, err
		}
		if buy {
			price = mark * 1.01 // 市价买单 +1%（IOC 吃单）
		} else {
			price = mark * 0.99 // 市价卖单 -1%
		}
	}

	action := map[string]any{
		"type": "order",
		"orders": []any{
			map[string]any{
				"a": idx,
				"b": buy,
				"p": strconv.FormatFloat(price, 'f', -1, 64),
				"s": strconv.FormatFloat(qty, 'f', -1, 64),
				"r": reduceOnly,
				"t": map[string]any{"limit": map[string]any{"tif": "Gtc"}},
			},
		},
	}
	nonce := time.Now().UnixMilli()
	sig, err := a.l1Sign(action, nonce)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrParams, err)
	}

	var out struct {
		Status  string `json:"status"`
		Response struct {
			Type string `json:"type"`
			Data struct {
				Statuses []struct {
					Resting struct {
						OID int64 `json:"oid"`
					} `json:"resting"`
				} `json:"statuses"`
			} `json:"data"`
		} `json:"response"`
	}
	if err := a.exchange(ctx, map[string]any{
		"action":        action,
		"nonce":         nonce,
		"signature":     map[string]string{"r": sig.R, "s": sig.S},
		"vaultAddress":  nil,
	}, &out); err != nil {
		return nil, err
	}
	if out.Status != "ok" || len(out.Response.Data.Statuses) == 0 {
		return nil, fmt.Errorf("%w: hyperliquid order failed: %s", ErrConn, out.Status)
	}
	return &OrderResult{
		ExchangeOrderID: strconv.FormatInt(out.Response.Data.Statuses[0].Resting.OID, 10),
		Status:          "NEW",
	}, nil
}

// SetStopLoss 止损（trigger order 骨架：签名结构同 placeOrder，未实盘验证）。
func (a *HyperliquidAdapter) SetStopLoss(ctx context.Context, symbol, side string, quantity, stopPrice float64, reduceOnly bool) (*OrderResult, error) {
	return nil, fmt.Errorf("%w: hyperliquid trigger orders pending verification", ErrNotImplemented)
}

// SetTakeProfit 止盈。
func (a *HyperliquidAdapter) SetTakeProfit(ctx context.Context, symbol, side string, quantity, takePrice float64, reduceOnly bool) (*OrderResult, error) {
	return nil, fmt.Errorf("%w: hyperliquid trigger orders pending verification", ErrNotImplemented)
}

// ========== L1 签名 ==========

// l1Sig L1 签名结果（r/s 十六进制）。
type l1Sig struct{ R, S string }

// l1Sign 主钱包模式签名：
//   actionHash = keccak256(actionStr) 前 16 字节
//   message    = 0x00 + actionHash + nonce(8B BE) + vault(32B 全 0)
func (a *HyperliquidAdapter) l1Sign(action map[string]any, nonce int64) (*l1Sig, error) {
	actionStr, err := json.Marshal(action) // 无空格序列化
	if err != nil {
		return nil, err
	}
	actionHash := keccak16(actionStr)

	msg := make([]byte, 1+16+8+32)
	msg[0] = 0x00
	copy(msg[1:17], actionHash)
	binary.BigEndian.PutUint64(msg[17:25], uint64(nonce))
	// vault 全 0（无 vault）

	seed, err := hex.DecodeString(strings.TrimPrefix(a.creds.PrivateKey, "0x"))
	if err != nil || len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("invalid ed25519 private key (need 32-byte hex seed)")
	}
	priv := ed25519.NewKeyFromSeed(seed)
	sig := ed25519.Sign(priv, msg)
	return &l1Sig{
		R: hex.EncodeToString(sig[:32]),
		S: hex.EncodeToString(sig[32:]),
	}, nil
}

// keccak16 keccak256 前 16 字节（LegacyKeccak256 ≠ SHA3-256）。
func keccak16(data []byte) []byte {
	h := sha3.NewLegacyKeccak256()
	_, _ = h.Write(data)
	return h.Sum(nil)[:16]
}

// ========== 内部 ==========

// info POST /info（无签名查询）。
func (a *HyperliquidAdapter) info(ctx context.Context, payload map[string]any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrParams, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base+"/info", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConn, err)
	}
	req.Header.Set("Content-Type", "application/json")
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

// exchange POST /exchange（签名下单）。
func (a *HyperliquidAdapter) exchange(ctx context.Context, payload map[string]any, out any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrParams, err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.base+"/exchange", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%w: %v", ErrConn, err)
	}
	req.Header.Set("Content-Type", "application/json")
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

// assetIndex 资产索引（meta.universe 顺序缓存，10 分钟）。
func (a *HyperliquidAdapter) assetIndex(ctx context.Context, symbol string) (int, error) {
	coin := strings.Split(symbol, "-")[0]
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.metaCache) > 0 && time.Since(a.metaAt) < 10*time.Minute {
		for i, name := range a.metaCache {
			if name == coin {
				return i, nil
			}
		}
	}
	var out struct {
		Universe []struct {
			Name string `json:"name"`
		} `json:"universe"`
	}
	if err := a.info(ctx, map[string]any{"type": "meta"}, &out); err != nil {
		return 0, err
	}
	names := make([]string, 0, len(out.Universe))
	for _, u := range out.Universe {
		names = append(names, u.Name)
	}
	a.metaCache = names
	a.metaAt = time.Now()
	for i, name := range names {
		if name == coin {
			return i, nil
		}
	}
	return 0, fmt.Errorf("%w: coin %s not in hyperliquid universe", ErrParams, coin)
}

// markPrice 最新价（metaAndAssetCtxs）。
func (a *HyperliquidAdapter) markPrice(ctx context.Context, symbol string) (float64, error) {
	coin := strings.Split(symbol, "-")[0]
	var out []struct {
		MidPx string `json:"midPx"`
	}
	if err := a.info(ctx, map[string]any{"type": "metaAndAssetCtxs"}, &out); err != nil {
		return 0, err
	}
	idx, err := a.assetIndex(ctx, symbol)
	if err != nil {
		return 0, err
	}
	if idx >= len(out) {
		return 0, fmt.Errorf("%w: no price ctx for %s", ErrConn, symbol)
	}
	mid := parseF64(out[idx].MidPx)
	if mid <= 0 {
		return 0, fmt.Errorf("%w: empty mark price for %s", ErrConn, coin)
	}
	return mid, nil
}
