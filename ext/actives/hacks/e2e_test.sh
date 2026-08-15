#!/usr/bin/env bash
# e2e_test.sh — 端到端测试新接口（IP 防女巫 / 延迟结算 / 别名 / 管理员）
# 用法: bash hacks/e2e_test.sh
# 注意: 所有 curl 请求来自 127.0.0.1（同 IP），正好用于验证同 IP 拦截。
set -eo pipefail
cd "$(dirname "$0")/.."

PORT="${PORT:-9127}"
# A 使用测试私钥对应地址：updateReferralAlias 需要钱包签名，地址必须与签名恢复一致
KEY_A="0x0000000000000000000000000000000000000000000000000000000000000001"
ADDR_A="0x7e5f4552091a69125d5dfcb7b8c2659029395bdf"
echo ">>> build server..."
go build -o /tmp/actives-server ./cmd/server
# 预生成别名签名（需在项目目录内 go run，之后脚本会 cd 到临时目录）
SIG=$(go run ./hacks/signmsg "$KEY_A" "satoshin" | tail -1)

TMP=$(mktemp -d)
echo ">>> temp dir: $TMP"
cd "$TMP"
# 管理密码放 flux_config.json 的 admin_password 字段；假私钥+空 rpc_url → FLUX 链禁用，不会连网
cat > flux_config.json <<'EOF'
{"private_key":"0x0000000000000000000000000000000000000000000000000000000000000001","admin_password":"testpass123"}
EOF
/tmp/actives-server -port "$PORT" -db ./db > server.log 2>&1 &
SRV=$!
# 就绪探测：进程存活 + 端口可连，最多等 10s
ready=false
for _ in $(seq 1 20); do
  if ! kill -0 "$SRV" 2>/dev/null; then
    echo ">>> server exited early:"; cat server.log; exit 1
  fi
  if curl -sf "http://127.0.0.1:${PORT}/healthz" >/dev/null 2>&1 || curl -sf "http://127.0.0.1:${PORT}/" >/dev/null 2>&1; then
    ready=true; break
  fi
  sleep 0.5
done
[ "$ready" = true ] || { echo ">>> server not ready:"; cat server.log; exit 1; }
trap 'kill $SRV 2>/dev/null || true; rm -rf "$TMP"' EXIT

GQL="http://127.0.0.1:${PORT}/gql"
run() { curl -sS --fail-with-body -X POST -H 'Content-Type: application/json' -d "$1" "$GQL" || { echo ">>> request failed: $1"; exit 1; }; echo; }

echo ""
echo "================ 1. 默认配置（双开关开启） ================"
run '{"query":"{ antiSybilConfig { ipCheckEnabled deferredSettlementEnabled } }"}'

echo ""
echo "================ 2. A 加入 ================"
run '{"query":"mutation { joinEarlyAccess(wallet: \"0x7e5f4552091a69125d5dfcb7b8c2659029395bdf\", email: \"a@test.com\") { success warning referralAwarded } }"}'

CODE=$(run '{"query":"{ earlyAccessUser(user: \"0x7e5f4552091a69125d5dfcb7b8c2659029395bdf\") { referralCode } }"}' | grep -o '"referralCode":"[^"]*"' | head -1 | cut -d'"' -f4)
[ -n "$CODE" ] || { echo ">>> 未能获取邀请码，请检查服务是否就绪"; exit 1; }
echo ">>> A 的自动邀请码: $CODE"

echo ""
echo "================ 3. B 用 A 的码加入（同 IP 127.0.0.1 → 应拦截: warning + referralAwarded=false） ================"
run "{\"query\":\"mutation { joinEarlyAccess(wallet: \\\"0xbbbb\\\", referralCode: \\\"$CODE\\\") { success warning referralAwarded } }\"}"

echo ""
echo "================ 4. A 的推荐信息（应无任何推荐） ================"
run '{"query":"{ earlyAccessReferral(user: \"0x7e5f4552091a69125d5dfcb7b8c2659029395bdf\") { code earnings pending referrals } }"}'

echo ""
echo "================ 5. 用密码修改配置（关 IP 检查、开延迟结算；无密码/错密码应拒绝） ================"
run '{"query":"mutation { updateAntiSybilConfig(wallet: \"0x7e5f4552091a69125d5dfcb7b8c2659029395bdf\", password: \"wrong\", ipCheckEnabled: false, deferredSettlementEnabled: true) { success message } }"}'
run '{"query":"mutation { updateAntiSybilConfig(wallet: \"0x7e5f4552091a69125d5dfcb7b8c2659029395bdf\", password: \"testpass123\", ipCheckEnabled: false, deferredSettlementEnabled: true) { success message } }"}'

echo ""
echo "================ 6. C 用 A 的码加入（IP 检查已关 → 奖励；延迟结算 → 邀请人积分 pending） ================"
run "{\"query\":\"mutation { joinEarlyAccess(wallet: \\\"0xcccc\\\", referralCode: \\\"$CODE\\\") { success warning referralAwarded } }\"}"

echo ""
echo "================ 7. A 的推荐信息（应 pending=1, earnings=0） ================"
run '{"query":"{ earlyAccessReferral(user: \"0x7e5f4552091a69125d5dfcb7b8c2659029395bdf\") { code earnings pending referrals } }"}'

echo ""
echo "================ 8. C 签到 → 触发结算 ================"
run '{"query":"mutation { dailyCheckin(wallet: \"0xcccc\") { success streak totalPoints } }"}'

echo ""
echo "================ 9. A 的推荐信息（应 earnings=75, pending=0） ================"
run '{"query":"{ earlyAccessReferral(user: \"0x7e5f4552091a69125d5dfcb7b8c2659029395bdf\") { code earnings pending referrals } }"}'

echo ""
echo "================ 10. A 设置自定义别名（需要钱包签名；无签名/错签名应拒绝） ================"
# 无签名 → 拒绝
run "{\"query\":\"mutation { updateReferralAlias(wallet: \\\"$ADDR_A\\\", alias: \\\"satoshin\\\", signature: \\\"\\\") { success warning } }\"}"
# 正确签名 → 成功
run "{\"query\":\"mutation { updateReferralAlias(wallet: \\\"$ADDR_A\\\", alias: \\\"satoshin\\\", signature: \\\"$SIG\\\") { success warning } }\"}"

echo ""
echo "================ 11. 别名可查 + 老链接仍可查 ================"
run "{\"query\":\"{ aliasUser: earlyAccessUserByReferralCode(referralCode: \\\"satoshin\\\") { wallet referralCode } oldLinkUser: earlyAccessUserByReferralCode(referralCode: \\\"$CODE\\\") { wallet } }\"}"

echo ""
echo "================ 12. 修改配置需要密码（无密码 → invalid password） ================"
run '{"query":"mutation { updateAntiSybilConfig(wallet: \"0xbbbb\", password: \"\", ipCheckEnabled: true, deferredSettlementEnabled: true) { success message } }"}'



echo ""
echo "================ 13. 错误密码 → invalid password ================"
run '{"query":"mutation { updateAntiSybilConfig(wallet: \"0xbbbb\", password: \"wrongpass\", ipCheckEnabled: true, deferredSettlementEnabled: true) { success message } }"}'

echo ""
echo "================ 14. 正确密码改配置成功 ================"
run '{"query":"mutation { updateAntiSybilConfig(wallet: \"0xbbbb\", password: \"testpass123\", ipCheckEnabled: true, deferredSettlementEnabled: true) { success message } }"}'

echo ""
echo ">>> done"
