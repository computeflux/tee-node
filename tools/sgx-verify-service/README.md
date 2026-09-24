# sgx-verify-service

ego eclient 的独立 HTTP 封装：非 TEE 节点（`IsEgo=false`）通过本服务验证
SGX report。验证逻辑由 ego 自带的 `eclient` 完成（cgo 静态链接
`/opt/ego/lib/liboehostverify.a`，参考
`samples/remote_attestation/ra_client/client.go`），部署为本地 systemd 服务，
不走 docker。服务无状态、fail-closed。

## 构建

需要装有 ego 的机器（cgo，头文件/库在 `/opt/ego`）：

```bash
cd apps/tee-node
CGO_CFLAGS=-I/opt/ego/include CGO_LDFLAGS=-L/opt/ego/lib go build -o sgx-verify-service ./tools/sgx-verify-service/
```

liboehostverify 为静态库，编译期链接，运行时只依赖系统 libcrypto/libc。

## 运行

```bash
./sgx-verify-service -addr 0.0.0.0:8004
```

| 参数 | 环境变量 | 默认值 |
|---|---|---|
| `-addr` | `SGX_VERIFY_SERVICE_ADDR` | `:8004` |
| `-max-concurrent` | `SGX_VERIFY_MAX_CONCURRENT` | `16`（并发上限，防止 DoS） |

## API

`POST /verify`

```json
{ "report": "<base64 原始 report 字节>" }
```

- `200`：`attestation.Report` JSON（`Data`/`UniqueID`/`SignerID`/`ProductID`
  为 base64，`TCBStatus` 为数字），与 `sgx-verify` 输出的 report 段完全一致，
  可直接 `json.Unmarshal` 进 `attestation.Report`。
- `400`：请求非法（非 base64 / 空 / 超大）
- `502`：sgx-verify 执行或验证失败（body 为错误信息，含 stderr）
- `503`：并发已满
- `504`：验证超时

`GET /healthz` → `ok`

示例：

```bash
curl -s localhost:8004/verify -d "{\"report\":\"$(base64 -w0 report.bin)\"}"
```

## 本地部署（systemd，不用 docker）

```bash
sh hack/sgx-verify-service-deploy.sh
```

- 构建二进制并安装到 `/usr/local/bin/sgx-verify-service`
- 创建 systemd 单元 `sgx-verify-service.service`（`Restart=always`），监听 `0.0.0.0:8004`
- 本机走 `127.0.0.1:8004`，远程节点走 `http://xiaobai.asyou.me:8004`

## 调用方

`ClientSgxVerify` 调用 `<SGX_VERIFY_SERVICE_URL>/verify`，统一用本地服务器上的服务：
`http://xiaobai.asyou.me:8004`（服务监听 `0.0.0.0:8004`）。

- 代码默认值、node1（`SVS_HOST`）、node2（`-e SGX_VERIFY_SERVICE_URL`）都用这个地址
- 可用 `SVS_HOST` 环境变量覆盖（见各部署脚本）

前置条件（网络侧）：`xiaobai.asyou.me` 的 DNS/DDNS 要指向本地服务器的公网 IP，
且路由器要有 TCP 8004 的端口映射；本机可加 `/etc/hosts` 条目指向 127.0.0.1 便于自测。

环境变量需在 `hack/build/enclave.json` 的 `env` 白名单里（已加），否则 ego run
不会把它传进 enclave。

服务不可达或验证失败时验证失败（fail-closed）。
