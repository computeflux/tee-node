#!/usr/bin/env bash
# 依赖漏洞扫描：用 govulncheck 检测当前模块（默认 tee-node）的已知漏洞。
#
# 用法：
#   ./hack/scan-vuln.sh                    # 扫描当前模块（tee-node）
#   ./hack/scan-vuln.sh ../actives         # 扫描指定模块目录
#   ./hack/scan-vuln.sh --all              # 扫描 apps/ 下所有 Go 模块
#   ./hack/scan-vuln.sh --json             # JSON 输出（供 CI / 工具消费）
#
# 退出码：0 = 无漏洞，1 = 发现漏洞，2 = 扫描失败
set -euo pipefail
cd "$(dirname "$0")/.."

# ── 定位 govulncheck ──────────────────────────────────────────────
GOVULNCHECK="$(command -v govulncheck 2>/dev/null || true)"
if [[ -z "$GOVULNCHECK" ]]; then
  GC_BIN="$(go env GOPATH)/bin/govulncheck"
  if [[ -x "$GC_BIN" ]]; then
    GOVULNCHECK="$GC_BIN"
  else
    echo ">> govulncheck 未安装，正在安装 ..."
    go install golang.org/x/vuln/cmd/govulncheck@latest
    GOVULNCHECK="$(go env GOPATH)/bin/govulncheck"
  fi
fi

# ── 收集要扫描的模块目录 ──────────────────────────────────────────
MODULES=()
SCAN_JSON=0

case "${1:-}" in
  --all)
    # apps/ 下所有含 go.mod 的兄弟模块
    APPS_DIR="$(cd "$(dirname "$0")/../.." && pwd)"
    while IFS= read -r f; do
      MODULES+=("$(dirname "$f")")
    done < <(find "$APPS_DIR" -maxdepth 2 -name go.mod -type f 2>/dev/null | sort)
    ;;
  --json)
    SCAN_JSON=1
    MODULES=(".")
    ;;
  "")
    MODULES=(".")
    ;;
  *)
    MODULES=("$1")
    ;;
esac

# ── 去重 ──────────────────────────────────────────────────────────
if [[ ${#MODULES[@]} -gt 1 ]]; then
  mapfile -t MODULES < <(printf '%s\n' "${MODULES[@]}" | sort -u)
fi

# ── 逐个扫描 ──────────────────────────────────────────────────────
TOTAL_VULN=0
FAILED=0

for mod in "${MODULES[@]}"; do
  if [[ ! -f "$mod/go.mod" ]]; then
    echo "!! 跳过 $mod（无 go.mod）"
    continue
  fi

  mod_name="$(basename "$mod")"
  echo ""
  echo "══════════════════════════════════════════════"
  echo "  扫描: $mod_name  ($mod)"
  echo "══════════════════════════════════════════════"

  if [[ $SCAN_JSON -eq 1 ]]; then
    # JSON 模式：直接输出 govulncheck JSON
    (cd "$mod" && "$GOVULNCHECK" -json ./... 2>&1) || true
    continue
  fi

  # 文本模式：提取漏洞摘要
  output="$(cd "$mod" && "$GOVULNCHECK" -show verbose ./... 2>&1)" || true

  # 提取 "Your code is affected by N vulnerabilities" 行
  affected_line="$(echo "$output" | grep -oP 'Your code is affected by \K[0-9]+' | head -1 || true)"
  affected="${affected_line:-0}"

  if [[ "$affected" -gt 0 ]]; then
    TOTAL_VULN=$((TOTAL_VULN + affected))
    echo ""
    echo "$output" | grep -E '^(Vulnerability #|    Module:|    Found in:|    Fixed in:|    More info:)' | sed 's/^/  /'
    echo ""
    echo "  ⚠️  $mod_name: $affected 个漏洞影响代码"
  else
    # 检查是否有 "No vulnerabilities found"
    if echo "$output" | grep -q 'No vulnerabilities found'; then
      echo "  ✅ $mod_name: 无漏洞"
    else
      # 可能扫描出错
      echo "  ❓ $mod_name: 无法确认（govulncheck 输出异常）"
      echo "$output" | tail -5 | sed 's/^/  /'
      FAILED=1
    fi
  fi
done

# ── 汇总 ──────────────────────────────────────────────────────────
echo ""
echo "══════════════════════════════════════════════"
if [[ $FAILED -ne 0 ]]; then
  echo "  扫描完成（部分模块异常）"
  echo "══════════════════════════════════════════════"
  exit 2
elif [[ $TOTAL_VULN -gt 0 ]]; then
  echo "  共发现 $TOTAL_VULN 个影响代码的漏洞"
  echo "  修复建议: go get <module>@<fixed-version> && go mod tidy"
  echo "══════════════════════════════════════════════"
  exit 1
else
  echo "  ✅ 全部模块无已知漏洞"
  echo "══════════════════════════════════════════════"
  exit 0
fi
