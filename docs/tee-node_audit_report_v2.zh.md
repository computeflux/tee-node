# Tee-Node 项目代码审计报告

## 1. 执行摘要

### 整体代码质量评估

Tee-Node 项目是一个基于 Go 语言的去中心化可信执行环境（TEE）节点，实现了阈值密码学（DKG/DSS）并与区块链网络（EVM 和 Substrate）集成。尽管架构高度复杂且涵盖了复杂的交互场景，但代码库存在若干**关键安全漏洞**、**严重并发缺陷**以及**部署阻塞问题**，在任何生产环境或公开预发布环境部署前必须予以解决。

### 最严重风险

| 优先级 | 问题描述 |
|--------|----------|
| **关键** | **生产环境双重解封缺陷**：数据库检索函数 `GetProtoMessage` 中存在关键逻辑错误，导致在正常生产环境（非测试环境）下执行失败，节点无法检索任何状态（DKG 密钥、验证者更新等）。 ✅ 已修复 |
| **关键** | **仓库中提交的明文私钥**：`hack/build/keys/private.pem` 中以明文形式提交了私钥，若在生产环境中复用将立即导致加密身份/签名被攻破。 ❌ 仍存在 |
| **高** | **API 网关过期绕过**：API 网关中间件验证 API 密钥时，绕过了本地缓存密钥的过期检查，允许过期令牌继续访问受限端点。 ✅ 已修复 |
| **高** | **GraphQL 配置泄漏**：公开查询返回 OAuth 客户端密钥和 SMTP 密码，攻击者可通过掩码推断出密钥的首尾字符，大幅降低暴力破解熵值。 ⚠️ 部分修复（已脱敏但未从 schema 移除） |
| **高** | **P2P 未认证写入（死代码）**：P2P 订阅处理程序在"secret"主题下直接将键值对写入数据库，未验证发送者身份或签名。虽然目前未使用，但代表了巨大的潜在安全漏洞。 ✅ 已修复 |
| **高** | **并发数据竞争与 Panic**：`PersistChan` 存在锁外不安全并发写入，导致队列状态不同步、潜在 Panic 和无限内存泄漏。`GetNodes` 在未获取读锁的情况下访问共享合约连接。 ✅ 已修复 |
| **高** | **可预测的恢复 OTP 与投递失败**：恢复 OTP 使用 `time.Now().UnixNano() % 1000000` 生成，高度可预测，且 OTP 仅存入 Pebble 却从未通过邮件发送。 ✅ 已修复 |

### 可运行/可测试性评估

- **当前无法在 macOS 上测试**：标准 macOS 编译因 SGX cgostubs 包中 `qsort_r` 定义冲突而失败，缺少构建标签限制。
- **损坏的相对导入路径**：`go.mod` 引用了 `../../wetee/ink.go`，该路径在仓库外部，标准环境中不存在。
- **SGX 密封要求**：密封逻辑在非 SGX 开发机上自动尝试执行 Intel SGX 指令而崩溃。
- **Go 工具链下载被阻止**：`go test ./...` 因沙箱策略阻止写入工具链缓存而失败。
- 项目中仍有 **49 个测试文件**，表明存在一定覆盖率，但当前无法验证通过/失败状态。

### 优先审查领域

1. GraphQL 认证/配置暴露
2. 密钥管理授权
3. 签名/令牌过期逻辑
4. 恢复流程
5. 部署/启动假设

---

## 2. 潜在缺陷

### [缺陷 1] GetProtoMessage 双重解封（生产环境阻塞） ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `pkg/store/db.go` |
| **函数** | `GetProtoMessage[T any]` |
| **严重级别** | **关键** |

**描述**：`GetProtoMessage` 调用 `GetKey` 检索数据后，再次调用 `util.Unseal`：

```go
v, err := GetKey(namespace, key) // <-- 内部已调用 util.Unseal(value, nil)！
...
value, err := util.Unseal(v, nil) // <-- 对解密后的明文再次解封！
```

**失败原因**：在生产环境中，`util.Unseal` 调用 EGo 的 `ecrypto.Unseal`。尝试将明文当作密文解密会导致解密错误。这导致所有 `GetProtoMessage` 调用失败，阻塞 DKG 状态和纪元转换。测试通过仅因为在 `go test` 下 `Unseal` 是空操作。

**修复措施**：已移除 `GetProtoMessage` 内部的第二次 `util.Unseal` 调用，改为直接调用 `protoio.ReadMessage(bytes.NewBuffer(v), val)` 反序列化。

---

### [缺陷 2] EVM GetNodes 不安全互斥锁处理 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `pkg/chains/evm/subnet.go` |
| **函数** | `GetNodes` |
| **严重级别** | **高** |

**描述**：与其他 EVM 查询（如 `GetValidatorList`）不同，`GetNodes` 在未获取 `e.mu.RLock()` 的情况下从 `e.subnet` 检索 workers 和 secrets。

**失败原因**：如果连接断开且另一线程执行 `reconnect()`，它会获取 `e.mu.Lock()` 来关闭并替换 `e.client` 和 `e.subnet`。如果 `GetNodes` 并发执行，它会访问 nil 或已关闭的客户端，导致数据竞争或空指针 Panic。

**修复措施**：在 `GetNodes` 的 `GetWorkers` 和 `GetSecrets` 合约调用外包裹 `e.mu.RLock() / e.mu.RUnlock()`，并添加与项目中其他查询方法一致的 `reconnect()` 错误重试机制。

---

### [缺陷 3] PersistChan 并发竞争、Panic 与内存泄漏 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `pkg/store/persist_channel.go` |
| **类型** | `PersistChan[T]` |
| **严重级别** | **高** |

**描述**：

1. **队列竞争**：`Push` 中写入 `c.listChan <- msg` 发生在 `c.mu` 锁外。并发推送可能导致消息进入 `c.listChan` 的顺序与追加到 `c.list` 的顺序不同。由于 `Start` 从队首弹出元素（`c.list[1:]`），会弹出错误的项目，导致数据损坏和数据库不同步。
2. **切片 Panic**：`c.list = c.list[1:]` 未检查 `c.list` 是否为空，如果队列状态不同步会导致索引越界 Panic。
3. **内存泄漏**：通过 `c.list[1:]` 弹出元素不会缩小底层数组容量或清除已解引用元素，导致内存使用量无限增长。
4. **关闭通道 Panic**：`Stop()` 在未加锁的情况下关闭 `c.listChan`，导致任何飞行中或后续的 `Push()` 调用在通道写入时 Panic。

**修复措施**：
- 引入内部 `sender()` goroutine 和 `in` channel，将所有发送到 `listChan` 的操作串行化，确保 `listChan` 中的消息顺序与 `list` 一致
- `Push()` 中先 `append` + `save`，再在锁内发送到 `in`，保证顺序一致性
- `Start()` 中处理数据前先检查 `len(c.list) > 0`，避免切片越界 Panic
- `Start()` 弹出元素时使用 `make` + `append` 重新分配切片，避免内存泄漏
- `Stop()` 加锁并设置 `closed` 标志后再关闭 `in`，避免并发关闭 Panic

---

### [缺陷 4] 可预测的恢复 OTP 与投递失败 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/auth.resolvers.go` |
| **函数** | `RequestRecovery` |
| **严重级别** | **高** |

**描述**：OTP 使用 `time.Now().UnixNano() % 1000000` 生成，高度可预测。更重要的是，OTP 存入 Pebble 却从未通过电子邮件发送或在 GraphQL 响应中返回。

**失败原因**：用户可以请求恢复，但他们永远不会收到验证和完成恢复所需的 OTP 代码。

**修复措施**：
- 使用 `authsvc.GenerateNumericOTP(6)`（基于 `crypto/rand` 的 CSPRNG）替代可预测的 `UnixNano() % 1000000`
- 调用 `r.EmailSvc.SendOTP(recoveryEmail, otp)` 将 OTP 实际投递到恢复邮箱
- OTP 存储在 Pebble 中并设置 10 分钟 TTL

---

### [缺陷 5] ResolveSignerEmail 忽略令牌过期 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/resolver.go` |
| **函数** | `ResolveSignerEmail` |
| **严重级别** | **高** |

**描述**：API 密钥解析仍然接受本地存储的令牌（如果 `IsActive` 为 true），但不检查过期时间。

**失败原因**：过期的本地 API 密钥仍可用于 `signTransaction` 和 `signMessage`。

**修复措施**：`ResolveSignerEmail` 已改为调用 `store.FindGatewayTokenByKey()`（内部已检查 `IsActive + IsExpired`），并额外显式检查 `!tk.IsExpired()` 作为双重保险，消除与 `resolveEmailFromToken()` 的验证漂移。

---

### [缺陷 6] OTPService 清理协程泄漏 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `pkg/services/auth/otp.go` |
| **函数** | `NewOTPService` |
| **严重级别** | **中** |

**描述**：`NewOTPService` 启动了一个带有 `for range ticker.C` 循环的后台清理协程，但没有提供停止它或关闭计时器的机制。

**失败原因**：在动态重新创建服务的环境中（如单元测试），这会无限泄漏协程，最终耗尽系统资源。

**修复措施**：
- `NewOTPService` 的签名改为 `NewOTPService(ctx context.Context, ttlMins int) *OTPService`
- 清理协程改为 `select { case <-ctx.Done(): return; case <-ticker.C: s.cleanup() }`，上下文取消时安全退出
- `main.go` 传入 `context.Background()`（在收到终止信号时通过取消根上下文传播）
- 测试使用 `setupTestOTP` 初始化 Pebble store，并新增了 `TestOTPService_ContextCancel_StopsCleanupGoroutine` 验证取消后不会 Panic 且 goroutine 退出

---

### [缺陷 7] OTP 生成存储 clientID 但验证忽略它 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/auth.resolvers.go`, `pkg/services/auth/otp.go` |
| **函数** | `VerifyOtp`, `OTPService.Verify` |
| **严重级别** | **中** |

**描述**：OTP 生成存储 `clientID`，但验证忽略它。

**失败原因**：尽管公共 API 暗示 OTP 是客户端绑定的，OTP 仍可在客户端或应用上下文之间重放。

**修复措施**：`OTPService.Generate` 将 `clientID` 拼接入存储值（`fmt.Sprintf("%d:%s:%s", expiresAt, otp, clientID)`）。`OTPService.Verify` 验证时强制匹配 `storedClientID != "" && storedClientID != clientID` 则拒绝，防止跨客户端重放。

---

### [缺陷 8] 密钥上传 RSA 密钥对在进程启动时生成且未持久化 ❌ 未修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/mod.go`, `graph/secret.resolvers.go` |
| **函数** | 包初始化, `SecretRsa`, `UploadSecret` |
| **严重级别** | **中** |

**描述**：用于密钥上传的 RSA 密钥对仍在进程启动时在内存中生成，从未持久化。

**失败原因**：客户端可以获取一个公钥，服务器重启后，由于加载了不同的私钥，后续解密失败。

**重现步骤**：获取 `secret_rsa`，准备加密载荷，重启进程，然后提交 `upload_secret`。

**建议修复**：有意识地持久化和轮换 RSA 密钥对，或对密钥进行版本控制并显式拒绝过时的密文。

---

### [缺陷 9] ListenAndServe / ListenAndServeTLS 忽略返回错误 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/mod.go`, `main.go` |
| **函数** | `StartServerWithResolver` |
| **严重级别** | **中** |

**描述**：`ListenAndServe` 和 `ListenAndServeTLS` 仍然忽略返回的错误。

**失败原因**：端口冲突或 TLS 文件问题可能导致启动失败，但无法结构化传播。

**修复措施**：
- `StartServerWithResolver` 改为返回 `error`，在 goroutine 中启动 `http.Server` 并通过 channel 接收启动错误
- `main.go` 中的调用方启动 goroutine 并打印服务器错误
- 启动失败（端口冲突、证书错误等）现在会被调用方感知

---

### [缺陷 10] 用户记录通过 fire-and-forget 协程异步写入链 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/auth.resolvers.go` |
| **函数** | `VerifyOtp`, `SocialLogin`, `SetPassword` |
| **严重级别** | **中** |

**描述**：用户记录仍通过 fire-and-forget 协程异步写入链，且错误被忽略。

**失败原因**：如果异步写入失败，跨节点状态可能在静默中发生分歧。

**修复措施**：
- 将三个调用点（`VerifyOtp`、`SocialLogin`、`SetPassword`）中的匿名 `go func()` 替换为统一的 `persistAuthUserToChain(chain, user)` 辅助函数
- `persistAuthUserToChain` 保留后台 goroutine 以避免阻塞登录响应，但不再静默忽略错误：
  - JSON 序列化失败会记录 `[AUTH] failed to marshal auth user ...`
  - 链上写入带 3 次指数退避重试（0ms / 500ms / 1000ms）
  - 最终失败记录 `[AUTH] failed to persist auth user ... after 3 retries: <error>`
- 引入 `authStoreAuthUserFn` 函数变量支持测试替换
- 新增测试 `TestPersistAuthUserToChain_RetryAndObservability` 验证失败时会重试至少 3 次并输出日志
- 新增测试 `TestPersistAuthUserToChain_NilChain_NoPanic` 验证 nil chain/user 安全

---

### [缺陷 11] Order-token 签发未实现 ❌ 未修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/auth.resolvers.go`, `graph/rest_handler.go` |
| **函数** | `ClaimOrderToken`, REST `/api/blockchain/token` |
| **严重级别** | **中** |

**描述**：Order-token 签发功能仍然未实现。

**失败原因**：依赖此集成的客户端将遇到运行时死胡同。

**重现步骤**：调用任一路由/变更。

**建议修复**：在实现完成前隐藏这些 API，或完成签名/订单验证逻辑。

---

### [缺陷 12] 共识清理原子性 TODO ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `tee-chain/tx_finalize.go`, `tee-chain/tx_partial_sign_rev.go`, `tee-chain/tx_to_realy.go` |
| **函数** | 交易 finalize / prepare 流程 |
| **严重级别** | **中** |

**描述**：`FinalizeTx` 中的清理操作（`DeleteSigOfTx`、`deleteBatchCallStore`）直接修改全局 store，而不是通过传入的 `txn` 事务执行。

**失败原因**：
- `SyncTxStart` 中先调用 `DeleteSigOfTx` 清理全局 store，再执行 `HubSyncStep2`；如果后者失败，`FinalizeTx` 返回 error，调用方不提交 `txn`，但清理已生效。
- `SyncTxEnd` 中 `HubSyncEnd` 和 batch 状态更新在 `txn` 内，但最后的 `DeleteSigOfTx` 和 `deleteBatchCallStore` 跳出事务直接写全局 store；如果在清理后、`txn.Commit()` 前崩溃，会出现"副存储已清理但链状态未更新"的不一致。
- 清理错误被 `_ = ...` 忽略。

**修复措施**：
1. `DeleteSigOfTx` 改为接受 `*dbstore.Txn` 参数，使用 `txn.DeletekeysByPrefix` 替代 `store.DeletekeysByPrefix(GLOABL_STATE, ...)`
2. `deleteBatchCallStore` 改为接受 `*dbstore.Txn` 参数并返回 error，使用 `txn.Delete` 替代 `store.DeleteKey(GLOABL_STATE, ...)`
3. `tx_finalize.go` 中的两个调用点均传入 `txn`，错误不再被忽略，失败时返回 error 触发事务回滚
4. 所有清理操作现在与链状态变更在同一个 `txn` 中，只有 `txn.Commit()` 成功后才会一起持久化

---

## 3. 安全风险

### [风险 1] 仓库中提交的明文私钥 ❌ 未修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `hack/build/keys/private.pem` |
| **严重级别** | **关键** |

**风险**：明文私钥被硬编码/提交到 Git 仓库。如果在生产环境中复用，将立即导致加密身份/签名被攻破。

**建议修复**：立即轮换密钥，使用 `git-filter-repo` 等工具从仓库和历史记录中删除它，并通过环境变量或安全密钥管理器在运行时注入密钥。

---

### [风险 2] GraphQL 配置泄漏（SMTP 与 OAuth 密钥）⚠️ 部分修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/auth.graphqls`, `graph/auth.resolvers.go` |
| **函数** | `AuthConfig` 查询 |
| **严重级别** | **高** |

**风险**：`AuthConfig` 查询返回 `smtpPassword`、`googleClientSecret`、`githubClientSecret` 和 `twitterClientSecret`。合约的 `GetConfig` 使用 `maskIfNonEmpty`，但仅对字符串中间进行掩码处理：

```go
return s[:2] + "****" + s[len(s)-2:] // 例如："se****et"
```

这向未认证的公共 API 调用者泄露了生产密钥的首尾两个字符，严重降低了暴力破解的熵值。

**当前状态**：`AuthGetPubConfig` 已增加脱敏逻辑，敏感字段（密码、secret）被 `maskIfNonEmpty` 替换为 `****` 或部分遮蔽。但密钥字段仍然存在于公共 GraphQL schema 中，建议在模型层完全移除而非仅脱敏。

---

### [风险 3] API 密钥验证中的过期绕过 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `pkg/api/gateway/cache.go`, `graph/auth_helpers.go` |
| **函数** | `FindTokenByKey`, `resolveEmailFromToken` |
| **严重级别** | **高** |

**描述**：当从本地存储（PebbleDB）解析 API 密钥时，`FindTokenByKey` 检查 `!tk.IsActive` 但**未检查** `tk.IsExpired()`。

**风险**：过期的 API 令牌仍然有效，继续授权 API 网关访问。

**修复措施**：
- `pkg/api/gateway/cache.go` 中的 `FindTokenByKey` 在本地 PebbleDB fallback 路径中新增 `tk.IsExpired()` 检查
- 过期 token 现在会被拒绝并记录 `[AUTH] token expired for ...` 日志，与 `store.FindGatewayTokenByKey` 的行为保持一致
- 新增 `pkg/api/gateway/cache_test.go` 覆盖四种场景：过期拒绝、活跃未过期通过、永不过期（ExpiresAt=0）通过、未激活拒绝

---

### [风险 4] 密钥与磁盘密钥缺少所有权检查 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/secret.resolvers.go`, `graph/auth.go` |
| **函数** | `UploadSecret` / `InitDiskKey` |
| **严重级别** | **高** |

**风险**：这些变更验证 API 密钥有效，但不检查 API 密钥的所有者是否与正在更新的目标用户地址匹配。这允许任何认证用户写入或覆盖其他用户的密钥。

**修复措施**：
1. 在 `graph/auth.go` 中新增导出的 `GatewayTokenFromContext(ctx)` 辅助函数，便于 resolver 获取当前认证 token
2. 在 `graph/secret.resolvers.go` 中新增 `checkSecretOwnership(ctx, user)`，比较 `user` 参数解析出的 `cstore.UniAddr` 与 `token.Addr`
3. `UploadSecret` 和 `InitDiskKey` 在执行业务逻辑前均调用 `checkSecretOwnership`，地址不匹配时返回 `"user address does not match authenticated token owner"`
4. 新增 `graph/secret_test.go` 覆盖三种场景：无 token、地址不匹配、地址匹配

---

### [风险 5] 过于宽泛的 CORS 配置 ❌ 未修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/mod.go` |
| **严重级别** | **高** |

**描述**：CORS 仍然配置为 `AllowedOrigins: []string{"https://*", "http://*"}` 且 `AllowCredentials: true`。

**风险**：过于宽泛的跨源认证访问面。

**建议修复**：使用显式的前端允许列表，避免在携带凭证时使用通配符来源。

---

### [风险 6] setAuthConfig 缺少 API 层授权 ❌ 未修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/auth.graphqls`, `graph/auth.resolvers.go` |
| **函数** | `setAuthConfig` |
| **严重级别** | **高** |

**描述**：`setAuthConfig` 仍然没有模式级认证指令或可见的 API 层授权检查。

**风险**：如果更深层的合约端执行不完整，可能导致配置被误写或未经授权写入。

**建议修复**：在 GraphQL 层添加显式的特权认证检查。

---

### [风险 7] 钱包登录缺少 nonce/重放保护 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/auth.resolvers.go` |
| **函数** | `LoginWithWallet` |
| **严重级别** | **高** |

**描述**：钱包登录仍然验证任意签名消息，而没有服务器颁发的 nonce 或重放跟踪。

**风险**：签名重放攻击。

**修复措施**：新增 `RequestWalletNonce` 生成随机 nonce 并存储 300 秒。`LoginWithWallet` 实现了完整保护：
1. 验证 nonce 存在且未过期
2. 检查 nonce 未被使用过
3. 验证 EIP-191 签名
4. 验证通过后立即标记 nonce 已使用

---

### [风险 8] 未认证的 P2P 写入接口（死代码） ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `pkg/dkg/handler.go` |
| **函数** | `handleSecretSave` |
| **严重级别** | **高** |

**描述**：订阅 P2P 网络上的 "secret" 主题，直接将键值对写入本地 "secret" 命名空间存储，而不验证发送者身份或签名。

**风险**：公共 P2P 网络上的攻击者可以发送恶意 "secret" 消息来覆盖或损坏节点内部数据。（注：当前为死代码，但如果激活则很危险）。

**修复措施**：已删除 `pkg/dkg/handler.go` 中的 `handleSecretSave` 函数及其未使用的导入（`errors`、`domain`、`store`）。

---

### [风险 9] Panic 响应泄露内部信息 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `pkg/api/gateway/middleware.go` |
| **严重级别** | **中** |

**风险**：Panic 响应仍然向客户端返回 `panic: <message>`。

**修复措施**：`RecoveryMiddleware` 中 panic 详情（含堆栈）仅记录服务端日志，客户端收到通用 `"internal server error"`，无信息泄露。

---

### [风险 10] STARTTLS 路径缺少显式 TLS 升级 ❌ 未修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `pkg/services/auth/email.go` |
| **严重级别** | **中** |

**描述**："STARTTLS" 路径仍然只使用 `smtp.SendMail`，没有显式的 TLS 升级处理。

**风险**：注释中的安全假设可能与真实传输保证不匹配。

**建议修复**：执行显式 SMTP 协商，或准确重命名/记录行为。

---

### [风险 11] generateAPIKey() 忽略 rand.Read 错误 ✅ 已修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `graph/auth_helpers.go` |
| **严重级别** | **中** |

**描述**：`generateAPIKey()` 和 `generateWalletNonce()` 仍然忽略 `rand.Read` 错误。

**风险**：在极少数失败情况下，令牌生成可能继续使用弱熵或零熵。

**修复措施**：
- `generateAPIKey` 和 `generateWalletNonce` 的签名从 `() string` 改为 `(string, error)`
- 显式检查 `rand.Read` 返回值，失败时返回 `fmt.Errorf("failed to generate ... entropy: %w", err)`
- 引入 `randReadFn` 函数变量支持测试替换
- `createUserAPIKey` 现在返回 `(string, error)`，将生成/存储错误向上传播
- 更新所有 4 处 `createUserAPIKey` 调用方（`VerifyOtp`、`SocialLogin`、`ResetPassword`、`LoginWithWallet`）处理错误并返回给客户端
- `RequestWalletNonce` 现在处理 `generateWalletNonce()` 的错误
- 新增测试覆盖成功路径和模拟 `rand.Read` 失败的错误路径

---

## 4. 运行时与部署问题

### [问题 1] 可移植性 / Mac 构建失败（CGO 签名冲突） ❌ 未修复 不兼容MAC

| 属性 | 内容 |
|------|------|
| **文件路径** | `pkg/util/cgostubs/qsort_r.go` |
| **严重级别** | **高** |

**描述**：实现了 GNU 风格签名的 `qsort_r` 来 stub Intel SGX 安全区内缺失的 musl libc 函数。然而，该签名与 macOS 标准 `<stdlib.h>` 中声明的 BSD 风格 `qsort_r` 冲突。

**影响**：项目在 macOS 上启用 CGO 时无法构建或测试。

**建议修复**：使用构建标签将 stub 编译限制在 Linux：

```go
//go:build linux && !darwin
```

---

### [问题 2] go.mod 中无效的 replace 指令 ❌ 未修复

| 属性 | 内容 |
|------|------|
| **文件路径** | `go.mod` |
| **严重级别** | **高** |

**描述**：包含：`replace github.com/wetee-dao/ink.go => ../../wetee/ink.go`

**影响**：在特定工作空间布局之外编译会失败，因为 `../../wetee/ink.go` 不存在。

**建议修复**：提交本地副本或引用特定的 git 修订版本。

---

### [问题 3] Intel SGX 密封在非 SGX 环境崩溃 ❌ 未修复 不兼容非SGX环境

| 属性 | 内容 |
|------|------|
| **文件路径** | `pkg/util/tee_seal.go` |
| **严重级别** | **高** |

**描述**：数据库写入依赖 `SealWithProductKey`，如果 `flag.Lookup("test.v") == nil`，它会调用 EGo 的 SGX `ecrypto.SealWithProductKey`。

**影响**：在非 SGX 开发机上本地运行节点（单元测试之外）会导致数据库写入失败，使开发/本地运行变得不可能。

**建议修复**：检查检测到的 TEE 类型（`teetypes.TeeType == 9999`）或环境变量（如 `CHAIN_ENV=local`），以在非安全区运行时绕过硬件加密。

---

### [问题 4] Go 工具链下载依赖 ❌ 未修复 AI幻觉

| 属性 | 内容 |
|------|------|
| **文件路径** | `go.mod` |
| **严重级别** | **高** |

**描述**：仓库目标 Go 1.26.3，该版本当前不存在（截至检查日期最新稳定版为 1.24.x）。Go 1.21+ 会尝试自动下载该工具链，导致 `toolchain not available` 错误。

**影响**：构建/测试可重复性仍然依赖外部工具链引导。

**建议修复**：将 `go.mod` 中的 Go 版本修正为实际可用的稳定版本（如 1.23.x 或 1.24.x）。

---

### [问题 5] LoadConfig 环境变量回退失效 ❌ 未修复 不需要环境回退

| 属性 | 内容 |
|------|------|
| **文件路径** | `pkg/api/auth/config.go` |
| **严重级别** | **高** |

**描述**：`LoadConfig` 注释声称回退到环境变量，但链上合约读取失败时直接返回零值 `&AuthConfig{}`，没有任何从 `os.Getenv` 读取环境变量并填充字段的逻辑。

**影响**：配置错误的部署可能在静默中以空密钥、模拟邮件回退和弱钱包派生输入运行。

**建议修复**：实现实际的环境加载或快速失败。

---

### [问题 6] 启动依赖硬编码相对文件 ❌ 未修复 这是区块链创世文件,必须有

| 属性 | 内容 |
|------|------|
| **文件路径** | `main.go` |
| **严重级别** | **高** |

**描述**：启动仍然依赖 `./token_chain_data/...` 下的硬编码相对文件（如 `priv_validator_key.json`、`priv_validator_state.json`）。

**影响**：进程对工作目录和缺失的启动资源很脆弱。

**建议修复**：将这些路径外部化并在启动时验证它们。

---

### [问题 7] 本地 Pebble DB 路径硬编码 ❌ 未修复 这也是必须的

| 属性 | 内容 |
|------|------|
| **文件路径** | `pkg/store/db.go` |
| **严重级别** | **中** |

**描述**：本地 Pebble DB 路径仍然硬编码为 `./token_chain_data/wetee`。

**影响**：部署可移植性和容器化路径控制仍然较弱。

**建议修复**：使 DB 路径可配置。

---

### [问题 8] 正常启动调用调试运行时加载 ❌ 已修复 

| 属性 | 内容 |
|------|------|
| **文件路径** | `main.go` |
| **严重级别** | **中** |

**描述**：`LoadRuntimeForDebug()` 仍在正常启动时被无条件调用。

**影响**：生产启动仍然似乎依赖于面向调试的运行时路径。

**建议修复**：分离调试和生产运行时加载行为。

---

## 5. 测试与可靠性

### 已知缺口与脆弱性

- **无测试空间**：本地系统磁盘空间不足（低于 250MB 空闲），阻止了依赖下载。
- **测试绕过缺陷**：单元测试掩盖了关键的双重解封和 SGX 密封缺陷，因为 `test.v` 将加密密封转换为空操作。测试需要覆盖模拟加密编解码器。
- **非原子操作**：最终确定块存储在 Pebble 事务批次之外完成，如果进程在中间终止，允许数据库状态不一致。

### 可靠性红旗

1. **异步链写入且忽略错误**：`graph/auth.resolvers.go` ❌ 未修复
2. **忽略随机生成错误**：`graph/auth_helpers.go` ❌ 未修复
3. **内存中的速率限制和 OTP 计数器在重启时重置**：`pkg/services/auth/otp.go` ⚠️ 部分修复（日发送上限已持久化，但 `sendRecords` 和 `verifyFailCount` 仍为内存 map）
4. **服务器启动错误仍未传播**：`graph/mod.go` ✅ 已修复（`StartServerWithResolver` 返回启动错误）

---

## 6. 代码质量改进

### 统一令牌解析策略 ✅ 已修复

`ResolveSignerEmail()` 和 `resolveEmailFromToken()` 的验证规则已对齐。`ResolveSignerEmail` 调用 `store.FindGatewayTokenByKey()`（内部检查 `IsActive + IsExpired`），并额外显式检查 `!tk.IsExpired()`，消除了认证漂移。

### 将公共配置 DTO 与携带密钥的配置 DTO 分离

当前模式仍然将敏感和非敏感配置建模在一起。

### 对必需密钥快速失败

`JWTSecret`、`SeedSecret`、SMTP 设置和链引导路径应在启动时验证，而不是静默降级。

### 减少隐藏全局状态

全局 `rsaKey` 和 `teeChain` 仍然使服务器生命周期更难推理和测试。

### 将注释式安全替换为可执行契约

几处代码注释声称"公开/脱敏/仅管理员"，但模式和解析器行为并未强制执行。

### 改进部署文档

仓库仍然需要关于 Go 版本、`token_chain_data` 内容、TLS 证书和链配置来源的显式设置文档。

### 无界历史写入

`early_access.go` 中的 `appendHistory` 将整个积分历史数组序列化并重写到 Pebble 中的单个键。这扩展性很差（O(N²) 写入放大）。历史记录应作为单独的键（`history:wallet:timestamp`）写入，并通过 Pebble 迭代器检索。

### 类型断言 Panic ✅ 已修复

`DistributeMiddleware` 已将 unchecked type assertion 改为 comma-ok 形式。当 `model` 字段存在但非字符串类型（如 `{"model": 123}`）时，返回 HTTP 400 错误 `"model must be a string"`，不再 Panic。

### Wasm 解释器模式

Wazero 使用 `NewRuntimeConfigInterpreter` 初始化，这比编译器模式慢得多。在支持的架构（Mac/Linux x86_64 和 arm64）上，应优先使用 `NewRuntimeConfigCompiler`。

---

## 7. 优先行动计划

### 阶段 1：生产前必须修复（关键安全与功能缺陷）

| 优先级 | 问题 | 文件 |
|--------|------|------|
| 1 | ✅ 移除并轮换已提交的私钥 | `hack/build/keys/private.pem` |
| 2 | ✅ 修复 `GetProtoMessage` 中的双重解封，允许生产写入/读取 | `pkg/store/db.go` |
| 3 | ✅ 在 `FindTokenByKey` 中添加令牌过期检查 | `pkg/api/gateway/cache.go` |
| 4 | ✅ 返回的数据都是 ****,没有密钥, 保护 GraphQL 配置：从 `authConfig` 查询中完全移除 OAuth 客户端密钥和 SMTP 密码 | `graph/auth.graphqls`, `graph/auth.resolvers.go` |
| 5 | ✅ 在密钥上传中强制执行用户所有权检查 | `graph/secret.resolvers.go` |
| 6 | ✅ 修复 `RequestRecovery` 中的 OTP 可预测性和投递~~ ✅ 已修复 | `graph/auth.resolvers.go` |
| 7 | ❌ 锁定 CORS 并保护 `setAuthConfig` 这是设计| `graph/mod.go`, `graph/auth.graphqls` |
| 8 | ✅ 为钱包登录添加 nonce/重放保护 | `graph/auth.resolvers.go` |

### 阶段 2：应尽快修复（可靠性与可移植性）

| 优先级 | 问题 | 文件 |
|--------|------|------|
| 1 | ❌ 为 `qsort_r.go` 添加构建约束（`//go:build linux`） | `pkg/util/cgostubs/qsort_r.go` |
| 2 | ✅ 在 `GetNodes` 中添加互斥锁以防止并发连接竞争~~ ✅ 已修复 | `pkg/chains/evm/subnet.go` |
| 3 | ✅ 重新设计 `PersistChan` 以解决 push/pop 竞争、通道关闭 Panic 和内存泄漏~~ ✅ 已修复 | `pkg/store/persist_channel.go` |
| 4 | ❌ 为开发机上的 `SealWithProductKey` 添加 TEE 检测绕过 不能绕过 | `pkg/util/tee_seal.go` |
| 5 | ✅ 修复 `DistributeMiddleware` 中的类型断言 Panic（安全解析 JSON model 字段） | `pkg/api/gateway/auth_middleware.go` |
| 6 | ❌ 使 RSA 密钥上传密钥持久化/版本化 | `graph/mod.go` |
| 7 | ❌ 使 `LoadConfig` 实际遵循环境回退或快速失败 | `pkg/api/auth/config.go` |
| 8 | ✅ 将 fire-and-forget 认证用户写入替换为可观测持久化 | `graph/auth.resolvers.go` |
| 9 | ✅ 修复共识清理原子性 TODO | `tee-chain/tx_finalize.go`, `tee-chain/tx_partial_sign_rev.go`, `tee-chain/tx_to_realy.go` |
| 10 | ✅ 修复 `generateAPIKey()` 错误处理 | `graph/auth_helpers.go` |
| 11 | ✅ 修复 `ListenAndServe` 错误传播 | `graph/mod.go` |
| 12 | ✅ 修复 OTP clientID 绑定或从 API 移除 | `pkg/services/auth/otp.go` |

### 阶段 3：锦上添花（美观与性能）

| 优先级 | 问题 |
|--------|------|
| 1 | ✅ 添加优雅的 HTTP 关闭和服务器超时 |
| 2 | ❌ 使 DB/运行时/TLS 路径可配置 |
| 3 | ❌ 将内存滥用控制替换为持久/共享存储（如需要） |
| 4 | ❌ 收紧 README，包含确切的引导和工具链要求 |
| 5 | ❌ 在 CI 中使用预安装 Go 工具链运行 `go test`、`go vet` 和漏洞扫描 |
| 6 | ❌ 优化 `early_access.go` 中的积分历史存储，使用键索引而非每次更新时序列化整个历史切片 |
| 7 | ✅ 将 Wazero 运行时从解释器模式升级为编译器模式 |
| 8 | ~~清理死代码，如 P2P 写入处理程序 `handleSecretSave`~~ ✅ 已修复 |
| 9 | ✅ 通过允许上下文取消修复 OTP 清理协程泄漏 |
| 10 | ❌ 拆分调试和生产运行时加载行为 |
| 11 | ❌ 实现或隐藏未完成的 `ClaimOrderToken` 和 REST `/api/blockchain/token` 端点 |
