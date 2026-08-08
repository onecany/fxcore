package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"fxcore/internal/api/v1/dto"
)

// HyperliquidProvider Hyperliquid JSON-RPC 数据源（§14.3）。
// POST /info {"type":"candleSnapshot","req":{"coin":"BTC","interval":"15m","startTime":...,"endTime":...}}
type HyperliquidProvider struct {
	client *http.Client
	base   string // 默认 https://api.hyperliquid.xyz/info
}

// NewHyperliquidProvider 构造。
func NewHyperliquidProvider(baseURL string) *HyperliquidProvider {
	if baseURL == "" {
		baseURL = "https://api.hyperliquid.xyz/info"
	}
	return &HyperliquidProvider{
		client: &http.Client{Timeout: 15 * time.Second},
		base:   baseURL,
	}
}

// Name 数据源名。
func (p *HyperliquidProvider) Name() string { return "hyperliquid" }

// candleSnapshot 响应条目：[ts, o, h, l, c, v]
type hlCandle []json.RawMessage

// Klines 取 K 线。symbol 入参 BTC-USDT → BTC。
func (p *HyperliquidProvider) Klines(ctx context.Context, symbol, interval string, limit int) ([]dto.KlineDTO, error) {
	coin := strings.Split(normalizeSymbol(symbol), "-")[0]
	limit = clampLimit(limit)
	// candleSnapshot 一次最多 5000 根；按 limit 回推 startTime
	endMs := nowMs()
	startMs := endMs - int64(limit)*intervalMillis(interval)

	payload := map[string]any{
		"type": "candleSnapshot",
		"req": map[string]any{
			"coin":      coin,
			"interval":  hlInterval(interval),
			"startTime": startMs,
			"endTime":   endMs,
		},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("hyperliquid marshal: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("hyperliquid status %d: %s", resp.StatusCode, truncateStr(string(raw), 200))
	}
	var candles []hlCandle
	if err := json.Unmarshal(raw, &candles); err != nil {
		return nil, fmt.Errorf("hyperliquid parse: %w", err)
	}
	out := make([]dto.KlineDTO, 0, len(candles))
	for _, c := range candles {
		if len(c) < 6 {
			continue
		}
		k, err := parseHLCandle(c)
		if err != nil {
			continue
		}
		out = append(out, k)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("hyperliquid: no candles for %s", coin)
	}
	return out, nil
}

func parseHLCandle(c hlCandle) (dto.KlineDTO, error) {
	var k dto.KlineDTO
	var err error
	ts, err := parseF(c[0])
	if err != nil {
		return k, err
	}
	k.Timestamp = int64(ts)
	if k.Open, err = parseF(c[1]); err != nil {
		return k, err
	}
	if k.High, err = parseF(c[2]); err != nil {
		return k, err
	}
	if k.Low, err = parseF(c[3]); err != nil {
		return k, err
	}
	if k.Close, err = parseF(c[4]); err != nil {
		return k, err
	}
	if k.Volume, err = parseF(c[5]); err != nil {
		return k, err
	}
	k.Timestamp /= 1000 // ms → s
	return k, nil
}

func parseF(raw json.RawMessage) (float64, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return strconv.ParseFloat(s, 64)
	}
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, err
	}
	return f, nil
}

// hlInterval hyperliquid 周期格式。
func hlInterval(interval string) string {
	switch interval {
	case "1m", "3m", "5m", "15m", "30m", "1h", "2h", "4h", "1d", "1w", "1M":
		return interval
	case "1H":
		return "1h"
	case "4H":
		return "4h"
	case "1D":
		return "1d"
	case "1W":
		return "1w"
	default:
		return "15m"
	}
}

// intervalMillis 周期毫秒（回推 startTime 用）。
func intervalMillis(interval string) int64 {
	switch interval {
	case "1m":
		return 60_000
	case "3m":
		return 180_000
	case "5m":
		return 300_000
	case "15m":
		return 900_000
	case "30m":
		return 1_800_000
	case "1h", "1H":
		return 3_600_000
	case "2h":
		return 7_200_000
	case "4h", "4H":
		return 14_400_000
	case "1d", "1D":
		return 86_400_000
	case "1w", "1W":
		return 604_800_000
	default:
		return 900_000
	}
}

func truncateStr(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
