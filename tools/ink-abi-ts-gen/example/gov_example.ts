// gov 合约调用 + 返回值解析示例
//
// 前置依赖:
//   npm i @polkadot/api @polkadot/api-contract @polkadot/types-codec
//
// 运行:
//   npx ts-node gov_example.ts

import { ApiPromise, WsProvider } from '@polkadot/api';
import { Abi, ContractPromise } from '@polkadot/api-contract';
import govAbi from './gov.json';
import type {
  BalanceOfResponse,
  ProposalResponse,
  TotalSupplyResponse,
  Proposal,
} from './gov';
import { decodeMessage, toPlain } from '../decoder';

// ═══════════════════════════════════════════════════════════════
// 配置
// ═══════════════════════════════════════════════════════════════

const RPC_URL = 'wss://your-node.example.com';   // 替换为实际 RPC 地址
const GOV_CONTRACT = '0x1234...';                  // gov 合约地址

// ═══════════════════════════════════════════════════════════════
// 方式一：推荐 —— Abi + decodeMessage（零手写解码）
// ═══════════════════════════════════════════════════════════════

async function queryBalanceOf(api: ApiPromise, owner: string): Promise<bigint> {
  const abi = new Abi(govAbi, api.registry);

  // 构造合约调用参数
  const call = abi.findMessage('balance_of');

  // dryRun 调用（查询不消耗 gas）
  const { result, output } = await api.call.contractsApi.call(
    owner,
    GOV_CONTRACT,
    0,           // value
    null,        // gasLimit (null = estimate)
    null,        // storageDepositLimit
    abi.encodeMessage(call, [owner]),
  );

  if (result.isErr) {
    throw new Error(`DryRun failed: ${result.asErr}`);
  }

  // output 是 Option<Bytes>，unwrap 后得到原始 SCALE 字节
  const raw = output.unwrap().toU8a();

  // 使用 decodeMessage 自动解析 Result<Balance, LangError>
  const balance = decodeMessage(abi, 'balance_of', raw) as BalanceOfResponse;

  // BalanceOfResponse 在 gov.ts 中定义为 string（u128）
  return BigInt(balance);
}

async function queryProposal(api: ApiPromise, proposalId: number): Promise<Proposal> {
  const abi = new Abi(govAbi, api.registry);

  const { result, output } = await api.call.contractsApi.call(
    GOV_CONTRACT,
    GOV_CONTRACT,
    0,
    null,
    null,
    abi.encodeMessage(abi.findMessage('proposal'), [proposalId]),
  );

  if (result.isErr) throw new Error(`DryRun failed: ${result.asErr}`);

  const raw = output.unwrap().toU8a();

  // decodeMessage 自动解包 Result.Ok，返回 Proposal 结构
  const proposal = decodeMessage(abi, 'proposal', raw) as ProposalResponse;
  return proposal;
}

// ═══════════════════════════════════════════════════════════════
// 方式二：直接使用 @polkadot/api-contract ContractPromise
// ═══════════════════════════════════════════════════════════════

async function queryWithContractPromise(
  api: ApiPromise,
  caller: string,
): Promise<void> {
  const contract = new ContractPromise(api, govAbi, GOV_CONTRACT);

  // 调用 balance_of（自动解码）
  const { result, output } = await contract.query.balanceOf(caller, {
    gasLimit: -1,
  }, caller);

  if (!result.isOk) {
    throw new Error(`Contract reverted: ${result.asErr}`);
  }

  // output 是 Result<Option<u128>, DispatchError>
  // 取到内层值
  const inner = output?.asOk?.asOk; // Option<u128>
  if (inner?.isSome) {
    console.log('Balance:', inner.unwrap().toString());
  }

  // ── 调用 total_supply ──
  const tsResult = await contract.query.totalSupply(caller, { gasLimit: -1 });
  if (tsResult.result.isOk && tsResult.output?.asOk?.isSome) {
    console.log('Total Supply:', tsResult.output.asOk.unwrap().toString());
  }

  // ── 调用 proposal（返回复杂 struct）──
  const propResult = await contract.query.proposal(caller, { gasLimit: -1 }, 1);
  if (propResult.result.isOk) {
    // 手动使用 toPlain 转换
    const proposal = toPlain(propResult.output) as Proposal;
    console.log('Proposal:', JSON.stringify(proposal, null, 2));
  }
}

// ═══════════════════════════════════════════════════════════════
// 方式三：批量查询（list）
// ═══════════════════════════════════════════════════════════════

async function listProposals(
  api: ApiPromise,
  caller: string,
  startKey: number | null,
  size: number,
): Promise<Proposal[]> {
  const abi = new Abi(govAbi, api.registry);

  const { result, output } = await api.call.contractsApi.call(
    caller,
    GOV_CONTRACT,
    0,
    null,
    null,
    abi.encodeMessage(abi.findMessage('proposals'), [startKey, size]),
  );

  if (result.isErr) throw new Error(`DryRun failed: ${result.asErr}`);

  const raw = output.unwrap().toU8a();

  // proposals 返回 Result<Vec<Proposal>, LangError>
  // decodeMessage 自动解包 Result → Vec → 递归每个 Proposal
  const proposals = decodeMessage(abi, 'proposals', raw) as Proposal[];
  return proposals;
}

// ═══════════════════════════════════════════════════════════════
// 主函数
// ═══════════════════════════════════════════════════════════════

async function main() {
  const api = await ApiPromise.create({ provider: new WsProvider(RPC_URL) });
  const caller = '5GrwvaEF5zXb26Fz9rcQpDWS57CtERHpNehXCPcNoHGKutQY'; // Alice

  try {
    console.log('=== Gov 合约调用示例 ===\n');

    // 1. 查询余额
    const balance = await queryBalanceOf(api, caller);
    console.log('Balance:', balance.toString());

    // 2. 查询单个 proposal
    const proposal = await queryProposal(api, 1);
    console.log('Proposal:', JSON.stringify(proposal, null, 2));

    // 3. 批量查询 proposals
    const list = await listProposals(api, caller, null, 10);
    console.log('Proposals count:', list.length);
    console.log('First proposal name:', list[0]?.Name ?? 'N/A');

    // 4. 使用 ContractPromise（方式二）
    await queryWithContractPromise(api, caller);

  } finally {
    await api.disconnect();
  }
}

main().catch(console.error);
