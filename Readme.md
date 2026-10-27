# tee-provider — TEE Subnet for Web3 Infrastructure

tee-provider is a **TEE (Trusted Execution Environment)**-based decentralized subnet composed of multiple nodes running inside TEEs. It maintains state consistency within the subnet via the **CometBFT** consensus protocol, and provides secure key management and secret distribution through **DKG (Distributed Key Generation)** and **Proxy Re-encryption**.

The tee-provider subnet can run as a **subnet of any public chain**, bringing off-chain secure computation, authentication services, and API gateway capabilities to any ecosystem.

---

## Architecture Overview

```
┌──────────────────────────────────────────────────────────────────┐
│                    Any Public Chain (L1 / L2)                    │
│  Polkadot · Ethereum · Solana · BNB Chain · any chain           │
│                                                                  │
│  ┌────────────────────────────────────────────────────────────┐  │
│  │  tee-provider Registry Contract (Registry·Staking·Slashing)│  │
│  │  - Node registration / deregistration                      │  │
│  │  - TEE remote attestation verification                     │  │
│  │  - Subnet state anchoring                                  │  │
│  └────────────────────────┬───────────────────────────────────┘  │
└───────────────────────────┼──────────────────────────────────────┘
                            │ Cross-chain communication / state sync
                            │
╔═══════════════════════════╧══════════════════════════════════════╗
║                 tee-provider Subnet (CometBFT)                   ║
║                                                                  ║
║  ┌────────────────────────────────────────────────────────────┐  ║
║  │                    Consensus Layer                          │  ║
║  │  - CometBFT ABCI application                                │  ║
║  │  - TEE remote attestation (Vote Extensions)                 │  ║
║  │  - Validator election & epoch management                    │  ║
║  └────────────────────────┬───────────────────────────────────┘  ║
║                           │                                      ║
║  ┌────────────────────────┴───────────────────────────────────┐  ║
║  │                   Secure Computation Layer (Crypto)         │  ║
║  │  - DKG (Distributed Key Generation) — threshold signatures  │  ║
║  │  - Proxy Re-encryption                                       │  ║
║  │  - TEE Attestation — SGX / TDX / SEV remote attestation      │  ║
║  └────────────────────────┬───────────────────────────────────┘  ║
║                           │                                      ║
║  ┌────────────────────────┴───────────────────────────────────┐  ║
║  │                    Service Layer                            │  ║
║  │                                                              │  ║
║  │  ┌─────────────────┐  ┌──────────────┐  ┌───────────────┐   │  ║
║  │  │  Web3Auth Svc   │  │ API Gateway  │  │ Secret Mgmt   │   │  ║
║  │  │                 │  │    Svc       │  │     Svc       │   │  ║
║  │  │ · OTP email     │  │ · Token mgmt │  │ · Encrypted   │   │  ║
║  │  │   auth          │  │ · Multi-     │  │   env vars    │   │  ║
║  │  │ · Social OAuth  │  │   provider   │  │ · Disk keys   │   │  ║
║  │  │ · Wallet mgmt   │  │   relay      │  │ · TEE reports │   │  ║
║  │  │ · Tx signing    │  │ · Rate limit │  │ · RSA keys    │   │  ║
║  │  │   (TEE)         │  │ · Billing    │  │               │   │  ║
║  │  │ · Key derivation│  │ · On-chain   │  │               │   │  ║
║  │  │   (TEE)         │  │   orders     │  │               │   │  ║
║  │  │ · Social recovery│ │              │  │               │   │  ║
║  │  └─────────────────┘  └──────────────┘  └───────────────┘   │  ║
║  └────────────────────────────────────────────────────────────┘  ║
║                                                                  ║
║  ┌────────────────────────────────────────────────────────────┐  ║
║  │                   P2P Network Layer                         │  ║
║  │  - Node discovery / seed nodes                              │  ║
║  │  - Message routing / Gossip                                 │  ║
║  │  - DKG channels / BFT Bridge                                │  ║
║  └────────────────────────────────────────────────────────────┘  ║
╚══════════════════════════════════════════════════════════════════╝
                            │
        ┌───────────────────┼───────────────────┐
        ▼                   ▼                   ▼
   ┌─────────┐        ┌──────────┐        ┌──────────┐
   │ Node 0  │        │  Node 1  │  ...   │  Node N  │
   │ (TEE)   │        │  (TEE)   │        │  (TEE)   │
   │ SGX/TDX │        │ SGX/TDX  │        │ SGX/TDX  │
   └─────────┘        └──────────┘        └──────────┘
```

---

## Project Modules

### 1. Sidechain / Consensus (`tee-chain/`)

The core of the subnet, implemented on top of CometBFT:

| Module | Description |
|------|------|
| `consensus.go` | ABCI application, handles the block lifecycle (CheckTx, PrepareProposal, ProcessProposal, FinalizeBlock, Commit) |
| `mod.go` | Sidechain initialization, node startup, validator configuration |
| `epoch.go` | Epoch management, auto-generates HubCalls to sync to the main chain |
| `hub_sync.go` | Main-chain state sync (validator set, chain config) |
| `chains.go` | Multi-chain connection management (add / remove / query external chains) |
| `tx_*.go` | Transaction processing pipeline (Prepare → Process → Partial Sign → Sign → Finalize) |

### 2. Cryptography Layer (`pkg/`)

| Sub-package | Description |
|------|------|
| `pkg/dkg` | Distributed Key Generation (DKG), threshold signatures based on kyber |
| `pkg/proxy-reenc` | Proxy re-encryption: securely converts an encrypted secret from the DKG public key to a client public key |
| `pkg/chains` | Public chain interface abstraction (supports Substrate / any chain) |
| `pkg/util` | Utility functions, logging, SS58 encoding |

### 3. GraphQL API (`graph/`)

Provides query and mutation interfaces for external clients:

```graphql
# Chain info
query { chain_info { network_label urls } validators }

# Secret management
mutation { upload_secret(index, secret, hash, user) init_disk_key(index, user) }
query { tee_report(hash) secret_rsa }

# Contract calls
mutation { contractCall(caller, callerType, contract, method, args, signature) }
query { contractDryRun(caller, callerType, contract, mut, method, args) }
```

### 4. TEE Remote Attestation

- Based on **Edgeless ego** (`github.com/edgelesssys/ego`) — Intel SGX
- Based on **go-sev-guest** (`github.com/google/go-sev-guest`) — AMD SEV
- TEE attestations embedded in Vote Extensions; every block is TEE-verified
- Key pairs are bound to a specific TEE instance

---

## Feature Integration — Contract + DKG Deep Design

### A. Web3Auth Backend → tee-provider (DKG Private Key Storage)

Migrate the authentication service from `web3auth/back` into tee-provider. **The core upgrade is replacing single-server HMAC key derivation with DKG threshold share storage.**

| web3auth feature | tee-provider layer | TEE security enhancement |
|---------------|--------------------|-------------|
| OTP email auth | HTTP: `services/auth/otp.go` | OTP generated and verified inside the TEE |
| Social OAuth login | HTTP: `services/auth/oauth.go` | Token exchange completed inside the TEE |
| Key derivation (email→seed→privkey) | Contract: `contracts/auth/` + `services/wallet/derive.go` | **Private key stored as shares, DKG threshold signing** |
| Transaction signing | Contract: `contracts/auth/` + `services/wallet/signer.go` | Threshold signing executed inside the enclave |
| Message signing | Same as above | Same as above |
| Password-encrypted private key | `services/wallet/encrypt.go` | Encryption completed inside the TEE |
| JWT issuance/verification | HTTP: `services/auth/jwt.go` | Keys stored in TEE-protected memory |
| Social recovery | Contract: `contracts/auth/` + `services/recovery/` | Recovery flow guaranteed by consensus |

### B. Token API Relay → tee-provider (Points Model)

Migrate the Relay functionality of `token` into tee-provider, **using a points model: users top up points, and fees are deducted at the exchange rate set by each Provider**.

| token feature | tee-provider layer | Decentralization enhancement |
|------------|--------------------|-------------|
| API Token management | Contract: `contracts/gateway/` + HTTP: `services/gateway/token.go` | State on-chain, subnet consensus |
| Multi-provider relay | HTTP: `services/gateway/relay.go` | Routing policy configured by contract |
| Rate limiting | HTTP: `services/gateway/ratelimit.go` | Distributed rate limiting |
| Points top-up | Contract: `contracts/gateway/` + HTTP: `services/gateway/points.go` | **On-chain top-up / deduction** |
| Exchange rate config | Contract: `contracts/gateway/` (each Provider sets its own) | **Provider self-pricing** |
| User management | Contract: `contracts/gateway/` + HTTP: `services/gateway/user.go` | DID identity |
| Provider management | Contract: `contracts/gateway/` + HTTP: `services/gateway/provider.go` | Config changes require consensus |
| Dashboard Web UI | `dashboard/` (static hosting) | API points to tee-provider |

**Core logic of the points model**:

```
User tops up 100 points
     │
     ▼
Call GPT-4:    Provider A sets 10 points/1K tokens → 1000 tokens used = 10 points deducted
Call Claude:   Provider B sets 5 points/1K tokens  → 1000 tokens used = 5 points deducted
Call local LLM: Provider C sets 1 point/1K tokens   → 1000 tokens used = 1 point deducted

✓ Each Provider independently sets its own model → points_per_1k rate
✓ One points balance works across all models, auto-converted at each Provider's rate
✓ No more plans/Product concepts — pure "top up points → pay per usage"
```

---

## Contract Layer Design — Auth + Gateway On-Chain

tee-provider has a built-in smart contract system (`tee-chain/contracts/`) following a unified routing pattern:
`contracts/contract.go` dispatches by contract name `call.Name` → each contract package implements `Mutation(call)` / `Query(call)`.

Existing entry point (after extension):

```go
// tee-chain/contracts/contract.go
func Mutation(call *model.ContractCall, runtime model.ContractApi) error {
    switch string(call.Name) {
    case "gov":     // existing: governance / tokens / proposals
        ins := gov.NewGov(runtime)
        return gov.GovMutation{Gov: *ins}.ExecCall(call)
    case "auth":    // new: Web3Auth contract
        ins := auth.NewAuth(runtime)
        return auth.AuthMutation{Auth: *ins}.ExecCall(call)
    case "gateway": // new: API gateway contract
        ins := gateway.NewGateway(runtime)
        return gateway.GatewayMutation{Gateway: *ins}.ExecCall(call)
    }
}
```

### Contract Directory Structure (planned)

```
tee-chain/contracts/
├── contract.go              # Contract routing entry
├── runtime.go               # Contract runtime context
├── gov/                     # existing: governance contract
│   ├── gov.go, gov_gen.go, types.go
│   ├── member.go, token.go, vote.go, proposal.go, track.go, treasury.go
│   └── curve.go, helper.go
├── auth/                    # new: Web3Auth contract
│   ├── auth.go              # Contract body (struct + Init)
│   ├── auth_gen.go          # Codegen (ExecCall / ExecQuery / MethodMap)
│   ├── types.go             # Type definitions (UserInfo, RecoveryConfig, ...)
│   ├── user.go              # User registration / query
│   ├── recovery.go          # Social recovery logic
│   └── abis/
│       └── auth.json        # Contract ABI
└── gateway/                 # new: API gateway contract
    ├── gateway.go           # Contract body
    ├── gateway_gen.go       # Codegen
    ├── types.go             # Type definitions (ModelInfo, ProviderInfo, TokenInfo, ...)
    ├── model.go             # Model registration / price update
    ├── provider.go           # Provider creation / model association
    ├── token.go             # API Token CRUD
    ├── points.go            # Points top-up / consumption / withdrawal
    └── abis/
        └── gateway.json     # Contract ABI
```

---

## Auth Contract — DKG Private Key Share Storage

**Core idea**: the user's private key is split into n shares via Shamir secret sharing; each TEE node holds one share, and signing aggregates t-of-n threshold signatures. **The complete private key never appears on any single node.**

### Key Lifecycle

```
① Auth → ② Derive → ③ Split → ④ Store → ⑤ Sign

① OTP / Social authentication
     │
     ▼
② email → seed = HMAC-SHA256(email, SECRET)    [inside TEE, in memory]
   seed → privKey (ECDSA)                       [inside TEE, ephemeral]
     │
     ▼
③ Shamir secret sharing (t-of-n):
   f(x) = privKey + a₁x + a₂x² + ... + aₜ₋₁xᵗ⁻¹
   share sᵢ = f(i),  i ∈ {1..n}
   commitment Cᵢ = sᵢ·G  (for later verification)
     │
     ▼
④ Each node stores sᵢ (TEE sealed encryption)
   On-chain Auth contract anchors: (user, share_hash, commitments_root)
     │
     ▼
⑤ Signing request:
   Each node: σᵢ = tss_sign(sᵢ, tx_hash)
   Aggregate:  σ = tss_aggregate({σᵢ}),  t ≥ threshold
   → returns the complete signature
```

### Auth Contract Methods

**Mutation (writes)**:

| Method | Selector | Params | Description |
|------|----------|------|------|
| `Init` | `0x53e39003` | — | Initialize the Auth contract (DKG public key, threshold) |
| `RegisterUser` | `0x...` | `(email_hash, share_commitment, recovery_email_hash)` | Register a user, anchor share commitments |
| `UpdateShareCommitment` | `0x...` | `(email_hash, new_commitment)` | Update share commitment (key rotation) |
| `SetRecovery` | `0x...` | `(email_hash, recovery_config)` | Set social recovery config |
| `RequestRecovery` | `0x...` | `(email_hash, new_commitment)` | Initiate recovery (requires ≥2 guardian approvals) |
| `ApproveRecovery` | `0x...` | `(email_hash, guardian_sig)` | Guardian approves recovery |
| `CompleteRecovery` | `0x...` | `(email_hash)` | Complete recovery (reset shares) |

**Query (reads)**:

| Method | Selector | Params | Returns |
|------|----------|------|------|
| `GetUser` | `0x...` | `email_hash` | `UserInfo` |
| `HasKey` | `0x...` | `email_hash` | `bool` |
| `GetRecovery` | `0x...` | `email_hash` | `RecoveryConfig` |
| `GetRecoveryStatus` | `0x...` | `email_hash` | `RecoveryStatus` |

### Auth Contract State Model

```go
type Auth struct {
    api           model.ContractApi
    users         *model.StoreMapping[H160, UserInfo]          // email_hash → UserInfo
    shareRoots    *model.StoreMapping[H160, H256]              // email_hash → share_merkle_root
    recoveries    *model.StoreMapping[H160, RecoveryConfig]     // email_hash → Recovery
    recoveryReqs  *model.StoreMapping[H160, RecoveryRequest]   // in-progress recovery requests
    dkgPubKey     *model.StoreValue[[]byte]                    // DKG public key
    threshold     *model.StoreValue[uint32]                    // threshold t
    totalNodes    *model.StoreValue[uint32]                    // total nodes n
}

type UserInfo struct {
    EmailHash       H160      // hash of the email (privacy protection)
    ShareRoot       H256      // Merkle root of share commitments
    DerivedAddress  []byte     // derived address (e.g. Ethereum address)
    RegisteredAt    uint64     // registration block height
}

type RecoveryConfig struct {
    Guardians       []H160    // list of guardian email_hashes
    Threshold       uint32    // minimum number of guardians required for recovery
    RecoveryEmail   []byte    // recovery email (stored encrypted)
}
```

### Signing Flow (HTTP → Contract → DKG)

```
POST /wallet/sign-transaction
  │
  ├─ ① JWT auth → obtain email_hash
  ├─ ② Query Auth.GetUser(email_hash) → confirm the key exists
  ├─ ③ Broadcast SignRequest{email_hash, tx_hash} to all DKG nodes via P2P
  │
  ├─ Each node (inside TEE):
  │   ├─ Load share sᵢ from TEE sealed storage
  │   ├─ Verify share commitment: sᵢ·G == Cᵢ
  │   ├─ Compute partial signature: σᵢ = TSS_Sign(sᵢ, tx_hash)
  │   └─ Return PartialSig{σᵢ, TEE_attestation}
  │
  ├─ ④ Collect ≥ threshold valid σᵢ
  ├─ ⑤ Aggregate: σ = TSS_Aggregate({σᵢ})
  └─ ⑥ Return the complete signature σ
```

---

## Gateway Contract — Points-Based API Gateway

**Core idea**: models are registered independently (each model has its own price and owner); a Provider associates registered models via modelIds. Users top up points, and the Relay deducts fees automatically based on the model price.

### Points Flow Model

```
+-- Model & Provider registration (suppliers) -------------------+
|                                                                |
|  ① Register model (RegisterModel)                              |
|     provider -> Gateway.RegisterModel(name, pointsPer1K)       |
|     -> modelId=1, owner=provider, model exists independently   |
|                                                                |
|  ② Create Provider (CreateProvider)                            |
|     provider -> Gateway.CreateProvider(name, baseUrl,          |
|                     [modelId1, modelId2], priority)            |
|     Provider associates a list of registered model IDs         |
+----------------------------------------------------------------+

+-- Points consumption (triggered automatically by Relay) -------+
|                                                                |
|  ③ User top-up (Recharge)                                      |
|     user.balance += amount                                     |
|                                                                |
|  ④ Consume (Consume)                                           |
|     model = Gateway.GetModel(modelId)                          |
|     cost = ceil(tokensUsed / 1000 * model.PointsPer1K)         |
|     user.balance -= cost                                       |
|     model.providerBalance += cost                              |
|                                                                |
|  ⑤ Withdraw (Withdraw)                                         |
|     provider -> Gateway.Withdraw(modelId, amount)              |
|     model.providerBalance -= amount                            |
+----------------------------------------------------------------+
```

### Gateway Contract Methods

**Mutation**:

| Method | Description |
|------|------|
| `Init` | Initialize the Gateway contract |
| `RegisterModel(owner, name, pointsPer1K)` | **Supplier registers a model and sets its points price** -> `modelId` |
| `UpdateModel(modelId, pointsPer1K)` | Update model price (owner only) |
| `DeactivateModel(modelId)` | Delist a model (owner only) |
| `CreateProvider(owner, name, baseUrl, modelIds, priority)` | Supplier creates a Provider, associating registered models |
| `UpdateProvider(providerId, modelIds, priority)` | Update the models associated with a Provider |
| `Recharge(user, amount)` | User tops up points (sudo/admin call only) |
| `Consume(tokenId, providerId, modelId, tokensUsed)` | Relay deduction: deduct points at the model price |
| `CreateToken(owner, rateLimit)` | Create an API Token (not bound to a model) |
| `RevokeToken(tokenId)` | Revoke a Token |
| `Withdraw(modelId, amount)` | Supplier withdraws earnings for that model |

**Query**:

| Method | Description |
|------|------|
| `GetBalance(user)` | Query a user's points balance |
| `GetModel(modelId)` | Query model details (name, pointsPer1K, owner) |
| `ListModels(startKey, size)` | Paginated list of all registered models |
| `GetToken(tokenId)` | Query Token details |
| `ListUserTokens(user, startKey, size)` | Paginated list of a user's Tokens |
| `GetProvider(providerId)` | Query a Provider and its associated modelIds |
| `ListActiveProviders(modelId)` | List available Providers supporting a model (by priority) |
| `GetUsage(tokenId, day)` | Query usage for a given day |

### Gateway Contract State Model

```go
type Gateway struct {
    api              model.ContractApi
    // User points balances: user -> balance
    balances         *model.StoreMapping[UniAddr, Amount]
    // Models: modelId -> ModelInfo (registered independently, each with its own price)
    models           *model.StoreMapping[uint64, ModelInfo]
    // Model earnings: modelId -> provider balance (accumulated per model)
    providerBalances *model.StoreMapping[uint64, Amount]
    // Providers: providerId -> ProviderInfo
    providers         *model.StoreMapping[uint64, ProviderInfo]
    // API Tokens
    tokens           *model.StoreMapping[uint64, TokenInfo]
    userTokens       *model.StoreList2D[UniAddr, uint64, uint64]
    // Usage records: "{tokenId}_{day}" -> DailyUsage
    usages           *model.StoreMapping[string, DailyUsage]
    nextModelId      *model.StoreValue[uint64]
    nextProviderId    *model.StoreValue[uint64]
    nextTokenId      *model.StoreValue[uint64]
}

// ModelInfo is an independently registered model with its own points price
type ModelInfo struct {
    Name        string      // model name, e.g. "gpt-4"
    Owner       UniAddr     // supplier address
    PointsPer1K uint64      // points consumed per 1000 tokens
    IsActive    bool
    CreatedAt   uint64
}

type ProviderInfo struct {
    Name       string
    BaseURL    string       // supplier API endpoint
    ModelIds   []uint64     // list of associated model IDs
    Priority   uint32       // routing priority
    IsActive   bool
    Owner      UniAddr      // supplier address
}

type TokenInfo struct {
    Owner      UniAddr
    RateLimit  uint32       // max requests per minute
    IsActive   bool
    CreatedAt  uint64
}

type DailyUsage struct {
    TokenId    uint64
    ProviderId  uint64
    ModelId    uint64
    Day        string       // "2025-01-15"
    Count      uint64       // number of calls
    TokensUsed uint64       // tokens consumed
    PointsUsed uint64       // points consumed
}
```

### Points Consumption Flow (Relay Middleware)

```
Client POST /v1/chat/completions
  Authorization: Bearer <api-key>
  Body: { "model": "gpt-4", ... }
  |
  v
+-- tee-provider HTTP Gateway -----------------------------------+
|                                                                |
| ① Parse api-key -> tokenId -> owner                            |
| ② Query Gateway.GetBalance(owner)                              |
|    -> reject if balance is insufficient                        |
| ③ Look up modelId from the requested model name                |
|    Query Gateway.GetModel(modelId) -> ModelInfo                |
| ④ Query Gateway.ListActiveProviders(modelId)                   |
|    -> pick the best Provider by Priority                       |
| ⑤ Forward the request to Provider.BaseURL -> receive response  |
| ⑥ Compute cost: cost = ceil(tokensUsed / 1000                  |
|                         * model.PointsPer1K)                   |
| ⑦ Mutation Gateway.Consume(tokenId, providerId,                |
|                              modelId, tokensUsed)              |
|    -> user.balance -= cost                                     |
|    -> model.providerBalance += cost                            |
| ⑧ Return the response to the client                            |
+----------------------------------------------------------------+
  |
  v
Supplier API (OpenAI / Claude / ...)
```

### Registration & Routing Example

```
① Supplier A registers models:
   RegisterModel(owner=A, name="gpt-4",      pointsPer1K=30)  -> modelId=1
   RegisterModel(owner=A, name="gpt-3.5",    pointsPer1K=3)   -> modelId=2

② Supplier B registers a model:
   RegisterModel(owner=B, name="claude-3",   pointsPer1K=25)  -> modelId=3

③ Supplier C registers a model:
   RegisterModel(owner=C, name="qwen-turbo", pointsPer1K=1)   -> modelId=4

④ Supplier A creates a Provider:
   CreateProvider(owner=A, name="OpenAI channel", baseUrl="https://api.openai.com",
                 modelIds=[1, 2], priority=10)  -> providerId=1

⑤ User calls gpt-4:
   request model="gpt-4" -> matches modelId=1 -> routed to providerId=1
   deduction: 1000 tokens * 30 points/1K = 30 points

⑥ Supplier A withdraws:
   Withdraw(modelId=1, amount=300)  // withdraw from gpt-4 earnings
```

> **Models and Providers are decoupled**: models are registered and priced independently via `RegisterModel`; Providers associate models via `modelIds`. The same supplier can offer different model combinations through multiple Providers, and the same model can be referenced by multiple Providers (each supplier prices it independently).

---

## Reusing DKG Private Key Storage with Existing Proxy Re-encryption

tee-provider already has mature proxy re-encryption infrastructure (`pkg/proxy-reenc`) used to securely distribute user-uploaded secrets (environment variables, disk keys). DKG private key storage **reuses the same DKG key material**:

| Crypto service | DKG material used | Purpose |
|-----------|---------------|------|
| Proxy Re-encryption (existing) | DKG public key + share `sᵢ` | Securely convert an encrypted secret from the DKG public key to a client public key |
| Threshold ECDSA (new) | DKG public key + share `sᵢ` | Threshold signing for user wallet keys |
| TEE Attestation (existing) | TEE report | Proves computation ran in a real enclave |

**Unified security model**: one DKG network provides both "proxy re-encryption" and "threshold signing" crypto services, run by the same validator set, sharing the same trust assumption (≥2/3 honest + TEE security).

---

## Directory Structure (planned)

```
tee-provider/
├── main.go                          # Entry point
├── go.mod / go.sum
├── Dockerfile
│
├── tee-chain/                      # Sidechain core (CometBFT ABCI)
│   ├── mod.go                       # Initialization
│   ├── consensus.go                 # Consensus state machine
│   ├── chains.go                    # Multi-chain management
│   ├── epoch.go                     # Epochs
│   ├── hub_sync.go                  # Main-chain sync
│   ├── secret_api.go                # Secret request handling
│   ├── secret_reply.go              # Secret reply handling
│   ├── secret_store.go              # Secret storage
│   ├── tx_*.go                      # Transaction processing
│   └── contracts/                   # On-chain contract ABIs
│
├── pkg/
│   ├── api/                         # External API (to be extended)
│   │   ├── dao/                     # DAO interfaces
│   │   ├── auth/                    # Web3Auth interfaces ← new
│   │   ├── gateway/                 # API gateway interfaces ← new
│   │   └── middleware/              # Middleware (CORS / Auth / RateLimit)
│   │
│   ├── chains/                      # Chain abstraction layer
│   ├── dkg/                         # Distributed key generation
│   ├── model/                       # Data models + Pebble DB
│   ├── network/                     # P2P network
│   ├── proxy-reenc/                 # Proxy re-encryption
│   └── util/                        # Utilities
│
├── services/                        # Service layer ← new
│   ├── auth/                        # Web3Auth service
│   │   ├── otp.go                   # OTP generation / verification
│   │   ├── oauth.go                 # Social login
│   │   ├── jwt.go                   # JWT issuance / verification
│   │   └── email.go                 # Email sending
│   │
│   ├── wallet/                      # Wallet service
│   │   ├── derive.go                # Key derivation (email → privkey)
│   │   ├── signer.go                # Transaction / message signing
│   │   └── encrypt.go               # Password-encrypted private key
│   │
│   ├── gateway/                     # API gateway service
│   │   ├── relay.go                 # Request relay
│   │   ├── token.go                 # API Token management
│   │   ├── points.go                # Points top-up / consumption / withdrawal
│   │   ├── ratelimit.go             # Rate limiting
│   │   └── provider.go              # Provider + exchange rate management
│   │
│   └── recovery/                    # Social recovery
│
├── graph/                           # GraphQL API (existing)
│   ├── mod.go
│   ├── resolver.go
│   ├── *.graphqls                   # Schema
│   └── *.resolvers.go               # Resolvers
│
├── ext/                             # External tools
├── hack/                            # Deployment / build scripts
├── docs/                            # Documentation
└── tools/                           # Tools
```

---

## Deployment Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                      Deployment Environment                  │
│                                                             │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                 Kubernetes Cluster                     │  │
│  │                                                       │  │
│  │  ┌──────────┐  ┌──────────┐  ┌──────────┐            │  │
│  │  │  tee-0   │  │  tee-1   │  │  tee-2   │  ...       │  │
│  │  │  (TEE)   │  │  (TEE)   │  │  (TEE)   │            │  │
│  │  │  SGX     │  │  SGX     │  │  SGX     │            │  │
│  │  └──────────┘  └──────────┘  └──────────┘            │  │
│  │                                                       │  │
│  │  Requires:                                            │  │
│  │  - Intel SGX DCAP drivers                             │  │
│  │  - Ego runtime                                        │  │
│  │  - PCCS (Provisioning Certification Caching Service)  │  │
│  └───────────────────────────────────────────────────────┘  │
│                                                             │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              External Dependencies                     │  │
│  │  - RPC endpoints of Polkadot / any chain              │  │
│  │  - SMTP server (email sending)                        │  │
│  │  - OAuth providers (Google / GitHub / Twitter)        │  │
│  │  - AI supplier APIs (OpenAI / Claude / ...)           │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## How to Start

```bash
# Environment variables
export CHAIN_ENV=local               # Main chain environment: local / test / main
export GQL_PORT=61000                # GraphQL port
export SIDE_CHAIN_PORT=61001         # P2P port
export SIDE_CHAIN_ADDR=0.0.0.0       # P2P listen address

# Email config (Web3Auth service)
export SMTP_HOST=smtp.example.com
export SMTP_PORT=587
export SMTP_FROM=auth@example.com
export SMTP_USER=auth@example.com
export SMTP_PASSWORD=your-password
export MOCK_EMAIL=true               # Dev mode: print OTP to console

# OAuth config
export GOOGLE_CLIENT_ID=xxx
export GOOGLE_CLIENT_SECRET=xxx
export GITHUB_CLIENT_ID=xxx
export GITHUB_CLIENT_SECRET=xxx
export TWITTER_CLIENT_ID=xxx
export TWITTER_CLIENT_SECRET=xxx

# TEE mode
export OE_MODE=SIM                   # Dev: SIM / Production: HW

# Start
go run main.go
```

---

## Relationship with Other Projects

| Project | Relationship |
|------|------|
| **web3auth** | All `web3auth/back` functionality migrates into tee-provider's `services/auth/` and `services/wallet/`; `web3auth/front` stays unchanged, with API addresses pointing to tee-provider |
| **token** | `token`'s relay and API management migrate into tee-provider's `services/gateway/`; the dashboard static pages are hosted by tee-provider; contract interactions reuse the existing `graph/contract.resolvers.go` |
| **contract** | Contracts are deployed on the target chain; tee-provider interacts with them via the `pkg/chains` module. New Auth / Gateway related contracts added |
| **dapp** | Frontend unchanged; API base address points to any node of the tee-provider subnet |
| **dashboard** | Admin panel unchanged; API base address points to the tee-provider subnet |

---

## Security Model

```
┌──────────────────────────────────────────────────────────┐
│                    Trust Model                            │
│                                                          │
│  Assumptions:                                             │
│  ✗ No single node is trusted                              │
│  ✓ TEE hardware is trusted (Intel SGX / AMD SEV)          │
│  ✓ More than 2/3 of nodes are honest (BFT assumption)     │
│  ✓ DKG threshold (t-of-n, at least t nodes honest)        │
│                                                          │
│  Attack surface:                                          │
│  - TEE side-channel attacks → rely on hardware vendor     │
│    security updates                                       │
│  - Consensus attacks (≥1/3 malicious) → CometBFT          │
│    safety guarantees                                      │
│  - DKG collusion (< threshold) → threshold crypto         │
│    guarantees                                             │
│  - Network partition → rely on main-chain finality        │
└──────────────────────────────────────────────────────────┘
```

---

## License

Apache 2.0
