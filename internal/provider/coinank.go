package provider

import (
	"context"

	"fxcore/internal/api/v1/dto"
)

// CoinAnkProvider CoinAnk 兜底数据源（§14.3：免 key + 可配 key 两套客户端合并）。
// 当前为接口骨架：端点契约确认后实现，链式切换自动接管兜底。
type CoinAnkProvider struct{}

// NewCoinAnkProvider 构造兜底数据源。
func NewCoinAnkProvider() *CoinAnkProvider {
	return &CoinAnkProvider{}
}

// Name 数据源名。
func (p *CoinAnkProvider) Name() string { return "coinank" }

// Klines 骨架：返回 ErrNotImplemented，链式跳过。
func (p *CoinAnkProvider) Klines(ctx context.Context, symbol, interval string, limit int) ([]dto.KlineDTO, error) {
	return nil, ErrNotImplemented
}
