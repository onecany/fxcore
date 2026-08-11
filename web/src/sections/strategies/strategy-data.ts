// 策略工作室常量与纯函数：交易风格档位 / 币源类型 / 币种转换。
import type { CoinSourceConfig, CoinSourceType } from '../../api/v1/types/contract';

export type Profile = 'balanced' | 'aggressive' | 'conservative' | 'scalping';

export interface ProfileDef {
  value: Profile;
  name: string;
  note: string;
  params: { k: string; v: string }[];
  patch: Record<string, unknown>;
}

export const PROFILES: ProfileDef[] = [
  {
    value: 'balanced',
    name: '均衡',
    note: '机会与风险的推荐平衡（默认）',
    params: [
      { k: 'MAX_POS', v: '3' },
      { k: 'LEV BTC/ETH', v: '5x' },
      { k: 'LEV ALT', v: '5x' },
      { k: 'CONF', v: '60%' },
      { k: 'TF', v: '15m' },
      { k: 'RR', v: '1.5' },
    ],
    patch: {
      prompt_variant: 'balanced',
      risk_control: {
        max_positions: 3,
        btc_eth_max_leverage: 5,
        altcoin_max_leverage: 5,
        btc_eth_max_position_value_ratio: 5,
        altcoin_max_position_value_ratio: 1,
        max_margin_usage: 0.3,
        min_position_size: 12,
        min_confidence: 0.6,
        min_risk_reward_ratio: 1.5,
      },
    },
  },
  {
    value: 'aggressive',
    name: '激进',
    note: '更高杠杆与更低置信门槛',
    params: [
      { k: 'MAX_POS', v: '3' },
      { k: 'LEV BTC/ETH', v: '10x' },
      { k: 'LEV ALT', v: '10x' },
      { k: 'CONF', v: '45%' },
      { k: 'TF', v: '15m' },
      { k: 'RR', v: '1.2' },
    ],
    patch: {
      prompt_variant: 'aggressive',
      risk_control: {
        max_positions: 3,
        btc_eth_max_leverage: 10,
        altcoin_max_leverage: 10,
        btc_eth_max_position_value_ratio: 8,
        altcoin_max_position_value_ratio: 2,
        max_margin_usage: 0.5,
        min_position_size: 12,
        min_confidence: 0.45,
        min_risk_reward_ratio: 1.2,
      },
    },
  },
  {
    value: 'conservative',
    name: '稳健',
    note: '少交易，仅对齐信号',
    params: [
      { k: 'MAX_POS', v: '1' },
      { k: 'LEV BTC/ETH', v: '3x' },
      { k: 'LEV ALT', v: '3x' },
      { k: 'CONF', v: '75%' },
      { k: 'TF', v: '1h' },
      { k: 'RR', v: '2.0' },
    ],
    patch: {
      prompt_variant: 'conservative',
      risk_control: {
        max_positions: 1,
        btc_eth_max_leverage: 3,
        altcoin_max_leverage: 3,
        btc_eth_max_position_value_ratio: 3,
        altcoin_max_position_value_ratio: 1,
        max_margin_usage: 0.2,
        min_position_size: 12,
        min_confidence: 0.75,
        min_risk_reward_ratio: 2.0,
      },
    },
  },
  {
    value: 'scalping',
    name: '剥头皮',
    note: '快节奏短周期，紧密止损',
    params: [
      { k: 'MAX_POS', v: '3' },
      { k: 'LEV BTC/ETH', v: '5x' },
      { k: 'LEV ALT', v: '5x' },
      { k: 'CONF', v: '50%' },
      { k: 'TF', v: '1m' },
      { k: 'RR', v: '1.0' },
    ],
    patch: {
      prompt_variant: 'scalping',
      risk_control: {
        max_positions: 3,
        btc_eth_max_leverage: 5,
        altcoin_max_leverage: 5,
        btc_eth_max_position_value_ratio: 5,
        altcoin_max_position_value_ratio: 1,
        max_margin_usage: 0.3,
        min_position_size: 12,
        min_confidence: 0.5,
        min_risk_reward_ratio: 1.0,
      },
    },
  },
];

/** 币源类型选项（对齐后端 dto.CoinSourceType；引擎实际消费 staticCoins + sourceType；ai500 已下线不提供） */
export const COIN_SOURCE_TYPES: { value: CoinSourceType; name: string; note: string }[] = [
  { value: 'static', name: 'static · 固定币种', note: '引擎实际消费（候选币 = staticCoins）' },
  { value: 'oi_top', name: 'oi_top · 持仓量 TOP', note: '高持仓量标的' },
  { value: 'oi_low', name: 'oi_low · 持仓量 LOW', note: '低持仓量标的' },
  { value: 'mixed', name: 'mixed · 混合', note: '多源混合' },
];

/** 逗号分隔字符串 ↔ 币种数组 */
export function coinsToStr(arr: string[] | undefined): string {
  return (arr ?? []).join(', ');
}
export function strToCoins(s: string): string[] {
  return s.split(',').map((c) => c.trim().toUpperCase()).filter(Boolean);
}

/** snake_case → camelCase（prompt_sections 前端字段名） */
export function camelKey(k: string): string {
  return k.replace(/_([a-z])/g, (_, ch: string) => ch.toUpperCase());
}

/** 编辑视图可编辑的币源字段（含 staticCoins/excludedCoins 数组展开） */
export type CsEditable = CoinSourceConfig & { staticCoins: string[]; excludedCoins: string[] };
