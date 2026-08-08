package provider

import (
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

// OKXProvider OKX 直连 K 线数据源（§14.3）。
// GET https://www.okx.com/api/v5/market/candles?instId=BTC-USDT-SWAP&bar=15m&limit=200
// 响应 data 数组元素为字符串数组：[ts,o,h,l,c,vol,...]
type OKXProvider struct {
	client *http.Client
	base   string
}

// NewOKXProvider 构造。
func NewOKXProvider(baseURL string) *OKXProvider {
	if baseURL == "" {
		baseURL = "https://www.okx.com/api/v5/market/candles"
	}
	return &OKXProvider{
		client: &http.Client{Timeout: 15 * time.Second},
		base:   baseURL,
	}
}

// Name 数据源名。
func (p *OKXProvider) Name() string { return "okx" }

// Klines 取 K 线。symbol 入参 BTC-USDT → BTC-USDT-SWAP（永续）。
func (p *OKXProvider) Klines(ctx context.Context, symbol, interval string, limit int) ([]dto.KlineDTO, error) {
	instID := normalizeSymbol(symbol)
	if !strings.Contains(instID, "-SWAP") {
		instID += "-SWAP"
	}
	limit = clampLimit(limit)

	u := fmt.Sprintf("%s?instId=%s&bar=%s&limit=%d", p.base, instID, okxBar(interval), limit)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "fxcore/1.0")
	resp, err := p.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("okx status %d: %s", resp.StatusCode, truncateStr(string(raw), 200))
	}
	var r struct {
		Code string     `json:"code"`
		Data [][]string `json:"data"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return nil, fmt.Errorf("okx parse: %w", err)
	}
	if r.Code != "0" {
		return nil, fmt.Errorf("okx error code %s", r.Code)
	}
	out := make([]dto.KlineDTO, 0, len(r.Data))
	for _, row := range r.Data {
		if len(row) < 6 {
			continue
		}
		k, err := parseOKXRow(row)
		if err != nil {
			continue
		}
		out = append(out, k)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("okx: no candles for %s", instID)
	}
	return out, nil
}

func parseOKXRow(row []string) (dto.KlineDTO, error) {
	var k dto.KlineDTO
	var err error
	if k.Timestamp, err = strconv.ParseInt(row[0], 10, 64); err != nil {
		return k, err
	}
	if k.Open, err = strconv.ParseFloat(row[1], 64); err != nil {
		return k, err
	}
	if k.High, err = strconv.ParseFloat(row[2], 64); err != nil {
		return k, err
	}
	if k.Low, err = strconv.ParseFloat(row[3], 64); err != nil {
		return k, err
	}
	if k.Close, err = strconv.ParseFloat(row[4], 64); err != nil {
		return k, err
	}
	if k.Volume, err = strconv.ParseFloat(row[5], 64); err != nil {
		return k, err
	}
	k.Timestamp /= 1000 // ms → s
	return k, nil
}

// okxBar OKX 周期格式。
func okxBar(interval string) string {
	switch interval {
	case "1m", "3m", "5m", "15m", "30m":
		return interval
	case "1h", "1H":
		return "1H"
	case "2h":
		return "2H"
	case "4h", "4H":
		return "4H"
	case "1d", "1D":
		return "1D"
	case "1w", "1W":
		return "1W"
	default:
		return "15m"
	}
}
