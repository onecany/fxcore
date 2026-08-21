package kernel

import (
	"strings"
	"testing"

	"fxcore/internal/api/v1/dto"
)

// baseLintConfig 测试基底：完整合法配置（对齐 service.DefaultConfig），
// 每个用例只改动要触发的字段，保证断言的是目标规则而非连带误报。
func baseLintConfig() dto.StrategyConfig {
	return dto.StrategyConfig{
		StrategyType:  "ai",
		Language:      "zh",
		PromptVariant: "balanced",
		CoinSource: dto.CoinSourceConfig{
			SourceType:  dto.CoinSourceStatic,
			StaticCoins: []string{"BTC-USDT", "ETH-USDT"},
		},
		Indicators: dto.IndicatorConfig{
			Klines: dto.KlineConfig{
				PrimaryTimeframe:     "15m",
				PrimaryCount:         200,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"1m", "5m", "15m"},
			},
			EnableRawKlines: true,
			EnableEMA:       true,
			EMAPeriods:      []int{7, 25, 99},
			EnableMACD:      true,
			EnableRSI:       true,
			RSIPeriods:      []int{14},
			EnableATR:       true,
			ATRPeriods:      []int{14},
			EnableBoll:      true,
			BollPeriods:     []int{20},
			EnableVolume:    true,
		},
		RiskControl: dto.RiskControlConfig{
			MaxPositions:                 3,
			BTCEthMaxLeverage:            5,
			AltcoinMaxLeverage:           5,
			BTCEthMaxPositionValueRatio:  5,
			AltcoinMaxPositionValueRatio: 1,
			MaxMarginUsage:               0.3,
			MinPositionSize:              12,
			MinRiskRewardRatio:           1.5,
			MinConfidence:                0.6,
		},
		PromptSections: &dto.PromptSections{
			RoleDefinition:   "你是资深量化交易员。",
			TradingFrequency: "每天最多 3 笔。",
			EntryStandards:   "EMA 金叉且 RSI 回踩确认才进场。",
			DecisionProcess:  "先分析再决策。",
		},
	}
}

// codes 提取命中规则的代码集合。
func codes(issues []dto.LintIssue) map[string]bool {
	m := make(map[string]bool, len(issues))
	for _, i := range issues {
		m[i.Code] = true
	}
	return m
}

func has(m map[string]bool, code string) bool { return m[code] }

func TestLintCleanConfig(t *testing.T) {
	issues := Lint(baseLintConfig())
	if len(issues) != 0 {
		t.Fatalf("合法配置不应有警告，得到 %d 条: %v", len(issues), issues)
	}
}

// ========== coin 族 ==========

func TestLintCoinStaticEmpty(t *testing.T) {
	cfg := baseLintConfig()
	cfg.CoinSource.StaticCoins = nil
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeCoinStaticEmpty) {
		t.Fatalf("期望命中 coin_static_empty，得到 %v", issues)
	}
}

func TestLintCoinAI500Deprecated(t *testing.T) {
	cfg := baseLintConfig()
	cfg.CoinSource.SourceType = dto.CoinSourceAI500
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeCoinAI500Deprecated) {
		t.Fatalf("期望命中 coin_ai500_deprecated，得到 %v", issues)
	}
}

func TestLintCoinBadFormat(t *testing.T) {
	cfg := baseLintConfig()
	cfg.CoinSource.StaticCoins = []string{"BTC-USDT", "BTC", "ETH/USDT"}
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeCoinBadFormat) {
		t.Fatalf("期望命中 coin_bad_format，得到 %v", issues)
	}
}

func TestLintCoinExcludedInStatic(t *testing.T) {
	cfg := baseLintConfig()
	cfg.CoinSource.ExcludedCoins = []string{"BTC-USDT"}
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeCoinExcludedInStatic) {
		t.Fatalf("期望命中 coin_excluded_in_static，得到 %v", issues)
	}
}

// ----=---- kline 族 =----

func TestLintKlinePeriodEmpty(t *testing.T) {
	cfg := baseLintConfig()
	cfg.Indicators.EMAPeriods = nil
	cfg.Indicators.RSIPeriods = nil
	issues := Lint(cfg)
	c := codes(issues)
	if !has(c, dto.CodeKlinePeriodEmpty) {
		t.Fatalf("期望命中 kline_period_empty，得到 %v", issues)
	}
	if !strings.Contains(issues[0].Title, "EMA") && !strings.Contains(issues[0].Title, "RSI") {
		t.Fatalf("周期为空警告应指明具体指标名，得到 %+v", issues[0])
	}
}

func TestLintKlineBlind(t *testing.T) {
	cfg := baseLintConfig()
	cfg.Indicators.EnableRawKlines = false
	cfg.Indicators.EnableEMA = false
	cfg.Indicators.EnableMACD = false
	cfg.Indicators.EnableRSI = false
	cfg.Indicators.EnableATR = false
	cfg.Indicators.EnableBoll = false
	cfg.Indicators.EnableVolume = false
	cfg.Indicators.EnableOI = false
	cfg.Indicators.EnableFundingRate = false
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeKlineBlind) {
		t.Fatalf("期望命中 kline_blind，得到 %v", issues)
	}
}

func TestLintKlineMTFEmpty(t *testing.T) {
	cfg := baseLintConfig()
	cfg.Indicators.Klines.SelectedTimeframes = nil
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeKlineMTFEmpty) {
		t.Fatalf("期望命中 kline_mtf_empty，得到 %v", issues)
	}
}

func TestLintKlineTFInvalid(t *testing.T) {
	cfg := baseLintConfig()
	cfg.Indicators.Klines.PrimaryTimeframe = "90m"
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeKlineTFInvalid) {
		t.Fatalf("期望命中 kline_tf_invalid，得到 %v", issues)
	}
}

// ----======== risk 族 =----

func TestLintRiskMaxPosZero(t *testing.T) {
	cfg := baseLintConfig()
	cfg.RiskControl.MaxPositions = 0
	c := codes(Lint(cfg))
	if !has(c, dto.CodeRiskMaxPosZero) {
		t.Fatalf("期望命中 risk_max_pos_zero，得到 %v", c)
	}
}

func TestLintRiskLeverageZero(t *testing.T) {
	cfg := baseLintConfig()
	cfg.RiskControl.AltcoinMaxLeverage = 0
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeRiskLeverageZero) {
		t.Fatalf("期望命中 risk_leverage_zero，得到 %v", issues)
	}
}

func TestLintRiskRRBelowOne(t *testing.T) {
	cfg := baseLintConfig()
	cfg.RiskControl.MinRiskRewardRatio = 0.5
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeRiskRRBelowOne) {
		t.Fatalf("期望命中 risk_rr_below_one，得到 %v", issues)
	}
}

func TestLintRiskConfidenceOut(t *testing.T) {
	cfg := baseLintConfig()
	cfg.RiskControl.MinConfidence = 1.5
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeRiskConfidenceOut) {
		t.Fatalf("期望命中 risk_confidence_out，得到 %v", issues)
	}
}

func TestLintRiskMarginZero(t *testing.T) {
	cfg := baseLintConfig()
	cfg.RiskControl.MaxMarginUsage = 0
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeRiskMarginZero) {
		t.Fatalf("期望命中 risk_margin_zero，得到 %v", issues)
	}
}

// ============ text 族 =----

func TestLintTextAllEmpty(t *testing.T) {
	cfg := baseLintConfig()
	cfg.CustomPrompt = ""
	cfg.PromptSections = &dto.PromptSections{}
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeTextAllEmpty) {
		t.Fatalf("期望命中 text_all_empty，得到 %v", issues)
	}
}

func TestLintTextContract(t *testing.T) {
	cfg := baseLintConfig()
	cfg.CustomPrompt = "直接给出结论，不要输出分析过程。"
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeTextContract) {
		t.Fatalf("期望命中 text_contract，得到 %v", issues)
	}
}

func TestLintTextOversize(t *testing.T) {
	cfg := baseLintConfig()
	cfg.PromptSections.DecisionProcess = strings.Repeat("相同", 2500) // 5000 字符
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeTextOversize) {
		t.Fatalf("期望命中 text_oversize，得到 %v", issues)
	}
}

func TestLintTextVariantScalping(t *testing.T) {
	cfg := baseLintConfig()
	cfg.PromptVariant = "scalping"
	cfg.Indicators.Klines.PrimaryTimeframe = "1d"
	issues := Lint(cfg)
	if !has(codes(issues), dto.CodeTextVariantTF) {
		t.Fatalf("期望命中 text_variant_tf（scalping×1d），得到 %v", issues)
	}
}

func TestLintTextVariantConservativeOK(t *testing.T) {
	cfg := baseLintConfig()
	cfg.PromptVariant = "conservative"
	cfg.Indicators.Klines.PrimaryTimeframe = "1h"
	issues := Lint(cfg)
	if has(codes(issues), dto.CodeTextVariantTF) {
		t.Fatalf("稳健×1h 不应命中风格矛盾，得到 %v", issues)
	}
}