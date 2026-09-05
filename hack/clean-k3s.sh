#!/bin/bash
# 清理 k3s 残留（本地 k3s 已废弃，node 已迁移到 docker）
# 需要 root 执行: sudo bash hack/clean-k3s.sh
set -e

# k3s 安装路径(可覆盖,默认标准安装位置)
K3S_CONTAINERD_SOCK="${K3S_CONTAINERD_SOCK:-/run/k3s/containerd/containerd.sock}"
K3S_DATA="${K3S_DATA:-/var/lib/rancher/k3s}"
K3S_ETC="${K3S_ETC:-/etc/rancher/k3s}"
# 用户级 kubeconfig 路径(可覆盖)
K3S_USER_KUBECONFIG="${K3S_USER_KUBECONFIG:-/home/wetee/.k3s.yaml}"

echo "==================================================="
echo " 清理 k3s 残留"
echo "==================================================="

# ── 1. 列出 k3s containerd 中的残留容器 ──
echo ""
echo "== [1/6] k3s containerd 中的残留容器 =="
K3S_CTR=$(ls $K3S_DATA/data/*/bin/ctr 2>/dev/null | head -1)
if [ -n "$K3S_CTR" ] && [ -S "$K3S_CONTAINERD_SOCK" ]; then
    $K3S_CTR --address $K3S_CONTAINERD_SOCK -n k8s.io containers list 2>/dev/null || echo "  (无法列出，containerd 可能已停止)"
else
    echo "  (未找到 k3s ctr 或 socket，跳过)"
fi

echo ""
echo "==================================================="
echo " 注意: 如果上方列表中有你需要保留的服务（如 picoclaw），"
echo " 请先 Ctrl-C 中断，迁移后再执行。"
echo "==================================================="
sleep 3

# ── 2. 停止并禁用 k3s 相关 systemd 服务 ──
echo ""
echo "== [2/6] 停止并禁用 k3s 服务 =="
for svc in k3s-ui k3s; do
    if systemctl list-unit-files | grep -q "^$svc.service"; then
        systemctl stop $svc.service 2>/dev/null || true
        systemctl disable $svc.service 2>/dev/null || true
        echo "  已停止并禁用 $svc.service"
    fi
done

# ── 3. 停止并删除 k3s containerd 中的所有容器 ──
echo ""
echo "== [3/6] 停止并删除 k3s 容器 =="
if [ -n "$K3S_CTR" ] && [ -S "$K3S_CONTAINERD_SOCK" ]; then
    # 先 kill 所有 task
    $K3S_CTR --address $K3S_CONTAINERD_SOCK -n k8s.io tasks kill --all --signal SIGKILL 2>/dev/null || true
    sleep 2
    # 再删除所有容器
    for cid in $($K3S_CTR --address $K3S_CONTAINERD_SOCK -n k8s.io containers list -q 2>/dev/null); do
        $K3S_CTR --address $K3S_CONTAINERD_SOCK -n k8s.io containers delete $cid 2>/dev/null || true
        echo "  已删除容器 $cid"
    done
    # 删除所有命名空间(含系统 ns)
    for ns in $($K3S_CTR --address $K3S_CONTAINERD_SOCK namespaces list -q 2>/dev/null); do
        $K3S_CTR --address $K3S_CONTAINERD_SOCK namespaces delete $ns 2>/dev/null || true
    done
else
    echo "  (无法连接 k3s containerd，将直接清理进程)"
fi

# ── 4. 清理残留进程（shim / kube-explorer）──
echo ""
echo "== [4/6] 清理残留进程 =="
# 所有 k3s 的 containerd-shim 孤儿进程
pkill -f "$K3S_DATA/data/.*/bin/containerd-shim" 2>/dev/null || true
# 残留的 kube-explorer（k3s-ui 的进程）
pkill -f "kube-explorer --kubeconfig=$K3S_USER_KUBECONFIG" 2>/dev/null || true
# k3s 的 containerd 主进程
pkill -f "$K3S_DATA/data/.*/bin/containerd" 2>/dev/null || true
sleep 2
echo "  已清理"

# ── 5. 备份数据目录与配置文件（可恢复，确认无误后再删）──
# 注意：节点链数据在 /srv/token_node*/token_chain_data（hostPath 挂载），不受影响。
# 这里只备份 k3s 自身的数据/配置，mv 改名保留，确认正常后可手动删除。
echo ""
echo "== [5/6] 备份数据目录与配置文件 =="
BAK="k3s.bak.$(date +%Y%m%d-%H%M%S)"
if [ -d "$K3S_DATA" ]; then
    mv "$K3S_DATA" "/var/lib/rancher/$BAK"
    echo "  已备份: $K3S_DATA -> /var/lib/rancher/$BAK"
fi
if [ -d "$K3S_ETC" ]; then
    mv "$K3S_ETC" "/etc/rancher/$BAK"
    echo "  已备份: $K3S_ETC -> /etc/rancher/$BAK"
fi
rm -rf /run/k3s
if [ -f "$K3S_USER_KUBECONFIG" ]; then
    mv "$K3S_USER_KUBECONFIG" "$K3S_USER_KUBECONFIG.bak"
    echo "  已备份: $K3S_USER_KUBECONFIG -> $K3S_USER_KUBECONFIG.bak"
fi
if [ -f /etc/systemd/system/k3s.service ]; then
    mv /etc/systemd/system/k3s.service /etc/systemd/system/k3s.service.bak
    echo "  已备份: k3s.service -> k3s.service.bak"
fi
if [ -f /etc/systemd/system/k3s-ui.service ]; then
    mv /etc/systemd/system/k3s-ui.service /etc/systemd/system/k3s-ui.service.bak
    echo "  已备份: k3s-ui.service -> k3s-ui.service.bak"
fi

# ── 6. 刷新 systemd ──
echo ""
echo "== [6/6] 刷新 systemd =="
systemctl daemon-reload
echo ""

echo "==================================================="
echo " k3s 清理完成（所有内容已备份，未删除）"
echo " 备份位置:"
echo "   /var/lib/rancher/$BAK"
echo "   /etc/rancher/$BAK"
echo "   $K3S_USER_KUBECONFIG.bak"
echo "   /etc/systemd/system/k3s*.service.bak"
echo ""
echo " 确认一切正常后，彻底删除备份:"
echo "   sudo rm -rf /var/lib/rancher/$BAK /etc/rancher/$BAK \\"
echo "            $K3S_USER_KUBECONFIG.bak /etc/systemd/system/k3s*.service.bak"
echo "==================================================="
