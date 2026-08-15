package service

import (
	"encoding/json"
	"strings"
	"testing"

	"fxcore/internal/api/v1/dto"
)

// MergeConfigInto prompt_sections 逐字段合并契约测试（2026-08 回归锁定）。
// 前端 draft 是"部分补丁"语义（只发用户编辑过的键），旧实现整体替换
// dst.PromptSections 导致未编辑段被清空（用户只编辑一段保存后其余三段丢失）。

func TestMergeConfigIntoPromptSectionsPartialUpdate(t *testing.T) {
	dst := dto.StrategyConfig{
		PromptSections: &dto.PromptSections{
			RoleDefinition:   "角色A",
			TradingFrequency: "频率B",
			EntryStandards:   "进场C",
			DecisionProcess:  "决策D",
		},
	}
	// 前端只编辑 role_definition 一段，draft 只含该键
	raw := json.RawMessage(`{"prompt_sections": {"role_definition": "角色A2"}}`)
	if err := MergeConfigInto(&dst, raw); err != nil {
		t.Fatal(err)
	}
	ps := dst.PromptSections
	if ps == nil {
		t.Fatal("prompt_sections 丢失")
	}
	if ps.RoleDefinition != "角色A2" {
		t.Errorf("role_definition 应更新为 A2, got %q", ps.RoleDefinition)
	}
	if ps.TradingFrequency != "频率B" {
		t.Errorf("未编辑段 trading_frequency 被清空: got %q, want 频率B", ps.TradingFrequency)
	}
	if ps.EntryStandards != "进场C" {
		t.Errorf("未编辑段 entry_standards 被清空: got %q, want 进场C", ps.EntryStandards)
	}
	if ps.DecisionProcess != "决策D" {
		t.Errorf("未编辑段 decision_process 被清空: got %q, want 决策D", ps.DecisionProcess)
	}
}

func TestMergeConfigIntoPromptSectionsExplicitClear(t *testing.T) {
	dst := dto.StrategyConfig{
		PromptSections: &dto.PromptSections{
			RoleDefinition:   "角色A",
			TradingFrequency: "频率B",
			EntryStandards:   "进场C",
			DecisionProcess:  "决策D",
		},
	}
	// 显式传空字符串 = 清空该段（指针区分"未提及"vs"清空"）
	raw := json.RawMessage(`{"prompt_sections": {"entry_standards": ""}}`)
	if err := MergeConfigInto(&dst, raw); err != nil {
		t.Fatal(err)
	}
	ps := dst.PromptSections
	if ps.RoleDefinition != "角色A" {
		t.Errorf("role_definition 不应变, got %q", ps.RoleDefinition)
	}
	if ps.EntryStandards != "" {
		t.Errorf("entry_standards 应被清空, got %q", ps.EntryStandards)
	}
	if ps.DecisionProcess != "决策D" {
		t.Errorf("decision_process 不应变, got %q", ps.DecisionProcess)
	}
}

func TestMergeConfigIntoPromptSectionsFromNil(t *testing.T) {
	// 存量无 prompt_sections（全新策略/DefaultConfig），前端第一次编辑一段
	dst := dto.StrategyConfig{}
	raw := json.RawMessage(`{"prompt_sections": {"trading_frequency": "频率X"}}`)
	if err := MergeConfigInto(&dst, raw); err != nil {
		t.Fatal(err)
	}
	ps := dst.PromptSections
	if ps == nil {
		t.Fatal("prompt_sections 应被创建")
	}
	if ps.TradingFrequency != "频率X" {
		t.Errorf("trading_frequency 应写入, got %q", ps.TradingFrequency)
	}
	if strings.TrimSpace(ps.RoleDefinition) != "" {
		t.Errorf("未提及段 role_definition 应为空, got %q", ps.RoleDefinition)
	}
}
