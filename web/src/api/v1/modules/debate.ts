// FXcore-web API v1 辩论模块（§11 debate + SSE 事件流）。
// 对应后端 /debates/* 10 端点；stream 用 EventSource（GET，免签名）。
import client from '../client';
import type {
  DebateSession,
  DebateParticipant,
  DebateMessage,
  DebateVote,
  DebatePersonality,
  CreateDebateRequest,
} from '../types/contract';

export interface SessionWithDetails {
  session: DebateSession;
  participants: DebateParticipant[];
  messages: DebateMessage[];
  votes: DebateVote[];
  consensus?: DebateVote;
}

export type DebateControlAction = 'start' | 'cancel';

/** 会话列表 */
export function listDebates(): Promise<DebateSession[]> {
  return client.get<DebateSession[]>('/debates');
}

/** 5 人格常量 */
export function debatePersonalities(): Promise<DebatePersonality[]> {
  return client.get<DebatePersonality[]>('/debates/personalities');
}

/** 创建会话（pending） */
export function createDebate(req: CreateDebateRequest): Promise<DebateSession> {
  return client.post<DebateSession>('/debates', req);
}

/** 会话详情（含参与者/消息/投票） */
export function getDebate(id: string): Promise<SessionWithDetails> {
  return client.get<SessionWithDetails>(`/debates/${id}`);
}

/** 控制：start | cancel */
export function controlDebate(id: string, action: DebateControlAction): Promise<DebateSession> {
  return client.post<DebateSession>(`/debates/${id}/${action}`);
}

/** 共识执行（completed 且有开仓共识时） */
export function executeDebate(id: string, traderId?: string): Promise<{ executed: boolean; message: string }> {
  return client.post(`/debates/${id}/execute`, { trader_id: traderId });
}

/** 删除会话 */
export function deleteDebate(id: string): Promise<null> {
  return client.delete<null>(`/debates/${id}`);
}

/** 消息列表 */
export function debateMessages(id: string): Promise<DebateMessage[]> {
  return client.get<DebateMessage[]>(`/debates/${id}/messages`);
}

/** 投票列表 */
export function debateVotes(id: string): Promise<DebateVote[]> {
  return client.get<DebateVote[]>(`/debates/${id}/votes`);
}

/** SSE 事件流（initial/round_start/message/round_end/vote/consensus/error + 心跳） */
export function debateStreamUrl(id: string): string {
  return `/api/v1/debates/${id}/stream`;
}

/** 解析 SSE 事件行（data: JSON） */
export interface DebateStreamEvent<T = unknown> {
  type: string;
  data: T;
}
