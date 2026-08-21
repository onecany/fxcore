// Strategies 页面：策略工作室（侧栏 + 毛玻璃 toolbar + 三栏编辑 + 底部 AI 试跑/提示词预览）。
// 编辑三栏：左交易风格/风控 → 中用户提示词 → 右币源/K 线/指标；数据层 SWR + 5s 轮询。
// 逻辑契约：draft 状态机、MergeConfigInto 部分合并、prompt_sections 逐字段合并（详见 editor-panels）。
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import useSWR from 'swr';
import * as strategyApi from '../api/v1/modules/strategies';
import * as modelApi from '../api/v1/modules/models';
import type { StrategyItem, CoinSourceConfig, AIModel, TestRunResult, KlineConfig, IndicatorConfig, LintIssue } from '../api/v1/types/contract';
import { PageHead, Alert, Terminal, Panel } from '../components/ui';
import { ConfigColumn, PromptColumn, MarketColumn } from '../sections/strategies/editor-panels';
import type { CsEditable } from '../sections/strategies/strategy-data';
import { errMsg } from '../utils/errorCodes';

export default function StrategiesPage() {
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [draft, setDraft] = useState<Record<string, unknown> | null>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [preview, setPreview] = useState<string | null>(null);
  const [newName, setNewName] = useState('');
  // AI 试跑状态
  const [testModelId, setTestModelId] = useState('');
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<TestRunResult | null>(null);
  // Prompt Lint 状态:lintIssues（draft 优先的配置检查结果）+ 是否在检查中
  const [lintIssues, setLintIssues] = useState<LintIssue[] | null>(null);
  const [linting, setLinting] = useState(false);
  const [lintError, setLintError] = useState<string | null>(null);

  // AI 模型列表（试跑选模型用）
  const { data: models } = useSWR<AIModel[]>(
    ['/models', 'studio-test'],
    async () => {
      const d = await modelApi.listModels();
      return d;
    },
    { refreshInterval: 30000 },
  );
  const modelList = useMemo(() => models ?? [], [models]);
  // 默认选中第一个模型（试跑必需）
  useEffect(() => {
    if (!testModelId && modelList.length > 0) {
      setTestModelId(modelList[0].id);
    }
  }, [modelList, testModelId]);

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

  // ===== K 线参数（config.indicators.klines）：draft 优先，fallback 后端配置 =====
  const klineCfg = useMemo(() => {
    const d = draft?.indicators as { klines?: Partial<KlineConfig> } | undefined;
    const base = selected?.config?.indicators?.klines;
    return {
      primaryTimeframe: d?.klines?.primaryTimeframe ?? base?.primaryTimeframe ?? '15m',
      primaryCount: d?.klines?.primaryCount ?? base?.primaryCount ?? 200,
      enableMultiTimeframe: d?.klines?.enableMultiTimeframe ?? base?.enableMultiTimeframe ?? true,
      selectedTimeframes: d?.klines?.selectedTimeframes ?? base?.selectedTimeframes ?? ['1m', '5m', '15m', '1h', '4h', '1d'],
    };
  }, [draft, selected]);

  const setKline = useCallback((patch: Partial<KlineConfig>) => {
    setDraft((prev) => {
      // 基础 = 完整现有 indicators（camel 键，提交时 client 自动 toSnake）。
      // 必须带全部指标字段——MergeConfigInto 对 indicators 是整体替换，只写 klines 会清零其它指标。
      const base = (prev?.indicators as Record<string, unknown> | undefined)
        ?? (selected?.config?.indicators as Record<string, unknown> | undefined)
        ?? {};
      const curK = (base.klines as Partial<KlineConfig> | undefined) ?? {};
      return {
        ...(prev ?? {}),
        indicators: { ...base, klines: { ...klineCfg, ...curK, ...patch } },
      };
    });
  }, [klineCfg, selected]);

  const toggleTF = useCallback((tf: string) => {
    const cur = klineCfg.selectedTimeframes.includes(tf)
      ? klineCfg.selectedTimeframes.filter((x) => x !== tf)
      : [...klineCfg.selectedTimeframes, tf];
    setKline({ selectedTimeframes: cur });
  }, [klineCfg, setKline]);

  const klineDirty = draft?.indicators !== undefined;

  // ===== 技术指标开关（EMA/MACD/RSI/BOLL）：config.indicators 顶层字段 =====
  const indicatorCfg = useMemo(() => {
    const d = draft?.indicators as Partial<IndicatorConfig> | undefined;
    const base = selected?.config?.indicators;
    return {
      enableEma: d?.enableEma ?? base?.enableEma ?? false,
      emaPeriods: d?.emaPeriods ?? base?.emaPeriods ?? [7, 25, 99],
      enableMacd: d?.enableMacd ?? base?.enableMacd ?? false,
      enableRsi: d?.enableRsi ?? base?.enableRsi ?? false,
      rsiPeriods: d?.rsiPeriods ?? base?.rsiPeriods ?? [14],
      enableBoll: d?.enableBoll ?? base?.enableBoll ?? false,
      bollPeriods: d?.bollPeriods ?? base?.bollPeriods ?? [20],
    };
  }, [draft, selected]);

  const setIndicator = useCallback((patch: Partial<IndicatorConfig>) => {
    setDraft((prev) => {
      const base = (prev?.indicators as Record<string, unknown> | undefined)
        ?? (selected?.config?.indicators as Record<string, unknown> | undefined)
        ?? {};
      // 强制合并 klines：MergeConfigInto 对 indicators 整体替换有前置条件
      // `Klines.PrimaryTimeframe != ""`——缺 klines 时整个 indicators 更新被静默跳过。
      return {
        ...(prev ?? {}),
        indicators: {
          ...base,
          klines: { ...klineCfg, ...((base.klines as Partial<KlineConfig> | undefined) ?? {}) },
          ...patch,
        },
      };
    });
  }, [klineCfg, selected]);

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

  const applyProfile = (p: { name: string; patch: Record<string, unknown> }) => {
    if (!selected) return;
    setDraft({ ...p.patch });
    setMsg(`「${p.name}」参数已暂存，点击保存生效`);
  };

  // ===== Prompt Lint：选中策略/编辑变化时自动检查（draft 优先，同 test-run 语义） =====
  const runLint = useCallback(async () => {
    if (!selected) return;
    setLinting(true); setLintError(null);
    try {
      const cfg = draft ? (draft as Record<string, unknown>) : selected.config;
      const r = await strategyApi.lintConfig(cfg as never);
      setLintIssues(r.issues ?? []);
    } catch (err) {
      setLintIssues(null);
      setLintError(String(err));
    } finally {
      setLinting(false);
    }
  }, [selected, draft]);

  // 立即检查跟随选中策略 id（runLint 引用会随 draft 变化,用 ref 保持选中触发稳定,
  // 否则编辑时「立即检查」与「防抖检查」双触发,每次击键打两个请求）
  const runLintRef = useRef(runLint);
  useEffect(() => {
    runLintRef.current = runLint;
  }, [runLint]);
  useEffect(() => {
    if (!selected) { setLintIssues(null); return; }
    void runLintRef.current();
  }, [selectedId]); // eslint-disable-line react-hooks/exhaustive-deps

  // 编辑 draft 时防抖检查（400ms，避免每次击键都打接口）
  useEffect(() => {
    if (!selected || !draft) return;
    const t = setTimeout(() => void runLint(), 400);
    return () => clearTimeout(t);
  }, [draft, selected, runLint]);

  // 选中切换清空 lint 残留（draft 为空时列表切换后 lint 结果即对上）
  useEffect(() => {
    setLintIssues(null);
    setLintError(null);
  }, [selectedId]);

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
      // 跳转到提示词阅览区（等渲染后平滑滚动）
      setTimeout(() => {
        document.getElementById('prompt-preview')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
      }, 80);
    } catch (err) { setError(String(err)); }
  };

  const runTest = async () => {
    if (!selected || !testModelId) return;
    setError(null);
    setTesting(true);
    setTestResult(null);
    try {
      // 测试用 draft 优先的 config（未保存的编辑也生效）；无 draft 用已保存 config
      const cfg = (draft ?? null) ? (draft as Record<string, unknown>) : selected.config;
      const r = await strategyApi.testRun({ config: cfg as never, modelId: testModelId });
      setTestResult(r);
    } catch (err) { setError(String(err)); } finally { setTesting(false); }
  };

  return (
    <section>
      <PageHead
        title="策略内核"
        lead={<>策略工作室 · 双路径提示词 · 共享段 <code>writeCommonDiscipline</code></>}
      />
      {error && <Alert kind="error">{errMsg(error)}</Alert>}
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
              onClick={() => { setSelectedId(s.id); setPreview(null); setDraft(null); }}
            >
              <span className="sb-name">{s.name}</span>
              <span className="sb-meta">
                {s.isActive ? '● 生效中' : '○ 未激活'} · {s.config?.strategyType ?? 'ai'}
              </span>
            </button>
          ))}
          {items.length === 0 && (
            <div className="muted mono sb-empty">// NO STRATEGIES</div>
          )}
          <div className="mt">
            <form onSubmit={(e) => void create(e)} className="row">
              <input
                className="sb-name sb-new"
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
            <div className="muted mono studio-empty">// SELECT OR CREATE A STRATEGY</div>
          ) : (
            <>
              {/* 毛玻璃操作条：策略名 + 状态 | 预览/激活/保存/删除 */}
              <div className="studio-toolbar glass-card">
                <div className="st-info">
                  <div className="st-name">{selected.name}</div>
                  <div className="muted st-meta">
                    {selected.isActive ? '● 全局生效中（唯一激活）' : '○ 未激活'} · ID {selected.id.slice(0, 10)}
                  </div>
                </div>
                <div className="st-actions">
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

              {/* ===== Prompt Lint 金色警告面板 ===== */}
              <div className={`lint-panel ${linting ? 'linting' : ''}`} data-issues={lintIssues?.length ?? 0}>
                {linting ? (
                  <span className="lint-head mono">⌛ 配置检查中…</span>
                ) : lintError ? (
                  <span className="lint-empty dim">◇ 配置检查失败：{errMsg(lintError)}</span>
                ) : !lintIssues || lintIssues.length === 0 ? (
                  <span className="lint-empty">
                    <span className="lint-pass">✓</span> 配置无冲突，另注意此检查基于当前编辑内容（未保存的修改同样生效）
                  </span>
                ) : (
                  <>
                    <div className="lint-head">
                      <span className="lint-gold">◉ {lintIssues.length} 个配置提醒</span>
                      <span className="lint-sub dim">
                        {lintIssues.filter((i) => i.severity === 'error').length} 严重 ·{' '}
                        {lintIssues.filter((i) => i.severity === 'warning').length} 建议
                      </span>
                    </div>
                    <ul className="lint-list">
                      {lintIssues.map((it, i) => (
                        <li key={i} className={`lint-item lint-${it.severity}`}>
                          <span className="lint-ico">{it.severity === 'error' ? '✕' : '⚠'}</span>
                          <div className="lint-body">
                            <div className="lint-title">
                              {it.title} <span className="lint-code dim mono">{it.code} · {it.field}</span>
                            </div>
                            <div className="lint-detail">{it.detail}</div>
                          </div>
                        </li>
                      ))}
                    </ul>
                  </>
                )}
              </div>

              {/* 编辑两栏：左风格/风控/币源 → 右提示词/K线/指标 */}
              <div className="studio-editor-grid">
                <div className="studio-col-config">
                  <ConfigColumn
                    selected={selected}
                    activeProfile={activeProfile}
                    draft={draft}
                    onApplyProfile={applyProfile}
                    cs={cs}
                    setCs={setCs}
                    csDirty={csDirty}
                  />
                </div>
                <div className="studio-col-prompt">
                  <PromptColumn draft={draft} selected={selected} setDraft={setDraft} />
                  <MarketColumn
                    klineCfg={klineCfg}
                    setKline={setKline}
                    toggleTF={toggleTF}
                    klineDirty={klineDirty}
                    indicatorCfg={indicatorCfg}
                    setIndicator={setIndicator}
                  />
                </div>
              </div>

              {/* AI 试跑与提示词预览（原提示词 tab 合并于此） */}
              <Panel title="AI 试跑与提示词预览" className="mb">
                <div className="test-bar">
                  <div className="field test-model">
                    <label>测试模型</label>
                    <select
                      value={testModelId}
                      onChange={(e) => setTestModelId(e.target.value)}
                      disabled={modelList.length === 0}
                    >
                      {modelList.length === 0 && <option value="">（无模型，请先到 ⬡ 模型 页创建）</option>}
                      {modelList.map((m) => (
                        <option key={m.id} value={m.id}>{m.name} · {m.provider}</option>
                      ))}
                    </select>
                  </div>
                  <button className="btn primary" onClick={() => void runTest()} disabled={!testModelId || testing || !selected}>
                    {testing ? '测试中…' : '⚡ AI 测试'}
                  </button>
                  {draft && (
                    <span className="dim test-draft-hint">● 使用未保存的编辑配置测试</span>
                  )}
                </div>

                {testResult && (
                  <div className="test-result">
                    {testResult.parsed ? (
                      <Alert kind="ok">✓ 解析成功 · {(testResult.decisions ?? []).length} 条决策 · {testResult.latencyMs}ms</Alert>
                    ) : (
                      <Alert kind="warn">⚠ 解析失败：{testResult.error ?? 'AI 输出无法解析为六值决策'}（原始输出见下，可检查提示词约束）</Alert>
                    )}
                    {(testResult.decisions ?? []).length > 0 && (
                      <div className="table-wrap test-table">
                        <table>
                          <thead>
                            <tr>
                              <th>动作</th><th>币种</th><th>数量</th><th>杠杆</th><th>止损</th><th>止盈</th><th>置信度</th>
                            </tr>
                          </thead>
                          <tbody>
                            {(testResult.decisions ?? []).map((d, i) => (
                              <tr key={i}>
                                <td className="mono">{d.action}</td>
                                <td>{d.symbol}</td>
                                <td className="mono">{d.quantity ?? '-'}</td>
                                <td className="mono">{d.leverage ? `${d.leverage}x` : '-'}</td>
                                <td className="mono">{d.stopLoss ?? '-'}</td>
                                <td className="mono">{d.takeProfit ?? '-'}</td>
                                <td className="mono">{d.confidence != null ? `${d.confidence}%` : '-'}</td>
                              </tr>
                            ))}
                          </tbody>
                        </table>
                      </div>
                    )}
                    <div className="mt">
                      <Terminal title={`AI RAW OUTPUT · ${testResult.latencyMs}ms`}>
                        {testResult.raw}
                      </Terminal>
                    </div>
                    <details className="test-prompt-details">
                      <summary className="dim mono">查看构建的系统提示词（{testResult.prompt.length} chars）</summary>
                      <div className="mt">
                        <Terminal title="SYSTEM PROMPT · TEST">{testResult.prompt}</Terminal>
                      </div>
                    </details>
                  </div>
                )}

                {preview ? (
                  <div id="prompt-preview" className="prompt-preview-target">
                    <Terminal title={`SYSTEM PROMPT · ${selected.name.toUpperCase().slice(0, 16)} · PREVIEW`}>
                      {preview}
                    </Terminal>
                  </div>
                ) : (
                  <div className="muted mono preview-hint">// 点击「⌨ 预览提示词」查看当前策略系统提示词</div>
                )}
              </Panel>
            </>
          )}
        </main>
      </div>
    </section>
  );
}
