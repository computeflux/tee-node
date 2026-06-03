# ink-abi-ts-gen

从 ink! ABI JSON 生成 TypeScript 类型定义的工具，配套 SCALE 解码器，实现 ink! 合约返回值的零手写类型安全解析。

## 目录结构

```
tools/ink-abi-ts-gen/
├── main.go          # Go 生成器：ABI JSON → TypeScript 类型
├── decoder.ts       # TypeScript 解码器：SCALE Codec → Plain JS Object
├── gen.sh           # 便捷脚本：编译 + 生成
├── gateway.ts       # gateway 合约类型（示例输出）
├── example/
│   ├── gateway.ts   # gateway 合约类型（示例输出）
│   ├── gov.ts       # gov 合约类型（示例输出）
│   └── gov_example.ts  # 调用示例
└── readme.md        # 本文档
```

## 快速开始

```bash
cd tools/ink-abi-ts-gen

# 生成 gov 合约类型
./gen.sh ../../tee-chain/abis/gov.json ./gov.ts

# 生成 gateway 合约类型
./gen.sh ../../tee-chain/abis/gateway.json ./gateway.ts
```

## 架构设计

### 1. 类型生成器（Go）

[`main.go`](main.go) 读取 ink! ABI v6 JSON，递归解析 `spec.messages` 和 `types`，输出 TypeScript 接口和类型别名。

#### 核心流程

```
ABI JSON
  ├── spec.messages[]
  │     └── 每个 message 的 returnType → 解析为 TS 类型表达式
  └── types[]
        └── 递归解析每种 type def，缓存结果避免循环引用
              → 输出 export interface / export type
```

#### 类型映射规则

| ink! ABI def | TypeScript 输出 | 说明 |
|---|---|---|
| `primitive` (u8~u64, i8~i64) | `number` | |
| `primitive` (u128, u256, i128, i256) | `string` | 防精度丢失 |
| `primitive` (bool) | `boolean` | |
| `primitive` (str) | `string` | |
| `composite` (struct) | `export interface Name { ... }` | |
| `sequence` (Vec<u8>) | `string` | ink! 字符串约定，UTF-8 解码 |
| `sequence` (Vec<T>) | `T[]` | T ≠ u8 |
| `array` ([u8; N]) | `string` | ink! 定长字符串 |
| `array` ([T; n]) | `T[]` | T ≠ u8 |
| `tuple` | `[T1, T2, ...]` | |
| `variant` Result<T, E> | 解包 Ok 分支作为返回值类型；如需完整类型则生成 `{ Ok: T } \| { Err: E }` | |
| `variant` Option<T> | `T \| null`（内联，不生成 named definition） | |
| `variant` 普通 enum | `{ VariantA: payload } \| { VariantB: null }` | |

> **Vec<u8> 和 [u8; N] 为什么生成 `string`？**
>
> ink! 合约中 `Vec<u8>` 和 `[u8; N]` 是字符串的标准表示方式。`decoder.ts` 在解码时会自动调用 `.toUtf8()` 将 `Vec<u8>` 转为 UTF-8 字符串。因此生成器将这类类型映射为 `string`，而非 `number[]`。
>
> 例如：
> ```typescript
> export interface ModelInfo {
>   Name: string;      // Vec<u8>  →  string
>   ModelID: string;   // Vec<u8>  →  string
>   APIKeys: string[]; // Vec<Vec<u8>> → string[]
> }
> ```

#### 特殊处理

- **`Vec<u8>` / `[u8; N]`**：在 `sequence` 和 `array` 分支中，通过 `isPrimitiveU8()` 检测元素类型，若为 `u8` 则直接输出 `string`，而非 `number[]`
- **`Result<T, E>`**：不依赖 variants 顺序（Ok/Err 可能互换位置），自动识别 Ok 和 Err 分支
- **`Option<T>`**：同理不依赖顺序，直接内联为 `T | null`，避免生成名为 `Option` 的接口污染全局命名空间
- **循环引用**：通过 `typeResult` map 缓存已解析的 type id，避免无限递归

### 2. SCALE 解码器（TypeScript）

[`decoder.ts`](decoder.ts) 提供零手写解码能力：给定 `Abi` 实例和原始 SCALE 字节，自动递归解码为 plain JavaScript 对象。

#### 核心 API

```typescript
import { decodeMessage, toPlain } from './decoder';
import type { ModelInfo } from './gateway';

// 一键解码 message 返回值（自动解包 Result<T, LangError>）
const model = decodeMessage<ModelInfo>(abi, 'get_model', rawBytes);

// 按类型名解码
const model2 = decodeType<ModelInfo>(abi, 'ModelInfo', rawBytes);

// 递归解码任意 Codec 实例
const plain = toPlain(codec);
```

#### 解码规则

| Codec 类型 | 解码结果 | 使用 API |
|---|---|---|
| `Option<T>` | `T \| null` | `.isSome` / `.unwrap()` |
| `Result<T, E>` | 自动解包 Ok；Err 抛异常 | `.isOk` / `.asOk` |
| `Vec<u8>` | UTF-8 字符串 | `.toUtf8()` |
| `Vec<T>` | `T[]` | `.map()` |
| `Tuple` | 数组 | `.map()` |
| `Struct` | 递归对象 | `Object.keys()` + 字段递归 |
| `u8`~`u64` / `i8`~`i64` | `number` | `.toNumber()` |
| `u128`/`u256`/`i128`/`i256` | `string` | `.toString()` |
| `Compact<T>` | `string` | `.toString()` |
| `H160`/`H256`/`H512` | hex 字符串 | `.toString()`（通过 `U8aFixed` 识别） |
| `Bool` | `boolean` | `.valueOf()` |
| `Text` / `Bytes` | `string` | `.toString()` |

#### 为什么不用 `toJSON()`

`@polkadot/types-codec` 的 `.toJSON()` 返回值格式可能随版本变化，不是稳定 API。本解码器使用以下稳定方法：

- `.toUtf8()`、`.toNumber()`、`.valueOf()`、`.toString()`
- `.unwrap()`、`.asOk`、`.isSome`、`.isOk`
- `.map()`（`Vec`/`Tuple` 遍历）

#### 类型安全设计

- **零 `any`**：全部使用 `unknown` + boolean 类型守卫 + 显式 cast
- **类型守卫返回 `boolean`**（而非 `is` 断言），避免 TypeScript 联合类型窄化叠加导致 `never`
- **泛型入口**：`decodeMessage<T>` 让调用方声明期望类型，编译期检查返回值类型

## 使用示例

### 生成类型

```bash
cd tools/ink-abi-ts-gen
./gen.sh ../../tee-chain/abis/gov.json ./gov.ts
```

### 解码合约返回值

```typescript
import { ApiPromise, WsProvider } from '@polkadot/api';
import { Abi } from '@polkadot/api-contract';
import govAbi from './gov.json';
import type { Proposal } from './gov';
import { decodeMessage } from './decoder';

const api = await ApiPromise.create({ provider: new WsProvider('wss://...') });
const abi = new Abi(govAbi, api.registry);

// dryRun 调用
const { result, output } = await api.call.contractsApi.call(
  caller, GOV_CONTRACT, 0, null, null,
  abi.encodeMessage(abi.findMessage('proposal'), [1]),
);

if (result.isErr) throw new Error('DryRun failed');

// 零手写解码：自动解包 Result → Proposal
const raw = output.unwrap().toU8a();
const proposal = decodeMessage<Proposal>(abi, 'proposal', raw);
console.log(proposal.Status.State);  // number
console.log(proposal.Call);           // CallContent | null
```

## 命令参考

```bash
# 便捷脚本（自动编译 + 生成）
./gen.sh <abi.json> [out.ts]

# 直接调用 Go 二进制
./ink-abi-ts-gen <abi.json> [out.ts]
```

## 依赖

- Go 1.21+（编译生成器）
- `@polkadot/api-contract`（运行时解码）
- `@polkadot/types-codec`（运行时解码）
