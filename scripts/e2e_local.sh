#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR=$(cd "$(dirname "$0")/.." && pwd)
OUT_DIR="$ROOT_DIR/test-output"
APP_BIN="$OUT_DIR/tgo-rtc-server"
LOG_FILE="$OUT_DIR/server.log"
PID_FILE="$OUT_DIR/server.pid"
SUMMARY="$OUT_DIR/e2e_summary.txt"

mkdir -p "$OUT_DIR"
: > "$SUMMARY"

say() { echo "[E2E] $*" | tee -a "$SUMMARY"; }

require() {
  if ! command -v "$1" >/dev/null 2>&1; then
    say "缺少依赖: $1"
    exit 1
  fi
}

require curl

is_up() {
  curl -sS "http://localhost:8080/health" | grep -q '"ok"' || return 1
}

start_server=1
if is_up; then
  say "检测到服务已就绪，跳过启动"
  start_server=0
else
  say "构建并启动服务..."
  (cd "$ROOT_DIR" && go build -o "$APP_BIN" main.go)
  ("$APP_BIN" >"$LOG_FILE" 2>&1 & echo $! >"$PID_FILE")
fi

# 等待健康
for i in {1..60}; do
  if is_up; then
    say "服务健康检查通过"
    break
  fi
  sleep 1
  if (( i % 10 == 0 )); then say "等待服务就绪中...($i s)"; fi
  if (( i == 60 )); then
    say "服务未就绪，最近日志："
    tail -n 200 "$LOG_FILE" || true
    exit 1
  fi
done

# 清理测试数据（避免重复创建房间或用户冲突）
say "清理旧的测试数据..."
mysql -u root tgo_rtc -e "DELETE FROM rtc_participant WHERE uid LIKE 'test_%'; DELETE FROM rtc_room WHERE creator LIKE 'test_%';" 2>/dev/null || say "清理数据失败（可能是首次运行）"

req() {
  local name="$1" method="$2" url="$3" data="${4:-}"
  local body="$OUT_DIR/${name}.json"
  local meta="$OUT_DIR/${name}.meta"
  if [[ -n "$data" ]]; then
    http_code=$(curl -sS -o "$body" -w "%{http_code}" -H 'Content-Type: application/json' -X "$method" --data-binary "$data" "$url" || true)
  else
    http_code=$(curl -sS -o "$body" -w "%{http_code}" -H 'Content-Type: application/json' -X "$method" "$url" || true)
  fi
  echo "HTTP_CODE=$http_code" > "$meta"
  say "$name: HTTP $http_code"
}

# 1) 创建房间（使用时间戳避免冲突）
TIMESTAMP=$(date +%s)
CHANNEL_ID="test_ch_${TIMESTAMP}"
CREATOR="test_u1_${TIMESTAMP}"
USER2="test_u2_${TIMESTAMP}"
USER3="test_u3_${TIMESTAMP}"
USER4="test_u4_${TIMESTAMP}"
USER5="test_u5_${TIMESTAMP}"

CREATE_PAY='{"source_channel_id":"'"$CHANNEL_ID"'","source_channel_type":0,"creator":"'"$CREATOR"'","rtc_type":1,"invite_on":1,"max_participants":3,"uids":["'"$USER2"'","'"$USER3"'"]}'
req create_room POST http://localhost:8080/api/v1/rooms "$CREATE_PAY"

ROOM_ID=$(sed -n 's/.*"room_id"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' "$OUT_DIR/create_room.json" | head -n1)
if [[ -z "${ROOM_ID:-}" ]]; then say "从创建响应中未能提取 room_id"; fi
say "room_id=$ROOM_ID"

# 2) 邀请参与者
req invite POST "http://localhost:8080/api/v1/rooms/$ROOM_ID/invite" '{"uids":["'"$USER4"'","'"$USER5"'"]}'

# 3) 加入房间
req join POST "http://localhost:8080/api/v1/rooms/$ROOM_ID/join" '{"uid":"'"$USER2"'"}'

# 4) 查询正在通话的成员
req calling POST http://localhost:8080/api/v1/participants/calling '{"uids":["'"$CREATOR"'","'"$USER2"'","'"$USER3"'","'"$USER4"'","'"$USER5"'"]}'

# 5) 离开房间
req leave POST "http://localhost:8080/api/v1/rooms/$ROOM_ID/leave" '{"uid":"'"$USER2"'"}'

# 结果校验
check_code() {
  local name="$1" expect="$2"
  local meta="$OUT_DIR/${name}.meta"
  local code=$(grep -oE 'HTTP_CODE=[0-9]+' "$meta" | cut -d= -f2)
  if [[ "$code" == "$expect" ]]; then
    say "[PASS] $name 期望 $expect 得到 $code"
  else
    say "[FAIL] $name 期望 $expect 得到 ${code:-N/A}"
  fi
}

check_field() {
  local file="$OUT_DIR/$1.json" key="$2"
  if grep -q '"'"$key"'"' "$file"; then
    say "[PASS] $1 响应包含字段 $key"
  else
    say "[FAIL] $1 响应缺少字段 $key"
  fi
}

check_array() {
  local file="$OUT_DIR/$1.json"
  # 检查是否以 [ 开头（数组格式）
  if head -c 1 "$file" | grep -q '\['; then
    say "[PASS] $1 响应是数组格式"
  else
    say "[FAIL] $1 响应不是数组格式"
  fi
}

# 检查 HTTP 状态码（所有接口都应该返回 200）
check_code create_room 200
check_code invite 200
check_code join 200
check_code calling 200
check_code leave 200

# 检查 create_room 响应字段
check_field create_room room_id
check_field create_room token
check_field create_room url

# 检查 join 响应字段
check_field join room_id
check_field join token
check_field join url

# 检查 calling 响应是数组格式
check_array calling

say "测试完成。详细响应见 $OUT_DIR/*.json，日志：$LOG_FILE"

# 结束后若我们启动了服务，则关闭
if [[ $start_server -eq 1 ]] && [[ -f "$PID_FILE" ]]; then
  kill "$(cat "$PID_FILE")" 2>/dev/null || true
  rm -f "$PID_FILE"
  say "已停止测试服务"
fi

