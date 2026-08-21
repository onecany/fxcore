package dto

import "encoding/json"

// ========== Prompt Lint 契约（§6 POST /strategies/lint） ==========
// 类型与规则代码放契约层：kernel 规则引擎引用（返回 []LintIssue），
// handler 直接透传，前端 module 按 code 精确渲染/过滤。

// LintSeverity 警告级别。
type LintSeverity string

const (
	SeverityError   LintSeverity = "error"
	SeverityWarning LintSeverity = "warning"
)

// LintIssue 单条静态检查结果。
type LintIssue struct {
	Code     string       `json:"code"`
	Severity LintSeverity `json:"severity"`
	Field    string       `json:"field"`
	Title    string       `json:"title"`
	Detail   string       `json:"detail"`
}

// 规则代码（前端按 code 渲染图标/行内跳转，新增规则在此登记）。
const (
	// coin 族
	CodeCoinStaticEmpty      = "coin_static_empty"      // static 无币 → 引擎兜底 BTC-USDT
	CodeCoinAI500Deprecated  = "coin_ai500_deprecated"  // ai500 已下线
	CodeCoinExcludedInStatic = "coin_excluded_in_static" // 排除币同时出现在静态币列表
	CodeCoinBadFormat        = "coin_bad_format"        // 币名格式非法（非 XXX-YYY）

	// kline 族
	CodeKlinePeriodEmpty = "kline_period_empty"  // 指标开关开了但周期清单空 → 引擎不输出该指标
	CodeKlineBlind      = "kline_blind"        // 全部数据开关关 → AI 拿不到任何行情
	CodeKlineMTFEmpty   = "kline_mtf_empty"    // 多时间框架开启但未选周期
	CodeKlineTFInvalid  = "kline_tf_invalid"   // 主周期不在合法集合

	// risk 族
	CodeRiskMaxPosZero    = "risk_max_pos_zero"     // max_positions=0 → MergeConfigInto 跳过 + 引擎兜底
	CodeRiskLeverageZero  = "risk_leverage_zero"    // 杠杆非正 → 引擎风控按异常值处理
	CodeRiskRRBelowOne    = "risk_rr_below_one"     // 最小盈亏比 < 1 → 风控形同虚设
	CodeRiskConfidenceOut = "risk_confidence_out"   // 置信度超 [0,1] → 提示词拼出荒谬百分比
	CodeRiskMarginZero    = "risk_margin_zero"      // 保证金占用上限 0 → 引擎跳过该检查

	// text 族
	CodeTextAllEmpty  = "text_all_empty"  // 四段提示词 + 自定义全部为空
	CodeTextContract  = "text_contract"   // 自定义段命令与内置输出契约冲突
	CodeTextVariantTF = "text_variant_tf" // 交易风格与主周期不匹配
	CodeTextOversize  = "text_oversize"   // 段落超长 → token 膨胀
)

// LintRequest 静态检查请求（§11 POST /strategies/lint）。
type LintRequest struct {
	Config json.RawMessage `json:"config" binding:"required"`
}

// LintResponse 静态检查响应：命中规则集合（空数组 = 无警告）。
type LintResponse struct {
	Issues []LintIssue `json:"issues"`
}