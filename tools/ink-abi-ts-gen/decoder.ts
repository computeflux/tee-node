// 通用 ink! Codec → Plain JS Object 解码器
// 无需为每种类型手写解码代码，递归自动处理所有类型

import { Abi } from '@polkadot/api-contract';
import {
  Vec, Struct, Enum, Option, Tuple, Compact, U8aFixed,
  u8, u16, u32, u64, U128, U256, i8, i16, i32, i64, I128, I256,
  Bool, Text, Bytes, Null,
} from '@polkadot/types-codec';

// ═══════════════════════════════════════════════════════════════
// 类型守卫（返回 boolean，避免 TypeScript 联合窄化导致 never）
// ═══════════════════════════════════════════════════════════════

function guardIsVec(codec: unknown): boolean {
  return codec instanceof Vec;
}

function guardIsStruct(codec: unknown): boolean {
  return codec instanceof Struct;
}

function guardIsEnum(codec: unknown): boolean {
  return codec instanceof Enum;
}

function guardIsOption(codec: unknown): boolean {
  return codec instanceof Option;
}

function guardIsTuple(codec: unknown): boolean {
  return codec instanceof Tuple;
}

function guardIsPrimitiveNumber(codec: unknown): boolean {
  return (
    codec instanceof u8 || codec instanceof u16 || codec instanceof u32 || codec instanceof u64 ||
    codec instanceof i8 || codec instanceof i16 || codec instanceof i32 || codec instanceof i64
  );
}

function guardIsPrimitiveBigInt(codec: unknown): boolean {
  return codec instanceof U128 || codec instanceof U256 ||
         codec instanceof I128 || codec instanceof I256;
}

function guardIsBool(codec: unknown): boolean {
  return codec instanceof Bool;
}

function guardIsText(codec: unknown): boolean {
  return codec instanceof Text;
}

function guardIsBytes(codec: unknown): boolean {
  return codec instanceof Bytes;
}

function guardIsCompact(codec: unknown): boolean {
  return codec instanceof Compact;
}

function guardIsHash(codec: unknown): boolean {
  if (!(codec instanceof U8aFixed)) return false;
  const raw = (codec as U8aFixed).toRawType();
  return raw === 'H160' || raw === 'H256' || raw === 'H512' || raw === '[u8;20]' || raw === '[u8;32]' || raw === '[u8;64]';
}

function guardIsNull(codec: unknown): boolean {
  return codec instanceof Null;
}

// Vec<u8> 判定：通过 toRawType() 获取规范类型字符串
function guardIsU8aString(codec: unknown): boolean {
  return guardIsVec(codec) && (codec as Vec<unknown>).toRawType() === 'Vec<u8>';
}

// ── 辅助函数 ─────────────────────────────────────────────────

function hasToString(val: unknown): val is { toString(): string } {
  return val !== null && typeof val === 'object' && typeof (val as Record<string, unknown>).toString === 'function';
}

function hasToJSON(val: unknown): val is { toJSON(): unknown } {
  return val !== null && typeof val === 'object' && typeof (val as Record<string, unknown>).toJSON === 'function';
}

// ═══════════════════════════════════════════════════════════════
// 核心递归解码器
// ═══════════════════════════════════════════════════════════════

/**
 * 将 @polkadot/types-codec 实例递归解码为 plain JavaScript 对象。
 *
 * 覆盖 ink! ABI 全部类型：
 *   - primitive (u8~u256, i8~i256, bool) → number / string / boolean
 *   - Vec<u8>  → UTF-8 字符串
 *   - Vec<T>   → T[]
 *   - array    → Vec<T>（polkadot 映射）→ T[]
 *   - composite (struct) → 递归对象
 *   - tuple    → 数组
 *   - variant  (Result/Option/enum)
 *       Result → 自动解包 Ok（Err 抛异常）
 *       Option → T | null
 *       普通 enum → { VariantName: payload }
 *   - Compact<T> → 字符串（防精度丢失）
 *   - H160/H256/H512 → hex 字符串
 *   - Text/Bytes → string
 */
export function toPlain(codec: unknown): unknown {
  if (codec == null) return null;

  // Option<T>
  if (guardIsOption(codec)) {
    const opt = codec as Option<unknown>;
    return opt.isSome ? toPlain(opt.unwrap()) : null;
  }

  // Result<T, E> —— 自动解包 Ok，Err 抛异常
  if (guardIsEnum(codec)) {
    const en = codec as Enum & { isOk?: boolean; isErr?: boolean; asOk?: unknown; asErr?: unknown };
    if (en.isOk !== undefined && en.isErr !== undefined) {
      if (en.isErr) {
        throw new Error(`Contract reverted: ${stringifyErr(en.asErr)}`);
      }
      return toPlain(en.asOk);
    }
    // 普通 Enum（非 Result）—— 返回 { VariantName: payload }
    const payload = en.value;
    return { [en.type]: payload == null ? null : toPlain(payload) };
  }

  // Vec<u8> → string（ink! 字符串约定）
  if (guardIsU8aString(codec)) {
    return (codec as Vec<unknown>).toUtf8();
  }

  // Vec<T> → T[]
  if (guardIsVec(codec)) {
    const vec = codec as Vec<unknown>;
    return vec.map((item: unknown) => toPlain(item));
  }

  // Tuple → 数组
  if (guardIsTuple(codec)) {
    const tuple = codec as Tuple;
    return tuple.map((item: unknown) => toPlain(item));
  }

  // Struct → 对象
  if (guardIsStruct(codec)) {
    const obj: Record<string, unknown> = {};
    const s = codec as Record<string, unknown>;
    for (const key of Object.keys(codec as object)) {
      const val = s[key];
      if (val === undefined) continue;
      obj[key] = toPlain(val);
    }
    return obj;
  }

  // 基础数字类型（小整数）
  if (guardIsPrimitiveNumber(codec)) {
    return (codec as u32).toNumber();
  }

  // 大整数（u128 / i128 / u256 / i256）→ 字符串防精度丢失
  if (guardIsPrimitiveBigInt(codec)) {
    return (codec as U128).toString();
  }

  // Compact<T>（压缩编码整数，如 Compact<u128>）
  if (guardIsCompact(codec)) {
    return (codec as Compact<unknown>).toString();
  }

  // Hash 类型 → hex 字符串
  if (guardIsHash(codec)) {
    return (codec as U8aFixed).toString();
  }

  // Bool
  if (guardIsBool(codec)) {
    return (codec as Bool).valueOf();
  }

  // Text / Bytes → string
  if (guardIsText(codec) || guardIsBytes(codec)) {
    return (codec as Text | Bytes).toString();
  }

  // Null
  if (guardIsNull(codec)) {
    return null;
  }

  // 兜底：如果有 toJSON 且无参数，优先用 toJSON()
  if (hasToJSON(codec)) {
    try {
      const json = codec.toJSON();
      if (json !== codec) return json;
    } catch {
      // ignore
    }
  }

  // 最终兜底
  if (hasToString(codec)) {
    return codec.toString();
  }
  return codec;
}

function stringifyErr(err: unknown): string {
  if (err == null) return 'unknown error';
  if (hasToString(err)) return err.toString();
  if (hasToJSON(err)) return JSON.stringify(err);
  return String(err);
}

// ═══════════════════════════════════════════════════════════════
// 便捷入口
// ═══════════════════════════════════════════════════════════════

/**
 * 使用 Abi 直接解码 message 返回值（自动处理 Result + 转 plain object）
 *
 * @param abi     通过 new Abi(abiJson) 创建的实例
 * @param label   message 名称，如 "get_model"
 * @param raw     链上返回的原始 SCALE 字节
 * @returns 解码后的 plain object
 *
 * @example
 *   const model = decodeMessage<ModelInfo>(abi, 'get_model', raw);
 */
export function decodeMessage<T = unknown>(abi: Abi, label: string, raw: Uint8Array): T {
  const msg = abi.findMessage(label);
  if (!msg) throw new Error(`Message "${label}" not found in ABI`);

  const codec = abi.registry.createType(msg.returnType!.toString(), raw);
  return toPlain(codec) as T;
}

/**
 * 按类型名解码（适用于已知类型 ID 或类型路径的场景）
 *
 * @param abi      Abi 实例
 * @param typeName 类型名，如 "ModelInfo" 或 "Vec<ModelInfo>"
 * @param raw      SCALE 字节
 * @returns 解码后的 plain object
 *
 * @example
 *   const model = decodeType<ModelInfo>(abi, 'ModelInfo', raw);
 */
export function decodeType<T = unknown>(abi: Abi, typeName: string, raw: Uint8Array): T {
  const codec = abi.registry.createType(typeName, raw);
  return toPlain(codec) as T;
}

// ═══════════════════════════════════════════════════════════════
// 使用示例
// ═══════════════════════════════════════════════════════════════

/*
import { Abi } from '@polkadot/api-contract';
import gatewayAbi from './gateway.json';
import type { ModelInfo, ProviderInfo } from './gateway';

const abi = new Abi(gatewayAbi);

// 1. 解码 get_model（Result<ModelInfo, LangError>）
const raw = new Uint8Array([...]);
const model = decodeMessage<ModelInfo>(abi, 'get_model', raw);
console.log(model.Name);      // "GPT-4o"
console.log(model.URI);       // "https://..."
console.log(model.APIKeys);   // ["sk-xxx", "sk-yyy"]
console.log(model.IsActive);  // true

// 2. 解码 list_models（Result<Vec<ModelInfo>, LangError>）
const list = decodeMessage<ModelInfo[]>(abi, 'list_models', raw);

// 3. 解码 get_provider
const provider = decodeMessage<ProviderInfo>(abi, 'get_provider', raw);

// 4. 按类型名直接解码（不通过 message）
const codec = abi.registry.createType('ModelInfo', raw);
const plain = toPlain(codec) as ModelInfo;
*/

// ═══════════════════════════════════════════════════════════════
// 为什么这样设计
// ═══════════════════════════════════════════════════════════════

/*
1. 零手写代码：不需要为 ModelInfo / ProviderInfo / APIKeyInfo 写任何解码逻辑，
   toPlain() 自动递归处理所有字段。

2. 零 any：全部使用 unknown + 类型守卫 + 泛型，TypeScript strict 模式也能通过。
   类型守卫返回 boolean（而非 is 断言），避免 TypeScript 联合类型窄化叠加导致 never。

3. 稳定：不依赖 toJSON()，而是使用 @polkadot/types-codec 提供的稳定 API：
   - .toUtf8() 转字符串
   - .toNumber() 转数字
   - .valueOf() 转 boolean
   - .unwrap() / .asOk 解包 Enum/Option
   - .map() 遍历 Vec

4. 类型安全：decodeMessage<T> 通过泛型参数让调用方声明期望类型：
   const model = decodeMessage<ModelInfo>(abi, 'get_model', raw);

5. 字符串自动识别：只要 ABI 里用的是 Vec<u8>，toPlain 就会自动调用 .toUtf8()，
   不需要手动判断。判断方式使用 toRawType() === 'Vec<u8>'，比 .name 更稳定。

6. 错误处理：Result.Err 会自动抛异常，不需要每次写 if (!result.isOk)。

7. 全覆盖：支持 ink! ABI 所有 def 类型
   primitive / composite / variant / sequence / array / tuple / compact
   以及 H160/H256/H512 等常用 substrate 类型（通过 U8aFixed + toRawType 识别）
*/
