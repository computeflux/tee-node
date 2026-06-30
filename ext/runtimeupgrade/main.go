package main

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/sha512"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"go/ast"
	"go/parser"
	"go/token"

	"github.com/ethereum/go-ethereum/common"
	ethcrypto "github.com/ethereum/go-ethereum/crypto"

	"github.com/computeflux/tee-provider/pkg/model"

	"golang.org/x/crypto/blake2b"
	"golang.org/x/crypto/pbkdf2"
)

const (
	callerType             = uint32(2)
	contractName           = "gov"
	methodName             = "runtime_upgrade"
	runtimeUpgradeSelector = 0xa4771d9e
	pollingTimeout         = 60 * time.Second
)

func httpClient() *http.Client {
	return &http.Client{Transport: &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}}
}

func main() {
	wasmPath := flag.String("wasm", os.Getenv("WASM_PATH"), "path to runtime.wasm")
	desc := flag.String("desc", os.Getenv("UPGRADE_DESC"), "upgrade description")
	versionSrc := flag.String("version-src", envOr("NATIVE_VERSION_SRC", "tee-chain/runtime/module/call.go"), "path containing const NativeVersion")
	gqlURL := flag.String("gql", envOr("TEE_NODE_GQL", "https://xiaobai.asyou.me:30118"), "GraphQL base URL")
	mnemonic := flag.String("mnemonic", os.Getenv("SUDO_MNEMONIC"), "BIP39 mnemonic")
	wait := flag.Bool("wait", true, "poll txStatus")
	flag.Parse()

	if *mnemonic == "" {
		die("--mnemonic or SUDO_MNEMONIC required")
	}

	priv, addr, err := deriveBIP44(*mnemonic)
	if err != nil {
		die("derive key: %v", err)
	}
	caller := "0x" + hex.EncodeToString(ethcrypto.CompressPubkey(&priv.PublicKey))
	fmt.Printf("pubkey:   %s\n", caller)
	fmt.Printf("address:  %s\n", addr.Hex())
	pubkey := addr.Hex()

	if *wasmPath == "" {
		die("--wasm or WASM_PATH required")
	}
	wasm, err := os.ReadFile(*wasmPath)
	if err != nil {
		die("read wasm: %v", err)
	}
	fmt.Printf("wasm:     %s (%d bytes)\n", *wasmPath, len(wasm))

	gql := strings.TrimRight(*gqlURL, "/") + "/gql"

	ver, err := readNativeVersion(*versionSrc)
	if err != nil {
		die("read NativeVersion: %v", err)
	}
	fmt.Printf("native:   %d (from %s)\n", ver, *versionSrc)

	sel := uint32Selector(runtimeUpgradeSelector)
	fmt.Printf("selector: 0x%02x%02x%02x%02x\n", sel[0], sel[1], sel[2], sel[3])

	h := blake2b.Sum256(wasm)
	fmt.Printf("codehash: 0x%s\n", hex.EncodeToString(h[:]))

	payload := scaleEncodePendingRuntimeUpgrade(ver, h[:], wasm, *desc, 0)
	fmt.Printf("payload:  %d bytes\n", len(payload))
	payloadHex := "0x" + hex.EncodeToString(payload)

	nonce, err := gqlContractNonce(gql, pubkey, int(callerType))
	if err != nil {
		die("contractNonce: %v", err)
	}
	fmt.Printf("nonce:    %d\n", nonce)

	c := &model.ContractCall{
		Name:   []byte(contractName),
		Method: sel[:],
		Args:   [][]byte{payload},
		Nonce:  nonce,
	}

	call := &model.SysCall{
		Payload: &model.SysCall_Contract{
			Contract: c,
		},
	}

	rawSigBytes, _ := call.BytesForSig()
	sig, err := signEVMPersonal(rawSigBytes, priv)
	if err != nil {
		die("sign: %v", err)
	}

	err = model.SideSysCallVerify(addr.Bytes(), callerType, call, sig)
	if err != nil {
		die("SubmitContractCall: contract verify: " + err.Error())
	}
	fmt.Printf("sig:      0x%s (%d bytes)\n", hex.EncodeToString(sig), len(sig))

	txHash, err := gqlContractCall(gql, pubkey, int(callerType), int(nonce),
		contractName, methodName, []string{payloadHex}, hex.EncodeToString(sig))
	if err != nil {
		die("contractCall: %v", err)
	}
	fmt.Printf("tx hash:  %s\n", txHash)

	if *wait {
		fmt.Println("polling txStatus ...")
		if err := waitTxStatus(gql, txHash, pollingTimeout); err != nil {
			die("txStatus: %v", err)
		}
	}
}

// ── BIP44 secp256k1 ───────────────────────────────────────

var curveN = ethcrypto.S256().Params().N

// deriveBIP44 从 BIP39 助记词派生 MetaMask 兼容的 secp256k1 密钥（m/44'/60'/0'/0/0）。
// 使用标准 BIP39 seed 派生（PBKDF2(password=mnemonic, salt="mnemonic"+passphrase)）。
// 注意：不同于 schnorrkel.SeedFromMnemonic（Substrate 用 entropy 做 password）。
func deriveBIP44(mnemonic string) (*ecdsa.PrivateKey, common.Address, error) {
	// 标准 BIP39: seed = PBKDF2(mnemonic, "mnemonic"+passphrase, 2048, 64, SHA512)
	seed := pbkdf2.Key([]byte(strings.TrimSpace(mnemonic)), []byte("mnemonic"), 2048, 64, sha512.New)

	// BIP32 主密钥: HMAC-SHA512("Bitcoin seed", seed)
	mac := hmac.New(sha512.New, []byte("Bitcoin seed"))
	mac.Write(seed[:])
	I := mac.Sum(nil)
	key := I[:32]
	chain := I[32:]

	// BIP44: m/44'/60'/0'/0/0
	for _, idx := range []uint32{0x8000002C, 0x8000003C, 0x80000000, 0, 0} {
		key, chain = deriveChild(key, chain, idx)
	}

	priv, err := ethcrypto.ToECDSA(key)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("ToECDSA: %w", err)
	}
	return priv, ethcrypto.PubkeyToAddress(priv.PublicKey), nil
}

// deriveChild BIP32 子密钥派生。
// idx >= 0x80000000 = hardened（用私钥计算），否则用公钥。
func deriveChild(key, chain []byte, idx uint32) ([]byte, []byte) {
	mac := hmac.New(sha512.New, chain)
	if idx >= 0x80000000 {
		mac.Write([]byte{0x00})
		mac.Write(key)
	} else {
		// 从私钥恢复压缩公钥（33 字节；BIP32 规定 serP 用压缩格式）
		priv, _ := ethcrypto.ToECDSA(key)
		mac.Write(ethcrypto.CompressPubkey(&priv.PublicKey))
	}
	var idxB [4]byte
	binary.BigEndian.PutUint32(idxB[:], idx)
	mac.Write(idxB[:])
	I := mac.Sum(nil)

	IL := new(big.Int).SetBytes(I[:32])
	newKey := new(big.Int).Add(IL, new(big.Int).SetBytes(key))
	newKey.Mod(newKey, curveN)

	k := newKey.Bytes()
	if len(k) < 32 {
		pad := make([]byte, 32)
		copy(pad[32-len(k):], k)
		k = pad
	}
	return k, I[32:]
}

// signEVMPersonal EIP-191 personal_sign，返回 65 字节 [R||S||V]。
func signEVMPersonal(msg []byte, priv *ecdsa.PrivateKey) ([]byte, error) {
	data := fmt.Sprintf("call_tee_contract:%s", hex.EncodeToString(msg))
	pre := fmt.Sprintf("\x19Ethereum Signed Message:\n%d", len(data))
	hash := ethcrypto.Keccak256Hash(append([]byte(pre), []byte(data)...))
	sig, err := ethcrypto.Sign(hash.Bytes(), priv)
	if err != nil {
		return nil, err
	}
	if sig[64] < 27 {
		sig[64] += 27
	}
	return sig, nil
}

// ── GQL ───────────────────────────────────────────────────

type gqlReq struct {
	Query     string         `json:"query"`
	Variables map[string]any `json:"variables"`
}
type gqlResp struct {
	Data   json.RawMessage
	Errors []struct{ Message string }
}

func gqlPost(url, q string, vars map[string]any, out any) error {
	fmt.Fprintf(os.Stderr, "→ %s\n", url)
	body, _ := json.Marshal(gqlReq{Query: q, Variables: vars})
	resp, err := httpClient().Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var gr gqlResp
	if err := json.Unmarshal(raw, &gr); err != nil {
		return fmt.Errorf("decode: %w (raw=%s)", err, raw)
	}
	if len(gr.Errors) > 0 {
		return fmt.Errorf("gql: %v", gr.Errors)
	}
	if out != nil {
		return json.Unmarshal(gr.Data, out)
	}
	return nil
}

func gqlContractNonce(url, caller string, t int) (uint64, error) {
	var out struct{ ContractNonce uint64 }
	if err := gqlPost(url,
		`query($c:String!,$t:Int!){ contractNonce(caller:$c,callerType:$t) }`,
		map[string]any{"c": caller, "t": t}, &out); err != nil {
		return 0, err
	}
	return out.ContractNonce, nil
}

func gqlContractCall(url, caller string, t, n int, contract, method string, args []string, sig string) (string, error) {
	var out struct{ ContractCall string }
	err := gqlPost(url,
		`mutation($c:String!,$t:Int!,$n:Int!,$p:String!,$m:String!,$a:[String!]!,$s:String!){
		  contractCall(caller:$c, callerType:$t, nonce:$n, contract:$p, method:$m, args:$a, signature:$s)
		}`,
		map[string]any{"c": caller, "t": t, "n": n, "p": contract, "m": method, "a": args, "s": sig},
		&out)
	return out.ContractCall, err
}

func waitTxStatus(url, hash string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var out struct{ TxStatus string }
		if err := gqlPost(url, `query($h:String!){ txStatus(hash:$h) }`, map[string]any{"h": hash}, &out); err != nil {
			return err
		}
		switch out.TxStatus {
		case "SUCCESS", "COMMITTED", "OK", "FINALIZED", "DONE":
			fmt.Printf("ok: %s\n", out.TxStatus)
			return nil
		case "FAILED", "REVERTED", "ERROR":
			return fmt.Errorf("tx failed: %s", out.TxStatus)
		case "PENDING", "PROCESSING", "QUEUED", "":
			fmt.Print(".")
			time.Sleep(2 * time.Second)
		default:
			if !strings.Contains(strings.ToUpper(out.TxStatus), "PEND") {
				fmt.Printf("ok: %s\n", out.TxStatus)
				return nil
			}
			fmt.Print(".")
			time.Sleep(2 * time.Second)
		}
	}
	return fmt.Errorf("timeout")
}

// ── AST ───────────────────────────────────────────────────

func uint32Selector(v uint32) [4]byte {
	return [4]byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)}
}

func readNativeVersion(p string) (uint32, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, p, nil, parser.ParseComments)
	if err != nil {
		return 0, err
	}
	for _, d := range f.Decls {
		gd, ok := d.(*ast.GenDecl)
		if !ok || gd.Tok != token.CONST {
			continue
		}
		for _, sp := range gd.Specs {
			vs := sp.(*ast.ValueSpec)
			for i, n := range vs.Names {
				if n.Name == "NativeVersion" && i < len(vs.Values) {
					v, err := evalUint32Expr(vs.Values[i])
					if err != nil {
						return 0, fmt.Errorf("NativeVersion: %w", err)
					}
					return v, nil
				}
			}
		}
	}
	return 0, fmt.Errorf("const NativeVersion not found in %s", p)
}

func evalUint32Expr(e ast.Expr) (uint32, error) {
	switch v := e.(type) {
	case *ast.BasicLit:
		n, err := strconv.ParseUint(v.Value, 0, 32)
		return uint32(n), err
	case *ast.CallExpr:
		if v.Fun.(*ast.Ident).Name != "uint32" || len(v.Args) != 1 {
			return 0, fmt.Errorf("unsupported call")
		}
		return evalUint32Expr(v.Args[0])
	}
	return 0, fmt.Errorf("unsupported expr")
}

// ── SCALE / misc ──────────────────────────────────────────

func scaleEncodePendingRuntimeUpgrade(ver uint32, codeHash, wasm []byte, desc string, activation uint64) []byte {
	out := make([]byte, 0, 4+1+len(codeHash)+1+len(wasm)+1+len(desc)+8)
	var buf [8]byte
	binary.LittleEndian.PutUint32(buf[:4], ver)
	out = append(out, buf[:4]...)
	out = append(out, encodeCompact(uint64(len(codeHash)))...)
	out = append(out, codeHash...)
	out = append(out, encodeCompact(uint64(len(wasm)))...)
	out = append(out, wasm...)
	out = append(out, encodeCompact(uint64(len(desc)))...)
	out = append(out, desc...)
	binary.LittleEndian.PutUint64(buf[:8], activation)
	out = append(out, buf[:8]...)
	return out
}

func encodeCompact(v uint64) []byte {
	switch {
	case v <= 0x3F:
		return []byte{byte(v) << 2}
	case v <= 0x3FFF:
		return []byte{byte(v)<<2 | 1, byte(v >> 6)}
	case v <= 0x3FFFFFFF:
		return []byte{byte(v)<<2 | 2, byte(v >> 6), byte(v >> 14), byte(v >> 22)}
	default:
		b := make([]byte, 4)
		binary.LittleEndian.PutUint32(b, uint32(v))
		return append([]byte{3}, b...)
	}
}

func uint64LE(v uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, v)
	return b
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func die(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "Error: "+format+"\n", args...)
	os.Exit(1)
}
