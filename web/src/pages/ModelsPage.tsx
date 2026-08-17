// 模型管理页：CRUD + 连通测试 + provider/model 联动下拉。
// 数据层 SWR（models + providers 双 key，写操作后 mutate）；列表项毛玻璃卡片化，测试区独立行。
import { useMemo, useState } from 'react';
import useSWR from 'swr';
import * as modelApi from '../api/v1/modules/models';
import type { AIModel } from '../api/v1/types/contract';
import { PageHead, Panel, Badge, Alert } from '../components/ui';
import { errMsg } from '../utils/errorCodes';

const fmtTime = (raw?: string): string => (raw ? new Date(raw).toLocaleString('zh-CN', { hour12: false }) : '-');
const keyMask = (prefix?: string): string => prefix || '未配置';

export default function ModelsPage() {
  // ===== SWR 数据层：模型库 + provider 目录 =====
  const { data: itemsData, mutate, error: loadError } = useSWR<AIModel[]>(
    ['/models', 'page'],
    () => modelApi.listModels(),
  );
  const { data: providersData } = useSWR<modelApi.ProviderOption[]>(
    ['/models/providers', 'page'],
    () => modelApi.listProviders(),
  );
  const items = useMemo(() => itemsData ?? [], [itemsData]);
  const providers = useMemo(() => providersData ?? [], [providersData]);

  const [error, setError] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  // 表单
  const [editingId, setEditingId] = useState<string | null>(null);
  const [name, setName] = useState('');
  const [provider, setProvider] = useState('');
  const [modelName, setModelName] = useState('');
  const [apiKey, setApiKey] = useState('');
  const [temperature, setTemperature] = useState('0.7');
  const [baseUrl, setBaseUrl] = useState(''); // 仅 custom provider 必填（config.base_url）
  // 每个模型的测试状态：testing 进行中 + result 成功/失败，绑定到对应模型卡片显示
  const [testStates, setTestStates] = useState<Record<string, { testing: boolean; success?: boolean; latency?: number; error?: string }>>({});

  const isCustom = provider === 'custom';

  // provider 联动模型下拉
  const modelOptions = useMemo(() => {
    const p = providers.find((x) => x.provider === provider);
    return p?.models ?? [];
  }, [providers, provider]);

  const startCreate = () => {
    setEditingId(null);
    setName(''); setProvider(''); setModelName(''); setApiKey(''); setTemperature('0.7'); setBaseUrl(''); setTestStates({}); setMsg(null); setError(null);
  };

  const startEdit = (m: AIModel) => {
    setEditingId(m.id);
    setName(m.name); setProvider(m.provider); setModelName(m.modelName); setApiKey(''); setTemperature(String(m.config?.temperature ?? 0.7));
    setBaseUrl(typeof m.config?.baseUrl === 'string' ? m.config.baseUrl : '');
    setMsg(null); setError(null);
  };

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!name.trim() || !provider || !modelName.trim()) { setError('请填写别名 / 提供商 / 模型'); return; }
    if (!editingId && !apiKey.trim()) { setError('创建模型必须提供 API Key'); return; }
    // custom provider 必须提供 https base_url（后端 probe 依赖它；config 键经 client toSnake 转 base_url）
    if (isCustom) {
      const b = baseUrl.trim();
      if (!b) { setError('custom 提供商必须填写 Base URL（https://...）'); return; }
      try {
        const u = new URL(b);
        if (u.protocol !== 'https:') { setError('Base URL 必须使用 https 协议'); return; }
      } catch {
        setError('Base URL 格式无效，应为 https://host[:port] 或 https://host[:port]/v1'); return;
      }
    }
    setBusy(true); setError(null); setMsg(null);
    try {
      const config: Record<string, unknown> = { temperature: Number(temperature) || 0.7 };
      if (isCustom) config.baseUrl = baseUrl.trim().replace(/\/+$/, '');
      const req = {
        name: name.trim(), provider: provider as never,
        modelName: modelName.trim(),
        apiKey: apiKey.trim(),
        config,
      };
      if (editingId) {
        await modelApi.updateModel(editingId, req);
        setMsg('模型已更新（未填 Key 则保留原 Key）');
      } else {
        await modelApi.createModel(req);
        setMsg('模型已创建（API Key 已 RSA 加密落库）');
      }
      startCreate();
      void mutate();
    } catch (err) { setError(String(err)); } finally { setBusy(false); }
  };

  const remove = async (m: AIModel) => {
    if (!window.confirm(`删除模型「${m.name}」？引用它的交易员将无法执行。`)) return;
    try {
      await modelApi.deleteModel(m.id);
      if (editingId === m.id) startCreate();
      void mutate();
    } catch (err) { setError(String(err)); }
  };

  const test = async (m: AIModel) => {
    setTestStates((prev) => ({ ...prev, [m.id]: { testing: true } }));
    try {
      const r = await modelApi.testModel(m.id);
      setTestStates((prev) => ({ ...prev, [m.id]: { testing: false, success: r.success, latency: r.latency, error: r.error } }));
    } catch (err) {
      setTestStates((prev) => ({ ...prev, [m.id]: { testing: false, success: false, error: String(err) } }));
    }
  };

  return (
    <section>
      <PageHead title="模型" lead={<>AI 模型配置 · API Key 经 RSA 加密落库，响应零泄漏 · 支持连通测试</>} />
      {loadError && <Alert kind="error">{errMsg(loadError)}</Alert>}
      {error && <Alert kind="error">{errMsg(error)}</Alert>}
      {msg && <Alert kind="ok">{msg}</Alert>}

      <div className="td-grid td-grid-2">
        {/* 左：模型库（卡片化列表） */}
        <Panel title={`模型库 (${items.length})`}>
          {items.length === 0 ? (
            <div className="muted mono model-empty">// NO MODELS（点右侧创建）</div>
          ) : (
            <div className="model-list">
              {items.map((m) => {
                const ts = testStates[m.id];
                return (
                  <div key={m.id} className="model-card" onClick={() => startEdit(m)}>
                    <div className="model-head">
                      <span className="model-name">{m.name}</span>
                      <Badge state={m.status} />
                    </div>
                    <div className="model-meta">
                      <span className="mono dim">{m.provider} / {m.modelName}</span>
                      <span className="mono dim">key {keyMask(m.apiKeyPrefix)}</span>
                    </div>
                    {/* 独立测试区 */}
                    <div className="model-test-row" onClick={(e) => e.stopPropagation()}>
                      <button className="btn ghost btn-sm" onClick={() => void test(m)} disabled={ts?.testing}>⌁ 测试</button>
                      {ts?.testing ? (
                        <span className="model-result testing">测试中…</span>
                      ) : ts?.success != null ? (
                        <span className={`model-result ${ts.success ? 'ok' : 'fail'}`}>
                          {ts.success ? `✓ ${ts.latency ?? '-'}ms` : `✗ ${ts.error ?? '失败'}`}
                        </span>
                      ) : (
                        <span className="model-result">测试 {fmtTime(m.lastTestAt)}</span>
                      )}
                    </div>
                    <div className="model-actions" onClick={(e) => e.stopPropagation()}>
                      <button className="btn ghost btn-sm" onClick={() => startEdit(m)}>编辑</button>
                      <button className="btn danger btn-sm" onClick={() => void remove(m)}>删除</button>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </Panel>

        {/* 右：表单（studio 分层：基础信息 / 连接 / 参数 三卡） */}
        <Panel title={editingId ? '编辑模型' : '新建模型'}>
          <form onSubmit={(e) => void submit(e)} className="model-form">
            {/* 基础信息卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">01</span>基础信息</div>
              <div className="field">
                <label className="form-label">别名（显示名）</label>
                <input value={name} onChange={(e) => setName(e.target.value)} placeholder="如：deepseek 主力" />
              </div>
              <div className="field">
                <label className="form-label">提供商</label>
                <select value={provider} onChange={(e) => { setProvider(e.target.value); setModelName(''); }}>
                  <option value="">— 选择提供商 —</option>
                  {providers.map((p) => <option key={p.provider} value={p.provider}>{p.provider}</option>)}
                </select>
              </div>
              <div className="field">
                <label className="form-label">模型</label>
                {isCustom ? (
                  <input
                    value={modelName}
                    onChange={(e) => setModelName(e.target.value)}
                    placeholder="自定义模型名，如 llama-3.1-8b / gpt-4o-mini"
                  />
                ) : (
                  <select value={modelName} onChange={(e) => setModelName(e.target.value)} disabled={!provider}>
                    <option value="">— 选择模型 —</option>
                    {modelOptions.map((mn) => <option key={mn} value={mn}>{mn}</option>)}
                  </select>
                )}
              </div>
            </div>

            {/* 连接卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">02</span>连接</div>
              <div className="field">
                <label className="form-label">
                  API Key {editingId ? '（留空保留原 Key）' : ''}
                </label>
                <input type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)} placeholder="sk-..." autoComplete="off" />
              </div>
              {isCustom && (
                <div className="field">
                  <label className="form-label">
                    Base URL <span className="form-required">（custom 必填，https）</span>
                  </label>
                  <input
                    value={baseUrl}
                    onChange={(e) => setBaseUrl(e.target.value)}
                    placeholder="https://your-endpoint.example.com/v1"
                    autoComplete="off"
                  />
                </div>
              )}
            </div>

            {/* 参数卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">03</span>参数</div>
              <div className="field">
                <label className="form-label">温度 temperature</label>
                <input type="number" step="0.1" min="0" max="2" value={temperature} onChange={(e) => setTemperature(e.target.value)} />
              </div>
            </div>

            <div className="form-actions">
              <button className="btn primary" type="submit" disabled={busy}>
                {busy ? '处理中…' : editingId ? '● 保存修改' : '＋ 创建模型'}
              </button>
              {editingId && (
                <button className="btn" type="button" onClick={startCreate}>取消</button>
              )}
            </div>
          </form>
        </Panel>
      </div>
    </section>
  );
}
