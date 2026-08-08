package kernel

import (
	"fmt"
	"strings"

	"fxcore/internal/api/v1/dto"
)

// BuildSystemPrompt 主路径系统提示词（API设计.md §16 行为契约的骨架实现）。
// 契约字段（币源/时间框架/风控/仓位）落盘；语言契约：内置段英文、用户段
// （custom_prompt/prompt_sections）VERBATIM 透传。完整双路径构建后续迭代。
func BuildSystemPrompt(cfg dto.StrategyConfig) string {
	var b strings.Builder
	b.WriteString("You are an experienced crypto futures trader operating an automated trading system.\n")
	b.WriteString("You must respond with STRICT XML output:\n")
	b.WriteString("<reasoning>brief analysis</reasoning>\n")
	b.WriteString("<decision>[JSON array]</decision>\n\n")

	fmt.Fprintf(&b, "Coin source: %s\n", cfg.CoinSource.SourceType)
	if len(cfg.CoinSource.StaticCoins) > 0 {
		fmt.Fprintf(&b, "Static coins: %s\n", strings.Join(cfg.CoinSource.StaticCoins, ", "))
	}
	if cfg.Indicators.Klines.PrimaryTimeframe != "" {
		fmt.Fprintf(&b, "Primary timeframe: %s (%d bars)\n", cfg.Indicators.Klines.PrimaryTimeframe, cfg.Indicators.Klines.PrimaryCount)
	}
	if cfg.RiskControl.MaxPositions > 0 {
		fmt.Fprintf(&b, "Max positions: %d\n", cfg.RiskControl.MaxPositions)
	}
	if cfg.RiskControl.BTCEthMaxLeverage > 0 {
		fmt.Fprintf(&b, "Leverage: BTC/ETH %d, altcoins %d\n", cfg.RiskControl.BTCEthMaxLeverage, cfg.RiskControl.AltcoinMaxLeverage)
	}
	if cfg.RiskControl.MinPositionSize > 0 {
		fmt.Fprintf(&b, "Min position size: %.0f USDT (BTC/ETH min 60)\n", cfg.RiskControl.MinPositionSize)
	}
	if cfg.RiskControl.MinRiskRewardRatio > 0 {
		fmt.Fprintf(&b, "Min risk/reward: %.1f | Min confidence: %.0f%%\n",
			cfg.RiskControl.MinRiskRewardRatio, cfg.RiskControl.MinConfidence*100)
	}

	// 决策动作契约（六值）
	b.WriteString("Decision actions: open_long | open_short | close_long | close_short | hold | wait\n")
	b.WriteString("Open actions MUST include: symbol, quantity, leverage, stop_loss, take_profit, confidence, risk_usd\n")
	b.WriteString("No formulas, no thousands separators, no tilde (~) in numbers.\n")

	// 用户编辑段 VERBATIM 透传（§9.3 语言契约：不过滤中文）
	if cfg.CustomPrompt != "" {
		b.WriteString("\n[User custom prompt - follow verbatim]\n" + cfg.CustomPrompt + "\n")
	}
	if cfg.PromptSections != nil {
		if cfg.PromptSections.RoleDefinition != "" {
			b.WriteString("\n[Role]\n" + cfg.PromptSections.RoleDefinition + "\n")
		}
		if cfg.PromptSections.TradingFrequency != "" {
			b.WriteString("\n[Trading frequency]\n" + cfg.PromptSections.TradingFrequency + "\n")
		}
		if cfg.PromptSections.EntryStandards != "" {
			b.WriteString("\n[Entry standards]\n" + cfg.PromptSections.EntryStandards + "\n")
		}
		if cfg.PromptSections.DecisionProcess != "" {
			b.WriteString("\n[Decision process]\n" + cfg.PromptSections.DecisionProcess + "\n")
		}
	}
	return b.String()
}

// BuildUserContext 用户上下文（余额/持仓/K线摘要 → 单轮 user 消息）。
func BuildUserContext(balance float64, positions []string, klinesSummary string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Account balance: %.2f USDT\n", balance)
	if len(positions) == 0 {
		b.WriteString("Current positions: none\n")
	} else {
		b.WriteString("Current positions:\n" + strings.Join(positions, "\n") + "\n")
	}
	if klinesSummary != "" {
		b.WriteString("Market data:\n" + klinesSummary)
	}
	b.WriteString("\nAnalyze and output your decision now.")
	return b.String()
}
