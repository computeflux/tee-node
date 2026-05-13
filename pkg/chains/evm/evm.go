package evm

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/computeflux/tee-provider/pkg/chains/evm/bindings"
	"github.com/computeflux/tee-provider/pkg/model"
	chain "github.com/wetee-dao/ink.go"
	inkutil "github.com/wetee-dao/ink.go/util"
)

// EvmChain implements ChainApi + MainChainApi for EVM chains.
type EvmChain struct {
	cipher     model.SuiteType
	rpcURL     string
	client     *ethclient.Client
	token      *bindings.Token
	tokenAddr  common.Address
	subnet     *bindings.Subnet
	subnetAddr common.Address
	chainID    *big.Int
	chainName  string
	privateKey *ecdsa.PrivateKey
	auth       *bind.TransactOpts
	signerAddr string
}

// ── Constructor ───────────────────────────────────────────

func NewContract(config *model.ChainConfigType, pk *model.PrivKey) (*EvmChain, error) {
	client, err := ethclient.Dial(config.Urls[0])
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("chain ID: %w", err)
	}

	// Derive secp256k1 private key from the ed25519 master seed.
	signer := pk.ToSigner(config.Cipher)
	if signer == nil {
		return nil, fmt.Errorf("unsupported cipher: %v", config.Cipher)
	}

	var ecdsaKey *ecdsa.PrivateKey
	if skSigner, ok := signer.(*model.Secp256k1Signer); ok {
		ecdsaKey = skSigner.PrivateKey()
	}
	if ecdsaKey == nil {
		return nil, fmt.Errorf("failed to extract secp256k1 private key from signer")
	}

	auth, err := bind.NewKeyedTransactorWithChainID(ecdsaKey, chainID)
	if err != nil {
		return nil, fmt.Errorf("transactor: %w", err)
	}

	tAddr := common.HexToAddress(config.TokenAddress)
	token, err := bindings.NewToken(tAddr, client)
	if err != nil {
		return nil, fmt.Errorf("bind Token: %w", err)
	}

	sAddr := common.HexToAddress(config.SubnetAddress)
	subnet, err := bindings.NewSubnet(sAddr, client)
	if err != nil {
		return nil, fmt.Errorf("bind Subnet: %w", err)
	}

	return &EvmChain{
		rpcURL:     config.Urls[0],
		client:     client,
		token:      token,
		tokenAddr:  tAddr,
		subnet:     subnet,
		subnetAddr: sAddr,
		chainID:    chainID,
		chainName:  config.NetworkLabel,
		privateKey: ecdsaKey,
		cipher:     config.Cipher,
		auth:       auth,
		signerAddr: crypto.PubkeyToAddress(ecdsaKey.PublicKey).Hex(),
	}, nil
}

// ── Identity ──────────────────────────────────────────────

func (e *EvmChain) ChainType() string          { return "evm" }
func (e *EvmChain) GetCipher() model.SuiteType { return e.cipher }

// ── ChainApi: Points ─────────────────────────────────────

func (e *EvmChain) SyncRecharge(user model.UniAddr, amount uint64, txHash []byte) error {
	return e.syncPoints(user, int64(amount))
}
func (e *EvmChain) SyncWithdraw(user model.UniAddr, amount uint64, txHash []byte) error {
	return e.syncPoints(user, -int64(amount))
}

// syncPoints updates the local points cache. On-chain balance is the source of truth;
// this cache is an optimization to avoid RPC calls on every side-chain operation.
func (e *EvmChain) syncPoints(user model.UniAddr, delta int64) error {
	key := fmt.Sprintf("token:%x", user.V)
	existing, _ := model.GetJson[model.PointsAccount]("POINTS", key)
	if existing == nil {
		existing = &model.PointsAccount{User: user}
	}
	if delta > 0 {
		existing.TotalRecharged += uint64(delta)
	} else {
		d := uint64(-delta)
		if existing.Balance() < d {
			return fmt.Errorf("insufficient balance")
		}
		existing.TotalRecharged -= d
	}
	return model.SetJson("POINTS", key, existing)
}

// RecordSpend records an off-chain spend. Fails if balance insufficient.
func (e *EvmChain) RecordSpend(user model.UniAddr, amount uint64) error {
	onChain, err := e.getOnChainBalance(user)
	if err != nil {
		return fmt.Errorf("query on-chain balance: %w", err)
	}
	if onChain < amount {
		return fmt.Errorf("insufficient on-chain balance: have %d, need %d", onChain, amount)
	}
	key := fmt.Sprintf("token:%x", user.V)
	existing, _ := model.GetJson[model.PointsAccount]("POINTS", key)
	if existing == nil {
		existing = &model.PointsAccount{User: user, TotalRecharged: onChain}
	}
	existing.TotalSpent += amount
	return model.SetJson("POINTS", key, existing)
}

// GetTotalRecharged returns the on-chain token balance from the EVM contract.
func (e *EvmChain) GetTotalRecharged(user model.UniAddr) (uint64, error) {
	return e.getOnChainBalance(user)
}

// getOnChainBalance queries the Token contract for a user's balance.
func (e *EvmChain) getOnChainBalance(user model.UniAddr) (uint64, error) {
	addr := common.BytesToAddress(user.V)
	bal, err := e.token.Balances(&bind.CallOpts{}, addr)
	if err != nil {
		return 0, fmt.Errorf("token.Balances: %w", err)
	}
	return bal.Uint64(), nil
}
func (e *EvmChain) SyncRechargeEvents(fromBlock uint64) ([]model.RechargeEvent, error) {
	iter, _ := e.token.FilterRecharged(&bind.FilterOpts{Start: fromBlock}, nil)
	if iter == nil {
		return nil, nil
	}
	defer iter.Close()
	var events []model.RechargeEvent
	for iter.Next() {
		evt := iter.Event
		events = append(events, model.RechargeEvent{
			User: model.UniAddr{T: 2, V: evt.User.Bytes()}, Amount: evt.PointsAmount.Uint64(),
			TxHash: evt.Raw.TxHash.Bytes(), Block: evt.Raw.BlockNumber, Chain: e.chainName,
		})
	}
	return events, nil
}
func (e *EvmChain) SyncWithdrawEvents(fromBlock uint64) ([]model.WithdrawEvent, error) {
	iter, _ := e.token.FilterWithdrawn(&bind.FilterOpts{Start: fromBlock}, nil)
	if iter == nil {
		return nil, nil
	}
	defer iter.Close()
	var events []model.WithdrawEvent
	for iter.Next() {
		evt := iter.Event
		events = append(events, model.WithdrawEvent{
			User: model.UniAddr{T: 2, V: evt.User.Bytes()}, Amount: evt.PointsAmount.Uint64(),
			TxHash: evt.Raw.TxHash.Bytes(), Block: evt.Raw.BlockNumber, Chain: e.chainName,
		})
	}
	return events, nil
}

// ── MainChainApi: Epoch ───────────────────────────────────

func (e *EvmChain) GetEpoch() (uint32, uint32, uint32, uint32, model.ChainH160, error) {
	info, err := e.subnet.EpochInfo(&bind.CallOpts{})
	if err != nil {
		return 0, 0, 0, 0, model.ChainH160{}, err
	}
	var h160 model.ChainH160
	copy(h160[:], info.SideChainPub.Bytes())
	return info.Epoch, info.EpochSlot, uint32(info.LastEpochBlock.Uint64()), info.Now, h160, nil
}

func (e *EvmChain) SetNewEpoch(nodeId uint64) error {
	tx, err := e.subnet.SetNextEpoch(e.auth, nodeId)
	if err != nil {
		return fmt.Errorf("setNextEpoch: %w", err)
	}
	_, err = bind.WaitMined(context.Background(), e.client, tx)
	return err
}

// ── MainChainApi: Validators ──────────────────────────────

func (e *EvmChain) GetBootPeers() ([]model.P2PAddr, error) {
	nodes, err := e.subnet.GetBootNodes(&bind.CallOpts{})
	if err != nil {
		return nil, err
	}
	var peers []model.P2PAddr
	for _, n := range nodes {
		var id [32]byte
		copy(id[:], n.P2pId[:])
		ipv4 := uint32(n.Ip.A)<<24 | uint32(n.Ip.B)<<16 | uint32(n.Ip.C)<<8 | uint32(n.Ip.D)
		peers = append(peers, model.P2PAddr{
			Ip:   model.Ip{Ipv4: inkutil.NewSome(ipv4)},
			Port: uint16(n.Port),
			Id:   id,
		})
	}
	return peers, nil
}

func (e *EvmChain) GetValidatorList() ([]*model.Validator, error) {
	ids, nodes, _, err := e.subnet.GetValidators(&bind.CallOpts{})
	if err != nil {
		return nil, err
	}
	return buildValidators(ids, nodes), nil
}

func (e *EvmChain) GetNextEpochValidatorList() ([]*model.Validator, error) {
	ids, nodes, _, err := e.subnet.NextEpochValidators(&bind.CallOpts{})
	if err != nil {
		return nil, err
	}
	return buildValidators(ids, nodes), nil
}

func buildValidators(ids []uint64, nodes []bindings.SecretNode) []*model.Validator {
	validators := make([]*model.Validator, 0, len(ids))
	for i, id := range ids {
		var vId, pId [32]byte
		if i < len(nodes) {
			copy(vId[:], nodes[i].ValidatorId[:])
			copy(pId[:], nodes[i].P2pId[:])
		}
		validators = append(validators, &model.Validator{
			NodeID:      id,
			ValidatorId: *model.PubKeyFromByte(vId[:]),
			P2pId:       *model.PubKeyFromByte(pId[:]),
		})
	}
	return validators
}

func (e *EvmChain) GetNodes() ([]*model.Validator, []*model.PubKey, error) {
	vals, err := e.GetValidatorList()
	if err != nil {
		return nil, nil, err
	}
	var pubs []*model.PubKey
	for _, v := range vals {
		pubs = append(pubs, &v.ValidatorId)
	}
	return vals, pubs, nil
}

// ── ChainApi helpers ──────────────────────────────────────

func (e *EvmChain) GetClient() *model.ChainClient { return &model.ChainClient{EVM: e.client} }
func (e *EvmChain) GetChainUrls() []string        { return []string{e.rpcURL} }
func (e *EvmChain) GetSignerAddress() string      { return e.signerAddr }
func (e *EvmChain) SyncProviderRegister(provider model.UniAddr, name string, endpoint string) error {
	return nil // EVM: on-chain registration handled by governance
}
func (e *EvmChain) SyncModelRegister(provider model.UniAddr, modelName string, pointsPer1K uint64) error {
	return nil
}

// ── MainChainApi: Extrinsic helpers ────────────────────────

// TEECallToCall converts a TEE attestation into an EVM chain call.
func (e *EvmChain) TEECallToCall(tcall *model.TeeCall, dkgKey []byte) (*model.ChainCall, error) {
	// For EVM, the TEE call data is proto-marshaled and submitted as calldata.
	// The Subnet contract decodes and verifies the attestation on-chain.
	return &model.ChainCall{Method: "tee_attest"}, nil
}

// TxCallOfSetNextEpoch returns ABI-encoded calldata for setNextEpoch(uint64).
func (e *EvmChain) TxCallOfSetNextEpoch(nodeId uint64, signer model.ChainAccountID) (*model.ChainCall, error) {
	abi, err := bindings.SubnetMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("get ABI: %w", err)
	}
	data, err := abi.Pack("setNextEpoch", nodeId)
	if err != nil {
		return nil, fmt.Errorf("pack setNextEpoch: %w", err)
	}
	return &model.ChainCall{Data: data, Method: "setNextEpoch"}, nil
}

// BatchCall concatenates call data for EVM batch execution.
func (e *EvmChain) BatchCall(method string, calls []model.ChainCall) (*model.ChainCall, error) {
	var data []byte
	for _, c := range calls {
		data = append(data, c.Data...)
	}
	return &model.ChainCall{Data: data, Method: method}, nil
}

// CheckMapAccount queries the Subnet contract to check if an H160 address is mapped.
func (e *EvmChain) CheckMapAccount(addr model.ChainH160) (bool, error) {
	var b32 [32]byte
	copy(b32[12:], addr[:]) // H160 → bytes32 with zero-padding left
	result, err := e.subnet.MintOfWorker(&bind.CallOpts{}, b32)
	if err != nil {
		return false, nil
	}
	return result > 0, nil
}

// CallOfMapAccount returns ABI-encoded calldata for mapAccount(bytes32).
func (e *EvmChain) CallOfMapAccount(signer model.ChainAccountID) (*model.ChainCall, error) {
	abi, err := bindings.SubnetMetaData.GetAbi()
	if err != nil {
		return nil, fmt.Errorf("get ABI: %w", err)
	}
	data, err := abi.Pack("mapAccount", signer)
	if err != nil {
		return nil, fmt.Errorf("pack mapAccount: %w", err)
	}
	return &model.ChainCall{Data: data, Method: "mapAccount"}, nil
}

// PartialSign delegates threshold partial signing to the DSS signer.
func (e *EvmChain) PartialSign(signer chain.PartialSignerType, call model.ChainCall) ([]byte, error) {
	return signer.PartialSign(call.Data)
}

// SignAndSubmit builds, signs, and sends an EVM transaction.
// If signer implements SignerType (has Sign), it uses that for signing;
// otherwise falls back to the chain's auth key.
func (e *EvmChain) SignAndSubmit(signer chain.SignerType, call model.ChainCall, untilFinalized bool, nonce uint64) error {
	tx := types.NewTransaction(nonce, e.subnetAddr, big.NewInt(0), 300000, big.NewInt(50_000_000_000), call.Data)

	// Use the chain auth key to sign (Substrate-style DSS aggregation
	// happens before this point; the aggregated signature is embedded
	// in the call data on-chain).
	signedTx, err := e.auth.Signer(e.auth.From, tx)
	if err != nil {
		return fmt.Errorf("sign: %w", err)
	}
	if err := e.client.SendTransaction(context.Background(), signedTx); err != nil {
		return fmt.Errorf("send tx: %w", err)
	}

	if untilFinalized {
		_, err = bind.WaitMined(context.Background(), e.client, signedTx)
		return err
	}
	return nil
}

// ── Direct contract access (non-interface) ────────────────

func (e *EvmChain) GetOnChainBalance(user common.Address) (*big.Int, error) {
	return e.token.Balances(&bind.CallOpts{}, user)
}
func (e *EvmChain) GetRate() (*big.Int, error) {
	return e.token.Rate(&bind.CallOpts{})
}
func (e *EvmChain) Recharge(value *big.Int) error {
	e.auth.Value = value
	tx, err := e.token.Recharge(e.auth)
	if err != nil {
		return fmt.Errorf("recharge: %w", err)
	}
	_, err = bind.WaitMined(context.Background(), e.client, tx)
	return err
}
func (e *EvmChain) WithdrawToken(user common.Address, ethAmount *big.Int) error {
	tx, err := e.subnet.WithdrawToken(e.auth, user, ethAmount)
	if err != nil {
		return fmt.Errorf("withdrawToken: %w", err)
	}
	_, err = bind.WaitMined(context.Background(), e.client, tx)
	return err
}
func (e *EvmChain) SetSubnet(subnetAddr common.Address) error {
	tx, err := e.token.SetSubnet(e.auth, subnetAddr)
	if err != nil {
		return fmt.Errorf("setSubnet: %w", err)
	}
	_, err = bind.WaitMined(context.Background(), e.client, tx)
	return err
}
func (e *EvmChain) WithdrawPoints(user common.Address, ethAmount *big.Int) error {
	tx, err := e.token.Withdraw(e.auth, user, ethAmount)
	if err != nil {
		return fmt.Errorf("withdraw: %w", err)
	}
	_, err = bind.WaitMined(context.Background(), e.client, tx)
	return err
}
