// 模型管理页：CRUD + 连通测试 + provider/model 联动下拉。
import { useCallback, useEffect, useMemo, useState } from 'react';
import * as modelApi from '../api/v1/modules/models';
import type { AIModel } from '../api/v1/types/contract';
import { PageHead, Panel, Badge, Alert } from '../components/ui';

const fmtTime = (raw?: string): string => (raw ? new Date(raw).toLocaleString('zh-CN', { hour12: false }) : '-');
const keyMask = (prefix?: string): string => prefix || '未配置';

export default function ModelsPage() {
  const [items, setItems] = useState<AIModel[]>([]);
  const [providers, setProviders] = useState<modelApi.ProviderOption[]>([]);
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
  // 每个模型的测试状态：testing 进行中 + result 成功/失败，绑定到对应模型行显示
  const [testStates, setTestStates] = useState<Record<string, { testing: boolean; success?: boolean; latency?: number; error?: string }>>({});

  const isCustom = provider === 'custom';

  const load = useCallback(async () => {
    try {
      const [ms, ps] = await Promise.all([modelApi.listModels(), modelApi.listProviders()]);
      setItems(ms ?? []);
      setProviders(ps ?? []);
      setError(null);
    } catch (e) { setError(String(e)); }
  }, []);

  useEffect(() => { void load(); }, [load]);

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
      void load();
    } catch (err) { setError(String(err)); } finally { setBusy(false); }
  };

  const remove = async (m: AIModel) => {
    if (!window.confirm(`删除模型「${m.name}」？引用它的交易员将无法执行。`)) return;
    try {
      await modelApi.deleteModel(m.id);
      if (editingId === m.id) startCreate();
      void load();
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
      {error && <Alert kind="error">{error}</Alert>}
      {msg && <Alert kind="ok">{msg}</Alert>}

      <div className="td-grid td-grid-2">
        {/* 左：模型列表 */}
        <Panel title={`模型库 (${items.length})`}>
          {items.length === 0 ? (
            <div className="muted mono" style={{ fontSize: 12 }}>// NO MODELS（点右侧创建）</div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
              {items.map((m) => {
                const ts = testStates[m.id];
                return (
                  <div
                    key={m.id}
                    className="sb-item"
                    style={{ cursor: 'pointer' }}
                    onClick={() => startEdit(m)}
                  >
                    <div className="row" style={{ justifyContent: 'space-between', width: '100%' }}>
                      <span className="sb-name">{m.name}</span>
                      <Badge state={m.status} />
                    </div>
                    <div className="row wrap" style={{ marginTop: 4, gap: 6 }}>
                      <span className="mono dim" style={{ fontSize: 11 }}>{m.provider} / {m.modelName}</span>
                      <span className="mono dim" style={{ fontSize: 11 }}>key {keyMask(m.apiKeyPrefix)}</span>
                    </div>
                    <div className="row" style={{ marginTop: 6, gap: 6 }}>
                      <button className="btn ghost" style={{ padding: '2px 8px', fontSize: 11 }} onClick={(e) => { e.stopPropagation(); void test(m); }} disabled={ts?.testing}>⌁ 测试</button>
                      <button className="btn danger" style={{ padding: '2px 8px', fontSize: 11 }} onClick={(e) => { e.stopPropagation(); void remove(m); }}>删除</button>
                      {ts?.testing ? (
                        <span className="mono dim" style={{ fontSize: 10, marginLeft: 'auto' }}>测试中…</span>
                      ) : ts?.success != null ? (
                        <span className="mono" style={{ fontSize: 10, marginLeft: 'auto', color: ts.success ? 'var(--fxcore-up)' : 'var(--fxcore-down)' }}>
                          {ts.success ? `✓ ${ts.latency ?? '-'}ms` : `✗ ${ts.error ?? '失败'}`}
                        </span>
                      ) : (
                        <span className="mono dim" style={{ fontSize: 10, marginLeft: 'auto' }}>测试 {fmtTime(m.lastTestAt)}</span>
                      )}
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </Panel>

        {/* 右：表单（studio 分层：基础信息 / 连接 / 参数 三卡） */}
        <Panel title={editingId ? '编辑模型' : '新建模型'}>
          <form onSubmit={(e) => void submit(e)} style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
            {/* 基础信息卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">01</span>基础信息</div>
              <label className="dim" style={{ fontSize: 11 }}>别名（显示名）</label>
              <input className="prompt-area" value={name} onChange={(e) => setName(e.target.value)} placeholder="如：deepseek 主力" />
              <label className="dim" style={{ fontSize: 11, marginTop: 8 }}>提供商</label>
              <select className="prompt-area" value={provider} onChange={(e) => { setProvider(e.target.value); setModelName(''); }}>
                <option value="">— 选择提供商 —</option>
                {providers.map((p) => <option key={p.provider} value={p.provider}>{p.provider}</option>)}
              </select>
              <label className="dim" style={{ fontSize: 11, marginTop: 8 }}>模型</label>
              {isCustom ? (
                <input
                  className="prompt-area"
                  value={modelName}
                  onChange={(e) => setModelName(e.target.value)}
                  placeholder="自定义模型名，如 llama-3.1-8b / gpt-4o-mini"
                />
              ) : (
                <select className="prompt-area" value={modelName} onChange={(e) => setModelName(e.target.value)} disabled={!provider}>
                  <option value="">— 选择模型 —</option>
                  {modelOptions.map((mn) => <option key={mn} value={mn}>{mn}</option>)}
                </select>
              )}
            </div>

            {/* 连接卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">02</span>连接</div>
              <label className="dim" style={{ fontSize: 11 }}>
                API Key {editingId ? '（留空保留原 Key）' : ''}
              </label>
              <input className="prompt-area" type="password" value={apiKey} onChange={(e) => setApiKey(e.target.value)} placeholder="sk-..." autoComplete="off" />
              {isCustom && (
                <>
                  <label className="dim" style={{ fontSize: 11, marginTop: 8 }}>
                    Base URL <span style={{ color: 'var(--fxcore-down)' }}>（custom 必填，https）</span>
                  </label>
                  <input
                    className="prompt-area"
                    value={baseUrl}
                    onChange={(e) => setBaseUrl(e.target.value)}
                    placeholder="https://your-endpoint.example.com/v1"
                    autoComplete="off"
                  />
                </>
              )}
            </div>

            {/* 参数卡 */}
            <div className="form-section">
              <div className="form-section-title"><span className="form-section-num">03</span>参数</div>
              <label className="dim" style={{ fontSize: 11 }}>温度 temperature</label>
              <input className="prompt-area" type="number" step="0.1" min="0" max="2" value={temperature} onChange={(e) => setTemperature(e.target.value)} />
            </div>

            <div className="row" style={{ gap: 8, marginTop: 4 }}>
              <button className="btn primary" type="submit" disabled={busy} style={{ flex: 1 }}>
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
