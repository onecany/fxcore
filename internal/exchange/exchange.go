// Package exchange 交易所适配器层（API设计.md §14.2）。
// 统一接口（约 18 个方法）+ 工厂注册表。参考实现：binance（完整可交易）；
// 其余 9 家 FormatQuantity 纯函数按契约实现，交易方法返回 ErrNotImplemented。
// 全部适配器必须走统一信封错误：连接失败归 ErrConn，风控/参数归 ErrParams。
package exchange

import (
	"context"
	"errors"
)

// 错误分类（handler 映射 1401/1001）。
var (
	ErrNotImplemented = errors.New("exchange: adapter not implemented")
	ErrConn           = errors.New("exchange: connection or credential error")
	ErrParams         = errors.New("exchange: invalid order parameters")
	ErrInsufficient   = errors.New("exchange: insufficient balance")
)

// Credentials 适配器凭据（已由 service 层解密）。
type Credentials struct {
	ExchangeType string
	APIKey       string
	SecretKey    string
	Passphrase   string
	// DEX 字段
	WalletAddr string
	PrivateKey string
	// lighter
	APIKeyPrivateKey string
	APIKeyIndex      int
	// okx 保证金模式（cross | isolated；§14.2 tdMode 取配置不硬编码）
	TDMode string
	// 网络
	BaseURL string // 空用默认（测试/代理场景可注入）
	Testnet bool
}

// OrderParams 下单参数。
type OrderParams struct {
	Symbol        string
	Side          string  // buy | sell
	PositionSide  string  // long | short
	Quantity      float64
	Price         float64 // 0 = 市价
	StopLoss      float64 // 可选
	TakeProfit    float64 // 可选
	Leverage      int
	ReduceOnly    bool
	ClientOrderID string // 幂等键（traderID+cycle+symbol+action）
}

// OrderResult 下单结果。
type OrderResult struct {
	ExchangeOrderID string
	ClientOrderID   string
	Status          string // NEW | FILLED | PARTIALLY_FILLED
	FilledQuantity  float64
	AvgFillPrice    float64
	Fee             float64
	FeeAsset        string
}

// Position 适配器层持仓视图。
type Position struct {
	Symbol        string
	Side          string // long | short
	Quantity      float64
	EntryPrice    float64
	MarkPrice     float64
	UnrealizedPnL float64
	Leverage      int
}

// Order 适配器层挂单视图。
type Order struct {
	ExchangeOrderID string
	Symbol          string
	Side            string
	Quantity        float64
	Price           float64
	Status          string
	ClientOrderID   string
}

// Adapter 统一交易所接口（§14.2 约 18 个方法）。
type Adapter interface {
	Name() string
	// 开平仓
	OpenLong(ctx context.Context, p OrderParams) (*OrderResult, error)
	OpenShort(ctx context.Context, p OrderParams) (*OrderResult, error)
	CloseLong(ctx context.Context, p OrderParams) (*OrderResult, error)
	CloseShort(ctx context.Context, p OrderParams) (*OrderResult, error)
	// 条件单
	SetStopLoss(ctx context.Context, symbol, side string, quantity, stopPrice float64, reduceOnly bool) (*OrderResult, error)
	SetTakeProfit(ctx context.Context, symbol, side string, quantity, takePrice float64, reduceOnly bool) (*OrderResult, error)
	// 查询
	GetBalance(ctx context.Context) (float64, error)
	GetPositions(ctx context.Context) ([]Position, error)
	GetOpenOrders(ctx context.Context) ([]Order, error)
	GetFills(ctx context.Context, sinceMS int64) ([]Fill, error) // OrderSync 用（24h 成交）
	// 工具
	FormatQuantity(ctx context.Context, symbol string, qty float64) (float64, error)
	GetLeverage(ctx context.Context, symbol string) (int, error)
	SetLeverage(ctx context.Context, symbol string, leverage int) error
}

// Fill 适配器层成交视图（OrderSync 解析用）。
type Fill struct {
	ExchangeTradeID string
	ExchangeOrderID string
	Symbol          string
	Side            string
	Price           float64
	Quantity        float64
	Commission      float64
	CommissionAsset string
	RealizedPnL     float64
	IsMaker         bool
	TimestampMS     int64
}

// New 通过注册表构造适配器（凭据已解密）。
func New(creds Credentials) (Adapter, error) {
	return newFromRegistry(creds)
}
