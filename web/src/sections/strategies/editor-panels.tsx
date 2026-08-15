// 策略工作室编辑三栏面板（纯展示子组件）：
// ConfigColumn（交易风格 + 风控摘要）/ PromptColumn（提示词四段 + 补充指令）/ MarketColumn（币源 + K 线 + 指标）。
// 所有编辑回调直接透传父级 setter（setDraft/setCs/setKline/setIndicator），逻辑与 draft 状态机零改动。
import type { Dispatch, SetStateAction } from 'react';
import type { StrategyItem, CoinSourceConfig, KlineConfig, IndicatorConfig, CoinSourceType } from '../../api/v1/types/contract';
import { PROFILES, COIN_SOURCE_TYPES, coinsToStr, strToCoins, camelKey, type ProfileDef, type CsEditable } from './strategy-data';

// K 线周期档位（与后端 dto.KlineConfig 对齐，DefaultConfig 默认 6 档全选）
const TF_OPTIONS = ['1m', '5m', '15m', '1h', '4h', '1d'] as const;

const SECTIONS = [
  { key: 'role_definition', label: '角色定义', ph: '# 例如：你是一位只交易 BTC/ETH 永续的日内动量交易员，风格果断、严守纪律……' },
  { key: 'trading_frequency', label: '交易频率', ph: '# 例如：仅在你格突破或回踩确认时出手，单日最多开仓 3 次……' },
  { key: 'entry_standards', label: '进场标准', ph: '# 例如：只进场 4h 周期放量突破且 RSI<70 的标的……' },
  { key: 'decision_process', label: '决策流程', ph: '# 例如：先看 4h 趋势方向，再看 15m 入场点，最后算风险回报比……' },
] as const;

type Draft = Record<string, unknown> | null;

/** ===== 左栏：交易风格 4 档位卡 + 风控摘要 + 币源 ===== */
export function ConfigColumn({ selected, activeProfile, draft, onApplyProfile, cs, setCs, csDirty }: {
  selected: StrategyItem;
  activeProfile: string;
  draft: Draft;
  onApplyProfile: (p: ProfileDef) => void;
  cs: CsEditable;
  setCs: (patch: Partial<CoinSourceConfig>) => void;
  csDirty: boolean;
}) {
  return (
    <>
      <section className="studio-section">
        <div className="studio-section-title">▸ 交易风格</div>
        <div className="style-cards">
          {PROFILES.map((p) => (
            <button
              key={p.value}
              className={`style-card ${(!draft && activeProfile === p.value) || (draft && draft.prompt_variant === p.value) ? 'active' : ''}`}
              onClick={() => onApplyProfile(p)}
            >
              <div className="sc-name">{p.name}</div>
              <div className="sc-note">{p.note}</div>
              <div className="sc-params">
                {p.params.map((pr) => (
                  <span key={pr.k} className="p">{pr.k} <b>{pr.v}</b></span>
                ))}
              </div>
            </button>
          ))}
        </div>
      </section>

      <section className="studio-section">
        <div className="studio-section-title">▸ 风控参数</div>
        <div className="risk-grid">
          <RiskCell k="MAX_POS" v={String(selected.config?.riskControl?.maxPositions ?? 3)} />
          <RiskCell k="LEV BTC/ETH" v={`${selected.config?.riskControl?.btcEthMaxLeverage ?? 5}x`} />
          <RiskCell k="LEV ALT" v={`${selected.config?.riskControl?.altcoinMaxLeverage ?? 5}x`} />
          <RiskCell k="MARGIN" v={`${Math.round((selected.config?.riskControl?.maxMarginUsage ?? 0.3) * 100)}%`} />
          <RiskCell k="MIN POS" v={`${selected.config?.riskControl?.minPositionSize ?? 12} U`} />
          <RiskCell k="CONF" v={`${Math.round((selected.config?.riskControl?.minConfidence ?? 0.6) * 100)}%`} />
        </div>
      </section>

      {/* 币源（Coin Source） */}
      <section className="studio-section">
        <div className="studio-section-title">
          <span>▸ 币源（Coin Source）</span>
          {csDirty && <span className="dirty-tag">● 已编辑，未保存</span>}
        </div>
        <div className="form-grid">
          <div className="field">
            <label>类型</label>
            <select value={cs.sourceType} onChange={(e) => setCs({ sourceType: e.target.value as CoinSourceType })}>
              {!COIN_SOURCE_TYPES.some((t) => t.value === cs.sourceType) && (
                <option value={cs.sourceType} disabled>{cs.sourceType}（已下线，请改选其他类型）</option>
              )}
              {COIN_SOURCE_TYPES.map((t) => <option key={t.value} value={t.value}>{t.name}</option>)}
            </select>
          </div>
          {(cs.sourceType === 'static' || cs.sourceType === 'mixed') && (
            <>
              <div className="field field-full">
                <label>静态币种（逗号分隔）</label>
                <input
                  value={coinsToStr(cs.staticCoins)}
                  onChange={(e) => setCs({ staticCoins: strToCoins(e.target.value) })}
                  placeholder="BTC-USDT, ETH-USDT"
                />
              </div>
              <div className="field field-full">
                <label>排除币种（逗号分隔，可选）</label>
                <input
                  value={coinsToStr(cs.excludedCoins)}
                  onChange={(e) => setCs({ excludedCoins: strToCoins(e.target.value) })}
                  placeholder="DOGE-USDT…"
                />
              </div>
            </>
          )}
          {cs.sourceType === 'ai500' && (
            <div className="field field-full">
              <label>AI 500 候选</label>
              <div className="row row-gap">
                <label className="checkbox-row">
                  <input type="checkbox" checked={cs.useAi500} onChange={(e) => setCs({ useAi500: e.target.checked })} /> 启用
                </label>
                <label className="dim mono">
                  上限
                  <input type="number" min={1} className="num-sm" value={cs.ai500Limit ?? 100} onChange={(e) => setCs({ ai500Limit: Number(e.target.value) || undefined })} />
                </label>
              </div>
            </div>
          )}
          {cs.sourceType === 'oi_top' && (
            <div className="field field-full">
              <label>持仓量 TOP</label>
              <div className="row row-gap">
                <label className="checkbox-row">
                  <input type="checkbox" checked={cs.useOiTop} onChange={(e) => setCs({ useOiTop: e.target.checked })} /> 启用
                </label>
                <label className="dim mono">
                  上限
                  <input type="number" min={1} className="num-sm" value={cs.oiTopLimit ?? 20} onChange={(e) => setCs({ oiTopLimit: Number(e.target.value) || undefined })} />
                </label>
              </div>
            </div>
          )}
          {cs.sourceType === 'oi_low' && (
            <div className="field field-full">
              <label>持仓量 LOW</label>
              <div className="row row-gap">
                <label className="checkbox-row">
                  <input type="checkbox" checked={cs.useOiLow} onChange={(e) => setCs({ useOiLow: e.target.checked })} /> 启用
                </label>
                <label className="dim mono">
                  上限
                  <input type="number" min={1} className="num-sm" value={cs.oiLowLimit ?? 20} onChange={(e) => setCs({ oiLowLimit: Number(e.target.value) || undefined })} />
                </label>
              </div>
            </div>
          )}
        </div>
        {cs.staticCoins.length > 0 && (
          <div className="chip-row chips-top">
            {cs.staticCoins.map((c) => <span key={c} className="chip">{c}</span>)}
          </div>
        )}
        <div className="dim src-note">
          {COIN_SOURCE_TYPES.find((t) => t.value === cs.sourceType)?.note}
        </div>
      </section>
    </>
  );
}

/** ===== 中栏：用户提示词四段 + 补充指令（主编辑区） ===== */
export function PromptColumn({ draft, selected, setDraft }: {
  draft: Draft;
  selected: StrategyItem;
  setDraft: Dispatch<SetStateAction<Draft>>;
}) {
  const customValue = (draft?.custom_prompt as string | undefined) ?? selected.config?.customPrompt ?? '';
  const customDirty = draft?.custom_prompt !== undefined && (draft.custom_prompt as string) !== (selected.config?.customPrompt ?? '');

  return (
    <section className="studio-section">
      <div className="studio-section-title">▸ 用户提示词（custom_prompt · prompt_sections）</div>
      <div className="dim prompt-desc">
        语言契约：用户段 VERBATIM 透传进系统提示词，内置段保持英文，用户段不过滤。留空则不出现在提示词中。
      </div>
      {SECTIONS.map((f) => (
        <div key={f.key} className="prompt-sec">
          <div className="prompt-sec-title">▶ {f.label}（{f.key}）</div>
          <textarea
            className="prompt-area"
            rows={4}
            placeholder={f.ph}
            value={(draft?.prompt_sections as Record<string, unknown> | undefined)?.[f.key] as string | undefined
              ?? (selected.config?.promptSections as Record<string, string> | undefined)?.[camelKey(f.key)] ?? ''}
            onChange={(e) => setDraft((prev) => {
              // 基础 = draft 内已编辑段 ∪ 已保存存量（camel 键转 snake 回填）。
              // 保证 draft.prompt_sections 始终带全四段：MergeConfigInto 对
              // prompt_sections 逐字段合并，但前端仍发全量（与 setKline 同模式，
              // 防御未来后端改回整体替换导致未编辑段被清空）。
              const base = { ...((prev?.prompt_sections as Record<string, unknown> | undefined) ?? {}) };
              const stored = selected.config?.promptSections as Record<string, string> | undefined;
              if (stored) {
                for (const [k, v] of Object.entries(stored)) {
                  const snake = k.replace(/[A-Z]/g, (ch) => `_${ch.toLowerCase()}`);
                  if (!(snake in base)) base[snake] = v;
                }
              }
              return {
                ...(prev ?? {}),
                prompt_sections: { ...base, [f.key]: e.target.value },
              };
            })}
          />
        </div>
      ))}
      <div className="prompt-sec-title prompt-sec-gap">▶ 补充指令（custom_prompt）</div>
      <textarea
        className="prompt-area"
        rows={4}
        placeholder="# 例如：优先选择 4h 周期突破形态的标的，避开消息面驱动的暴涨暴跌……"
        value={customValue}
        onChange={(e) => setDraft((prev) => ({ ...(prev ?? {}), custom_prompt: e.target.value }))}
        onFocus={(e) => { if (e.target.value === (selected.config?.customPrompt ?? '')) setDraft((prev) => ({ ...(prev ?? {}), custom_prompt: e.target.value })); }}
      />
      <div className="prompt-foot">
        <span className="mono dim">{customValue.length} chars · 中文/英文均可</span>
        <span className="dim">
          {customDirty
            ? <span className="dirty-tag">● 已编辑，未保存</span>
            : <span className="muted">未修改</span>}
        </span>
      </div>
    </section>
  );
}

/** ===== 右栏：K 线参数 + 技术指标 ===== */
export function MarketColumn({ klineCfg, setKline, toggleTF, klineDirty, indicatorCfg, setIndicator }: {
  klineCfg: Partial<KlineConfig> & { primaryTimeframe: string; primaryCount: number; enableMultiTimeframe: boolean; selectedTimeframes: string[] };
  setKline: (patch: Partial<KlineConfig>) => void;
  toggleTF: (tf: string) => void;
  klineDirty: boolean;
  indicatorCfg: Partial<IndicatorConfig> & { enableEma: boolean; emaPeriods: number[]; enableMacd: boolean; enableRsi: boolean; rsiPeriods: number[]; enableBoll: boolean; bollPeriods: number[] };
  setIndicator: (patch: Partial<IndicatorConfig>) => void;
}) {
  return (
    <div className="kline-ind-grid">
      {/* K 线参数 */}
      <section className="studio-section">
        <div className="studio-section-title">▸ K 线参数</div>
        <div className="field">
          <label>主周期 · K 线数量</label>
          <div className="row row-gap">
            <select
              value={klineCfg.primaryTimeframe}
              onChange={(e) => setKline({ primaryTimeframe: e.target.value })}
              className="grow"
            >
              {TF_OPTIONS.map((tf) => <option key={tf} value={tf}>{tf}</option>)}
            </select>
            <input
              type="number"
              min={20}
              max={1000}
              step={10}
              value={klineCfg.primaryCount}
              onChange={(e) => setKline({ primaryCount: Number(e.target.value) || 200 })}
              className="num-md"
            />
          </div>
        </div>
        <div className="field">
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={klineCfg.enableMultiTimeframe}
              onChange={(e) => setKline({ enableMultiTimeframe: e.target.checked })}
            /> 多时间框架分析
          </label>
        </div>
        {klineCfg.enableMultiTimeframe && (
          <div className="chip-row chips-top">
            {TF_OPTIONS.map((tf) => (
              <button
                key={tf}
                type="button"
                className={`chip ${klineCfg.selectedTimeframes.includes(tf) ? 'on' : ''}`}
                onClick={() => toggleTF(tf)}
              >
                {tf}
              </button>
            ))}
          </div>
        )}
        <div className="prompt-foot">
          <span className="mono dim">
            PRIMARY {klineCfg.primaryTimeframe} · {klineCfg.primaryCount} BARS
            {klineCfg.enableMultiTimeframe && klineCfg.selectedTimeframes.length > 0
              ? ` · MTF ${klineCfg.selectedTimeframes.join('/')}` : ''}
          </span>
          {klineDirty && <span className="dirty-tag">● 已编辑，未保存</span>}
        </div>
      </section>

      {/* 技术指标 */}
      <section className="studio-section">
        <div className="studio-section-title">▸ 技术指标（EMA / MACD / RSI / BOLL）</div>
        <div className="field">
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={indicatorCfg.enableEma}
              onChange={(e) => setIndicator({ enableEma: e.target.checked })}
            /> EMA 指数均线
          </label>
          {indicatorCfg.enableEma && (
            <div className="row row-gap ind-row">
              <input
                type="text"
                value={indicatorCfg.emaPeriods.join(',')}
                placeholder="7,25,99"
                onChange={(e) => {
                  const periods = e.target.value.split(',').map((s) => Number(s.trim())).filter((n) => Number.isFinite(n) && n > 0);
                  setIndicator({ emaPeriods: periods });
                }}
                className="num-md"
              />
              <span className="dim mono">周期（逗号分隔）</span>
            </div>
          )}
        </div>
        <div className="field">
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={indicatorCfg.enableMacd}
              onChange={(e) => setIndicator({ enableMacd: e.target.checked })}
            /> MACD 指标
          </label>
        </div>
        <div className="field">
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={indicatorCfg.enableRsi}
              onChange={(e) => setIndicator({ enableRsi: e.target.checked })}
            /> RSI 相对强弱
          </label>
          {indicatorCfg.enableRsi && (
            <div className="row row-gap ind-row">
              <input
                type="text"
                value={indicatorCfg.rsiPeriods.join(',')}
                placeholder="14"
                onChange={(e) => {
                  const periods = e.target.value.split(',').map((s) => Number(s.trim())).filter((n) => Number.isFinite(n) && n > 0);
                  setIndicator({ rsiPeriods: periods });
                }}
                className="num-md"
              />
              <span className="dim mono">周期（逗号分隔）</span>
            </div>
          )}
        </div>
        <div className="field">
          <label className="checkbox-row">
            <input
              type="checkbox"
              checked={indicatorCfg.enableBoll}
              onChange={(e) => setIndicator({ enableBoll: e.target.checked })}
            /> BOLL 布林带
          </label>
          {indicatorCfg.enableBoll && (
            <div className="row row-gap ind-row">
              <input
                type="text"
                value={indicatorCfg.bollPeriods.join(',')}
                placeholder="20"
                onChange={(e) => {
                  const periods = e.target.value.split(',').map((s) => Number(s.trim())).filter((n) => Number.isFinite(n) && n > 0);
                  setIndicator({ bollPeriods: periods });
                }}
                className="num-md"
              />
              <span className="dim mono">周期（逗号分隔）</span>
            </div>
          )}
        </div>
      </section>
    </div>
  );
}

function RiskCell({ k, v }: { k: string; v: string }) {
  return (
    <div className="risk-cell">
      <div className="risk-cell-label">{k}</div>
      <div className="risk-cell-value">{v}</div>
    </div>
  );
}
