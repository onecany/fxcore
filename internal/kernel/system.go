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
	b.WriteString("# Exclusive Role Definition \| Rigid Locked\n")
	if cfg.PromptSections != nil {
		if cfg.PromptSections.RoleDefinition != "" {
			b.WriteString("\n" + cfg.PromptSections.RoleDefinition + "\n")
		}else{
			b.WriteString("You are an experienced crypto futures trader operating an automated trading system.\n")

		}
	}
	b.WriteString("# Mandatory Output Format \| Absolutely Unchangeable\n")
	b.WriteString("All responses **must contain only standardized XML structure** with no extra text, explanations, comments, formulas or auxiliary symbols\. Only precise integers and decimals are permitted for all numerical values\. Never use thousand separators, tildes, unit suffixes, or approximate rounded values\.\n")
	b.WriteString("Fixed exclusive output format:\n\n")
	b.WriteString("```Plain Text\n\n")
	b.WriteString("<reasoning>Concise logical analysis summary (1-3 sentences covering market cycle, core technical signals, multi-timeframe verification results, volume/OI coordination, and risk control judgment)</reasoning>\n")
	b.WriteString("<decision>Standard JSON array (only fields specified by the rules)</decision>\n")
	b.WriteString("```\n\n")

	b.WriteString("# Global Fixed Trading Parameters \| Permanently Locked \(No Modifications Allowed\)\n\n")
	b.WriteString("- Asset Source: Static fixed coin pool\. No manual additions, deletions or substitutions are allowed\.\n\n")

	fmt.Fprintf(&b, "- Coin source: %s\n", cfg.CoinSource.SourceType)
	if len(cfg.CoinSource.StaticCoins) > 0 {
		fmt.Fprintf(&b, "- Tradable Assets: %s\n", strings.Join(cfg.CoinSource.StaticCoins, ", "))
	}
	if cfg.Indicators.Klines.PrimaryTimeframe != "" {
		fmt.Fprintf(&b, "- Core Analysis Timeframe: %s (%d bars)\n", cfg.Indicators.Klines.PrimaryTimeframe, cfg.Indicators.Klines.PrimaryCount)
	}
	if cfg.RiskControl.MaxPositions > 0 {
		fmt.Fprintf(&b, "- Maximum Concurrent Positions: %d\n", cfg.RiskControl.MaxPositions)
	}
	if cfg.RiskControl.BTCEthMaxLeverage > 0 {
		fmt.Fprintf(&b, "- Leverage: BTC/ETH %d, altcoins %d\n", cfg.RiskControl.BTCEthMaxLeverage, cfg.RiskControl.AltcoinMaxLeverage)
	}
	if cfg.RiskControl.MinPositionSize > 0 {
		fmt.Fprintf(&b, "- Min position size: %.0f USDT (BTC/ETH min 60)\n", cfg.RiskControl.MinPositionSize)
	}
	if cfg.RiskControl.MinRiskRewardRatio > 0 {
		fmt.Fprintf(&b, "- Min risk/reward: %.1f | Min confidence: %.0f%%\n",
			cfg.RiskControl.MinRiskRewardRatio, cfg.RiskControl.MinConfidence*100)
	}

	// 决策动作契约（六值）
	b.WriteString("# Trading Action Enumeration \| Fixed Valid Values Only\n\n")
	b.WriteString("Only six trading actions are permitted: open\_long, open\_short, close\_long, close\_short, hold, wait\n\n")
	b.WriteString("All position\-opening actions**must include the complete mandatory field set without omission**: symbol, quantity, leverage, stop\_loss, take\_profit, confidence, risk\_usd\. All field values must be pure numeric values with no additional characters, symbols or unit labels\.\n")

	// 模式变体（§9.5 writeModeVariant；四种模式，内置段英文契约）
	writeModeVariant(&b, cfg.PromptVariant)

	// 用户编辑段 VERBATIM 透传（§9.3 语言契约：不过滤中文）
	if cfg.CustomPrompt != "" {
		b.WriteString("\n[User custom prompt - follow verbatim]\n" + cfg.CustomPrompt + "\n")
	}
	if cfg.PromptSections != nil {
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

// writeModeVariant 模式变体段（§9.5：balanced 默认 / aggressive / conservative / scalping）。
// 内置段保持英文（模型契约稳定性）；未知值回退 balanced。
func writeModeVariant(sb *strings.Builder, variant string) {
	switch variant {
	case "aggressive":
		sb.WriteString("Mode: Aggressive. Higher leverage tolerance, faster entries, accept smaller risk/reward; never exceed the leverage caps above.\n")
	case "conservative":
		sb.WriteString("Mode: Conservative. Fewer trades, only high-conviction setups where multiple timeframes align; prefer waiting over acting.\n")
	case "scalping":
		sb.WriteString("Mode: Scalping. Short timeframes, tight stops, quick scalps; prioritize capital preservation and frequent small wins.\n")
	default:
		sb.WriteString("Mode: Balanced. Recommended balance of opportunity and risk; no extreme sizing or overtrading.\n")
	}
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
