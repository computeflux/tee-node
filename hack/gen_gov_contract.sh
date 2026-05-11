#!/usr/bin/env bash
# 从合约包 AST 生成 dao_gen.go（ExecCall / ExecQuery）及 side-chain/abis/dao.json（ink metadata 在内存中由 AST + 内建类型注册表构建，无默认 JSON 模板）。
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
exec go run ./tools/contractgen \
  -dir ./side-chain/contracts/gov \
  -out ./side-chain/contracts/gov/gov_gen.go \
  -mutation GovMutation \
  -query GovQuery \
  -struct Gov \
  -skip-mutation Delete
