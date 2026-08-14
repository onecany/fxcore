#!/bin/bash
# fxcore v1 HMAC 签名冒烟（阶段2 实测 21/21 PASS）。
# 覆盖：登录取 sign_secret / 免签名读端点 / 签名写端点 / 凭据零泄漏 /
# 默认 config 校验 / activate / preview-prompt / crypto aad 防重放 / 无签名写被拒。
# 复用：改 BASE（独立端口如 18080，别打 live server）+ ADMIN 凭据即可。
# 前置：go build -o /tmp/fxcore-server ./cmd/server 并起服务。
set -u
BASE="http://127.0.0.1:18080/api/v1"
JAR=$(mktemp)
PASS=0; FAIL=0

# 签名：输出 "ts|nonce|sig"（read 时必须 IFS='|' read -r，见 signed-api-smoke-testing 陷阱 10）
sign_headers() {
  local path="$1" body="$2" secret="$3"
  local ts nonce sig
  ts=$(date +%s)
  nonce=$(python3 -c "import secrets;print(secrets.token_hex(12))")
  sig=$(python3 -c "
import hmac,hashlib,sys
ts='$ts'; path='$path'; body='$body'; nonce='$nonce'; secret='$secret'
mac=hmac.new(secret.encode(), f'{ts}{path}{body}{nonce}'.encode(), hashlib.sha256).hexdigest()
print(mac)
")
  echo "$ts|$nonce|$sig"
}

check() {
  local name="$1" expect="$2" actual="$3"
  if [ "$expect" = "$actual" ]; then
    PASS=$((PASS+1)); echo "PASS  $name"
  else
    FAIL=$((FAIL+1)); echo "FAIL  $name  (expect=$expect got=$actual)"
  fi
}

code() { python3 -c "import json,sys;print(json.load(sys.stdin).get('code','ERR'))" 2>/dev/null; }

# ---- 1. 登录（sign_secret 每启动随机，绝不能硬编码） ----
LOGIN=$(curl -s -c "$JAR" -b "$JAR" -X POST "$BASE/auth/login" -H 'Content-Type: application/json' \
  -d '{"email":"admin@fxcore.local","password":"admin123"}')
SECRET=$(echo "$LOGIN" | python3 -c "import json,sys;print(json.load(sys.stdin)['data'].get('sign_secret',''))" 2>/dev/null)
if [ -z "$SECRET" ]; then
  echo "FATAL: login failed, no sign_secret: $LOGIN"; exit 1
fi
echo "PASS  login (sign_secret acquired)"
PASS=$((PASS+1))

# ---- 2. 读端点（免签名） ----
check "GET /strategies/default-config" 0 "$(curl -s -b "$JAR" "$BASE/strategies/default-config" | code)"
check "GET /exchanges (empty)" 0 "$(curl -s -b "$JAR" "$BASE/exchanges" | code)"
check "GET /crypto/public-key" 0 "$(curl -s -b "$JAR" "$BASE/crypto/public-key" | code)"
check "GET /status" 0 "$(curl -s -b "$JAR" "$BASE/status" | code)"
check "GET /competition" 0 "$(curl -s -b "$JAR" "$BASE/competition" | code)"
check "GET /symbols" 0 "$(curl -s -b "$JAR" "$BASE/symbols" | code)"
check "GET /klines?symbol=BTC-USDT" 0 "$(curl -s -b "$JAR" "$BASE/klines?symbol=BTC-USDT" | code)"
check "GET /decisions/latest (404)" 1004 "$(curl -s -b "$JAR" "$BASE/decisions/latest" | code)"
check "GET /telegram (404)" 1004 "$(curl -s -b "$JAR" "$BASE/telegram" | code)"

# ---- 3. 写端点（签名） ----
# 3a. 缺必填凭据应 1001
IFS='|' read -r TS NONCE SIG <<< "$(sign_headers '/api/v1/exchanges' '{"exchange_type":"binance","account_name":"t1"}' "$SECRET")"
BODY=$(curl -s -b "$JAR" -X POST "$BASE/exchanges" -H 'Content-Type: application/json' \
  -H "X-Timestamp: $TS" -H "X-Nonce: $NONCE" -H "X-Signature: $SIG" \
  -d '{"exchange_type":"binance","account_name":"t1"}')
check "POST /exchanges (missing creds -> 1001)" 1001 "$(echo "$BODY" | code)"

# 3b. 合法创建 + 凭据零泄漏 + 脱敏
IFS='|' read -r TS NONCE SIG <<< "$(sign_headers '/api/v1/exchanges' '{"exchange_type":"binance","account_name":"t1","api_key":"abc123","secret_key":"def456"}' "$SECRET")"
BODY=$(curl -s -b "$JAR" -X POST "$BASE/exchanges" -H 'Content-Type: application/json' \
  -H "X-Timestamp: $TS" -H "X-Nonce: $NONCE" -H "X-Signature: $SIG" \
  -d '{"exchange_type":"binance","account_name":"t1","api_key":"abc123","secret_key":"def456"}')
EXID=$(echo "$BODY" | python3 -c "import json,sys;print(json.load(sys.stdin)['data'].get('id',''))" 2>/dev/null)
if [ -n "$EXID" ]; then check "POST /exchanges (valid)" 0 "$(echo "$BODY" | code)"; else echo "FAIL  POST /exchanges (valid): $BODY"; FAIL=$((FAIL+1)); fi
if echo "$BODY" | grep -q 'def456'; then echo "FAIL  exchange response leaks secret_key"; FAIL=$((FAIL+1)); else check "exchange response no credential leak" OK OK; fi
# 短 key（≤8 字符）全掩码 "****"
check "exchange api_key_prefix masked" "****" "$(echo "$BODY" | python3 -c "import json,sys;print(json.load(sys.stdin)['data'].get('api_key_prefix',''))" 2>/dev/null)"

# 3c. 创建策略（config 缺省用默认，max_positions=3）
IFS='|' read -r TS NONCE SIG <<< "$(sign_headers '/api/v1/strategies' '{"name":"default-test"}' "$SECRET")"
BODY=$(curl -s -b "$JAR" -X POST "$BASE/strategies" -H 'Content-Type: application/json' \
  -H "X-Timestamp: $TS" -H "X-Nonce: $NONCE" -H "X-Signature: $SIG" \
  -d '{"name":"default-test"}')
SID=$(echo "$BODY" | python3 -c "import json,sys;print(json.load(sys.stdin)['data'].get('id',''))" 2>/dev/null)
if [ -n "$SID" ]; then check "POST /strategies (valid)" 0 "$(echo "$BODY" | code)"; else echo "FAIL  POST /strategies: $BODY"; FAIL=$((FAIL+1)); fi
MP=$(echo "$BODY" | python3 -c "import json,sys;print(json.load(sys.stdin)['data']['config'].get('risk_control',{}).get('max_positions',''))" 2>/dev/null)
check "strategy default max_positions=3" "3" "$MP"

# 3d. 激活策略
IFS='|' read -r TS NONCE SIG <<< "$(sign_headers "/api/v1/strategies/$SID/activate" '' "$SECRET")"
check "POST /strategies/:id/activate" 0 "$(curl -s -b "$JAR" -X POST "$BASE/strategies/$SID/activate" -H "X-Timestamp: $TS" -H "X-Nonce: $NONCE" -H "X-Signature: $SIG" | code)"
check "GET /strategies/active now exists" 0 "$(curl -s -b "$JAR" "$BASE/strategies/active" | code)"

# 3e. preview-prompt（骨架应回显配置中的币种）
PREV_BODY='{"config":{"coin_source":{"source_type":"static","static_coins":["BTC"]}}}'
IFS='|' read -r TS NONCE SIG <<< "$(sign_headers '/api/v1/strategies/preview-prompt' "$PREV_BODY" "$SECRET")"
BODY=$(curl -s -b "$JAR" -X POST "$BASE/strategies/preview-prompt" -H 'Content-Type: application/json' \
  -H "X-Timestamp: $TS" -H "X-Nonce: $NONCE" -H "X-Signature: $SIG" -d "$PREV_BODY")
if echo "$BODY" | grep -q "BTC"; then check "POST /strategies/preview-prompt" 0 "$(echo "$BODY" | code)"; else echo "FAIL  preview-prompt: $BODY"; FAIL=$((FAIL+1)); fi

# 3f. crypto/decrypt 防重放（aad 不匹配应 1001）
DEC_BODY='{"wrapped_key":"eA==","iv":"AAAAAAAAAAAAAAAAAAAAAA==","ciphertext":"eA==","aad":"wrong","ts":"1"}'
IFS='|' read -r TS NONCE SIG <<< "$(sign_headers '/api/v1/crypto/decrypt' "$DEC_BODY" "$SECRET")"
check "POST /crypto/decrypt (aad mismatch -> 1001)" 1001 "$(curl -s -b "$JAR" -X POST "$BASE/crypto/decrypt" -H 'Content-Type: application/json' -H "X-Timestamp: $TS" -H "X-Nonce: $NONCE" -H "X-Signature: $SIG" -d "$DEC_BODY" | code)"

# 3g. 无签名写操作应被拒
check "POST /strategies (no signature -> 1001)" 1001 "$(curl -s -b "$JAR" -X POST "$BASE/strategies" -H 'Content-Type: application/json' -d '{"name":"x"}' | code)"

# ---- 汇总 ----
echo "=============================="
echo "PASS=$PASS FAIL=$FAIL"
rm -f "$JAR"
[ "$FAIL" -eq 0 ]
