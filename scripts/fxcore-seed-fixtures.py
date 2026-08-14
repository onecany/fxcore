#!/usr/bin/env python3
"""FXcore 假数据灌库脚本（直写 sqlite，服务 WAL 并发安全）。
用法: python3 fxcore_seed.py [db路径]   默认 /tmp/fxcore-persist/fxcore.db
幂等: 重复运行会先清空业务表再灌（保留 users 的 admin）。
定稿于 2026-08-09 会话（用户验收链路：/dashboard?trader=<uuid> 直达 + 各页面假数据）。
关键约定（踩坑后固化的）:
  - 交易员/模型/账户/策略 ID 用 32 位 hex uuid（与引擎 newID() 一致），URL ?trader=<uuid> 才真实
  - 列名以 PRAGMA table_info 为准（GORM 拆列名: PnL→pn_l / AIProvider→a_iprovider）
  - CLOSED 持仓必须给 closed_at 时间（None 会被平仓历史过滤丢），OPEN 才 None
  - json.RawMessage 列（candidate_coins/execution_log/config/payload/extra）存 json.dumps 裸数组，
    GORM serializer:json 双向兼容
  - 时间戳格式 'YYYY-MM-DD HH:MM:SS.000000'（mattn sqlite 默认）
  - AI 模型 key 是假的（sk-****），不要点"测试连接"
"""
import sqlite3, sys, json, time, random
from datetime import datetime, timedelta, timezone

DB = sys.argv[1] if len(sys.argv) > 1 else "/tmp/fxcore-persist/fxcore.db"
conn = sqlite3.connect(DB)
cur = conn.cursor()

def now_s(): return datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M:%S.000000")
def ts_minus(mins): return (datetime.now(timezone.utc) - timedelta(minutes=mins)).strftime("%Y-%m-%d %H:%M:%S.000000")
def rid(): return ''.join(random.choices('0123456789abcdef', k=32))

# ---------- 清理（幂等重跑） ----------
TABLES = ["orders","fills","positions","decision_records","equity_snapshots",
          "backtest_runs","backtest_equities","backtest_trades","backtest_decisions","backtest_checkpoints",
          "debate_sessions","debate_participants","debate_messages","debate_votes",
          "traders","strategies","exchanges","ai_models","telegram_configs"]
for t in TABLES:
    cur.execute(f"DELETE FROM {t}")

admin_id = cur.execute("SELECT id FROM users LIMIT 1").fetchone()[0]
print(f"admin: {admin_id}")

# ---------- AI 模型 ----------
models = [
    ("m1", "deepseek-v4", "deepseek", "deepseek-v4-flash", "sk-****a1b2"),
    ("m2", "gpt-4o", "gpt", "gpt-4o", "sk-****c3d4"),
    ("m3", "claude-sonnet", "claude", "claude-sonnet-4-5", "sk-****e5f6"),
]
for mid, name, prov, mn, prefix in models:
    cur.execute("INSERT INTO ai_models (id,user_id,name,provider,model_name,api_key_enc,api_key_prefix,status,config,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?,?,?)",
                (mid, admin_id, name, prov, mn, "ENC:v1:"+rid(), prefix, "active", '{"temperature":0.7}', now_s(), now_s()))

# ---------- 交易所账户 ----------
exchanges = [
    ("ex1", "币安主力", "binance", "main-futures", True, "ab****yz", "ENC:v1:"+rid(), "ENC:v1:"+rid(), ""),
    ("ex2", "Bybit 永续", "bybit", "bybit-perp", True, "cd****wx", "ENC:v1:"+rid(), "ENC:v1:"+rid(), ""),
    ("ex3", "OKX 备用", "okx", "okx-futures", True, "ef****uv", "ENC:v1:"+rid(), "ENC:v1:"+rid(), "ENC:v1:"+rid()),
    ("ex4", "Hyperliquid DEX", "hyperliquid", "hl-main", True, "", "", "", ""),
]
for eid, an, et, name, en, pre, akey, skey, pp in exchanges:
    cur.execute("""INSERT INTO exchanges (id,user_id,exchange_type,account_name,enabled,testnet,api_key_prefix,api_key_enc,secret_key_enc,passphrase_enc,hyperliquid_wallet_addr,deleted_at,created_at,updated_at)
                   VALUES (?,?,?,?,?,0,?,?,?,?,'0x9f8E5fBf2b3A1c4d5E6f7a8B9c0D1e2F3a4B5c6D',NULL,?,?)""",
                (eid, admin_id, et, name, 1 if en else 0, pre, akey, skey, pp, now_s(), now_s()))

# ---------- 策略 ----------
strategies = [
    ("st1", "BTC 均衡策略", "balanced", True, {"risk_control":{"max_positions":3,"btc_eth_max_leverage":5,"altcoin_max_leverage":5,"btc_eth_max_position_value_ratio":5,"altcoin_max_position_value_ratio":1,"max_margin_usage":0.3,"min_position_size":12,"min_confidence":0.6,"min_risk_reward_ratio":1.5},"prompt_variant":"balanced","coin_source":{"source_type":"static","static_coins":["BTCUSDT","ETHUSDT"]}}),
    ("st2", "SOL 激进策略", "aggressive", False, {"risk_control":{"max_positions":3,"btc_eth_max_leverage":10,"altcoin_max_leverage":10,"btc_eth_max_position_value_ratio":8,"altcoin_max_position_value_ratio":2,"max_margin_usage":0.5,"min_position_size":12,"min_confidence":0.45,"min_risk_reward_ratio":1.2},"prompt_variant":"aggressive","coin_source":{"source_type":"static","static_coins":["SOLUSDT","DOGEUSDT","ARBUSDT"]}}),
    ("st3", "ETH 稳健策略", "conservative", False, {"risk_control":{"max_positions":1,"btc_eth_max_leverage":3,"altcoin_max_leverage":3,"btc_eth_max_position_value_ratio":3,"altcoin_max_position_value_ratio":1,"max_margin_usage":0.2,"min_position_size":12,"min_confidence":0.75,"min_risk_reward_ratio":2.0},"prompt_variant":"conservative","coin_source":{"source_type":"static","static_coins":["ETHUSDT"]}}),
]
for sid, sn, style, active, cfg in strategies:
    cur.execute("INSERT INTO strategies (id,user_id,name,description,is_active,is_default,is_public,config,created_at,updated_at) VALUES (?,?,?,?,?,0,0,?,?,?)",
                (sid, admin_id, sn, f"{style} 风格假策略", 1 if active else 0, json.dumps(cfg, ensure_ascii=False), ts_minus(600), now_s()))

# ---------- 交易员（ID 用 32 位 hex uuid，与引擎 newID() 一致） ----------
tid1, tid2, tid3 = rid(), rid(), rid()
def trader_json(o): return json.dumps(o, ensure_ascii=False)
traders = [
    (tid1, "BTC 多头一号", "binance", "m1", "st1", "running",
     {"max_position_size":200,"stop_loss":0.02,"take_profit":0.05,"max_daily_loss":0.1},
     {"interval":300}, {"total_pnl":1284.5,"win_rate":0.62,"trade_count":18,"daily_pnl":42.3}),
    (tid2, "SOL 趋势猎手", "bybit", "m2", "st2", "paused",
     {"max_position_size":300,"stop_loss":0.03,"take_profit":0.08,"max_daily_loss":0.15},
     {"interval":180}, {"total_pnl":-356.2,"win_rate":0.44,"trade_count":27,"daily_pnl":-18.9}),
    (tid3, "ETH 稳健磐石", "okx", "m3", "st3", "idle",
     {"max_position_size":150,"stop_loss":0.015,"take_profit":0.04,"max_daily_loss":0.08},
     {"interval":600}, {"total_pnl":652.1,"win_rate":0.71,"trade_count":12,"daily_pnl":5.6}),
]
for tid, tn, ex, mid, sid, st, rc, sch, mtr in traders:
    prov = next((p for m, _, p, _, _ in models if m == mid), "deepseek")
    cur.execute("""INSERT INTO traders (id,user_id,name,exchange,model_config,strategy_id,risk_config,schedule,status,metrics,created_at,updated_at)
                   VALUES (?,?,?,?,?,?,?,?,?,?,?,?)""",
                (tid, admin_id, tn, ex,
                 trader_json({"provider": prov, "model_id": mid, "parameters": {"temperature": 0.7}}),
                 sid, trader_json(rc), trader_json(sch), st, trader_json(mtr), ts_minus(700), now_s()))

# ---------- 订单 ----------
orders = [
    ("o1",tid1,"ex1","EX-BIN-1001","t1-12-BTCUSDT-open_long","BTCUSDT","buy","long","limit","GTC",0.5,61250.0,0,"FILLED",0.5,61245.0,0.612,"USDT",5,0,0),
    ("o2",tid1,"ex1","EX-BIN-1002","t1-12-ETHUSDT-open_long","ETHUSDT","buy","long","market","IOC",2.5,3210.5,0,"FILLED",2.5,3208.2,0.802,"USDT",5,0,0),
    ("o3",tid2,"ex2","EX-BYB-2001","t2-7-SOLUSDT-open_long","SOLUSDT","buy","long","market","IOC",40,142.8,0,"FILLED",40,142.5,0.570,"USDT",10,0,0),
    ("o4",tid2,"ex2","EX-BYB-2002","t2-7-DOGEUSDT-open_short","DOGEUSDT","sell","short","limit","GTC",3000,0.1725,0,"NEW",0,0,0,"USDT",10,0,0),
    ("o5",tid3,"ex3","EX-OKX-3001","t3-3-ETHUSDT-open_long","ETHUSDT","buy","long","market","IOC",1.0,3205.0,0,"FILLED",1.0,3204.5,0.320,"USDT",3,0,0),
    ("o6",tid1,"ex1","EX-BIN-1003","t1-13-BTCUSDT-close_long","BTCUSDT","sell","long","market","IOC",0.5,62800.0,0,"FILLED",0.5,62810.0,0.628,"USDT",5,0,1),
]
for oid, tid, exid, eoid, coid, sym, side, ps, otype, tif, qty, price, sp, st, fq, afp, com, ca, lev, ro, cp in orders:
    cur.execute("""INSERT INTO orders (id,trader_id,exchange_id,exchange_order_id,client_order_id,symbol,side,position_side,type,time_in_force,quantity,price,stop_price,status,filled_quantity,avg_fill_price,commission,commission_asset,leverage,reduce_only,close_position,created_at,updated_at)
                   VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)""",
                (oid,tid,exid,eoid,coid,sym,side,ps,otype,tif,qty,price,sp,st,fq,afp,com,ca,lev,ro,cp,ts_minus(180),ts_minus(10)))

# ---------- 成交 ----------
fills = [
    ("f1",tid1,"o1","EX-BIN-1001","TX-BIN-9001","BTCUSDT","buy",61245.0,0.5,30622.5,0.612,"USDT",0,0),
    ("f2",tid1,"o2","EX-BIN-1002","TX-BIN-9002","ETHUSDT","buy",3208.2,2.5,8020.5,0.802,"USDT",0,0),
    ("f3",tid2,"o3","EX-BYB-2001","TX-BYB-7001","SOLUSDT","buy",142.5,40,5700.0,0.570,"USDT",0,0),
    ("f4",tid3,"o5","EX-OKX-3001","TX-OKX-5001","ETHUSDT","buy",3204.5,1.0,3204.5,0.320,"USDT",0,0),
    ("f5",tid1,"o6","EX-BIN-1003","TX-BIN-9003","BTCUSDT","sell",62810.0,0.5,31405.0,0.628,"USDT",782.5,0),
]
for fid, tid, oid, eoid, etid, sym, side, price, qty, qq, com, ca, pnl, mk in fills:
    cur.execute("""INSERT INTO fills (id,trader_id,order_id,exchange_order_id,exchange_trade_id,symbol,side,price,quantity,quote_quantity,commission,commission_asset,realized_pn_l,is_maker,created_at)
                   VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)""",
                (fid,tid,oid,eoid,etid,sym,side,price,qty,qq,com,ca,pnl,mk,ts_minus(170)))

# ---------- 持仓（CLOSED 必须给 closed_at，OPEN 才 None） ----------
positions = [
    ("p1",tid1,"ex1","BTCUSDT","long",0.5,61245.0,782.5,"OPEN",0.5,0.5,62850.0,802.5,5,"OPEN",None,0,0,0,0,"engine"),
    ("p2",tid1,"ex1","ETHUSDT","long",2.5,3208.2,46.0,"OPEN",2.5,2.5,3226.6,46.0,5,"OPEN",None,0,0,0,0,"engine"),
    ("p3",tid2,"ex2","SOLUSDT","long",40,142.5,-320.0,"CLOSED",40,0,139.5,-320.0,10,"CLOSED","closed",0,0,0,0,"engine"),
    ("p4",tid3,"ex3","ETHUSDT","long",1.0,3204.5,18.5,"CLOSED",1.0,0,3223.0,18.5,3,"CLOSED","closed",0,0,0,0,"engine"),
]
for pid, tid, exid, sym, side, size, ep, pnl, st, eq, qty, mp, upnl, lev, status, closed, _,_,_,_, src in positions:
    is_closed = closed is not None and status == "CLOSED"
    cur.execute("""INSERT INTO positions (id,trader_id,exchange_id,symbol,side,size,entry_price,pn_l,opened_at,closed_at,entry_quantity,quantity,mark_price,unrealized_pn_l,leverage,status,entry_time,exit_time,exit_price,realized_pn_l,fee,close_reason,source)
                   VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)""",
                (pid,tid,exid,sym,side,size,ep,pnl,ts_minus(120),ts_minus(45) if is_closed else None,
                 eq,qty,mp,upnl,lev,status,
                 int(time.time())-7200, int(time.time())-2700 if is_closed else None,
                 62810.0 if is_closed else 0, pnl if is_closed else 0, 0.6, "tp" if is_closed else None, src))

# ---------- 决策记录 ----------
decisions = [
    ("d1",tid1,12,"BTCUSDT",True,'[{"symbol":"BTCUSDT","action":"open_long","quantity":0.5,"leverage":5,"stop_loss":60200,"take_profit":64500,"confidence":0.82,"risk_usd":150}]',2,3),
    ("d2",tid1,13,"BTCUSDT",True,'[{"symbol":"BTCUSDT","action":"close_long","quantity":0.5,"leverage":5,"confidence":0.9,"risk_usd":0}]',3,4),
    ("d3",tid1,14,"BTCUSDT",True,'[{"symbol":"BTCUSDT","action":"wait"}]',4,5),
    ("d4",tid2,7,"SOLUSDT",True,'[{"symbol":"SOLUSDT","action":"open_long","quantity":40,"leverage":10,"stop_loss":138.2,"take_profit":154.0,"confidence":0.67,"risk_usd":172}]',5,6),
    ("d5",tid2,8,"SOLUSDT",False,'',6,7),
    ("d6",tid3,3,"ETHUSDT",True,'[{"symbol":"ETHUSDT","action":"wait"}]',7,8),
    ("d7",tid1,15,"ETHUSDT",True,'[{"symbol":"ETHUSDT","action":"hold"}]',8,9),
    ("d8",tid2,9,"DOGEUSDT",True,'[{"symbol":"DOGEUSDT","action":"open_short","quantity":3000,"leverage":10,"stop_loss":0.181,"take_profit":0.163,"confidence":0.71,"risk_usd":88}]',9,10),
]
for did, tid, cyc, sym, ok, dj, a, b in decisions:
    sp = (f"You are an experienced crypto futures trader.\nCoin source: static\nPrimary timeframe: 15m (30 bars)\n"
          f"Max positions: 3 | BTC/ETH leverage: 5 | Altcoin leverage: 5\n"
          f"Min position size: 12 USDT | Min risk/reward: 1.5 | Min confidence: 60%\n"
          f"Mode: Balanced. Recommended balance of opportunity and risk; no extreme sizing or overtrading.\n"
          f"Decision actions: open_long | open_short | close_long | close_short | hold | wait\n")
    ip = f"Cycle {cyc} | Account balance: 10284.50 USDT | Open positions: [{sym}] | K-lines: 15m x 30 bars\nCandidate Coins (3 coins): {sym}, ETHUSDT, SOLUSDT\n"
    cot = "Multi-timeframe: 1h up, 15m up, 5m up. Volume confirms. OI rising.\n" if ok else "AI call failed: upstream_empty_output"
    raw = "<reasoning>trend aligned</reasoning>\n<decision>" + dj + "</decision>" if ok else ""
    cur.execute("""INSERT INTO decision_records (id,trader_id,cycle_number,timestamp,system_prompt,input_prompt,cot_trace,decision_json,raw_response,candidate_coins,execution_log,success,error_message,ai_request_duration_ms)
                   VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)""",
                (did,tid,cyc,ts_minus(a),sp,ip,cot,dj,raw,
                 json.dumps([sym,"ETHUSDT","SOLUSDT"]),
                 json.dumps([{"step":"place_order","ok":True}]) if ok else None,
                 1 if ok else 0, "" if ok else "AI didn't output JSON decision, entering safe wait mode",
                 2341 if ok else 47000))

# ---------- 权益快照（曲线） ----------
base = 10000.0
for i in range(15):
    tid = random.choice([tid1,tid2,tid3])
    base += random.uniform(-60, 90)
    cur.execute("""INSERT INTO equity_snapshots (id,trader_id,timestamp,total_equity,balance,unrealized_pn_l,position_count,margin_used_pct)
                   VALUES (?,?,?,?,?,?,?,?)""",
                (rid(), tid, ts_minus(150-i*10), round(base,2), round(base*0.92,2), round(random.uniform(-50,120),2), random.randint(0,3), round(random.uniform(0.05,0.28),3)))

# ---------- 回测（completed run 全套） ----------
bt_id = f"bt_demo_{int(time.time())}"
cur.execute("""INSERT INTO backtest_runs (run_id,user_id,config_json,state,label,symbol_count,progress_pct,equity_last,max_drawdown_pct,liquidated,prompt_template,a_iprovider,ai_model,last_error,created_at,updated_at)
               VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)""",
            (bt_id, admin_id,
             json.dumps({"symbols":["BTCUSDT","ETHUSDT"],"start_time":int(time.time())-86400,"end_time":int(time.time()),"cadence":15,"initial_balance":10000,"fill_policy":"next_open","leverage":5,"prompt_variant":"balanced"}),
             "completed", "BTC/ETH 双币回放", 2, 100.0, 10482.5, 8.4, 0,
             "balanced template", "deepseek", "deepseek-v4-flash", "",
             ts_minus(300), now_s()))
eq = 10000.0
for i in range(10):
    eq += random.uniform(-80, 120)
    cur.execute("INSERT INTO backtest_equities (id,run_id,timestamp,equity,available,pn_l,pn_l_pct,drawdown_pct,cycle) VALUES (?,?,?,?,?,?,?,?,?)",
                (rid(), bt_id, int(time.time())-86400+i*3600, round(eq,2), round(eq*0.9,2), round(eq-10000,2), round((eq-10000)/10000*100,2), round(random.uniform(0,8.4),2), i+1))
bt_trades = [
    (bt_id,1,"BTCUSDT","open_long","long",0.4,61200.0,1.2,0.61,"OPEN",5,1,0.4,0),
    (bt_id,2,"ETHUSDT","open_long","long",3.0,3190.0,2.1,0.96,"OPEN",5,2,3.0,0),
    (bt_id,3,"BTCUSDT","close_long","long",0.4,63100.0,1.5,0.76,760.0,5,3,0.0,0),
    (bt_id,4,"ETHUSDT","close_long","long",3.0,3250.0,1.8,0.98,180.0,5,4,0.0,0),
    (bt_id,5,"BTCUSDT","open_long","long",0.5,62200.0,1.9,0.31,"OPEN",5,5,0.5,0),
]
for r, cyc, sym, act, side, qty, price, fee, slip, pnl, lev, cycle, pa, liq in bt_trades:
    cur.execute("""INSERT INTO backtest_trades (id,run_id,timestamp,symbol,action,side,quantity,price,fee,slippage,order_value,realized_pn_l,leverage,cycle,position_after,liquidation,note)
                   VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)""",
                (rid(), bt_id, int(time.time())-86400+cyc*3600, sym, act, side, qty, price, fee, slip, qty*price, pnl, lev, cycle, pa, liq, ""))
for cyc in range(1,5):
    cur.execute("INSERT INTO backtest_decisions (id,run_id,cycle,payload) VALUES (?,?,?,?)",
                (rid(), bt_id, cyc, json.dumps({"symbol":random.choice(["BTCUSDT","ETHUSDT"]),"action":"open_long" if cyc%2 else "wait","confidence":0.72})))
cur.execute("INSERT INTO backtest_checkpoints (run_id,payload) VALUES (?,?)",
            (bt_id, json.dumps({"symbols":["BTCUSDT"],"cycle":5,"equity":10482.5})))

# ---------- 辩论 ----------
debates = [
    ("db1","BTC 多空辩论","running","BTCUSDT",3,1,"balanced",False),
    ("db2","ETH 趋势辩论","completed","ETHUSDT",2,2,"aggressive",True),
]
for did, dn, st, sym, mr, cr, pv, ae in debates:
    cur.execute("""INSERT INTO debate_sessions (id,name,strategy_id,status,symbol,max_rounds,current_round,interval_minutes,prompt_variant,auto_execute,trader_id,consensus,created_at,updated_at)
                   VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)""",
                (did,dn,None,st,sym,mr,cr,5,pv,1 if ae else 0,None,
                 json.dumps({"action":"open_long","symbol":"BTCUSDT","confidence":0.78}) if st=="completed" else None,
                 ts_minus(200), now_s()))
parts = [
    ("db1","m1","deepseek-v4-flash","bull","#22d3ee",1),
    ("db1","m2","gpt-4o","bear","#fb7185",2),
    ("db1","m3","claude-sonnet-4-5","analyst","#a78bfa",3),
    ("db2","m1","deepseek-v4-flash","bull","#22d3ee",1),
    ("db2","m2","gpt-4o","risk_manager","#fbbf24",2),
]
for did, mid, mn, ps, col, so in parts:
    cur.execute("INSERT INTO debate_participants (id,session_id,ai_model_id,ai_model_name,provider,personality,color,speak_order) VALUES (?,?,?,?,?,?,?,?)",
                (rid(), did, mid, mn, "deepseek" if "deepseek" in mn else ("gpt" if "gpt" in mn else "claude"), ps, col, so))
msgs = [
    ("db1",1,"多头视角：4h 通道突破确认，资金费率转正，回调至 61500 可加仓。","bull","deepseek-v4-flash","看多 BTC：突破 62000 后回踩不破，量价齐升，目标 64500。",60),
    ("db1",1,"空头视角：OI 创新高但价格滞涨，ETF 流入放缓，警惕假突破回撤。","bear","gpt-4o","81500 是强阻力区，此处追多盈亏比差，建议等待。",55),
    ("db1",1,"客观评估：多空分歧在 62000-62800 区间，成交量未确认方向。","analyst","claude-sonnet-4-5","VIX 级别波动收敛，等 4h 收线定方向。",50),
    ("db1",2,"多头：收盘站稳 62800，确认突破，目标 64500 不变。","bull","deepseek-v4-flash","加仓信号出现。",48),
    ("db1",2,"空头：缩量上涨不可持续，日内大概率回踩 61800。","bear","gpt-4o","减仓或对冲。",46),
    ("db2",1,"ETH 联动 BTC，突破 3250 后空间打开。","bull","deepseek-v4-flash","看多 ETH 至 3350。",42),
    ("db2",1,"风控提示：ETH 波动率上升，建议杠杆降至 3x 以下。","risk_manager","gpt-4o","控制回撤优先。",40),
    ("db2",2,"最终裁决：共识偏多，但仓位控制 30% 以内。","risk_manager","gpt-4o","CONSENSUS 达成。",38),
]
for did, rnd, _title, ps, model, content, mins in msgs:
    cur.execute("INSERT INTO debate_messages (id,session_id,round,participant_id,personality,ai_model_name,content,timestamp,created_at) VALUES (?,?,?,?,?,?,?,?,?)",
                (rid(), did, rnd, rid(), ps, model, content, int(time.time())-mins*60, ts_minus(mins)))
votes = [
    ("db1",1,"m1","open_long","BTCUSDT",0.82,5,0.3,0.02,0.05,"4h 突破确认"),
    ("db1",1,"m2","wait","BTCUSDT",0.55,0,0,0,0,"假突破风险"),
    ("db1",2,"m1","open_long","BTCUSDT",0.85,5,0.4,0.02,0.05,"站稳 62800"),
    ("db1",2,"m2","close_long","BTCUSDT",0.6,5,0.3,0,0,"缩量滞涨"),
    ("db2",1,"m1","open_long","ETHUSDT",0.78,5,0.35,0.02,0.05,"联动突破"),
    ("db2",2,"m1","open_long","ETHUSDT",0.74,3,0.3,0.02,0.04,"降低杠杆"),
]
for did, rnd, mid, act, sym, conf, lev, pp, sl, tp, rs in votes:
    cur.execute("""INSERT INTO debate_votes (id,session_id,ai_model_id,action,symbol,confidence,leverage,position_pct,stop_loss_pct,take_profit_pct,reasoning,extra)
                   VALUES (?,?,?,?,?,?,?,?,?,?,?,?)""",
                (rid(), did, mid, act, sym, conf, lev, pp, sl, tp, rs, None))

# ---------- Telegram ----------
cur.execute("INSERT INTO telegram_configs (id,bot_token_enc,chat_id,username,bound_at,model_id,language,created_at,updated_at) VALUES (?,?,?,?,?,?,?,?,?)",
            (rid(), "ENC:v1:"+rid(), "472553190", "@onecany", ts_minus(500), "m1", "zh", ts_minus(500), now_s()))

conn.commit()
conn.close()
print(f"假数据灌库完成: {DB}")
print(f"  模型 {len(models)} / 交易所 {len(exchanges)} / 策略 {len(strategies)} / 交易员 {len(traders)}")
print(f"  订单 {len(orders)} / 成交 {len(fills)} / 持仓 {len(positions)} / 决策 {len(decisions)}")
print(f"  回测 {bt_id} / 辩论 2 会话 {len(msgs)} 消息 {len(votes)} 投票 / Telegram 1")
print(f"  交易员 uuid: {tid1} / {tid2} / {tid3}")
print(f"  直达 URL: http://127.0.0.1:6080/?trader={tid1}")
