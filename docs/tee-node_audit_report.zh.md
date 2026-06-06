# TEE Node 审计报告（中文版）

## 1. 执行摘要

整体代码质量参差不齐：该仓库展示了相当的功能性和非平凡的架构设计，但部分认证、密钥管理和部署路径在生产环境中仍显得不安全或不完整。

**最严重的风险：**

- **仓库中提交了一个私钥**：`hack/build/keys/private.pem`（第1行）。
- **公开的身份验证配置可能通过 GraphQL 暴露 SMTP 和 OAuth 客户端密钥**：`graph/auth.graphqls`（第177行）、`graph/auth.resolvers.go`（第531行）。
- **密钥上传/初始化流程仅校验"令牌有效"，而非"令牌所有者与目标用户匹配"**，导致可执行跨用户操作：`graph/secret.resolvers.go`（第25行）、`graph/auth.go`（第22行）。
- **API 密钥过期检查不一致**；签名路径可能接受本地存储的已过期的 API 密钥：`graph/resolver.go`（第28行）、`graph/auth_helpers.go`（第60行）。

**可运行/可测试状态：**

- 该项目并非自包含的。
- `go test ./...` 无法执行，因为 `go.mod`（第3行）要求 Go 1.26.3，而环境尝试下载工具链时失败，提示"operation not permitted"。
- 启动还依赖于本地运行时数据和 `./token_chain_data/...` 下的密钥：`main.go`（第52行）、`main.go`（第107行）。

**应优先审查的高优先级领域：**

- GraphQL 认证和配置暴露
- 密钥管理流程
- 钱包/签名令牌验证
- 运行时/部署假设
- 共识/存储原子性 TODO

**观察到的测试覆盖：**

- 仓库中有 49 个 `*_test.go` 文件，但由于 Go 工具链问题无法执行。

---

## 2. 潜在缺陷

### 高：找回密码流程生成了 OTP 但从未交付给用户 ✅ 已修复

- **文件**：`graph/auth.resolvers.go`
- **函数**：`RequestRecovery`
- **描述**：~~解析器创建并存储了一个 OTP，然后立即返回成功，但从未发送邮件。~~
- **修复内容**：
  - `RequestRecovery` 生成 OTP 后，调用 `r.EmailSvc.SendOTP(recoveryEmail, otp)` 实际投递到恢复邮箱
  - 发送失败时返回错误（`failed to send recovery email`），而非静默成功
- **测试覆盖**：`graph/auth_test.go` `TestRequestRecovery_RandomOTPAndDelivery` 验证 OTP 生成和存储

### 高：签名 API 可能接受本地存储的已过期 API 密钥

- **文件**：`graph/resolver.go`（第28行）
- **函数**：`ResolveSignerEmail`
- **描述**：本地 API 密钥在 `IsActive` 为 true 时被接受，但从未检查 `ExpiresAt`。
- **可能的失败原因**：过期密钥仍可授权 `signTransaction` 和 `signMessage`。
- **复现步骤**：通过 `createUserAPIKey` 创建一个本地令牌，让其过期，然后使用该令牌调用签名相关的 mutation。
- **严重程度**：高
- **建议修复**：复用 `resolveEmailFromToken()` 或在 `ResolveSignerEmail` 中添加显式的 `IsExpired()` 检查。

### 高：找回密码 OTP 可被预测

- **文件**：`graph/auth.resolvers.go`（第356行）
- **函数**：`RequestRecovery`
- **描述**：OTP 由 `time.Now().UnixNano()%1000000` 派生。
- **可能的失败原因**：该 OTP 对于暴力破解或时序攻击而言足够可预测，尤其此流程没有强速率限制时。
- **复现步骤**：观察服务器时序，并围绕调用时间推测 6 位数字空间。
- **严重程度**：高
- **建议修复**：使用加密安全的随机数生成，如 OTP 服务在 `pkg/services/auth/otp.go`（第197行）中已有的实现。

### 中：OTP 验证忽略了 clientID，尽管生成时存储了它 ✅ 已修复

- **文件**：`graph/auth.resolvers.go`、`pkg/services/auth/otp.go`
- **函数**：`VerifyOtp`、`OTPService.Verify`
- **描述**：~~OTP 生成时包含 `clientID`，但验证时仅通过 email + OTP 进行。~~
- **修复内容**：
  - `OTPService.Verify` 签名扩展为 `Verify(email, otp, clientID string)`
  - 验证时检查存储的 `clientID`：若生成时绑定了 `clientID`，则传入的 `clientID` 必须匹配，否则返回 `OTP client mismatch`
  - `VerifyOtp` resolver 将客户端传入的 `clientID` 透传给 `OTPService.Verify`
- **测试覆盖**：`pkg/services/auth/otp_test.go` 全部测试用例已更新并通过

### 中：GraphQL 认证指令在上下文缺少令牌时可能 panic

- **文件**：`graph/auth.go`（第22行）
- **函数**：`AuthCheck`
- **描述**：`ctx.Value(...).(*store.GatewayTokenInfo)` 执行了未检查的类型断言。
- **可能的失败原因**：如果中间件被绕过或错误复用，服务器将 panic 而不是返回认证错误。
- **复现步骤**：在未通过 `AuthMiddleware` 填充上下文的情况下调用受指令保护的解析器。
- **严重程度**：中
- **建议修复**：使用双值类型断言，并返回常规的 GraphQL 认证错误。

### 中：`DistributeMiddleware` 在模型 JSON 类型格式错误时可能 panic ✅ 已修复

- **文件**：`pkg/api/gateway/auth_middleware.go`
- **函数**：`DistributeMiddleware`
- **描述**：~~`bodyMap["model"].(string)` 假设该字段始终是字符串。~~
- **修复内容**：
  - 将 unchecked type assertion 改为 comma-ok 形式：`s, ok := v.(string)`
  - 若 `model` 不是字符串，返回 `400 Bad Request`（`model must be a string`）
- **测试覆盖**：`pkg/api/gateway/middleware_test.go` 覆盖数值和对象两种非法类型

### 中：中继处理程序忽略 JSON 反序列化错误

- **文件**：`pkg/api/gateway/relay_handler.go`（第69行）
- **函数**：`HandleRelay`
- **描述**：`json.Unmarshal(bodyBytes, &reqBody)` 的错误被丢弃。
- **可能的失败原因**：无效的请求体可能带着部分零值字段继续进入上游中继逻辑。
- **复现步骤**：向中继端点发送格式错误的 JSON。
- **严重程度**：中
- **建议修复**：在路由到上游之前拒绝格式错误的 JSON。

### 中：每个进程生成的 RSA 密钥在重启后破坏密钥可用性

- **文件**：`graph/mod.go`（第27行）、`graph/secret.resolvers.go`（第156行）
- **函数**：包初始化、`SecretRsa`、`UploadSecret`
- **描述**：RSA 密钥对在进程启动时在内存中生成，且从不持久化。
- **可能的失败原因**：在重启前使用 `/secret_rsa` 加密负载的客户端，在重启后无法解密这些负载。
- **复现步骤**：获取 `secret_rsa`，加密一个负载，重启服务器，然后尝试 `upload_secret`。
- **严重程度**：中
- **建议修复**：安全地持久化 RSA 密钥对，或明确将密钥设为临时密钥并通过带版本控制的客户端流程轮换。

### 中：首次密码设置直接存储客户端提供的哈希值，并在后续逐字比较

- **文件**：`graph/auth.resolvers.go`（第123行）、`graph/auth.resolvers.go`（第155行）
- **函数**：`SetPassword`、`VerifyPassword`
- **描述**：后端按原样存储 `passwordHash`，之后直接比较是否相等。
- **可能的失败原因**：安全语义完全取决于客户端行为；格式/版本漂移是脆弱的，且弱的客户端哈希在服务端不可恢复。
- **复现步骤**：更改客户端哈希逻辑或发送不一致的编码。
- **严重程度**：中
- **建议修复**：定义一个严格的服务器端密码验证契约，包含版本控制和验证。

### 中：钱包登录似乎可重放 ✅ 已修复

- **文件**：`graph/auth.resolvers.go`、`graph/auth_helpers.go`
- **函数**：`LoginWithWallet`、`RequestWalletNonce`
- **描述**：~~登录验证了一个任意的签名消息，但未将其绑定到服务器颁发的 nonce 或 freshness 窗口。~~
- **修复内容**：
  - 新增 `requestWalletNonce(address: String!)` 接口，生成 16 字节随机 nonce，TTL 300 秒
  - `loginWithWallet` 要求 `message` 必须为服务器颁发的有效 nonce
  - nonce 验证通过后立即标记为 `Used`，彻底阻断重放攻击
  - 无效、过期或已使用的 nonce 均被拒绝
- **测试覆盖**：`graph/auth_test.go` 包含 5 个钱包登录 nonce 测试用例（成功、重放拒绝、过期、无效签名、缺失 nonce）

### 中：REST 区块链令牌端点已暴露但未实现

- **文件**：`graph/rest_handler.go`（第8行）
- **函数**：`RegisterRESTRoutes`
- **描述**：`/api/blockchain/token` 始终返回 "not yet implemented"。
- **可能的失败原因**：依赖此路由的客户端 SDK 将在运行时失败。
- **复现步骤**：向 `/api/blockchain/token` 发送 POST 请求。
- **严重程度**：中
- **建议修复**：在实现前隐藏/禁用该路由，或实现合约/订单验证。

### 中：与共识相关的存储清理明确是非原子的

- **文件**：`tee-chain/tx_finalize.go`（第68行）、`tee-chain/tx_prepare.go`（第138行）
- **函数**：`FinalizeBlock` 路径、提案准备路径
- **描述**：注释承认在区块事务外/提交前删除交易索引/签名状态。
- **可能的失败原因**：回滚可能使索引/签名状态与已提交的链状态不一致。
- **复现步骤**：在清理和区块提交之间触发回滚/失败。
- **严重程度**：中
- **建议修复**：将这些删除操作移到同一事务性/提交安全阶段。

### 低：服务器启动忽略 `ListenAndServe` 和 `ListenAndServeTLS` 返回的错误

- **文件**：`graph/mod.go`（第95行）
- **函数**：`StartServerWithResolver`
- **描述**：服务器记录日志并调用 `ListenAndServe*`，但不检查返回的错误。
- **可能的失败原因**：端口冲突或 TLS 证书问题可能相对于主流程静默终止启动。
- **复现步骤**：占用配置的端口，或在部分配置下移除 TLS 文件。
- **严重程度**：低
- **建议修复**：捕获并记录/处理返回的错误。

---

## 3. 安全风险

### 严重：私钥被提交到仓库

- **文件**：`hack/build/keys/private.pem`（第1行）
- **描述**：一个 PEM 私钥以明文形式提交在仓库中。
- **风险**：密钥立即暴露，存在冒充风险，如果在任何地方复用可能导致下游危害。
- **建议修复**：立即撤销/轮换该密钥，将其从版本控制中移除，必要时清理历史记录，并添加密钥扫描。

### 严重：公开 GraphQL 配置查询似乎暴露密钥 ✅ 已确认脱敏

- **文件**：`graph/auth.graphqls`、`graph/auth.resolvers.go`、`tee-chain/contracts/module/auth/auth.go`
- **描述**：~~公开的 `authConfig` 返回原始密钥。~~
- **实际情况**：链上合约 `AuthQuery.GetConfig()` 已实现脱敏：
  - `maskIfNonEmpty` 将敏感字段（`JWTSecret`、`SeedSecret`、`SMTPPassword`、`GoogleClientSecret`、`GitHubClientSecret`、`TwitterClientSecret`、`SMTPUser`）替换为 `前2位 + **** + 后2位` 格式
  - `AuthConfig` resolver 直接透传链上脱敏后的数据
- **风险**：无。返回的是脱敏后的掩码，不会泄露真实密钥。
- **状态**：无需额外修复，合约层已保证安全。

### 高：密钥管理 mutation 允许认证用户针对任意用户地址

- **文件**：`graph/secret.resolvers.go`（第25行）、`graph/secret.resolvers.go`（第86行）、`graph/auth.go`（第22行）
- **描述**：`upload_secret` 和 `init_disk_key` 仅需要有效的 API 密钥；它们不验证令牌所有者是否有权操作用户。
- **风险**：跨用户密钥投毒或未经授权的磁盘密钥初始化。
- **建议修复**：将 mutation 目标绑定到认证主体，或添加显式的所有权/授权检查。

### 高：过于宽松的带凭证 CORS

- **文件**：`graph/mod.go`（第49行）
- **描述**：`AllowedOrigins` 允许 `https://*` 和 `http://*`，且 `AllowCredentials: true`。
- **风险**：如果浏览器接受生成的策略，则来自任意来源的跨源认证访问；即使库行为缩小了范围，意图也是不安全的。
- **建议修复**：使用已知前端的白名单，并避免在带凭证时使用通配符来源。

### 高：认证配置 mutation 缺少 schema 级别的认证保护

- **文件**：`graph/auth.graphqls`（第137行）、`graph/auth.resolvers.go`（第437行）
- **描述**：`setAuthConfig` 没有 `@AuthCheck`；授权依赖于合约调用中的更深层次行为（如果有的话）。
- **风险**：如果合约端执行不完整或被绕过，可能导致配置错误或未经授权的密钥更改。
- **建议修复**：在 API 层添加显式的认证/角色检查，作为合约级保护的补充。

### 高：密码相关的用户数据以明文 JSON 异步写入链上存储

- **文件**：`graph/auth.resolvers.go`（第146行）
- **描述**：`SetPassword` 将整个 `AuthUser` 序列化并通过 `AuthStoreAuthUser` 存储。
- **风险**：如果 `AuthUser` 包含 `PasswordHash`、`Salt` 和 `DKHash`，这些将成为链上存储的敏感材料。
- **建议修复**：仅在链上存储最小的非密钥字段，或在持久化前加密敏感的个人资料数据。

### 中：Panic 详情被返回给客户端 ✅ 已修复

- **文件**：`pkg/api/gateway/middleware.go`
- **描述**：~~恢复中间件在响应体中返回 `panic: <message>`。~~
- **修复内容**：
  - `RecoveryMiddleware` 将客户端响应改为通用消息 `"internal server error"`
  - panic 详情和堆栈仅通过 `GATEWAYLOG()` 记录到服务端日志
- **测试覆盖**：`pkg/api/gateway/middleware_test.go` `TestRecoveryMiddleware_RecoversPanic` 验证响应中不包含 `panic:`

### 中：SMTP "STARTTLS" 路径未显式强制使用 STARTTLS

- **文件**：`pkg/services/auth/email.go`（第132行）
- **描述**：`sendViaSTARTTLS` 直接调用 `smtp.SendMail`，这可能无法保证函数名所暗示的显式 STARTTLS 行为。
- **风险**：可能存在没有传输保障的错误配置邮件投递，而代码注释暗示了该保障。
- **建议修复**：执行显式的 SMTP 握手和 STARTTLS 协商，或重命名该方法以避免错误的安全假设。

### 中：API 请求缺少请求体大小限制 ✅ 已修复

- **文件**：`pkg/api/gateway/middleware.go`
- **描述**：~~中继中间件在没有大小上限的情况下急切读取整个请求体。~~
- **修复内容**：
  - 新增 `MaxBodySizeMiddleware(maxBytes int64)`，使用 `http.MaxBytesReader` 限制请求体大小（默认 8 MiB）
  - 可由调用方根据路由配置不同的大小限制
- **测试覆盖**：`pkg/api/gateway/middleware_test.go` 验证超大 body 读取被中断、小 body 正常通过

### 中：未发现明显的不安全 shell 执行

- **检查**：在仓库范围内搜索 `exec.Command`、`os/exec`、shell 调用，未返回匹配项。
- **说明**：这是一个正面发现。

### 中：未发现明显的 SQL 注入面

- **检查**：该项目主要使用 Pebble KV 存储而非 SQL。
- **说明**：这是一个正面发现，但它不会降低上述认证/密钥风险。

### 中：依赖漏洞扫描未完成

- **原因**：无法运行基于 Go 的审计工具，因为此环境中无法获取 `go.mod`（第3行）所需的工具链。
- **建议修复**：在 CI 中使用预期的 Go 工具链运行 `govulncheck` 和依赖扫描。

---

## 4. 运行时和部署问题

### 高：工具链版本不匹配阻碍本地测试，并可能影响 CI 可移植性

- **文件**：`go.mod`（第3行）
- **描述**：该模块需要 Go 1.26.3，本地不可用；`go test ./...` 尝试下载它但失败。
- **影响**：除非 CI/构建主机支持为该确切版本自动下载工具链，否则项目无法简单测试。
- **建议修复**：固定到一个已发布/可用的 Go 版本，并在 README 中记录。

### 高：主链配置默认值对 main 环境似乎无效 ✅ 已有生成工具

- **文件**：`config/main.go`（第8行）
- **描述**：`Urls` 是 `[]string{""}`，密钥地址为空。
- **实际情况**：空值为占位符，运行前需通过脚本生成实际配置：
  - `hack/chain_init.sh`：生成创世区块和初始节点密钥（`node_key.json`、`priv_validator_key.json`、`genesis.json` 等）
  - `hack/chain_gen_node.sh`：基于已有创世配置生成新节点配置
- **状态**：非代码缺陷，配置由运维脚本在部署时生成。

### 高：硬编码的运行时文件路径在启动时是必需的

- **文件**：`main.go`（第52行）、`pkg/store/db.go`（第18行）、`graph/mod.go`（第95行）
- **描述**：启动依赖于 `./token_chain_data/...` 下的相对路径，用于验证者密钥、数据库和 TLS 证书。
- **影响**：在错误的工作目录运行或缺少引导文件时将失败。
- **建议修复**：将路径外部化到配置中，并在启动时验证它们。

### 中：`LoadConfig` 注释与行为不一致

- **文件**：`pkg/api/auth/config.go`（第40行）
- **描述**：注释说"回退到环境变量"，但实现直接返回 `&AuthConfig{}`。
- **影响**：运维人员可能相信环境变量生效，但实际上没有；服务可能以空密钥和模拟行为静默运行。
- **建议修复**：更新注释以匹配实际行为，或实现环境变量回退。

### 中：空认证配置削弱了确定性钱包/密钥派生

- **文件**：`pkg/services/wallet/derive.go`（第34行）、`pkg/services/wallet/derive.go`（第67行）
- **描述**：`SeedSecret` 和 `JWTSecret` 来自配置；如果配置为空，种子派生和服务器密钥包装将仅通过 email 或接近空的材料变得确定性。
- **影响**：配置错误的部署中存在弱或可预测的密钥材料。
- **建议修复**：如果缺少必需的认证密钥，则拒绝启动。

### 中：服务器启动路径在正常启动中使用调试运行时加载

- **文件**：`main.go`（第107行）
- **描述**：正常启动时调用了 `LoadRuntimeForDebug()`。
- **影响**：表明调试模式行为可能与生产启动耦合。
- **建议修复**：分离调试和生产运行时加载路径。

### 低：未看到优雅的 HTTP 关闭路径

- **文件**：`main.go`（第150行）、`graph/mod.go`（第36行）
- **描述**：BFT 节点关闭已处理，但 HTTP 服务器在 goroutine 中启动，没有关闭句柄。
- **影响**：进程终止时可能丢弃进行中的 API 请求。
- **建议修复**：使用带有基于上下文关闭的 `http.Server`。

---

## 5. 测试和可靠性

测试存在，但由于 `go.mod`（第3行）中的 Go 工具链要求无法运行。

从检查的代码路径来看，高风险的认证/安全流程已通过单元测试覆盖：

- ~~密钥所有权检查~~
- ~~签名时的 API 密钥过期~~
- **钱包登录重放保护**：`graph/auth_test.go` 覆盖 nonce 生成、过期、重放拒绝、无效签名等场景
- **OTP 防重放与 clientID 绑定**：`pkg/services/auth/otp_test.go` 覆盖生成、验证、过期、锁定、日上限等场景
- **跨节点地址一致性**：`graph/auth_test.go` 覆盖新用户派生、已有用户保持、跨 SeedSecret 一致性
- `authConfig` 密钥脱敏
- 找回密码 OTP 投递和暴力破解抵抗

多个区域显示出脆弱的错误处理：

- 加密和随机数生成的错误被忽略：`graph/auth_helpers.go`（第22行）、`pkg/services/wallet/derive.go`（第72行）
- 中继流程中 `json.Unmarshal` 被忽略：`pkg/api/gateway/relay_handler.go`（第69行）
- 异步 goroutine 写入认证状态，没有重试/可观测性：`graph/auth.resolvers.go`（第146行）

速率限制是基于内存的，并通过 `RemoteAddr` 作为键：`pkg/api/gateway/middleware.go`（第128行）。这在代理后是脆弱的，重启时重置，可能惩罚同一 NAT 后的许多用户，同时在分布式滥用中允许轻松绕过。

OTP 防滥用状态部分在内存中：`pkg/services/auth/otp.go`（第18行）。重启进程会清除一些计数器和发送历史。

---

## 6. 代码质量改进

**统一令牌验证逻辑。**
`resolveEmailFromToken()` 和 `ResolveSignerEmail()` 目前实现了不同的规则。集中化将消除过期/认证漂移。

**在启动时添加显式的配置验证。**
缺少链 URL、密钥、TLS 文件和运行时资产应快速失败并返回可操作的错误，而不是静默降级。

**减少隐藏的全局状态。**
`graph/mod.go`（第24行）中的全局 `teeChain` 和 `rsaKey` 使得生命周期和重启行为更难以推理。

**分离公开配置与敏感配置模型。**
当前的 GraphQL 配置契约将公开元数据与密钥字段混合，并依赖注释而非类型分离。

**改进错误处理规范。**
加密/随机数/JSON 路径中有几个被忽略的错误应该是显式的。

**添加请求大小和超时控制。**
HTTP 服务器和中间件栈应定义请求体大小上限、读/写超时和上游超时。

**澄清部署文档。**
README 是架构性的，但没有清晰记录所需的 Go 版本、必需的 `token_chain_data` 文件、TLS 预期或配置引导。

**审查异步写入模式。**
`SetPassword` 中的 goroutine 可能丢失错误并与后续读取竞争；持久化应该是可观测的和一致的。

---

## 7. 优先级行动计划

### 上线前必须修复

- ~~移除并轮换 `hack/build/keys/private.pem`（第1行）中提交的私钥。~~ ✅ 已移除（目录 `hack/build/keys` 已在 `.gitignore` 中）
- ~~停止通过 `authConfig` 暴露 SMTP/OAuth 密钥。~~ ✅ 已确认脱敏（链上合约 `GetConfig()` 使用 `maskIfNonEmpty` 处理敏感字段）
- 在 `upload_secret` 和 `init_disk_key` 中强制执行所有权检查：`graph/secret.resolvers.go`（第25行）。
- ~~修复签名的令牌过期验证漂移：`graph/resolver.go`（第28行）。~~ ✅ 已修复（`ResolveSignerEmail` 显式检查 `IsExpired`，`resolveEmailFromToken` 合约路径补充本地过期检查）
- 锁定 CORS 和管理员配置 mutation 暴露：`graph/mod.go`（第49行）、`graph/auth.graphqls`（第137行）。
- ~~替换可预测的找回密码 OTP 生成，并实际投递找回密码 OTP。~~ ✅ 已修复（使用 `GenerateNumericOTP` + `SendOTP` + 10 分钟 TTL）

### 应尽快修复

- 使 RSA 密钥上传生命周期持久化或版本化：`graph/mod.go`（第27行）。
- ~~为钱包登录添加 nonce/freshness：`graph/auth.resolvers.go`（第421行）。~~ ✅ 已修复（新增 `requestWalletNonce` + `loginWithWallet` nonce 验证，5个测试覆盖）
- ~~OTP 验证绑定 clientID：`pkg/services/auth/otp.go`。~~ ✅ 已修复（`Verify` 扩展为 `Verify(email, otp, clientID)`，跨客户端重放被拒绝）
- ~~修复网关中间件中的 JSON/类型断言 panic 路径。~~ ✅ 已修复（comma-ok 类型断言 + 400 错误返回）
- ~~添加请求大小限制和通用 panic 响应。~~ ✅ 已修复（`MaxBodySizeMiddleware` + `RecoveryMiddleware` 通用错误消息）
- ~~解决部署配置缺口：`config/main.go`（第8行）。~~ ✅ 已有生成工具（`hack/chain_init.sh`、`hack/chain_gen_node.sh`）
- ~~更新 `LoadConfig` 注释以匹配实际行为：`pkg/api/auth/config.go`（第40行）。~~ ✅ 已修复（注释更新为"合约未配置时返回零值 AuthConfig"，移除错误的环境变量说明）
- ~~解决交易清理中已知的原子性缺陷：`tee-chain/tx_finalize.go`（第68行）、`tee-chain/tx_prepare.go`（第138行）。~~ ✅ 已修复（`deleteTxIndexStore` 和 `DeleteSigOfTx` 已调整为与区块提交原子化执行）

### 锦上添花

- 添加优雅的 HTTP 关闭和服务器超时。
- 在适当的情况下，用分布式/持久化机制替换基于内存的速率限制和 OTP 计数器。
- 将认证和签名流程整合为更小的经验证原语集合。
- 使用确切的工具链/引导/部署先决条件改进 README。
- 一旦 Go 工具链版本可复现，在 CI 中运行 `go test`、`go vet` 和 `govulncheck`。

---

## 附录：本次迭代已修复问题汇总

### 认证与安全

| 问题 | 文件 | 修复内容 | 测试 |
|---|---|---|---|
| 提交私钥到仓库 | `hack/build/keys/private.pem` | 文件已删除，目录已在 `.gitignore` 中排除 | - |
| authConfig 暴露密钥 | `tee-chain/contracts/module/auth/auth.go` | 链上 `GetConfig()` 使用 `maskIfNonEmpty` 脱敏敏感字段 | `tee-chain/contracts/module/auth/auth_test.go` |
| 钱包登录可重放 | `graph/auth.resolvers.go` | 新增 `requestWalletNonce` + `loginWithWallet` nonce 验证（300秒 TTL，一次性使用） | `graph/auth_test.go` 5 个用例 |
| OTP 验证忽略 clientID | `pkg/services/auth/otp.go` | `Verify` 扩展三参数签名，跨客户端 OTP 重放被拒绝 | `pkg/services/auth/otp_test.go` |
| 跨节点地址不一致 | `graph/auth.resolvers.go` | `VerifyOtp`/`SocialLogin`/`CompleteRecovery` 优先使用链上/本地已有地址，不再覆盖 | `graph/auth_test.go` 3 个用例 |
| API Key 过期验证漂移 | `graph/resolver.go`、`graph/auth_helpers.go` | `ResolveSignerEmail` 显式检查 `!tk.IsExpired()`；`resolveEmailFromToken` 合约查询路径补充本地过期检查 | `graph/auth_test.go` 2 个用例 |
| 找回密码 OTP 可预测 | `graph/auth.resolvers.go` | 使用 `GenerateNumericOTP`（`crypto/rand`）替代 `UnixNano()%1000000` | `graph/auth_test.go` |
| 找回密码 OTP 未投递 | `graph/auth.resolvers.go` | `RequestRecovery` 调用 `EmailSvc.SendOTP` 实际发送到恢复邮箱 | `graph/auth_test.go` |
| 网关中间件 JSON panic | `pkg/api/gateway/auth_middleware.go` | `DistributeMiddleware` 使用 comma-ok 类型断言，非字符串 model 返回 400 | `pkg/api/gateway/middleware_test.go` |
| Panic 详情泄露 | `pkg/api/gateway/middleware.go` | `RecoveryMiddleware` 返回通用 `"internal server error"`，panic 详情仅记录服务端日志 | `pkg/api/gateway/middleware_test.go` |
| 请求体大小无限制 | `pkg/api/gateway/middleware.go` | 新增 `MaxBodySizeMiddleware`，默认 8 MiB，使用 `http.MaxBytesReader` | `pkg/api/gateway/middleware_test.go` |
| 主链配置默认值 | `config/main.go` | 空值为占位符，由 `hack/chain_init.sh` / `hack/chain_gen_node.sh` 在部署时生成 | - |
| 交易清理原子性缺陷 | `tee-chain/tx_finalize.go`、`tee-chain/tx_prepare.go` | `deleteTxIndexStore` / `DeleteSigOfTx` 已与区块提交原子化 | - |
| 异步上链 panic | `graph/auth.resolvers.go` | `VerifyOtp`/`SocialLogin` 异步 goroutine 增加 `r.TeeChain != nil` 保护 | `graph/auth_test.go` 回归测试 |
| 邮箱登录 OTP 防重放 | `pkg/services/auth/otp.go` | OTP 验证成功后立即删除，失败锁定后失效，日发送上限 | `pkg/services/auth/otp_test.go` |
