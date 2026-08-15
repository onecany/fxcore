package kernel

import (
	"strings"
	"testing"

	"fxcore/internal/api/v1/dto"
)

// BuildSystemPrompt 角色段兜底契约测试（2026-08 回归锁定）。
// PromptSections 为 nil（全新策略/DefaultConfig 无自定义段）时也必须输出默认角色，
// 不能只输出标题无内容；自定义 RoleDefinition 存在时覆盖默认。

func TestBuildSystemPromptRoleFallback(t *testing.T) {
	cases := []struct {
		name string
		cfg  dto.StrategyConfig
		want string // 期望出现在输出中的角色描述子串
	}{
		{
			name: "nil prompt sections uses default role",
			cfg:  dto.StrategyConfig{},
			want: "You are an experienced crypto futures trader operating an automated trading system.",
		},
		{
			name: "empty role definition uses default role",
			cfg: dto.StrategyConfig{
				PromptSections: &dto.PromptSections{RoleDefinition: ""},
			},
			want: "You are an experienced crypto futures trader operating an automated trading system.",
		},
		{
			name: "custom role definition wins",
			cfg: dto.StrategyConfig{
				PromptSections: &dto.PromptSections{RoleDefinition: "你是一位只交易 BTC/ETH 永续的日内动量交易员"},
			},
			want: "你是一位只交易 BTC/ETH 永续的日内动量交易员",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			out := BuildSystemPrompt(c.cfg)
			if !strings.Contains(out, c.want) {
				t.Errorf("prompt 缺少 %q\n---\n%s", c.want, out)
			}
			// 角色标题必须存在且下面紧接内容（不出现空标题）
			if !strings.Contains(out, "# Exclusive Role Definition | Rigid Locked") {
				t.Errorf("缺少角色段标题")
			}
		})
	}
}

// BuildSystemPrompt 用户自定义段 VERBATIM 透传 + 留空段不输出（2026-08 契约锁定）。
func TestBuildSystemPromptUserSections(t *testing.T) {
	cfg := dto.StrategyConfig{
		PromptSections: &dto.PromptSections{
			RoleDefinition:   "自定义角色",
			TradingFrequency: "自定义频率",
			// EntryStandards 留空 → 不输出
			DecisionProcess: "自定义决策流程",
		},
		CustomPrompt: "自定义补充指令",
	}
	out := BuildSystemPrompt(cfg)
	for _, want := range []string{"自定义角色", "自定义频率", "自定义决策流程", "自定义补充指令"} {
		if !strings.Contains(out, want) {
			t.Errorf("用户段 %q 应 VERBATIM 透传, 缺失:\n%s", want, out)
		}
	}
	if strings.Contains(out, "Entry standards") {
		t.Errorf("留空的 entry_standards 不应输出:\n%s", out)
	}
}
