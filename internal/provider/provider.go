// Package provider 行情数据源链（API设计.md §14.3）。
// 统一 Kline 结构 + KlineProvider 接口 + 链式故障切换（hyperliquid → okx → coinank）。
// 全部端点硬编码公网地址，无用户可控 URL（无 SSRF 面）；复用 httpclient 拨号层防护。
package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"fxcore/internal/api/v1/dto"
)

// ErrNotImplemented 数据源未接入（CoinAnk 兜底骨架）。
var ErrNotImplemented = errors.New("provider: not implemented")

// KlineProvider 统一数据源接口。
type KlineProvider interface {
	// Klines 取 K 线（时间降序或升序均可，调用方不依赖顺序）。
	Klines(ctx context.Context, symbol, interval string, limit int) ([]dto.KlineDTO, error)
	// Name 数据源名（日志/排障用）。
	Name() string
}

// Chain 链式数据源：依次尝试，全部失败返回聚合错误。
type Chain struct {
	providers []KlineProvider
}

// NewChain 构造链。
func NewChain(providers ...KlineProvider) *Chain {
	return &Chain{providers: providers}
}

// Name 数据源名。
func (c *Chain) Name() string { return "chain" }

// Klines 依序尝试各数据源。
func (c *Chain) Klines(ctx context.Context, symbol, interval string, limit int) ([]dto.KlineDTO, error) {
	if len(c.providers) == 0 {
		return nil, errors.New("provider: empty chain")
	}
	var errs []string
	for _, p := range c.providers {
		klines, err := p.Klines(ctx, symbol, interval, limit)
		if err == nil && len(klines) > 0 {
			return klines, nil
		}
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", p.Name(), err))
		} else {
			errs = append(errs, fmt.Sprintf("%s: empty response", p.Name()))
		}
	}
	return nil, errors.New("provider chain exhausted: " + strings.Join(errs, "; "))
}

// normalizeSymbol 标准化币对 → 各源内部格式。
func normalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}

// clampLimit 钳制 K 线数量（1..1000）。
func clampLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}

// nowMs 当前毫秒时间戳。
func nowMs() int64 { return time.Now().UnixMilli() }
