// MSW Mock 交易员处理器（API设计.md 第四步）：
// 使用 faker 生成全量 Mock 数据，覆盖分页、状态过滤、字段过滤场景。
// 响应为 snake_case（模拟真实后端），client.ts 会归一化为 camelCase。
import { faker } from '@faker-js/faker';
import { http, HttpResponse } from 'msw';

const STATUSES = ['idle', 'running', 'paused', 'stopped', 'error'] as const;
const EXCHANGES = ['binance', 'hyperliquid', 'aster', 'bybit', 'okx'] as const;
const PROVIDERS = ['deepseek', 'qwen', 'claude', 'gpt', 'gemini', 'custom'] as const;
const SYMBOLS = ['BTC-USDT', 'ETH-USDT', 'SOL-USDT', 'DOGE-USDT', 'BNB-USDT', 'XRP-USDT'] as const;

// 后端 snake_case 形态（与 internal/api/v1/dto 对齐）
interface MockRiskConfig {
  max_position_size: number;
  stop_loss: number;
  take_profit: number;
  max_daily_loss: number;
}

interface MockModelConfig {
  provider: string;
  model_id: string;
  parameters?: Record<string, unknown>;
}

interface MockTrader {
  id: string;
  name: string;
  exchange: string;
  model_config: MockModelConfig;
  strategy_id: string;
  risk_config: MockRiskConfig;
  schedule?: { interval: number; active_hours: { start: string; end: string }[] };
  status: string;
  metrics: { total_pnl: number; win_rate: number; trade_count: number; daily_pnl: number };
  created_at: string;
  updated_at: string;
}

interface MockPosition {
  id: string;
  symbol: string;
  side: 'long' | 'short';
  size: number;
  entry_price: number;
  pnl: number;
  trader_id: string;
  opened_at: string;
  closed_at?: string;
}

function makeTrader(): MockTrader {
  const pnl = faker.number.float({ min: -5000, max: 8000, fractionDigits: 2 });
  return {
    id: faker.string.uuid(),
    name: faker.person.fullName(),
    exchange: faker.helpers.arrayElement(EXCHANGES),
    model_config: {
      provider: faker.helpers.arrayElement(PROVIDERS),
      model_id: faker.string.uuid(),
      parameters: { temperature: faker.number.float({ min: 0, max: 1, fractionDigits: 2 }) },
    },
    strategy_id: faker.helpers.arrayElement(['grid-1', 'dca-1', 'ai-1']),
    risk_config: {
      max_position_size: faker.number.int({ min: 500, max: 5000 }),
      stop_loss: faker.number.float({ min: 1, max: 10, fractionDigits: 1 }),
      take_profit: faker.number.float({ min: 5, max: 20, fractionDigits: 1 }),
      max_daily_loss: faker.number.int({ min: 100, max: 1000 }),
    },
    status: faker.helpers.arrayElement(STATUSES),
    metrics: {
      total_pnl: pnl,
      win_rate: faker.number.float({ min: 0.3, max: 0.85, fractionDigits: 2 }),
      trade_count: faker.number.int({ min: 0, max: 500 }),
      daily_pnl: faker.number.float({ min: -500, max: 800, fractionDigits: 2 }),
    },
    created_at: faker.date.past({ years: 1 }).toISOString(),
    updated_at: faker.date.recent({ days: 7 }).toISOString(),
  };
}

function makePosition(): MockPosition {
  const side = faker.helpers.arrayElement(['long', 'short'] as const);
  const size = faker.number.float({ min: 0.01, max: 5, fractionDigits: 3 });
  const entry = faker.number.float({ min: 100, max: 100000, fractionDigits: 2 });
  return {
    id: faker.string.uuid(),
    symbol: faker.helpers.arrayElement(SYMBOLS),
    side,
    size,
    entry_price: entry,
    pnl: side === 'long' ? faker.number.float({ min: -200, max: 400, fractionDigits: 2 }) : 0,
    trader_id: faker.string.uuid(),
    opened_at: faker.date.recent({ days: 14 }).toISOString(),
  };
}

// 全量数据池（47 条交易员 + 12 条持仓，覆盖多页）
const traders: MockTrader[] = Array.from({ length: 47 }, makeTrader);
const positions: MockPosition[] = Array.from({ length: 12 }, makePosition);

function paginate(items: unknown[], page: number, size: number) {
  const start = (page - 1) * size;
  return items.slice(start, start + size);
}

function pickFields<T extends Record<string, unknown>>(item: T, fields: string): Record<string, unknown> {
  const want = new Set(fields.split(',').map((f) => f.trim()).filter(Boolean));
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(item)) {
    if (want.has(k)) out[k] = v;
  }
  return out;
}

const ok = (data: unknown, requestId: string) => ({
  code: 0,
  message: 'success',
  data,
  timestamp: Date.now(),
  requestId,
});

// 资源不存在：code:1004 + HTTP 404（与后端 error.go 语义一致，L2）
const notFound = () =>
  new HttpResponse(
    JSON.stringify({ code: 1004, message: 'resource not found', data: null, timestamp: Date.now(), requestId: 'mock-404' }),
    { status: 404, headers: { 'Content-Type': 'application/json' } },
  );

export const traderHandlers = [
  // GET /traders：分页 + 状态过滤 + 字段过滤
  http.get('/api/v1/traders', ({ request }) => {
    const url = new URL(request.url);
    const page = Math.max(1, Number(url.searchParams.get('page') || 1));
    const size = Math.min(100, Math.max(1, Number(url.searchParams.get('size') || 20)));
    const status = url.searchParams.get('status');
    const fields = url.searchParams.get('fields');

    let filtered = status ? traders.filter((t) => t.status === status) : traders;
    const total = filtered.length;
    let items: unknown[] = paginate(filtered, page, size);
    if (fields) {
      items = items.map((t) => pickFields(t as Record<string, unknown>, fields));
    }
    return HttpResponse.json(
      ok(
        {
          items,
          pagination: { page, page_size: size, total, total_pages: Math.max(1, Math.ceil(total / size)) },
        },
        'mock-traders-list',
      ),
    );
  }),

  // GET /traders/:id
  http.get('/api/v1/traders/:id', ({ params }) => {
    const t = traders.find((x) => x.id === params.id);
    if (!t) return notFound();
    return HttpResponse.json(ok(t, 'mock-trader-get'));
  }),

  // POST /traders
  http.post('/api/v1/traders', async ({ request }) => {
    const body = (await request.json()) as Partial<MockTrader>;
    const t: MockTrader = {
      id: faker.string.uuid(),
      name: body.name || 'mock-trader',
      exchange: body.exchange || 'binance',
      model_config: body.model_config || { provider: 'deepseek', model_id: faker.string.uuid() },
      strategy_id: body.strategy_id || 'grid-1',
      risk_config: body.risk_config || { max_position_size: 1000, stop_loss: 5, take_profit: 10, max_daily_loss: 200 },
      status: 'idle',
      metrics: { total_pnl: 0, win_rate: 0, trade_count: 0, daily_pnl: 0 },
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    };
    traders.unshift(t);
    return HttpResponse.json(ok(t, 'mock-trader-create'));
  }),

  // PATCH /traders/:id
  http.patch('/api/v1/traders/:id', async ({ params, request }) => {
    const t = traders.find((x) => x.id === params.id);
    if (!t) return notFound();
    const patch = (await request.json()) as Partial<MockTrader>;
    Object.assign(t, patch, { updated_at: new Date().toISOString() });
    return HttpResponse.json(ok(t, 'mock-trader-patch'));
  }),

  // 状态机控制：start / pause / resume / stop
  ...(['start', 'pause', 'resume', 'stop'] as const).map((action) =>
    http.post(`/api/v1/traders/:id/${action}`, ({ params }) => {
      const t = traders.find((x) => x.id === params.id);
      if (!t) return notFound();
      const next: Record<string, string> = { start: 'running', pause: 'paused', resume: 'running', stop: 'stopped' };
      if (action === 'start' && t.status === 'running') {
        return HttpResponse.json(
          { code: 1204, message: 'trader is already running', data: null, timestamp: Date.now(), requestId: 'mock-1204' },
          { status: 409 },
        );
      }
      if (action === 'pause' && !['running'].includes(t.status)) {
        return HttpResponse.json(
          { code: 1004, message: 'illegal state transition', data: null, timestamp: Date.now(), requestId: 'mock-1004' },
          { status: 404 },
        );
      }
      t.status = next[action];
      t.updated_at = new Date().toISOString();
      return HttpResponse.json(ok(null, `mock-trader-${action}`));
    }),
  ),

  // GET /positions：symbol 过滤 + 字段过滤
  http.get('/api/v1/positions', ({ request }) => {
    const url = new URL(request.url);
    const symbol = url.searchParams.get('symbol');
    const fields = url.searchParams.get('fields');
    let items: unknown[] = symbol ? positions.filter((p) => p.symbol === symbol) : positions;
    if (fields) {
      items = items.map((p) => pickFields(p as Record<string, unknown>, fields));
    }
    return HttpResponse.json(ok(items, 'mock-positions'));
  }),

  // DELETE /positions/:id 平仓
  http.delete('/api/v1/positions/:id', async ({ params, request }) => {
    const p = positions.find((x) => x.id === params.id);
    if (!p) return notFound();
    const body = (await request.json().catch(() => ({}))) as { pnl?: number };
    p.pnl = body.pnl ?? 0;
    p.closed_at = new Date().toISOString();
    return HttpResponse.json(ok(p, 'mock-position-close'));
  }),
];
