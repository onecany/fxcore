// Debate 页面：辩论竞技场 + 创建 + SSE 事件流实时终端。
import { useCallback, useEffect, useRef, useState } from 'react';
import * as debateApi from '../api/v1/modules/debate';
import * as strategyApi from '../api/v1/modules/strategies';
import type { DebateSession, StrategyItem } from '../api/v1/types/contract';
import { PageHead, Panel, Badge, Alert, Terminal, StatCard } from '../components/ui';

interface StreamEvent { type: string; data: unknown; }

export default function DebatePage() {
  const [sessions, setSessions] = useState<DebateSession[]>([]);
  const [models, setModels] = useState<{ id: string; name: string }[]>([]);
  const [strategies, setStrategies] = useState<StrategyItem[]>([]);
  const [form, setForm] = useState({
    title: '',
    symbol: 'BTC-USDT',
    maxRounds: '2',
    strategyId: '',
    participantModelIds: [] as string[],
  });
  const [error, setError] = useState<string | null>(null);
  const [msg, setMsg] = useState<string | null>(null);
  const [stream, setStream] = useState<StreamEvent[]>([]);
  const [streamingId, setStreamingId] = useState<string | null>(null);
  const streamRef = useRef<EventSource | null>(null);

  const load = useCallback(async () => {
    try { setSessions(await debateApi.listDebates()); setError(null); } catch (e) { setError(String(e)); }
  }, []);

  useEffect(() => { void load(); }, [load]);

  useEffect(() => {
    import('../api/v1/modules/models').then((m) =>
      m.listModels().then((list) => setModels(list.map((x) => ({ id: x.id, name: x.name })))),
    ).catch(() => {});
  }, []);

  // 策略列表（创建辩论必选策略，后端 strategy_id required）
  useEffect(() => {
    strategyApi.listStrategies().then((r) => setStrategies(r.items ?? [])).catch(() => {});
  }, []);

  useEffect(() => () => streamRef.current?.close(), []);

  const toggleModel = (id: string) =>
    setForm((f) => ({
      ...f,
      participantModelIds: f.participantModelIds.includes(id)
        ? f.participantModelIds.filter((x) => x !== id)
        : [...f.participantModelIds, id],
    }));

  const create = async (e: React.FormEvent) => {
    e.preventDefault();
    setMsg(null); setError(null);
    if (!form.strategyId) {
      setError('请选择策略（辩论基于策略的风控与提示词）');
      return;
    }
    if (form.participantModelIds.length < 2) {
      setError('至少选择 2 个模型参与者');
      return;
    }
    try {
      const s = await debateApi.createDebate({
        name: form.title || '未命名辩论',
        strategyId: form.strategyId,
        symbol: form.symbol,
        participants: form.participantModelIds,
        maxRounds: Number(form.maxRounds),
      });
      setMsg(`辩论会话已创建：${s.id}`);
      void load();
    } catch (err) { setError(String(err)); }
  };

  const control = async (id: string, action: debateApi.DebateControlAction) => {
    setError(null);
    try {
      await debateApi.controlDebate(id, action);
      if (action === 'start') openStream(id);
      void load();
    } catch (err) { setError(String(err)); }
  };

  const openStream = (id: string) => {
    streamRef.current?.close();
    setStreamingId(id);
    setStream([]);
    const es = new EventSource(debateApi.debateStreamUrl(id));
    es.onmessage = (ev) => {
      try {
        const parsed = JSON.parse(ev.data) as StreamEvent;
        setStream((s) => [...s.slice(-99), parsed]);
      } catch { /* 心跳忽略 */ }
    };
    es.onerror = () => { es.close(); setStreamingId(null); };
    streamRef.current = es;
  };

  const running = sessions.filter((s) => s.status === 'running' || s.status === 'voting').length;

  return (
    <section>
      <PageHead
        title="辩论竞技场"
        lead={<>5 人格仲裁 · SSE 事件流 <code>round_start → message → round_end → vote → consensus</code></>}
      />
      {error && <Alert kind="error">{error}</Alert>}
      {msg && <Alert kind="ok">{msg}</Alert>}

      <div className="stat-grid">
        <StatCard label="进行中" value={String(running)} tone={running > 0 ? 'up' : 'plain'} />
        <StatCard label="会话总数" value={String(sessions.length)} />
        <StatCard label="参与者" value={String(models.length)} hint="可用 AI 模型" />
      </div>

      <Panel title="发起辩论" className="mb">
        <form onSubmit={(e) => void create(e)}>
          <div className="form-grid">
            <div className="field">
              <label>策略（辩论的风控与提示词基础）</label>
              <select
                value={form.strategyId}
                onChange={(e) => setForm((f) => ({ ...f, strategyId: e.target.value }))}
                required
              >
                <option value="">— 选择策略 —</option>
                {strategies.map((st) => (
                  <option key={st.id} value={st.id}>
                    {st.name}{st.isActive ? '（生效中）' : ''}
                  </option>
                ))}
              </select>
              {strategies.length === 0 && <span className="dim" style={{ fontSize: 11 }}>无可用策略（先去策略页创建）</span>}
            </div>
            <div className="field">
              <label>标题</label>
              <input value={form.title} onChange={(e) => setForm((f) => ({ ...f, title: e.target.value }))} placeholder="e.g. btc-weekly-outlook" />
            </div>
            <div className="field">
              <label>币种</label>
              <input value={form.symbol} onChange={(e) => setForm((f) => ({ ...f, symbol: e.target.value }))} />
            </div>
            <div className="field">
              <label>轮数（1-5）</label>
              <input type="number" min={1} max={5} value={form.maxRounds} onChange={(e) => setForm((f) => ({ ...f, maxRounds: e.target.value }))} />
            </div>
          </div>
          <div className="mt">
            <span className="dim" style={{ fontSize: 12, marginRight: 8 }}>参与者（≥2）：</span>
            {models.length === 0 && <span className="muted mono">无可用模型</span>}
            {models.map((m) => (
              <label key={m.id} className="row" style={{ display: 'inline-flex', marginRight: 14, cursor: 'pointer' }}>
                <input
                  type="checkbox"
                  style={{ accentColor: 'var(--fxcore-accent)' }}
                  checked={form.participantModelIds.includes(m.id)}
                  onChange={() => toggleModel(m.id)}
                />
                <span className="dim" style={{ fontSize: 13 }}>{m.name}</span>
              </label>
            ))}
          </div>
          <div className="mt">
            <button className="btn primary" type="submit">⚡ 创建会话</button>
          </div>
        </form>
      </Panel>

      <Panel title="会话列表" className="mb">
        <div className="table-wrap"><table className="data-table">
          <thead>
            <tr><th>名称</th><th>策略</th><th>币种</th><th>状态</th><th>轮次</th><th>操作</th></tr>
          </thead>
          <tbody>
            {sessions.map((s) => {
              const st = strategies.find((x) => x.id === s.strategyId);
              return (
                <tr key={s.id}>
                  <td className="mono">{s.name}</td>
                  <td className="mono dim">{st?.name ?? (s.strategyId ? s.strategyId.slice(0, 8) : '—')}</td>
                  <td className="mono">{s.symbol}</td>
                  <td><Badge state={s.status} /></td>
                  <td className="mono">{s.currentRound}/{s.maxRounds}</td>
                  <td>
                    <span className="row">
                      {s.status === 'pending' && <button className="btn primary" onClick={() => void control(s.id, 'start')}>▶ 开始</button>}
                      {s.status === 'running' && <button className="btn danger" onClick={() => void control(s.id, 'cancel')}>■ 取消</button>}
                      {streamingId === s.id && (
                        <button className="btn ghost" onClick={() => { streamRef.current?.close(); setStreamingId(null); }}>关闭流</button>
                      )}
                    </span>
                  </td>
                </tr>
              );
            })}
            {sessions.length === 0 && <tr><td colSpan={6} className="empty">// NO DEBATES ON RECORD</td></tr>}
          </tbody>
        </table></div>
      </Panel>

      {streamingId && (
        <Terminal title={`EVENT STREAM · ${streamingId.slice(0, 8)} · LIVE`}>
          {stream.map((ev, i) => (
            <div key={i} style={{ marginBottom: 3 }}>
              <span className="ev-type">[{ev.type}]</span>{' '}
              <span className="ev-data">{typeof ev.data === 'string' ? ev.data : JSON.stringify(ev.data)}</span>
            </div>
          ))}
          {stream.length === 0 && <span className="muted">· 等待事件…</span>}
        </Terminal>
      )}
    </section>
  );
}
