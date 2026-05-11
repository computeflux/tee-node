# tee-dsecret 逻辑架构图

## 1. 系统初始化流程

```mermaid
graph TB
    Start[程序启动] --> InitDB[初始化数据库]
    InitDB --> InitP2PKey[初始化P2P密钥]
    InitP2PKey --> InitValidatorKey[加载验证器密钥]
    InitValidatorKey --> InitNodeKey[初始化主链节点密钥]
    InitNodeKey --> ConnectMainChain[连接主链]
    ConnectMainChain --> InitSideChain[初始化侧链节点]
    InitSideChain --> InitBFT[初始化BFT共识节点]
    InitBFT --> InitDKG[初始化DKG分布式密钥生成]
    InitDKG --> StartDKG[启动DKG服务]
    StartDKG --> StartGraphQL[启动GraphQL服务器]
    StartGraphQL --> Running[系统运行中]
    
    InitSideChain --> GetBootPeers[获取启动节点]
    GetBootPeers --> ConfigP2P[配置P2P网络]
    ConfigP2P --> ConfigBFT[配置BFT共识]
    ConfigBFT --> CreateNode[创建BFT节点]
    CreateNode --> AddReactor[添加DKG Reactor]
    
    InitDKG --> RestoreState[恢复DKG状态]
    RestoreState --> CreatePersistChan[创建持久化通道]
```

## 2. 秘密上传流程

```mermaid
sequenceDiagram
    participant User as 用户
    participant GraphQL as GraphQL API
    participant SideChain as 侧链
    participant DKG as DKG模块
    participant DB as 数据库
    
    User->>GraphQL: upload_secret(secret, hash, user, index)
    GraphQL->>GraphQL: RSA解密secret
    GraphQL->>GraphQL: 验证hash
    GraphQL->>SideChain: Encrypt(msg)
    SideChain->>DKG: GetDkgPubkey()
    DKG-->>SideChain: DKG公钥
    SideChain->>SideChain: proxy_reenc.EncryptSecret()
    Note over SideChain: 生成encCmt和encScrt
    SideChain-->>GraphQL: 加密数据
    GraphQL->>GraphQL: 构建TeeCall
    GraphQL->>GraphQL: IssueReport(TEE报告)
    GraphQL->>SideChain: SubmitTx(上传交易)
    SideChain->>SideChain: ProcessTx(处理交易)
    SideChain->>SideChain: FinalizeTx(最终化交易)
    SideChain->>DB: SaveSecret(user, index, data)
    DB-->>SideChain: 保存成功
    SideChain-->>GraphQL: 交易成功
    GraphQL-->>User: 返回true
```

## 3. 秘密解密流程（Pod启动时）

```mermaid
sequenceDiagram
    participant Pod as Pod/TEE Worker
    participant SideChain as 侧链节点
    participant Validators as 验证节点群
    participant DKG as DKG模块
    participant DB as 数据库
    
    Pod->>SideChain: BroadcastReencryptReq(PodStart)
    Note over Pod: 包含Pod公钥、Secret IDs、Disk IDs
    
    SideChain->>Validators: 广播重加密请求
    Note over Validators: 发送到所有验证节点
    
    loop 每个验证节点
        Validators->>Validators: HandleReencryptReq()
        Validators->>DB: GetSecrets(namespace, secretIds)
        Validators->>DB: GetDiskKeys(namespace, diskIds)
        Validators->>DKG: 获取DKG份额
        Validators->>Validators: proxy_reenc.Reencrypt()
        Note over Validators: 使用DKG份额和Pod公钥<br/>生成重加密份额
        Validators->>Validators: EncodeDecryptShare()
        Validators->>SideChain: 发送重加密响应
    end
    
    SideChain->>SideChain: 收集阈值数量的响应
    Note over SideChain: threshold = 2/3 + 1
    
    SideChain->>SideChain: VerifyReencryptResp()
    Note over SideChain: 验证每个节点的重加密响应
    
    SideChain->>SideChain: Recover重加密承诺
    Note over SideChain: 使用Lagrange插值恢复xncCmt
    
    SideChain->>DB: GetSecrets()
    SideChain->>DB: GetDiskKeys()
    
    SideChain->>SideChain: 构建DecryptResp
    Note over SideChain: 包含DKG公钥、<br/>加密的Secrets和DiskKeys、<br/>重加密承诺xncCmt
    
    SideChain-->>Pod: DecryptResp
    Pod->>Pod: DecryptSecret()
    Note over Pod: 使用Pod私钥解密<br/>encScrt - xncCmt + rdrSk * dkgPk = K
```

## 4. DKG分布式密钥生成流程

```mermaid
graph TB
    Start[开始DKG] --> CheckEpoch{检查Epoch}
    CheckEpoch -->|新Epoch| GetValidators[获取验证器列表]
    GetValidators --> InitDKG[初始化DKG协议]
    InitDKG --> GenerateDeal[生成Deal]
    GenerateDeal --> BroadcastDeal[广播Deal到所有节点]
    
    BroadcastDeal --> ReceiveDeal[接收其他节点的Deal]
    ReceiveDeal --> ProcessDeal[处理Deal]
    ProcessDeal --> GenerateResponse[生成Response]
    GenerateResponse --> BroadcastResponse[广播Response]
    
    BroadcastResponse --> ReceiveResponse[接收Response]
    ReceiveResponse --> ProcessResponse[处理Response]
    ProcessResponse --> CheckThreshold{达到阈值?}
    
    CheckThreshold -->|否| WaitMore[等待更多Response]
    WaitMore --> ReceiveResponse
    
    CheckThreshold -->|是| GenerateKeyShare[生成密钥份额]
    GenerateKeyShare --> SaveState[保存DKG状态]
    SaveState --> DKGReady[DKG就绪]
    
    DKGReady --> EpochChange{新Epoch?}
    EpochChange -->|是| StartNewEpoch[开始新Epoch DKG]
    StartNewEpoch --> GetValidators
    EpochChange -->|否| DKGReady
```

## 5. 侧链交易处理流程

```mermaid
graph TB
    ReceiveTx[接收交易] --> UnpackTx[解包TxBox]
    UnpackTx --> ParseTx[解析Tx]
    ParseTx --> CheckType{交易类型}
    
    CheckType -->|Empty| EmptyTx[空交易]
    CheckType -->|EpochStart| EpochStart[设置Epoch状态]
    CheckType -->|EpochEnd| EpochEnd[计算验证器更新<br/>设置新Epoch]
    CheckType -->|SyncTxStart| SyncStart[开始同步交易]
    CheckType -->|SyncTxEnd| SyncEnd[结束同步交易]
    CheckType -->|HubCall| ProcessHubCall[处理HubCall]
    
    ProcessHubCall --> HubCallType{HubCall类型}
    HubCallType -->|PodStart| SkipPodStart[跳过PodStart]
    HubCallType -->|PodMint| SkipPodMint[跳过PodMint]
    HubCallType -->|UploadSecret| SaveSecret[保存Secret到DB]
    HubCallType -->|InitDisk| SaveDiskKey[保存DiskKey到DB]
    
    SaveSecret --> FinalizeComplete[最终化完成]
    SaveDiskKey --> FinalizeComplete
    EpochStart --> FinalizeComplete
    EpochEnd --> FinalizeComplete
    SyncStart --> FinalizeComplete
    SyncEnd --> FinalizeComplete
    EmptyTx --> FinalizeComplete
    
    FinalizeComplete --> CheckHubTx{是Hub交易?}
    CheckHubTx -->|是| SendPartialSign[发送部分签名]
    CheckHubTx -->|否| End[结束]
    SendPartialSign --> End
```

## 6. 代理重加密（Proxy Re-encryption）流程

```mermaid
graph LR
    subgraph 加密阶段
        A1[原始秘密K] --> A2[选择随机数r]
        A2 --> A3[计算encCmt = rG]
        A2 --> A4[计算rsG = r * dkgPk]
        A4 --> A5[计算encScrt = rsG + K]
        A3 --> A6[存储encCmt, encScrt]
        A5 --> A6
    end
    
    subgraph 重加密阶段
        B1[接收Pod公钥xG] --> B2[获取DKG份额ski]
        B2 --> B3[计算xrG = xG + encCmt]
        B3 --> B4[计算xncSki = ski * xrG]
        B4 --> B5[生成随机数ri]
        B5 --> B6[计算UiHat = ri * xrG]
        B5 --> B7[计算HiHat = ri * G]
        B6 --> B8[计算挑战ei = Hash]
        B7 --> B8
        B4 --> B8
        B8 --> B9[计算证明fi = ri + ei * ski]
        B9 --> B10[返回份额和证明]
    end
    
    subgraph 验证阶段
        C1[接收重加密份额] --> C2[重构UiHat和HiHat]
        C2 --> C3[计算挑战ei']
        C3 --> C4{ei' == ei?}
        C4 -->|是| C5[验证通过]
        C4 -->|否| C6[验证失败]
    end
    
    subgraph 恢复阶段
        D1[收集阈值份额] --> D2[Lagrange插值]
        D2 --> D3[恢复xncCmt = rsG + xsG]
    end
    
    subgraph 解密阶段
        E1[接收xncCmt和encScrt] --> E2[计算xsG = x * dkgPk]
        E2 --> E3[计算rsG = xncCmt - xsG]
        E3 --> E4[计算K = encScrt - rsG]
        E4 --> E5[恢复原始秘密]
    end
```

## 7. 系统组件架构

```mermaid
graph TB
    subgraph 主链连接
        MainChain[主链连接模块]
        MainChain --> GetValidators[获取验证器列表]
        MainChain --> GetBootPeers[获取启动节点]
        MainChain --> SubmitTx[提交交易到主链]
    end
    
    subgraph 侧链节点
        SideChain[侧链应用]
        BFTNode[BFT共识节点]
        P2PNetwork[P2P网络]
        
        SideChain --> BFTNode
        BFTNode --> P2PNetwork
        SideChain --> SecretStore[秘密存储]
        SideChain --> SecretAPI[秘密API]
        SideChain --> TxProcess[交易处理]
    end
    
    subgraph DKG模块
        DKG[DKG实例]
        DKGHandler[DKG消息处理]
        DKGPersist[DKG持久化]
        
        DKG --> DKGHandler
        DKG --> DKGPersist
        DKG --> Consensus[共识处理]
        DKG --> Deal[Deal处理]
    end
    
    subgraph GraphQL API
        GraphQL[GraphQL服务器]
        Mutation[变更操作]
        Query[查询操作]
        
        GraphQL --> Mutation
        GraphQL --> Query
        Mutation --> UploadSecret[上传秘密]
        Mutation --> InitDiskKey[初始化磁盘密钥]
    end
    
    subgraph 数据库
        DB[PebbleDB]
        SecretDB[秘密存储]
        DKGDB[DKG状态存储]
        
        DB --> SecretDB
        DB --> DKGDB
    end
    
    subgraph 加密模块
        ProxyReenc[代理重加密]
        Encrypt[加密函数]
        Reencrypt[重加密函数]
        Verify[验证函数]
        Recover[恢复函数]
        Decrypt[解密函数]
        
        ProxyReenc --> Encrypt
        ProxyReenc --> Reencrypt
        ProxyReenc --> Verify
        ProxyReenc --> Recover
        ProxyReenc --> Decrypt
    end
    
    MainChain --> SideChain
    SideChain --> DKG
    GraphQL --> SideChain
    SideChain --> DB
    SideChain --> ProxyReenc
    DKG --> ProxyReenc
```

## 8. 数据流图

```mermaid
flowchart TD
    Start([用户操作]) --> Choice{操作类型}
    
    Choice -->|上传秘密| UploadFlow[上传流程]
    Choice -->|Pod启动| DecryptFlow[解密流程]
    Choice -->|初始化磁盘| DiskFlow[磁盘初始化流程]
    
    UploadFlow --> RSAEnc[RSA加密]
    RSAEnc --> HashVerify[Hash验证]
    HashVerify --> DKGEnc[DKG公钥加密]
    DKGEnc --> BuildTx[构建交易]
    BuildTx --> TEEReport[生成TEE报告]
    TEEReport --> SubmitSideChain[提交到侧链]
    SubmitSideChain --> BFTConsensus[BFT共识]
    BFTConsensus --> SaveDB[(保存到数据库)]
    
    DecryptFlow --> PodRequest[Pod请求解密]
    PodRequest --> Broadcast[广播到验证节点]
    Broadcast --> Reencrypt[节点重加密]
    Reencrypt --> Collect[收集阈值响应]
    Collect --> Verify[验证响应]
    Verify --> Recover[恢复重加密承诺]
    Recover --> Return[返回给Pod]
    Return --> PodDecrypt[Pod本地解密]
    
    DiskFlow --> GenKey[生成随机密钥]
    GenKey --> DKGEnc
```

## 关键概念说明

### DKG (Distributed Key Generation)
- **目的**: 在多个验证节点之间分布式生成一个共享密钥
- **阈值**: 需要 2/3 + 1 个节点才能恢复密钥
- **用途**: 用于加密和解密用户秘密

### 代理重加密 (Proxy Re-encryption)
- **目的**: 允许将使用DKG公钥加密的密文转换为使用Pod公钥加密的密文
- **特点**: 验证节点不需要知道原始秘密，只需要使用自己的DKG份额进行重加密
- **安全性**: 使用NIZK（零知识证明）确保重加密的正确性

### 侧链 (Side Chain)
- **共识**: 使用CometBFT (Tendermint) BFT共识
- **存储**: 使用PebbleDB存储加密的秘密数据
- **功能**: 处理秘密的上传、存储和重加密请求

### TEE (Trusted Execution Environment)
- **支持**: SGX、SEV-SNP、TDX
- **用途**: 生成TEE报告，证明代码在可信环境中运行
- **验证**: 主链验证TEE报告的有效性

## 安全特性

1. **分布式密钥**: 密钥分散在多个节点，单个节点无法恢复完整密钥
2. **阈值加密**: 需要超过2/3的节点参与才能解密
3. **零知识证明**: 重加密过程使用NIZK证明，确保正确性
4. **TEE验证**: 使用TEE报告确保代码在可信环境中运行
5. **端到端加密**: 从用户上传到Pod解密全程加密


