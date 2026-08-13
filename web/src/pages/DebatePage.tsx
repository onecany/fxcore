// Debate 页面：辩论竞技场 + 创建 + SSE 事件流实时终端 + 会话详情。
import { useCallback, useEffect, useRef, useState } from 'react';
import * as debateApi from '../api/v1/modules/debate';
import * as strategyApi from '../api/v1/modules/strategies';
import type { DebateSession, DebateStatus, StrategyItem } from '../api/v1/types/contract';
import { PageHead, Panel, Badge, Alert, Terminal, StatCard } from '../components/ui';

interface StreamEvent { type: string; data: unknown; }

// 状态产品文案映射（不暴露 pending/running 等内部状态机术语）
const STATUS_LABEL: Record<DebateStatus, string> = {
  pending: '排队中',
  running: '辩论中',
  voting: '投票中',
  completed: '已完成',
  cancelled: '已取消',
};

// 事件类型产品文案（SSE 流显示用）
const EVENT_LABEL: Record<string, string> = {
  initial: '初始状态',
  round_start: '轮次开始',
  message: '发言',
  round_end: '轮次结束',
  vote: '投票',
  consensus: '共识',
  error: '错误',
};

const ACTION_LABEL: Record<string, string> = {
  open_long: '开多',
  open_short: '开空',
  close_long: '平多',
  close_short: '平空',
  hold: '持有',
  wait: '观望',
};

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
  const [detailId, setDetailId] = useState<string | null>(null);
  const [detail, setDetail] = useState<debateApi.SessionWithDetails | null>(null);
  const [detailLoading, setDetailLoading] = useState(false);
  const [polling, setPolling] = useState(false);

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

  // 5s 轮询：会话进行中自动刷新进度；无活跃会话则停
  useEffect(() => {
    if (!polling) return;
    const iv = setInterval(() => void load(), 5000);
    return () => clearInterval(iv);
  }, [polling, load]);

  useEffect(() => {
    setPolling(sessions.some((s) => s.status === 'running' || s.status === 'voting'));
  }, [sessions]);

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
      setForm((f) => ({ ...f, title: '', participantModelIds: [] }));
      void load();
    } catch (err) { setError(String(err)); }
  };

  const control = async (id: string, action: debateApi.DebateControlAction) => {
    setError(null);
    try {
      const s = await debateApi.controlDebate(id, action);
      setMsg(action === 'start' ? `辩论已开始：${s.name ?? id.slice(0, 8)}` : `辩论已取消：${s.name ?? id.slice(0, 8)}`);
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
        setStream((s) => [...s.slice(-149), parsed]);
      } catch { /* 心跳忽略 */ }
    };
    // 不手动 close：EventSource 原生自动重连（网络抖动自动恢复）；停止由关闭按钮/卸载负责
    es.onerror = () => { /* 保留连接，等待原生重连 */ };
    streamRef.current = es;
  };

  const closeStream = () => {
    streamRef.current?.close();
    streamRef.current = null;
    setStreamingId(null);
    setStream([]);
  };

  const openDetail = async (id: string) => {
    if (detailId === id) { setDetailId(null); setDetail(null); return; } // 再次点击收起
    setDetailId(id);
    setDetailLoading(true);
    try {
      setDetail(await debateApi.getDebate(id));
    } catch (e) {
      setError(String(e));
      setDetail(null);
    } finally {
      setDetailLoading(false);
    }
  };

  const execute = async (id: string) => {
    if (!window.confirm('将共识决策下发给交易员执行？')) return;
    try {
      const r = await debateApi.executeDebate(id);
      setMsg(r.message);
    } catch (err) { setError(String(err)); }
  };

  const remove = async (id: string) => {
    if (!window.confirm('删除该辩论会话？')) return;
    try {
      await debateApi.deleteDebate(id);
      if (detailId === id) { setDetailId(null); setDetail(null); }
      if (streamingId === id) closeStream();
      void load();
    } catch (err) { setError(String(err)); }
  };

  const running = sessions.filter((s) => s.status === 'running' || s.status === 'voting').length;

  return (
    <section>
      <PageHead
        title="辩论竞技场"
        lead={<>多模型观点碰撞 · 轮次推进 · 共识决策</>}
      />
      {error && <Alert kind="error">{error}</Alert>}
      {msg && <Alert kind="ok">{msg}</Alert>}

      <div className="stat-grid">
        <StatCard label="进行中" value={String(running)} tone={running > 0 ? 'up' : 'plain'} hint={polling ? '5s 自动刷新' : undefined} />
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
            <div className="db-participants">
              {models.map((m) => (
                <label
                  key={m.id}
                  className={`db-chip ${form.participantModelIds.includes(m.id) ? 'on' : ''}`}
                >
                  <input
                    type="checkbox"
                    checked={form.participantModelIds.includes(m.id)}
                    onChange={() => toggleModel(m.id)}
                  />
                  <span>{m.name}</span>
                </label>
              ))}
            </div>
          </div>
          <div className="mt">
            <button className="btn primary" type="submit">⚡ 创建会话</button>
          </div>
        </form>
      </Panel>

      <Panel title="会话列表" className="mb">
        <div className="row" style={{ marginBottom: 8 }}>
          <span className="dim" style={{ fontSize: 11 }}>{polling ? '5s 自动刷新' : '手动刷新'}</span>
          <button className="btn ghost" style={{ marginLeft: 'auto', padding: '2px 10px', fontSize: 11 }} onClick={() => void load()}>⟳ 刷新</button>
        </div>
        <div className="table-wrap"><table className="data-table">
          <thead>
            <tr><th>名称</th><th>策略</th><th>币种</th><th>状态</th><th>轮次</th><th>操作</th></tr>
          </thead>
          <tbody>
            {sessions.map((s) => {
              const st = strategies.find((x) => x.id === s.strategyId);
              return (
                <tr
                  key={s.id}
                  style={{ cursor: 'pointer' }}
                  onClick={() => void openDetail(s.id)}
                  className={detailId === s.id ? 'row-active' : ''}
                >
                  <td className="mono">{s.name}</td>
                  <td className="mono dim">{st?.name ?? (s.strategyId ? s.strategyId.slice(0, 8) : '—')}</td>
                  <td className="mono">{s.symbol}</td>
                  <td>
                    <Badge state={s.status} />
                    <span className="dim" style={{ fontSize: 10, marginLeft: 4 }}>{STATUS_LABEL[s.status] ?? s.status}</span>
                  </td>
                  <td className="mono">{s.currentRound}/{s.maxRounds}</td>
                  <td onClick={(e) => e.stopPropagation()}>
                    <span className="row">
                      {s.status === 'pending' && <button className="btn primary" onClick={() => void control(s.id, 'start')}>▶ 开始</button>}
                      {s.status === 'running' && <button className="btn danger" onClick={() => void control(s.id, 'cancel')}>■ 取消</button>}
                      {s.status === 'completed' && (
                        <button className="btn" onClick={() => void execute(s.id)}>⇢ 执行共识</button>
                      )}
                      {streamingId === s.id && (
                        <button className="btn ghost" onClick={closeStream}>停止监听</button>
                      )}
                      <button className="btn danger" onClick={() => void remove(s.id)}>删除</button>
                    </span>
                  </td>
                </tr>
              );
            })}
            {sessions.length === 0 && <tr><td colSpan={6} className="empty">// NO DEBATES ON RECORD</td></tr>}
          </tbody>
        </table></div>
      </Panel>

      {detailId && (
        <Panel title={`会话详情 · ${detailId.slice(0, 8)}`} className="mb">
          {detailLoading && <div className="muted mono">加载详情…</div>}
          {detail && (
            <div className="db-detail">
              <div className="row" style={{ gap: 16, marginBottom: 10 }}>
                <span className="dim" style={{ fontSize: 12 }}>策略 <b className="mono">{detail.strategyId.slice(0, 8)}</b></span>
                <span className="dim" style={{ fontSize: 12 }}>轮次 <b className="mono">{detail.currentRound}/{detail.maxRounds}</b></span>
                <span className="dim" style={{ fontSize: 12 }}>币种 <b className="mono">{detail.symbol}</b></span>
                <span className="dim" style={{ fontSize: 12 }}>风格 <b className="mono">{detail.promptVariant}</b></span>
              </div>
              <div className="db-detail-cols">
                <div>
                  <div className="db-detail-title">发言记录</div>
                  {detail.messages.length === 0 && <div className="muted mono" style={{ fontSize: 12 }}>暂无发言</div>}
                  {detail.messages.slice(-30).map((m) => (
                    <div key={m.id} className="db-msg">
                      <div className="row">
                        <span className="mono dim" style={{ fontSize: 10 }}>R{m.round}</span>
                        <span className="mono" style={{ fontSize: 11, color: m.personality ? 'var(--fxcore-accent)' : undefined }}>
                          {m.aiModelName ?? m.personality ?? '?'}
                        </span>
                      </div>
                      <div className="dim" style={{ fontSize: 12, whiteSpace: 'pre-wrap' }}>{m.content}</div>
                    </div>
                  ))}
                </div>
                <div>
                  <div className="db-detail-title">投票共识</div>
                  {detail.votes.length === 0 && <div className="muted mono" style={{ fontSize: 12 }}>暂无投票</div>}
                  {detail.votes.map((v, i) => (
                    <div key={i} className="db-vote">
                      <span className="mono" style={{ fontSize: 11 }}>{v.aiModelId.slice(0, 8)}</span>
                      <span className="mono" style={{ fontSize: 11, marginLeft: 8, color: v.action === 'open_long' ? 'var(--fxcore-up)' : v.action === 'open_short' ? 'var(--fxcore-down)' : undefined }}>
                        {ACTION_LABEL[v.action] ?? v.action}
                      </span>
                      <span className="mono dim" style={{ fontSize: 11, marginLeft: 8 }}>{Math.round(v.confidence * 100)}%</span>
                    </div>
                  ))}
                  {detail.consensus && (
                    <div className="db-consensus">
                      共识：<span style={{ color: detail.consensus.action === 'open_long' ? 'var(--fxcore-up)' : detail.consensus.action === 'open_short' ? 'var(--fxcore-down)' : undefined }}>
                        {ACTION_LABEL[detail.consensus.action] ?? detail.consensus.action}
                      </span>
                      <span className="mono dim" style={{ marginLeft: 8 }}>{Math.round(detail.consensus.confidence * 100)}%</span>
                    </div>
                  )}
                </div>
              </div>
            </div>
          )}
        </Panel>
      )}

      {streamingId && (
        <Terminal title={`辩论实况 · ${streamingId.slice(0, 8)} · LIVE`}>
          {stream.map((ev, i) => (
            <div key={i} style={{ marginBottom: 3 }}>
              <span className="ev-type">[{EVENT_LABEL[ev.type] ?? ev.type}]</span>{' '}
              <span className="ev-data">{typeof ev.data === 'string' ? ev.data : JSON.stringify(ev.data)}</span>
            </div>
          ))}
          {stream.length === 0 && <span className="muted">· 等待事件…</span>}
        </Terminal>
      )}
    </section>
  );
}
