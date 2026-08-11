// Strategies 页面：策略工作室（顶栏 + 280px 侧栏 + 二级 tab）。
// 编辑 = 风格档位（§9.5 四种模式）→ 保存写 risk_control/prompt_variant（MergeConfigInto 部分合并）。
// 提示词 tab = preview-prompt 终端窗。数据层 SWR + 5s 轮询。
import { useCallback, useEffect, useMemo, useState } from 'react';
import useSWR from 'swr';
import * as strategyApi from '../api/v1/modules/strategies';
import type { StrategyItem, CoinSourceConfig, CoinSourceType } from '../api/v1/types/contract';
import { PageHead, Alert, Terminal } from '../components/ui';
import { PROFILES, COIN_SOURCE_TYPES, coinsToStr, strToCoins, camelKey, type ProfileDef, type CsEditable } from '../sections/strategies/strategy-data';

export default function StrategiesPage() {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [subTab, setSubTab] = useState<'editor' | 'prompt'>('editor');
  const [draft, setDraft] = useState<Record<string, unknown> | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const [newName, setNewName] = useState('');

  // 策略列表：SWR + 5s 轮询（激活状态自动刷新）
  const { data, mutate } = useSWR<StrategyItem[]>(
    ['/strategies', 'studio'],
    async () => {
      const d = await strategyApi.listStrategies();
      return d.items;
    },
    { refreshInterval: 5000 },
  );
  const items = useMemo(() => data ?? [], [data]);

  // 列表加载后确保有选中项
  useEffect(() => {
    if (!items.some((s) => s.id === selectedId)) {
      setSelectedId(items[0]?.id ?? null);
    }
  }, [items, selectedId]);

  const selected = useMemo(() => items.find((s) => s.id === selectedId) ?? null, [items, selectedId]);

  const activeProfile = useMemo(() => {
    const rc = selected?.config?.riskControl;
    if (!rc) return 'balanced';
    const conf = rc.minConfidence ?? 0.6;
    const lev = rc.btcEthMaxLeverage ?? 5;
    if (conf <= 0.48) return 'aggressive';
    if (conf >= 0.7) return 'conservative';
    if (lev <= 3) return 'conservative';
    return 'balanced';
  }, [selected]);

  // ===== 币源（Coin Source）编辑视图：draft 优先，fallback 后端配置 =====
  // draft.coin_source 用 camel 键（与 contract 对齐），提交时 client 自动 toSnake。
  const cs: CsEditable = useMemo(() => {
    const d = draft?.coin_source as CoinSourceConfig | undefined;
    const base = selected?.config?.coinSource;
    return {
      sourceType: d?.sourceType ?? base?.sourceType ?? 'static',
      staticCoins: d?.staticCoins ?? base?.staticCoins ?? [],
      excludedCoins: d?.excludedCoins ?? base?.excludedCoins ?? [],
      useAi500: d?.useAi500 ?? base?.useAi500 ?? false,
      ai500Limit: d?.ai500Limit ?? base?.ai500Limit,
      useOiTop: d?.useOiTop ?? base?.useOiTop ?? false,
      oiTopLimit: d?.oiTopLimit ?? base?.oiTopLimit,
      useOiLow: d?.useOiLow ?? base?.useOiLow ?? false,
      oiLowLimit: d?.oiLowLimit ?? base?.oiLowLimit,
    };
  }, [draft, selected]);

  const setCs = useCallback((patch: Partial<CoinSourceConfig>) => {
    setDraft((prev) => ({ ...(prev ?? {}), coin_source: { ...cs, ...patch } }));
  }, [cs]);

  const csDirty = draft?.coin_source !== undefined;

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newName.trim()) return;
    setMsg(null); setError(null);
    try {
      const s = await strategyApi.createStrategy({ name: newName.trim() });
      setMsg(`策略「${newName.trim()}」已创建（默认风控生效）`);
      setNewName('');
      setSelectedId(s.id);
      void mutate();
    } catch (err) { setError(String(err)); }
  };

  const activate = async (id: string) => {
    try {
      const s = await strategyApi.activateStrategy(id);
      setMsg(`已激活：${s.name}`);
      void mutate();
    } catch (err) { setError(String(err)); }
  };

  const remove = async (id: string) => {
    if (!window.confirm('删除该策略？')) return;
    try { await strategyApi.deleteStrategy(id); void mutate(); } catch (err) { setError(String(err)); }
  };

  const applyProfile = (p: ProfileDef) => {
    if (!selected) return;
    setDraft({ ...p.patch });
    setMsg(`「${p.name}」参数已暂存，点击保存生效`);
  };

  const save = async () => {
    if (!selected || !draft) return;
    setSaving(true); setError(null); setMsg(null);
    try {
      await strategyApi.updateStrategy(selected.id, { config: draft as never });
      setMsg('已保存（MergeConfigInto 部分合并，未提及字段保留）');
      setDraft(null);
      void mutate();
    } catch (err) { setError(String(err)); } finally { setSaving(false); }
  };

  const showPreview = async () => {
    if (!selected) return;
    setError(null);
    try {
      const p = await strategyApi.previewPrompt(selected.config);
      setPreview(p.prompt);
      setSubTab('prompt');
    } catch (err) { setError(String(err)); }
  };

  return (
    <section>
      <PageHead
        title="策略内核"
        lead={<>策略工作室 · 双路径提示词 · 共享段 <code>writeCommonDiscipline</code></>}
      />
      {error && <Alert kind="error">{error}</Alert>}
      {msg && <Alert kind="ok">{msg}</Alert>}

      <div className="studio-grid">
        {/* ===== 左侧：策略列表 ===== */}
        <aside className="studio-sidebar">
          <div className="sb-head">
            <span className="sb-title">Strategy Library</span>
            <span className="sb-count">{items.length}</span>
          </div>
          {items.map((s) => (
            <button
              key={s.id}
              className={`sb-item ${s.id === selectedId ? 'active' : ''}`}
              onClick={() => { setSelectedId(s.id); setSubTab('editor'); setPreview(null); setDraft(null); }}
            >
              <span className="sb-name">{s.name}</span>
              <span className="sb-meta">
                {s.isActive ? '● 生效中' : '○ 未激活'} · {s.config?.strategyType ?? 'ai'}
              </span>
            </button>
          ))}
          {items.length === 0 && (
            <div className="muted mono" style={{ padding: '14px 8px', fontSize: 12 }}>// NO STRATEGIES</div>
          )}
          <div className="mt">
            <form onSubmit={(e) => void create(e)} className="row">
              <input
                style={{ flex: 1, minWidth: 0 }}
                className="sb-name"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                placeholder="新策略名称…"
              />
              <button className="btn primary" type="submit" disabled={!newName.trim()}>+</button>
            </form>
          </div>
        </aside>

        {/* ===== 右侧：主区 ===== */}
        <main className="studio-main">
          {!selected ? (
            <div className="muted mono" style={{ padding: '40px 0', textAlign: 'center' }}>
              // SELECT OR CREATE A STRATEGY
            </div>
          ) : (
            <>
              <div className="row wrap" style={{ justifyContent: 'space-between', marginBottom: 12 }}>
                <div>
                  <div style={{ fontSize: 17, fontWeight: 600, color: 'var(--fxcore-text)', letterSpacing: '0.02em' }}>
                    {selected.name}
                  </div>
                  <div className="muted" style={{ fontSize: 12, marginTop: 2 }}>
                    {selected.isActive ? '● 全局生效中（唯一激活）' : '○ 未激活'} · ID {selected.id.slice(0, 10)}
                  </div>
                </div>
                <div className="row">
                  <button className="btn" onClick={() => void showPreview()} disabled={!selected.config}>
                    ⌨ 预览提示词
                  </button>
                  {!selected.isActive && (
                    <button className="btn primary" onClick={() => void activate(selected.id)}>激活</button>
                  )}
                  <button className="btn" onClick={() => void save()} disabled={!draft || saving}>
                    {saving ? '保存中…' : draft ? '● 保存' : '保存'}
                  </button>
                  <button className="btn danger" onClick={() => void remove(selected.id)}>删除</button>
                </div>
              </div>

              <div className="sub-tabs">
                <button className={`sub-tab ${subTab === 'editor' ? 'active' : ''}`} onClick={() => setSubTab('editor')}>
                  ✦ 编辑
                </button>
                <button className={`sub-tab ${subTab === 'prompt' ? 'active' : ''}`} onClick={() => { setSubTab('prompt'); if (!preview) void showPreview(); }}>
                  ⌨ 提示词
                </button>
              </div>

              {subTab === 'editor' ? (
                <>
                  {/* 交易风格 */}
                  <section style={{ border: '1px solid var(--fxcore-panel-border)', borderRadius: 'var(--fxcore-r-md)', background: 'var(--fxcore-panel)', padding: 14, marginBottom: 12 }}>
                    <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--fxcore-text)', marginBottom: 10, letterSpacing: '0.06em' }}>
                      ▸ 交易风格（§9.5 writeModeVariant）
                    </div>
                    <div className="style-cards">
                      {PROFILES.map((p) => (
                        <button
                          key={p.value}
                          className={`style-card ${(!draft && activeProfile === p.value) || (draft && draft.prompt_variant === p.value) ? 'active' : ''}`}
                          onClick={() => applyProfile(p)}
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

                  {/* 币源（Coin Source）— 可编辑，写入 draft.coin_source */}
                  <section style={{ border: '1px solid var(--fxcore-panel-border)', borderRadius: 'var(--fxcore-r-md)', background: 'var(--fxcore-panel)', padding: 14, marginBottom: 12 }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'baseline', marginBottom: 10 }}>
                      <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--fxcore-text)', letterSpacing: '0.06em' }}>
                        ▸ 币源（Coin Source）
                      </div>
                      {csDirty && <span style={{ fontSize: 11, color: 'var(--fxcore-accent)' }}>● 已编辑，未保存</span>}
                    </div>
                    <div className="form-grid" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))' }}>
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
                          <div className="field" style={{ gridColumn: '1 / -1' }}>
                            <label>静态币种（逗号分隔）</label>
                            <input
                              value={coinsToStr(cs.staticCoins)}
                              onChange={(e) => setCs({ staticCoins: strToCoins(e.target.value) })}
                              placeholder="BTC-USDT, ETH-USDT"
                            />
                          </div>
                          <div className="field" style={{ gridColumn: '1 / -1' }}>
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
                        <div className="field" style={{ gridColumn: '1 / -1' }}>
                          <label>AI 500 候选</label>
                          <div className="row" style={{ gap: 14 }}>
                            <label className="checkbox-row" style={{ gap: 6 }}>
                              <input type="checkbox" checked={cs.useAi500} onChange={(e) => setCs({ useAi500: e.target.checked })} /> 启用
                            </label>
                            <label className="dim mono" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                              上限
                              <input type="number" min={1} style={{ width: 90 }} value={cs.ai500Limit ?? 100} onChange={(e) => setCs({ ai500Limit: Number(e.target.value) || undefined })} />
                            </label>
                          </div>
                        </div>
                      )}
                      {cs.sourceType === 'oi_top' && (
                        <div className="field" style={{ gridColumn: '1 / -1' }}>
                          <label>持仓量 TOP</label>
                          <div className="row" style={{ gap: 14 }}>
                            <label className="checkbox-row" style={{ gap: 6 }}>
                              <input type="checkbox" checked={cs.useOiTop} onChange={(e) => setCs({ useOiTop: e.target.checked })} /> 启用
                            </label>
                            <label className="dim mono" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                              上限
                              <input type="number" min={1} style={{ width: 90 }} value={cs.oiTopLimit ?? 20} onChange={(e) => setCs({ oiTopLimit: Number(e.target.value) || undefined })} />
                            </label>
                          </div>
                        </div>
                      )}
                      {cs.sourceType === 'oi_low' && (
                        <div className="field" style={{ gridColumn: '1 / -1' }}>
                          <label>持仓量 LOW</label>
                          <div className="row" style={{ gap: 14 }}>
                            <label className="checkbox-row" style={{ gap: 6 }}>
                              <input type="checkbox" checked={cs.useOiLow} onChange={(e) => setCs({ useOiLow: e.target.checked })} /> 启用
                            </label>
                            <label className="dim mono" style={{ display: 'flex', alignItems: 'center', gap: 6 }}>
                              上限
                              <input type="number" min={1} style={{ width: 90 }} value={cs.oiLowLimit ?? 20} onChange={(e) => setCs({ oiLowLimit: Number(e.target.value) || undefined })} />
                            </label>
                          </div>
                        </div>
                      )}
                    </div>
                    {cs.staticCoins.length > 0 && (
                      <div className="chip-row" style={{ marginTop: 8 }}>
                        {cs.staticCoins.map((c) => <span key={c} className="chip">{c}</span>)}
                      </div>
                    )}
                    <div className="dim" style={{ fontSize: 11, marginTop: 8 }}>
                      {COIN_SOURCE_TYPES.find((t) => t.value === cs.sourceType)?.note}
                    </div>
                  </section>

                  {/* 风控摘要 */}
                  <section style={{ border: '1px solid var(--fxcore-panel-border)', borderRadius: 'var(--fxcore-r-md)', background: 'var(--fxcore-panel)', padding: 14 }}>
                    <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--fxcore-text)', marginBottom: 10, letterSpacing: '0.06em' }}>
                      ▸ 风控参数（§14.1 强制公式）
                    </div>
                    <div className="stat-grid" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(130px, 1fr))', marginBottom: 0 }}>
                      <RiskCell k="MAX_POS" v={String(selected.config?.riskControl?.maxPositions ?? 3)} />
                      <RiskCell k="LEV BTC/ETH" v={`${selected.config?.riskControl?.btcEthMaxLeverage ?? 5}x`} />
                      <RiskCell k="LEV ALT" v={`${selected.config?.riskControl?.altcoinMaxLeverage ?? 5}x`} />
                      <RiskCell k="MARGIN" v={`${Math.round((selected.config?.riskControl?.maxMarginUsage ?? 0.3) * 100)}%`} />
                      <RiskCell k="MIN POS" v={`${selected.config?.riskControl?.minPositionSize ?? 12} U`} />
                      <RiskCell k="CONF" v={`${Math.round((selected.config?.riskControl?.minConfidence ?? 0.6) * 100)}%`} />
                    </div>
                  </section>
                  {/* 用户提示词（VERBATIM 透传） */}
                  <section style={{ border: '1px solid var(--fxcore-panel-border)', borderRadius: 'var(--fxcore-r-md)', background: 'var(--fxcore-panel)', padding: 14, marginTop: 12 }}>
                    <div style={{ fontSize: 13, fontWeight: 600, color: 'var(--fxcore-text)', marginBottom: 4, letterSpacing: '0.06em' }}>
                      ▸ 用户提示词（custom_prompt · prompt_sections）
                    </div>
                    <div className="dim" style={{ fontSize: 11, marginBottom: 8 }}>
                      语言契约 §9.3：用户段 VERBATIM 透传进系统提示词，内置段保持英文，用户段不过滤。留空则不出现在提示词中。
                    </div>
                    {/* 分段定义（prompt_sections：角色/频率/进场/决策） */}
                    {[
                      { key: 'role_definition', label: '角色定义', ph: '# 例如：你是一位只交易 BTC/ETH 永续的日内动量交易员，风格果断、严守纪律……' },
                      { key: 'trading_frequency', label: '交易频率', ph: '# 例如：仅在你格突破或回踩确认时出手，单日最多开仓 3 次……' },
                      { key: 'entry_standards', label: '进场标准', ph: '# 例如：只进场 4h 周期放量突破且 RSI<70 的标的……' },
                      { key: 'decision_process', label: '决策流程', ph: '# 例如：先看 4h 趋势方向，再看 15m 入场点，最后算风险回报比……' },
                    ].map((f) => (
                      <div key={f.key} style={{ marginBottom: 10 }}>
                        <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--fxcore-text-dim)', marginBottom: 6 }}>
                          ▶ {f.label}（{f.key}）
                        </div>
                        <textarea
                          className="prompt-area"
                          rows={2}
                          placeholder={f.ph}
                          value={(draft?.prompt_sections as Record<string, unknown> | undefined)?.[f.key] as string | undefined
                            ?? (selected.config?.promptSections as Record<string, string> | undefined)?.[camelKey(f.key)] ?? ''}
                          onChange={(e) => setDraft((prev) => ({
                            ...(prev ?? {}),
                            prompt_sections: { ...((prev?.prompt_sections as Record<string, unknown>) ?? {}), [f.key]: e.target.value },
                          }))}
                        />
                      </div>
                    ))}
                    <div style={{ fontSize: 12, fontWeight: 600, color: 'var(--fxcore-text-dim)', margin: '4px 0 6px' }}>
                      ▶ 补充指令（custom_prompt）
                    </div>
                    <textarea
                      className="prompt-area"
                      rows={7}
                      placeholder={"# 例如：优先选择 4h 周期突破形态的标的，避开消息面驱动的暴涨暴跌……"}
                      value={(draft?.custom_prompt as string | undefined) ?? selected.config?.customPrompt ?? ''}
                      onChange={(e) => setDraft((prev) => ({ ...(prev ?? {}), custom_prompt: e.target.value }))}
                      onFocus={(e) => { if (e.target.value === (selected.config?.customPrompt ?? '')) setDraft((prev) => ({ ...(prev ?? {}), custom_prompt: e.target.value })); }}
                    />
                    <div className="row" style={{ justifyContent: 'space-between', marginTop: 6 }}>
                      <span className="mono dim" style={{ fontSize: 11 }}>
                        {(draft?.custom_prompt as string | undefined)?.length ?? (selected.config?.customPrompt ?? '').length} chars · 中文/英文均可
                      </span>
                      <span className="dim" style={{ fontSize: 11 }}>
                        {draft?.custom_prompt !== undefined && (draft.custom_prompt as string) !== (selected.config?.customPrompt ?? '')
                          ? <span style={{ color: 'var(--fxcore-accent)' }}>● 已编辑，未保存</span>
                          : <span className="muted">未修改</span>}
                      </span>
                    </div>
                  </section>
                </>
              ) : (
                <>
                  {preview ? (
                    <Terminal title={`SYSTEM PROMPT · ${selected.name.toUpperCase().slice(0, 16)} · PREVIEW`}>
                      {preview}
                    </Terminal>
                  ) : (
                    <div className="muted mono">// 加载提示词…</div>
                  )}
                </>
              )}
            </>
          )}
        </main>
      </div>
    </section>
  );
}

function RiskCell({ k, v }: { k: string; v: string }) {
  return (
    <div className="stat-card" style={{ padding: '10px 12px' }}>
      <div className="label">{k}</div>
      <div className="value" style={{ fontSize: 16 }}>{v}</div>
    </div>
  );
}
