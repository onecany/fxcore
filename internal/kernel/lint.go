package kernel

import (
	"fmt"
	"regexp"
	"strings"

	"fxcore/internal/api/v1/dto"
)

// ========== Prompt Lint 规则引擎 ==========
// 静态检查策略配置中「会被引擎静默忽略或产生误导行为」的问题。
// 价值定位：配置保存成功 ≠ 引擎按用户想法执行。每条规则都对照引擎真实消费逻辑
// （candidateSymbols 兜底、BuildKlineContext 周期防漏、MergeConfigInto 整体替换
// 前置条件、BuildSystemPrompt 置信度拼装），不做拍脑袋的风格建议。
// 纯函数、零网络/AI 调用，输入是合并默认值后的完整配置。
// 类型与规则代码定义在 dto 契约层（dto/lint.go），本包只实现检查逻辑。

// 合法币源类型（对齐前端 COIN_SOURCE_TYPES 常量，ai500 已下线）。
var validCoinTypes = map[dto.CoinSourceType]bool{
	dto.CoinSourceStatic: true,
	dto.CoinSourceOITop:  true,
	dto.CoinSourceOILow:  true,
	dto.CoinSourceMixed:  true,
}

// 合法周期（引擎拉 K 线用）。
var validTimeframes = map[string]bool{
	"1m": true, "3m": true, "5m": true, "15m": true, "30m": true,
	"1h": true, "2h": true, "4h": true, "8h": true, "12h": true, "1d": true, "1w": true,
}

var coinPattern = regexp.MustCompile(`^[A-Za-z0-9]{2,12}-[A-Za-z0-9]{2,10}$`)

// Lint 对策略配置做静态检查，返回全部命中规则。
func Lint(cfg dto.StrategyConfig) []dto.LintIssue {
	var issues []dto.LintIssue
	issues = append(issues, lintCoin(cfg)...)
	issues = append(issues, lintKline(cfg)...)
	issues = append(issues, lintRisk(cfg)...)
	issues = append(issues, lintText(cfg)...)
	return issues
}

// ========== coin 族 ==========

func lintCoin(cfg dto.StrategyConfig) []dto.LintIssue {
	var out []dto.LintIssue
	cs := cfg.CoinSource

	// 已下线类型
	if !validCoinTypes[cs.SourceType] {
		detail := "币源类型 %q 已下线或不存在，保存后引擎会继续按旧配置取数，请改选其他类型（如 static / oi_top）。"
		out = append(out, dto.LintIssue{
			Code: dto.CodeCoinAI500Deprecated, Severity: dto.SeverityError, Field: "coin_source.source_type",
			Title:  "币源类型已下线",
			Detail: fmt.Sprintf(detail, cs.SourceType),
		})
	}

	// static 且无币 → 引擎兜底 BTC-USDT（engine.candidateSymbols）
	if cs.SourceType == dto.CoinSourceStatic && len(cs.StaticCoins) == 0 {
		out = append(out, dto.LintIssue{
			Code: dto.CodeCoinStaticEmpty, Severity: dto.SeverityError, Field: "coin_source.static_coins",
			Title:  "静态币源为空",
			Detail: "未配置任何币种，引擎会兜底只交易 BTC-USDT。请至少添加一个币种（如 BTC-USDT）。",
		})
	}

	if len(cs.StaticCoins) > 0 {
		var bad []string
		for _, c := range cs.StaticCoins {
			if !coinPattern.MatchString(c) {
				bad = append(bad, c)
			}
		}
		if len(bad) > 0 {
			out = append(out, dto.LintIssue{
				Code: dto.CodeCoinBadFormat, Severity: dto.SeverityError, Field: "coin_source.static_coins",
				Title:  "币名格式非法",
				Detail: fmt.Sprintf("以下币名不符合「基础币-计价币」格式，会被引擎当作无效符号：%s。", strings.Join(bad, ", ")),
			})
		}
		// 排除币与静态币重叠（排除逻辑只对非静态币生效，重叠等于没排除）
		excluded := map[string]bool{}
		for _, c := range cs.ExcludedCoins {
			excluded[c] = true
		}
		var dup []string
		for _, c := range cs.StaticCoins {
			if excluded[c] {
				dup = append(dup, c)
			}
		}
		if len(dup) > 0 {
			out = append(out, dto.LintIssue{
				Code: dto.CodeCoinExcludedInStatic, Severity: dto.SeverityWarning, Field: "coin_source.excluded_coins",
				Title:  "排除币与静态币重叠",
				Detail: fmt.Sprintf("以下币种同时在静态列表和排除列表：%s。排除对该策略不生效，请从排除列表移除。", strings.Join(dup, ", ")),
			})
		}
	}
	return out
}

// ========== kline 族 ==========

func lintKline(cfg dto.StrategyConfig) []dto.LintIssue {
	var out []dto.LintIssue
	ind := cfg.Indicators

	// 主周期合法性
	if !validTimeframes[ind.Klines.PrimaryTimeframe] {
		out = append(out, dto.LintIssue{
			Code: dto.CodeKlineTFInvalid, Severity: dto.SeverityError, Field: "indicators.klines.primary_timeframe",
			Title:  "主周期非法",
			Detail: fmt.Sprintf("%q 不是合法周期（支持 1m/3m/5m/15m/30m/1h/2h/4h/8h/12h/1d/1w），引擎取不到对应 K 线。", ind.Klines.PrimaryTimeframe),
		})
	}

	// 多周期开启但未选周期
	if ind.Klines.EnableMultiTimeframe && len(ind.Klines.SelectedTimeframes) == 0 {
		out = append(out, dto.LintIssue{
			Code: dto.CodeKlineMTFEmpty, Severity: dto.SeverityWarning, Field: "indicators.klines.selected_timeframes",
			Title:  "多周期未选周期",
			Detail: "已开启多周期分析，但未勾选任何周期，引擎只会按主周期取数。",
		})
	}

	// 开关开但周期空 → 引擎不输出该指标
	if ind.EnableEMA && len(ind.EMAPeriods) == 0 {
		out = append(out, periodIssue("EMA", "7,25,99", "indicators.ema_periods"))
	}
	if ind.EnableRSI && len(ind.RSIPeriods) == 0 {
		out = append(out, periodIssue("RSI", "14", "indicators.rsi_periods"))
	}
	if ind.EnableATR && len(ind.ATRPeriods) == 0 {
		out = append(out, periodIssue("ATR", "14", "indicators.atr_periods"))
	}
	if ind.EnableBoll && len(ind.BollPeriods) == 0 {
		out = append(out, periodIssue("BOLL", "20", "indicators.boll_periods"))
	}

	// 全部数据开关关 → AI 看不到行情
	anyData := ind.EnableRawKlines || ind.EnableEMA || ind.EnableMACD || ind.EnableRSI ||
		ind.EnableATR || ind.EnableBoll || ind.EnableVolume || ind.EnableOI || ind.EnableFundingRate
	if !anyData {
		out = append(out, dto.LintIssue{
			Code: dto.CodeKlineBlind, Severity: dto.SeverityError, Field: "indicators",
			Title:  "AI 拿不到行情数据",
			Detail: "原始 K 线与全部技术指标均已关闭，AI 决策时看不到任何市场数据，只能盲猜。请至少开启一项。",
		})
	}
	return out
}

func periodIssue(name, suggestion, path string) dto.LintIssue {
	return dto.LintIssue{
		Code: dto.CodeKlinePeriodEmpty, Severity: dto.SeverityError, Field: path,
		Title:  name + " 周期为空",
		Detail: fmt.Sprintf("%s 已开启但未配置周期，引擎不会输出该指标。请填至少一个周期（如 %s）。", name, suggestion),
	}
}

// ========== risk 族 ==========

func lintRisk(cfg dto.StrategyConfig) []dto.LintIssue {
	var out []dto.LintIssue
	rc := cfg.RiskControl

	if rc.MaxPositions <= 0 {
		out = append(out, dto.LintIssue{
			Code: dto.CodeRiskMaxPosZero, Severity: dto.SeverityError, Field: "risk_control.max_positions",
			Title:  "同时持仓上限无效",
			Detail: "max_positions ≤ 0 时引擎按默认 3 处理，且该配置更新会被跳过。请显式设为 ≥1。",
		})
	}
	if rc.BTCEthMaxLeverage <= 0 || rc.AltcoinMaxLeverage <= 0 {
		out = append(out, dto.LintIssue{
			Code: dto.CodeRiskLeverageZero, Severity: dto.SeverityError, Field: "risk_control.btc_eth_max_leverage",
			Title:  "杠杆上限无效",
			Detail: "BTC/ETH 或山寨币杠杆上限 ≤ 0，开仓时杠杆会被风控按异常值处理。请设为 ≥1。",
		})
	}
	if rc.MinRiskRewardRatio > 0 && rc.MinRiskRewardRatio < 1 {
		out = append(out, dto.LintIssue{
			Code: dto.CodeRiskRRBelowOne, Severity: dto.SeverityWarning, Field: "risk_control.min_risk_reward_ratio",
			Title:  "盈亏比低于 1",
			Detail: fmt.Sprintf("最小盈亏比 %.1f < 1，意味着止损空间大于止盈空间，长期期望为负。建议 ≥1.5。", rc.MinRiskRewardRatio),
		})
	}
	if rc.MinConfidence < 0 || rc.MinConfidence > 1 {
		out = append(out, dto.LintIssue{
			Code: dto.CodeRiskConfidenceOut, Severity: dto.SeverityError, Field: "risk_control.min_confidence",
			Title:  "置信度阈值越界",
			Detail: fmt.Sprintf("置信度 %.2f 超出 [0,1]，提示词会拼出超出 100%% 的荒谬要求，AI 无法满足。请设为 0-1。", rc.MinConfidence),
		})
	}
	if rc.MaxMarginUsage <= 0 {
		out = append(out, dto.LintIssue{
			Code: dto.CodeRiskMarginZero, Severity: dto.SeverityError, Field: "risk_control.max_margin_usage",
			Title:  "保证金占用上限无效",
			Detail: "保证金占用上限 ≤ 0 时引擎跳过占用检查，满仓风险无防护。请设为 >0（如 0.3 = 30%）。",
		})
	}
	return out
}

// ========== text 族 ==========

// 用户段提示词特征检查：与内核解析契约（fence 提取 + reasoning 剥离 + 六值 JSON）
// 冲突的写法会「静默杀死」AI 决策——保存成功但运行时解析失败降级 SafeWait。
func lintText(cfg dto.StrategyConfig) []dto.LintIssue {
	var out []dto.LintIssue
	sections := []string{cfg.CustomPrompt}
	if cfg.PromptSections != nil {
		sections = append(sections,
			cfg.PromptSections.RoleDefinition,
			cfg.PromptSections.TradingFrequency,
			cfg.PromptSections.EntryStandards,
			cfg.PromptSections.DecisionProcess,
		)
	}
	allEmpty := true
	for _, s := range sections {
		if s != "" {
			allEmpty = false
		}
	}
	if allEmpty {
		out = append(out, dto.LintIssue{
			Code: dto.CodeTextAllEmpty, Severity: dto.SeverityWarning, Field: "prompt_sections",
			Title:  "未定制交易指令",
			Detail: "四段提示词与自定义指令均为空，AI 只用内置模板决策，交易行为不受你的策略偏好约束。建议至少填写交易频率与入场标准。",
		})
	}

	for name, s := range map[string]string{"custom_prompt": cfg.CustomPrompt, "prompt_sections": joinSections(cfg)} {
		if s == "" {
			continue
		}
		if len(s) > 2000 {
			out = append(out, dto.LintIssue{
				Code: dto.CodeTextOversize, Severity: dto.SeverityWarning, Field: name,
				Title:  "提示词段落超长",
				Detail: fmt.Sprintf("%s 超过 2000 字符。超长指令会稀释关键约束并膨胀 token 成本，建议精简或拆分段落。", name),
			})
		}
		for _, bad := range []string{"不要输出", "只输出决策", "不要分析", "不分析", "无需分析", "直接给结论", "省略分析", "不要解释", "no reasoning", "don't explain"} {
			if strings.Contains(s, bad) {
				out = append(out, dto.LintIssue{
					Code: dto.CodeTextContract, Severity: dto.SeverityError, Field: name,
					Title:  "指令要求跳过分析，会破坏决策格式",
					Detail: fmt.Sprintf("%s 中出现「%s」：每个决策必须带 reasoning 分析段（解析链依赖它）。省略后模型输出会被判为非法决策静默丢弃，AI 将无单可开。请保留分析步骤。", name, bad),
				})
				break
			}
		}
	}

	// 风格与主周期矛盾
	switch cfg.PromptVariant {
	case "scalping":
		tf := cfg.Indicators.Klines.PrimaryTimeframe
		if tf == "1d" || tf == "4h" || tf == "1w" {
			out = append(out, variantTFIssue("剥头皮", tf, "应使用 ≤15m 的短周期"))
		}
	case "conservative":
		tf := cfg.Indicators.Klines.PrimaryTimeframe
		if tf == "1m" || tf == "5m" {
			out = append(out, variantTFIssue("稳健", tf, "建议使用 ≥15m 周期等待高确认信号"))
		}
	}
	return out
}

func variantTFIssue(variant, tf, suggestion string) dto.LintIssue {
	return dto.LintIssue{
		Code: dto.CodeTextVariantTF, Severity: dto.SeverityWarning, Field: "prompt_variant",
		Title:  variant + "风格与主周期不匹配",
		Detail: fmt.Sprintf("主周期 %s 与 %s 风格矛盾：%s。", tf, variant, suggestion),
	}
}

func joinSections(cfg dto.StrategyConfig) string {
	if cfg.PromptSections == nil {
		return ""
	}
	return strings.Join([]string{
		cfg.PromptSections.RoleDefinition,
		cfg.PromptSections.TradingFrequency,
		cfg.PromptSections.EntryStandards,
		cfg.PromptSections.DecisionProcess,
	}, "\n")
}