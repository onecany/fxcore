// Debate 页面：辩论竞技场（分卡表单）+ 会话卡片列表 + SSE 实况终端 + 会话详情。
// 数据层 SWR：sessions 有进行中会话 5s 自动轮询（refreshInterval 动态），models/strategies 各自 SWR。
import { useEffect, useMemo, useRef, useState } from 'react';
import useSWR from 'swr';
import * as debateApi from '../api/v1/modules/debate';
import * as strategyApi from '../api/v1/modules/strategies';
import * as modelApi from '../api/v1/modules/models';
import type { DebateSession, DebateStatus, StrategyItem } from '../api/v1/types/contract';
import { PageHead, Panel, Badge, Alert, Terminal } from '../components/ui';
import { errMsg } from '../utils/errorCodes';

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

const actionColor = (action?: string): string | undefined =>
  action === 'open_long' ? 'var(--fxcore-up)'
    : action === 'open_short' ? 'var(--fxcore-down)'
    : undefined;

export default function DebatePage() {
  // ===== SWR 数据层：会话（动态轮询）+ 模型 + 策略 =====
  const { data: sessionsData, mutate, error: loadError } = useSWR<DebateSession[]>(
    ['/debates', 'page'],
    () => debateApi.listDebates(),
    { refreshInterval: (d) => (d ?? []).some((s) => s.status === 'running' || s.status === 'voting') ? 5000 : 0 },
  );
  const sessions = useMemo(() => sessionsData ?? [], [sessionsData]);
  const { data: modelsData } = useSWR<{ id: string; name: string }[]>(
    ['/models', 'debate'],
    () => modelApi.listModels().then((list) => list.map((x) => ({ id: x.id, name: x.name }))),
  );
  const models = useMemo(() => modelsData ?? [], [modelsData]);
  const { data: strategiesData } = useSWR<{ items: StrategyItem[] }>(
    ['/strategies', 'debate'],
    () => strategyApi.listStrategies(),
  );
  const strategies = useMemo(() => strategiesData?.items ?? [], [strategiesData]);

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
      void mutate();
    } catch (err) { setError(String(err)); }
  };

  const control = async (id: string, action: debateApi.DebateControlAction) => {
    setError(null);
    try {
      const s = await debateApi.controlDebate(id, action);
      setMsg(action === 'start' ? `辩论已开始：${s.name ?? id.slice(0, 8)}` : `辩论已取消：${s.name ?? id.slice(0, 8)}`);
      if (action === 'start') openStream(id);
      void mutate();
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
      void mutate();
    } catch (err) { setError(String(err)); }
  };

  const running = sessions.filter((s) => s.status === 'running' || s.status === 'voting').length;

  return (
    <section>
      <PageHead
        title="辩论竞技场"
        lead={<>多模型观点碰撞 · 轮次推进 · 共识决策</>}
      />
      {loadError && <Alert kind="error">{errMsg(loadError)}</Alert>}
      {error && <Alert kind="error">{errMsg(error)}</Alert>}
      {msg && <Alert kind="ok">{msg}</Alert>}

      {/* KPI 概览（毛玻璃） */}
      <div className="stat-grid">
        <div className="stat-card">
          <div className="label">进行中</div>
          <div className={`value ${running > 0 ? 'up' : ''}`}>{running}</div>
          <div className="hint">{running > 0 ? '5s 自动刷新' : '已就绪'}</div>
        </div>
        <div className="stat-card">
          <div className="label">会话总数</div>
          <div className="value">{sessions.length}</div>
          <div className="hint">本会话</div>
        </div>
        <div className="stat-card">
          <div className="label">参与者</div>
          <div className="value">{models.length}</div>
          <div className="hint">可用 AI 模型</div>
        </div>
      </div>

      <Panel title="发起辩论" className="mb">
        <form onSubmit={(e) => void create(e)} className="db-form">
          {/* 01 配置卡 */}
          <div className="form-section">
            <div className="form-section-title"><span className="form-section-num">01</span>配置</div>
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
                {strategies.length === 0 && <span className="dim db-field-hint">无可用策略（先去策略页创建）</span>}
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
          </div>
          {/* 02 参与者卡 */}
          <div className="form-section">
            <div className="form-section-title"><span className="form-section-num">02</span>参与者（≥2）</div>
            {models.length === 0 ? (
              <span className="muted mono">无可用模型（先去模型页创建）</span>
            ) : (
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
            )}
          </div>
          <div className="form-actions">
            <button className="btn primary" type="submit">⚡ 创建会话</button>
          </div>
        </form>
      </Panel>

      <Panel title={`会话列表 (${sessions.length})`} className="mb">
        <div className="db-table-head">
          <span className="dim db-auto-hint">{running > 0 ? '● 有进行中会话，5s 自动刷新' : '已就绪 · 无进行中会话'}</span>
          <button className="btn ghost btn-sm" onClick={() => void mutate()}>⟳ 刷新</button>
        </div>
        {sessions.length === 0 ? (
          <div className="muted mono db-empty">// NO DEBATES ON RECORD</div>
        ) : (
          <div className="db-session-list">
            {sessions.map((s) => {
              const st = strategies.find((x) => x.id === s.strategyId);
              return (
                <div
                  key={s.id}
                  className={`db-session-card ${detailId === s.id ? 'active' : ''}`}
                  onClick={() => void openDetail(s.id)}
                >
                  <div className="db-session-head">
                    <div className="db-session-info">
                      <span className="db-session-name mono">{s.name}</span>
                      <span className="db-session-meta">
                        {st?.name ?? (s.strategyId ? s.strategyId.slice(0, 8) : '—')} · {s.symbol}
                      </span>
                    </div>
                    <span className="db-session-state">
                      <Badge state={s.status} />
                      <span className="dim">{STATUS_LABEL[s.status] ?? s.status}</span>
                    </span>
                  </div>
                  <div className="db-session-foot">
                    <span className="db-session-round mono">R{s.currentRound}/{s.maxRounds}</span>
                    <span className="db-session-actions" onClick={(e) => e.stopPropagation()}>
                      {s.status === 'pending' && <button className="btn primary btn-sm" onClick={() => void control(s.id, 'start')}>▶ 开始</button>}
                      {s.status === 'running' && <button className="btn danger btn-sm" onClick={() => void control(s.id, 'cancel')}>■ 取消</button>}
                      {s.status === 'completed' && (
                        <button className="btn btn-sm" onClick={() => void execute(s.id)}>⇢ 执行共识</button>
                      )}
                      {streamingId === s.id && (
                        <button className="btn ghost btn-sm" onClick={closeStream}>停止监听</button>
                      )}
                      <button className="btn danger btn-sm" onClick={() => void remove(s.id)}>删除</button>
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </Panel>

      {detailId && (
        <Panel title={`会话详情 · ${detailId.slice(0, 8)}`} className="mb">
          {detailLoading && <div className="muted mono">加载详情…</div>}
          {detail && (
            <div className="db-detail">
              <div className="db-detail-head">
                <span className="dim">策略 <b className="mono">{detail.strategyId.slice(0, 8)}</b></span>
                <span className="dim">轮次 <b className="mono">{detail.currentRound}/{detail.maxRounds}</b></span>
                <span className="dim">币种 <b className="mono">{detail.symbol}</b></span>
                <span className="dim">风格 <b className="mono">{detail.promptVariant}</b></span>
              </div>
              <div className="db-detail-cols">
                <div>
                  <div className="db-detail-title">发言记录</div>
                  {detail.messages.length === 0 && <div className="muted mono db-empty-sm">暂无发言</div>}
                  {detail.messages.slice(-30).map((m) => (
                    <div key={m.id} className="db-msg">
                      <div className="row db-msg-head">
                        <span className="mono dim">R{m.round}</span>
                        <span className="mono" style={{ color: m.personality ? 'var(--fxcore-accent)' : undefined }}>
                          {m.aiModelName ?? m.personality ?? '?'}
                        </span>
                      </div>
                      <div className="dim db-msg-content">{m.content}</div>
                    </div>
                  ))}
                </div>
                <div>
                  <div className="db-detail-title">投票共识</div>
                  {detail.votes.length === 0 && <div className="muted mono db-empty-sm">暂无投票</div>}
                  {detail.votes.map((v, i) => (
                    <div key={i} className="db-vote">
                      <span className="mono">{v.aiModelId.slice(0, 8)}</span>
                      <span className="mono db-vote-action" style={{ color: actionColor(v.action) }}>
                        {ACTION_LABEL[v.action] ?? v.action}
                      </span>
                      <span className="mono dim">{Math.round(v.confidence * 100)}%</span>
                    </div>
                  ))}
                  {detail.consensus && (
                    <div className="db-consensus">
                      共识：<span style={{ color: actionColor(detail.consensus.action) }}>
                        {ACTION_LABEL[detail.consensus.action] ?? detail.consensus.action}
                      </span>
                      <span className="mono dim">{Math.round(detail.consensus.confidence * 100)}%</span>
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
            <div key={i} className="ev-row">
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
