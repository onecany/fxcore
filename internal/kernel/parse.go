// Package kernel 决策输出解析与提示词构建（API设计.md §16 行为契约）。
// 当前实现决策解析器（ParseDecision）；完整双路径提示词构建随引擎层迭代。
package kernel

import (
	"encoding/json"
	"fmt"
	"strings"

	"fxcore/internal/model"
)

// ValidActions 决策动作六值。
var ValidActions = map[string]bool{
	model.ActionOpenLong:   true,
	model.ActionOpenShort:  true,
	model.ActionCloseLong:  true,
	model.ActionCloseShort: true,
	model.ActionHold:       true,
	model.ActionWait:       true,
}

// SafeWait fallback 决策（§16：解析失败回退 safe wait，置信 50）。
func SafeWait() []model.DecisionAction {
	return []model.DecisionAction{{Action: model.ActionWait, Confidence: 50}}
}

// ParseDecision 从模型原始输出提取决策数组（§16 解析容错链）：
// 剥 reasoning 标签 → JSON fence/decision 标签提取 → 修全角字符 → 校验六值。
// 任何一步失败返回 error（调用方回退 SafeWait 并记录 1405 降级）。
func ParseDecision(raw string) ([]model.DecisionAction, error) {
	body := extractDecisionBody(raw)
	if body == "" {
		return nil, fmt.Errorf("kernel: no decision block in output")
	}
	body = FixFullWidth(body)
	body = strings.TrimSpace(body)

	var actions []model.DecisionAction
	if err := json.Unmarshal([]byte(body), &actions); err != nil {
		// 模型偶发输出单对象而非数组（如 {"action":"wait",...}）——归一化为数组
		var single model.DecisionAction
		if err2 := json.Unmarshal([]byte(body), &single); err2 == nil {
			actions = []model.DecisionAction{single}
		} else {
			return nil, fmt.Errorf("kernel: invalid decision json: %w", err)
		}
	}
	if len(actions) == 0 {
		return nil, fmt.Errorf("kernel: empty decision array")
	}
	for i, a := range actions {
		if !ValidActions[a.Action] {
			return nil, fmt.Errorf("kernel: invalid action %q at index %d", a.Action, i)
		}
		if strings.TrimSpace(a.Symbol) == "" && a.Action != model.ActionWait && a.Action != model.ActionHold {
			return nil, fmt.Errorf("kernel: missing symbol at index %d", i)
		}
	}
	return actions, nil
}

// extractDecisionBody 提取决策 JSON 主体：
// 1. <decision>...</decision> 标签
// 2. ```json ... ``` fence
// 3. 剥离 <reasoning>...</reasoning> 后剩余部分
func extractDecisionBody(raw string) string {
	if i := strings.Index(raw, "<decision>"); i >= 0 {
		rest := raw[i+len("<decision>"):]
		if j := strings.Index(rest, "</decision>"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	if i := strings.Index(raw, "```json"); i >= 0 {
		rest := raw[i+len("```json"):]
		if j := strings.Index(rest, "```"); j >= 0 {
			return rest[:j]
		}
		return rest
	}
	// 剥离 reasoning 块
	body := raw
	for {
		s := strings.Index(body, "<reasoning>")
		e := strings.Index(body, "</reasoning>")
		if s >= 0 && e > s {
			body = body[:s] + body[e+len("</reasoning>"):]
			continue
		}
		break
	}
	return body
}

// FixFullWidth 修全角标点（模型常输出中文标点破坏 JSON）。导出供 debate 等复用。
func FixFullWidth(s string) string {
	replacer := strings.NewReplacer(
		"，", ",",
		"：", ":",
		"；", ";",
		"（", "(",
		"）", ")",
		"｛", "{",
		"｝", "}",
		"［", "[",
		"］", "]",
		"“", "\"",
		"”", "\"",
		"‘", "'",
		"’", "'",
		"～", "~",
	)
	return replacer.Replace(s)
}
