// 共享运行时（chain 包）模板：tee-chain 客户端。
//
// 模板为 raw string（反引号定界），其中反引号一律写成 btGate 占位符（渲染时替换），
// 因为生成代码里的结构体 tag 需要反引号。
//
// 与生成代码的关系：合约目录的 calls.go 只依赖 chain 包的 Ink 入口
// （QueryInk / DryRunInk / CallInk / CallOfInk）与几个参数类型，收发细节全在 chain 包内。
package main

// chainPkgDoc chain 包注释（放在 client.go 的 package 之前）。
const chainPkgDoc = `// Package chain 是 tee-chain 合约调用客户端：tee-node GraphQL（只读查询 / 变更预演）
// + CometBFT RPC（交易提交 / 确认），签名器可插拔（见 signer.go）。
// 本包由 sdkgen 生成，勿手改。
`

// chainClientSrc chain 包客户端：类型 + 参数 + Ink 入口 + 交易与提交。
const chainClientSrc = `// 调用方类型（链上 caller_type）。
const (
	CallerTypeNode    uint32 = 0 // 链节点 ed25519（仅交易路径；GraphQL dryRun / nonce 不支持）
	CallerTypeSr25519 uint32 = 1 // sr25519（polkadot 体系）
	CallerTypeEVM     uint32 = 2 // EVM secp256k1（20 字节地址）
	CallerTypeEd25519 uint32 = 3 // ed25519（32 字节公钥）
)

// ChainClient tee-chain 客户端：
//
//   - 只读查询 / 变更预演：tee-node GraphQL contractDryRun
//   - 变更提交：签名器签名 → protobuf Tx → CometBFT broadcast_tx_sync → 轮询确认
//
// 签名器（Signer，见 signer.go）可插拔：内置 Ed25519（calltype 3）/ Sr25519（calltype 1），
// 其他方案（EVM secp256k1、链节点 ed25519、远程 KMS）实现 Signer 即可。
type ChainClient struct {
	signer    Signer
	callerPub []byte
	rpcURL    string
	gqlURL    string
	http      *http.Client

	// Debug 打印调用明细（dryRun 参数等）。
	Debug bool
}

// InitClient 构造客户端；signer 可为 nil（只做只读查询）。
func InitClient(rpcURL, gqlURL string, signer Signer) (*ChainClient, error) {
	c := &ChainClient{
		rpcURL: strings.TrimSuffix(rpcURL, "/"),
		gqlURL: strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(gqlURL, "/gql"), "/graphql"), "/"),
		http:   &http.Client{Timeout: 10 * time.Second},
	}
	c.SetSigner(signer)
	return c, nil
}

// SetSigner 设置 / 替换签名器（nil = 只读）；Tx.caller 随签名器变化。
func (c *ChainClient) SetSigner(s Signer) {
	c.signer = s
	if s == nil {
		c.callerPub = nil
		return
	}
	c.callerPub = s.Public()
}

// Signer 返回当前签名器（nil = 只读）。
func (c *ChainClient) Signer() Signer { return c.signer }

// Caller 返回签名器公钥（Tx.caller；未配置签名器时 nil）。
func (c *ChainClient) Caller() []byte { return c.callerPub }

// CallerHex 返回签名器公钥 hex（0x 前缀；未配置签名器时空串）。
func (c *ChainClient) CallerHex() string {
	if c.callerPub == nil {
		return ""
	}
	return "0x" + hex.EncodeToString(c.callerPub)
}

// NextNonce 查询本 caller 在链上的下一可用 nonce（GraphQL contractNonce）。
// 链上 nonce 从 1 起，无交易记录时返回 1；提交路径每次都用它，不缓存。
func (c *ChainClient) NextNonce() (uint64, error) {
	if c.signer == nil {
		return 0, fmt.Errorf("signer not configured")
	}
	return c.queryNonce(c.callerPub, c.signer.CallerType())
}

// queryNonce 查询指定 caller 的下一可用 nonce。
func (c *ChainClient) queryNonce(caller []byte, callerType uint32) (uint64, error) {
	body, err := json.Marshal(map[string]any{
		"query":     "query($c: String!, $t: Int!){ contractNonce(caller: $c, callerType: $t) }",
		"variables": map[string]any{"c": "0x" + hex.EncodeToString(caller), "t": int(callerType)},
	})
	if err != nil {
		return 0, err
	}
	data, err := c.gqlPost(body)
	if err != nil {
		return 0, err
	}
	var v struct {
		ContractNonce uint64 @BT@json:"contractNonce"@BT@
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return 0, fmt.Errorf("decode contractNonce: %w", err)
	}
	if v.ContractNonce == 0 {
		return 0, fmt.Errorf("contractNonce returned 0 (caller 0x%x type %d)", caller, callerType)
	}
	return v.ContractNonce, nil
}

// DryRunParams 只读查询 / 变更预演的调用方身份。
// 零值按匿名只读处理（EVM 零地址，callerType=2）：GraphQL contractDryRun / contractNonce
// 不接受 callerType 0；需要 caller 维度（如 owner → provider）时用 ParamsFromSigner / EVMParams。
type DryRunParams struct {
	Caller     []byte
	CallerType uint32
}

// ExecParams 提交参数。
type ExecParams struct {
	// Signer 本次提交使用的签名器；nil 时用 ChainClient 上的签名器。
	Signer Signer
	// Nonce 指定 nonce；0 时提交前查询链上下一可用 nonce。
	Nonce uint64
}

// DefaultDryRunParams 匿名只读参数（EVM 零地址）。
func DefaultDryRunParams() DryRunParams {
	return DryRunParams{Caller: make([]byte, 20), CallerType: CallerTypeEVM}
}

// ParamsFromSigner 以签名器身份查询（owner 维度查询等需要真实 caller 的场景）。
func ParamsFromSigner(s Signer) DryRunParams {
	if s == nil {
		return DefaultDryRunParams()
	}
	return DryRunParams{Caller: s.Public(), CallerType: s.CallerType()}
}

// EVMParams 以 EVM 地址（0x + 20 字节 hex）为 caller 查询。
func EVMParams(addr string) (DryRunParams, error) {
	b, err := hex.DecodeString(strings.TrimPrefix(strings.TrimPrefix(addr, "0x"), "0X"))
	if err != nil {
		return DryRunParams{}, fmt.Errorf("invalid EVM address %q: %w", addr, err)
	}
	if len(b) != 20 {
		return DryRunParams{}, fmt.Errorf("invalid EVM address %q: want 20 bytes, got %d", addr, len(b))
	}
	return DryRunParams{Caller: b, CallerType: CallerTypeEVM}, nil
}

// DefaultExecParams 默认提交参数：签名器取客户端上的，nonce 自动查询。
func DefaultExecParams() ExecParams { return ExecParams{} }

// resolve 归一化调用方：零值 → 匿名只读（EVM 零地址）。
func (p DryRunParams) resolve() ([]byte, uint32, error) {
	caller, callerType := p.Caller, p.CallerType
	if len(caller) == 0 {
		caller, callerType = make([]byte, 20), CallerTypeEVM
	}
	if callerType == CallerTypeNode {
		return nil, 0, fmt.Errorf("callerType 0 (node) is not supported by GraphQL contractDryRun")
	}
	return caller, callerType, nil
}

// Ink 生成代码的合约客户端接口（每个合约包的 <Name> 结构体实现）。
type Ink interface {
	Client() *ChainClient
	ContractName() string
}

// ContractInput 调用输入：selector + 参数（生成类型 / types.U256 / util.Option / 基础类型，
// 逐个按 SCALE 编码，与链上解码参数同一格式）。
type ContractInput struct {
	Selector string
	Args     []any
}

// QueryInk 只读查询：contractDryRun(mut=false)，返回 SCALE 解码后的返回值。
// 合约错误经 GraphQL errors 返回。
func QueryInk[T any](contract Ink, params DryRunParams, input ContractInput) (T, error) {
	var zero T
	raw, err := contract.Client().Query(params, contract.ContractName(), input)
	if err != nil {
		return zero, err
	}
	var v T
	if err := scale.NewDecoder(bytes.NewReader(raw)).Decode(&v); err != nil {
		return zero, fmt.Errorf("decode %s return: %w", contract.ContractName(), err)
	}
	return v, nil
}

// DryRunInk 变更预演：contractDryRun(mut=true)，只执行不落链、不消耗 nonce；
// 合约业务错误（状态校验失败等）在此带出。
func DryRunInk(contract Ink, params DryRunParams, input ContractInput) error {
	return contract.Client().DryRun(params, contract.ContractName(), input)
}

// CallInk 提交变更：查询 nonce → 签名 → broadcast_tx_sync → 轮询确认。
func CallInk(contract Ink, params ExecParams, input ContractInput) error {
	return contract.Client().Submit(params, contract.ContractName(), input)
}

// CallOfInk 构造变更交易（nonce + 签名），不提交。
func CallOfInk(contract Ink, params ExecParams, input ContractInput) (*Tx, error) {
	return contract.Client().CallOf(params, contract.ContractName(), input)
}

// Query 只读查询（contractDryRun mut=false），返回链上返回值的 SCALE 原始字节。
func (c *ChainClient) Query(params DryRunParams, contract string, input ContractInput) ([]byte, error) {
	args, err := scaleEncodeAll(input.Args...)
	if err != nil {
		return nil, fmt.Errorf("scale encode args: %w", err)
	}
	return c.contractDryRun(params, contract, input.Selector, args, false)
}

// DryRun 变更预演（contractDryRun mut=true）；合约错误经 GraphQL errors 返回。
func (c *ChainClient) DryRun(params DryRunParams, contract string, input ContractInput) error {
	args, err := scaleEncodeAll(input.Args...)
	if err != nil {
		return fmt.Errorf("scale encode args: %w", err)
	}
	_, err = c.contractDryRun(params, contract, input.Selector, args, true)
	return err
}

// Submit 提交变更：构造并签名交易（nonce 取 params.Nonce，为 0 时查询链上下一可用 nonce）
// → broadcast_tx_sync → 轮询确认。
func (c *ChainClient) Submit(params ExecParams, contract string, input ContractInput) error {
	signer, err := c.signerOf(params)
	if err != nil {
		return err
	}
	tx, err := c.CallOf(params, contract, input)
	if err != nil {
		return err
	}
	hash, err := c.broadcastTx(tx.Bytes())
	if err != nil {
		// 该链无 CheckTx 语义：广播被拒时 log 可能为空，用同一次调用重放取合约侧具体错误。
		return fmt.Errorf("%w (dry-run: %s)", err, c.dryRunError(ParamsFromSigner(signer), contract, input))
	}
	return c.waitTxConfirmed(hash)
}

// CallOf 构造交易：解析 nonce → 编码 ContractCall → 对签名原文签名，返回未广播的 Tx。
func (c *ChainClient) CallOf(params ExecParams, contract string, input ContractInput) (*Tx, error) {
	signer, err := c.signerOf(params)
	if err != nil {
		return nil, err
	}
	args, err := scaleEncodeAll(input.Args...)
	if err != nil {
		return nil, fmt.Errorf("scale encode args: %w", err)
	}
	selector := mustHexBytes(input.Selector)
	if len(selector) == 0 {
		return nil, fmt.Errorf("invalid selector %q", input.Selector)
	}

	nonce := params.Nonce
	if nonce == 0 {
		nonce, err = c.queryNonce(signer.Public(), signer.CallerType())
		if err != nil {
			return nil, fmt.Errorf("query nonce: %w", err)
		}
	}

	// ContractCall{name, method[:4], args, nonce}
	call := protoMsg(
		protoBytes(1, []byte(contract)),
		protoBytes(2, selector),
		protoRepeatedBytes(3, args),
		protoVarint(4, nonce),
	)
	// SysCall{contract: ContractCall}（字段 7，embedded message）
	tx := &Tx{
		Caller:     signer.Public(),
		CallerType: signer.CallerType(),
		Nonce:      nonce,
		Call:       protoMsg(protoBytesField(7, call)),
	}
	// 签名原文与链上 model.ContractForSig 一致：
	// <Bytes>{contract}{method[:4]}{args...}{nonce 大端 8B}</Bytes>
	sig, err := signer.Sign(contractForSig([]byte(contract), selector, args, nonce))
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	tx.Signature = sig
	return tx, nil
}

// Broadcast 广播交易（broadcast_tx_sync）并轮询确认；CallOf 拿到的 Tx 可直接用它提交。
func (c *ChainClient) Broadcast(tx *Tx) error {
	hash, err := c.broadcastTx(tx.Bytes())
	if err != nil {
		return err
	}
	return c.waitTxConfirmed(hash)
}

// signerOf 取本次提交的签名器：params.Signer 优先，其次客户端签名器。
func (c *ChainClient) signerOf(params ExecParams) (Signer, error) {
	s := params.Signer
	if s == nil {
		s = c.signer
	}
	if s == nil {
		return nil, fmt.Errorf("signer not configured")
	}
	return s, nil
}

// contractDryRun 走 tee-node GraphQL contractDryRun：
// mut=false → 只读查询，返回 hex 解码后的 SCALE 字节；mut=true → 变更预演，无返回数据。
func (c *ChainClient) contractDryRun(params DryRunParams, contract, selector string, args [][]byte, mut bool) ([]byte, error) {
	caller, callerType, err := params.resolve()
	if err != nil {
		return nil, err
	}
	callerHex := "0x" + hex.EncodeToString(caller)
	hexArgs := make([]string, 0, len(args))
	for _, a := range args {
		hexArgs = append(hexArgs, "0x"+hex.EncodeToString(a))
	}
	query := fmt.Sprintf(
		"query($m: String!, $a: [String!]!){ contractDryRun(caller: %q, callerType: %d, contract: %q, mut: %t, method: $m, args: $a) }",
		callerHex, callerType, contract, mut,
	)
	body, err := json.Marshal(map[string]any{
		"query":     query,
		"variables": map[string]any{"m": selector, "a": hexArgs},
	})
	if err != nil {
		return nil, err
	}
	if c.Debug {
		fmt.Printf("[ dryRun ] contract=%s mut=%t selector=%s caller=%s args=%v\n",
			contract, mut, selector, callerHex, hexArgs)
	}
	data, err := c.gqlPost(body)
	if err != nil {
		return nil, err
	}
	var v struct {
		ContractDryRun string @BT@json:"contractDryRun"@BT@
	}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("decode contractDryRun: %w", err)
	}
	if mut {
		return nil, nil
	}
	raw, err := hex.DecodeString(strings.TrimPrefix(v.ContractDryRun, "0x"))
	if err != nil {
		return nil, fmt.Errorf("decode contractDryRun hex: %w", err)
	}
	return raw, nil
}

// dryRunError 用 mut=true 重放一次调用，取合约侧具体错误（广播被拒时的解释来源）。
func (c *ChainClient) dryRunError(params DryRunParams, contract string, input ContractInput) string {
	if err := c.DryRun(params, contract, input); err != nil {
		return err.Error()
	}
	return "dry-run succeeded (未复现合约错误，原拒绝可能是瞬时问题：节点 / TEE 繁忙)"
}

// gqlPost 发送 GraphQL 请求，返回 data 字段；GraphQL errors 以 error 返回。
func (c *ChainClient) gqlPost(body []byte) (json.RawMessage, error) {
	req, err := http.NewRequest(http.MethodPost, c.gqlURL+"/gql", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gql http %d", resp.StatusCode)
	}
	var out struct {
		Data   json.RawMessage
		Errors []json.RawMessage
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if len(out.Errors) > 0 {
		return nil, fmt.Errorf("gql errors: %s", string(out.Errors[0]))
	}
	return out.Data, nil
}

// broadcastTx 提交到 CometBFT broadcast_tx_sync（tx 为 base64），返回交易 hash。
func (c *ChainClient) broadcastTx(tx []byte) (string, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      "sdkgen",
		"method":  "broadcast_tx_sync",
		"params":  map[string]any{"tx": base64.StdEncoding.EncodeToString(tx)},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest(http.MethodPost, c.rpcURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	// 完整读取响应：log 可能为空（如 gRPC UNAVAILABLE），只有完整响应能定位链侧拒绝原因。
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("broadcast_tx_sync read response: %w", err)
	}
	rawStr := truncateForLog(raw)
	var out struct {
		Result struct {
			Code uint32
			Log  string
			Hash string
		}
		Error *struct {
			Message string
			Data    string
		}
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("broadcast_tx_sync decode response: %w (raw: %s)", err, rawStr)
	}
	if out.Error != nil {
		return "", fmt.Errorf("broadcast_tx_sync: %s %s (raw: %s)", out.Error.Message, out.Error.Data, rawStr)
	}
	if out.Result.Code != 0 {
		return out.Result.Hash, fmt.Errorf("broadcast_tx_sync rejected: code=%d log=%q (raw: %s)",
			out.Result.Code, out.Result.Log, rawStr)
	}
	return out.Result.Hash, nil
}

// waitTxConfirmed 轮询 CometBFT /tx 直到交易进块（code=0 为成功）。
// 查询报错（含尚未建索引的 "tx not found"）继续轮询到超时，超时信息带上最后一次错误。
func (c *ChainClient) waitTxConfirmed(hash string) error {
	deadline := time.Now().Add(60 * time.Second)
	lastErr := ""
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("tx not confirmed within 60s: %s (last: %s)", hash, lastErr)
		}
		resp, err := c.http.Get(c.rpcURL + "/tx?hash=0x" + hash)
		if err != nil {
			lastErr = err.Error()
			time.Sleep(2 * time.Second)
			continue
		}
		var out struct {
			Result *struct {
				TxResult struct {
					Code int    @BT@json:"code"@BT@
					Log  string @BT@json:"log"@BT@
				} @BT@json:"tx_result"@BT@
			} @BT@json:"result"@BT@
			Error *struct {
				Data string @BT@json:"data"@BT@
			} @BT@json:"error"@BT@
		}
		_ = json.NewDecoder(resp.Body).Decode(&out)
		resp.Body.Close()
		if out.Result != nil {
			if out.Result.TxResult.Code == 0 {
				return nil
			}
			return fmt.Errorf("tx failed: %s (code %d)", out.Result.TxResult.Log, out.Result.TxResult.Code)
		}
		if out.Error != nil {
			lastErr = out.Error.Data
		}
		time.Sleep(2 * time.Second)
	}
}

// Tx tee-chain 交易：Tx{caller=10, caller_type=12, call=13, signature=11} 的
// protobuf wire 编码（见 Bytes）。
type Tx struct {
	// Caller 发起方公钥（链上 caller）。
	Caller []byte
	// CallerType 链上 caller_type（见 CallerType* 常量）。
	CallerType uint32
	// Nonce 链上 nonce（签名原文的一部分，防重放）。
	Nonce uint64
	// Call SysCall{contract: ContractCall{name, method, args, nonce}} 的 proto 编码。
	Call []byte
	// Signature 对签名原文（contractForSig）的签名。
	Signature []byte
}

// Bytes protobuf wire 编码，可直接用于 CometBFT broadcast_tx_sync。
func (t *Tx) Bytes() []byte {
	return protoMsg(
		protoBytes(10, t.Caller),
		protoVarint(12, uint64(t.CallerType)),
		protoBytes(13, t.Call),
		protoBytes(11, t.Signature),
	)
}

// Hash CometBFT 交易 hash（sha256(Bytes) 小写 hex），提交前可用于日志 / 追踪。
func (t *Tx) Hash() string {
	sum := sha256.Sum256(t.Bytes())
	return hex.EncodeToString(sum[:])
}
`

// chainCodecSrc chain 包编解码：SCALE 参数编码 + proto wire + 签名原文。
const chainCodecSrc = `// 本文件是生成代码与链上共用的编解码：SCALE 参数、protobuf wire、签名原文。

// contractForSig 签名原文，与链上 model.ContractForSig 一致：
// <Bytes>{contract}{method[:4]}{args...}{nonce 大端 8B}</Bytes>。
func contractForSig(contract, method []byte, args [][]byte, nonce uint64) []byte {
	var buf bytes.Buffer
	buf.WriteString("<Bytes>")
	buf.Write(contract)
	buf.Write(method)
	for _, a := range args {
		buf.Write(a)
	}
	var nb [8]byte
	binary.BigEndian.PutUint64(nb[:], nonce)
	buf.Write(nb[:])
	buf.WriteString("</Bytes>")
	return buf.Bytes()
}

// scaleEncodeAll 逐个 SCALE 编码合约参数（每个参数一个字节串，与链上 ContractCall.Args
// 布局一致）。参数为 Go 类型：基础类型按 Go 类型编码（bool / 整数 / string / []byte），
// 生成类型、types.U256、util.Option 等走各自的 Encode（Encodeable 接口）。
func scaleEncodeAll(vals ...any) ([][]byte, error) {
	out := make([][]byte, 0, len(vals))
	for _, v := range vals {
		var buf bytes.Buffer
		if err := scale.NewEncoder(&buf).Encode(v); err != nil {
			return nil, err
		}
		out = append(out, buf.Bytes())
	}
	return out, nil
}

// ── protobuf wire format（最小实现，与链上 model.Tx / SysCall / ContractCall 一致）──

// protoVarint 编码 varint 字段（field << 3 | 0）。
func protoVarint(field uint, v uint64) []byte {
	b := appendVarint(nil, uint64(field)<<3)
	return appendVarint(b, v)
}

// protoBytes 编码 bytes 字段（field << 3 | 2）。
func protoBytes(field uint, v []byte) []byte {
	return protoBytesField(field, v)
}

// protoRepeatedBytes 编码 repeated bytes 字段（每个元素一个 tag）。
func protoRepeatedBytes(field uint, vs [][]byte) []byte {
	var out []byte
	for _, v := range vs {
		out = append(out, protoBytes(field, v)...)
	}
	return out
}

// protoBytesField 编码长度前缀字段 / 内嵌 message。
func protoBytesField(field uint, inner []byte) []byte {
	b := appendVarint(nil, uint64(field)<<3|2)
	b = appendVarint(b, uint64(len(inner)))
	return append(b, inner...)
}

// protoMsg 顺序拼接各字段编码。
func protoMsg(parts ...[]byte) []byte {
	var out []byte
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

func appendVarint(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

// mustHexBytes hex 字符串 → 字节（"0x" 前缀可省；非法输入返回 nil）。
func mustHexBytes(s string) []byte {
	b, err := hex.DecodeString(strings.TrimPrefix(strings.TrimPrefix(s, "0x"), "0X"))
	if err != nil {
		return nil
	}
	return b
}

// truncateForLog 截断过长的链响应供错误日志使用（保留前 2KB）。
func truncateForLog(b []byte) string {
	const limit = 2048
	if len(b) <= limit {
		return string(b)
	}
	return string(b[:limit]) + "…(truncated, total " + strconv.Itoa(len(b)) + " bytes)"
}
`

// chainSignerSrc chain 包签名器接口与调用方类型（与具体密码库解耦）。
const chainSignerSrc = `// Signer 交易签名器（可插拔）：任何签名方案实现该接口即可注入，本包不依赖具体密码库。
//
// 内置 ed25519（calltype 3）与 sr25519（calltype 1）见 signer_builtin.go；
// 其他方案（EVM secp256k1 calltype 2、链节点 ed25519 calltype 0、远程 KMS / HSM）
// 同样实现本接口即可，例如把签名请求转发给外部密钥服务。
type Signer interface {
	// Sign 对交易签名原文签名（原文构造见 contractForSig）。
	// 各方案按链上验签规则处理原文：calltype 3（ed25519）直接签名；
	// calltype 0（节点 ed25519）对 >256 字节原文先用 blake2b-256 预哈希（见 PreHashIfLong）。
	Sign(msg []byte) ([]byte, error)
	// Public 返回调用方公钥（Tx.caller；sr25519 / ed25519 为 32 字节，EVM 为 20 字节）。
	Public() []byte
	// CallerType 返回链上 caller_type（见 CallerType* 常量）。
	CallerType() uint32
}

// PreHashIfLong 原文 >256 字节时先做 blake2b-256 预哈希：链上对节点 ed25519 签名
// （calltype 0，Ed25519SignVerify）与代码签名采用该规则；calltype 1/2/3 的验签不做预哈希。
func PreHashIfLong(msg []byte) []byte {
	if len(msg) > 256 {
		h := blake2b.Sum256(msg)
		return h[:]
	}
	return msg
}
`

// chainSignerBuiltinSrc chain 包内置签名器：ed25519 与 sr25519（-no-signers 时不生成）。
const chainSignerBuiltinSrc = `// 内置签名器：ed25519（calltype 3）与 sr25519（calltype 1）。
// seed 为 32 字节 hex（"0x" 前缀可省）；其他签名方案实现 Signer 接口即可（见 signer.go）。

// Ed25519Signer ed25519 签名器（calltype 3）。
type Ed25519Signer struct {
	kp subkey.KeyPair
}

// NewEd25519Signer 从 32 字节 seed（hex）构造 ed25519 签名器。
func NewEd25519Signer(seedHex string) (*Ed25519Signer, error) {
	kp, err := keyPairFromSeedHex("ed25519", ed25519.Scheme{}, seedHex)
	if err != nil {
		return nil, err
	}
	return &Ed25519Signer{kp: kp}, nil
}

// Sign 对原文签名（calltype 3 的验签不做预哈希）。
func (s *Ed25519Signer) Sign(msg []byte) ([]byte, error) { return s.kp.Sign(msg) }

// Public 返回公钥（32 字节）。
func (s *Ed25519Signer) Public() []byte { return s.kp.Public() }

// CallerType 链上 caller_type=3。
func (s *Ed25519Signer) CallerType() uint32 { return CallerTypeEd25519 }

// KeyPair 返回底层密钥对（签名非交易数据，如 TEE quote 节点签名 / CodeSignature）。
func (s *Ed25519Signer) KeyPair() subkey.KeyPair { return s.kp }

// Sr25519Signer sr25519 签名器（calltype 1，polkadot 体系）。
type Sr25519Signer struct {
	kp subkey.KeyPair
}

// NewSr25519Signer 从 32 字节 seed（hex）构造 sr25519 签名器。
func NewSr25519Signer(seedHex string) (*Sr25519Signer, error) {
	kp, err := keyPairFromSeedHex("sr25519", sr25519.Scheme{}, seedHex)
	if err != nil {
		return nil, err
	}
	return &Sr25519Signer{kp: kp}, nil
}

// Sign 对原文签名。
func (s *Sr25519Signer) Sign(msg []byte) ([]byte, error) { return s.kp.Sign(msg) }

// Public 返回公钥（32 字节）。
func (s *Sr25519Signer) Public() []byte { return s.kp.Public() }

// CallerType 链上 caller_type=1。
func (s *Sr25519Signer) CallerType() uint32 { return CallerTypeSr25519 }

// KeyPair 返回底层密钥对（签名非交易数据）。
func (s *Sr25519Signer) KeyPair() subkey.KeyPair { return s.kp }

// keyPairFromSeedHex 从 32 字节 hex seed 构造 subkey 密钥对。
func keyPairFromSeedHex(schemeName string, scheme subkey.Scheme, seedHex string) (subkey.KeyPair, error) {
	seed, err := hex.DecodeString(strings.TrimPrefix(strings.TrimPrefix(seedHex, "0x"), "0X"))
	if err != nil {
		return nil, fmt.Errorf("%s seed: %w", schemeName, err)
	}
	if len(seed) != 32 {
		return nil, fmt.Errorf("%s seed: want 32 bytes, got %d", schemeName, len(seed))
	}
	kp, err := scheme.FromSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("%s seed: %w", schemeName, err)
	}
	return kp, nil
}
`
