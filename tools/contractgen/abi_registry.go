package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// inkRegistry 在内存中构建 ink metadata v6 的 types 数组，按需注册类型。
type inkRegistry struct {
	types         []map[string]any
	resultQLang   map[int]int           // inner type id -> Result<T, LangError> type id
	goTypeToID    map[string]int        // Go 类型名 -> type id 映射
	langErrorID   int                   // LangError 类型的 ID
	errorID       int                   // 合约 Error 类型的 ID
	addressID     int                   // Address 类型的 ID
	u256ID        int                   // U256 类型的 ID
	messageResID  int                   // MessageResult 类型的 ID
	balanceID     int                   // Balance 类型的 ID
	blockNumberID int                   // BlockNumber 类型的 ID
	hashID        int                   // Hash 类型的 ID
	errVariants   []errorVariant        // 合约错误变体
	structTypes   map[string]structType // 结构体类型定义
}

// newInkRegistry 创建 registry 并初始化基础类型
func newInkRegistry(errVariants []errorVariant, structTypes map[string]structType) *inkRegistry {
	r := &inkRegistry{
		resultQLang: make(map[int]int),
		goTypeToID:  make(map[string]int),
		errVariants: errVariants,
		structTypes: structTypes,
	}

	// 按固定顺序注册基础类型（与 ink metadata v6 兼容）
	// ID 0-6: 原语类型
	primitiveOrder := []string{"u8", "u16", "u32", "u64", "i64", "u128", "bool"}
	for _, name := range primitiveOrder {
		id := r.appendType(typeDefs[name])
		r.goTypeToID[name] = id
	}

	// ID 7: U256（用于 model.Amount）- 包含 [u64; 4] 字段
	r.u256ID = r.appendType(r.buildU256Type())
	r.goTypeToID["U256"] = r.u256ID
	r.goTypeToID["model.Amount"] = r.u256ID

	// ID 8: LangError（ink 框架错误）
	r.langErrorID = r.appendType(typeDefs["LangError"])

	// ID 9: Error（合约错误）- 动态生成
	r.errorID = r.appendType(r.buildErrorType())

	// ID 10: MessageResult（mutation 返回类型）
	r.messageResID = r.appendType(r.buildMessageResultType())

	// ID 11: Address（model.UniAddr）
	r.addressID = r.appendType(r.buildUniAddrType())
	r.goTypeToID["model.UniAddr"] = r.addressID

	// 注册更多类型将按需动态添加
	return r
}

// ensureArrayU64x4 确保 [u64; 4] 类型存在
func (r *inkRegistry) ensureArrayU64x4() int {
	key := "[u64; 4]"
	if id, ok := r.goTypeToID[key]; ok {
		return id
	}
	u64ID := r.goTypeToID["u64"]
	id := r.appendType(map[string]any{
		"def": map[string]any{
			"array": map[string]any{
				"len":  4,
				"type": u64ID,
			},
		},
	})
	r.goTypeToID[key] = id
	return id
}

// buildU256Type 构建 U256 类型定义
// U256 包含一个 [u64; 4] 类型的字段
func (r *inkRegistry) buildU256Type() map[string]any {
	arrayID := r.ensureArrayU64x4()
	return map[string]any{
		"def": map[string]any{
			"composite": map[string]any{
				"fields": []any{
					map[string]any{
						"type":     arrayID,
						"typeName": "[u64; 4]",
					},
				},
			},
		},
		"path": []any{"U256"},
	}
}

// buildUniAddrType 构建 UniAddr 类型定义
// UniAddr { T uint32, V []byte }
func (r *inkRegistry) buildUniAddrType() map[string]any {
	u32ID := r.goTypeToID["u32"]
	bytesID := r.ensureBytes()
	return map[string]any{
		"def": map[string]any{
			"composite": map[string]any{
				"fields": []any{
					map[string]any{"name": "T", "type": u32ID},
					map[string]any{"name": "V", "type": bytesID},
				},
			},
		},
		"path": []any{"UniAddr"},
	}
}

// buildErrorType 构建 Error 类型定义
func (r *inkRegistry) buildErrorType() map[string]any {
	variants := make([]any, len(r.errVariants))
	for i, v := range r.errVariants {
		variants[i] = map[string]any{
			"fields": []any{},
			"index":  v.index,
			"name":   v.name,
		}
	}
	return map[string]any{
		"def": map[string]any{
			"variant": map[string]any{
				"variants": variants,
			},
		},
		"path": []any{"Error"},
	}
}

// buildMessageResultType 构建 MessageResult 类型定义
func (r *inkRegistry) buildMessageResultType() map[string]any {
	return map[string]any{
		"def": map[string]any{
			"variant": map[string]any{
				"variants": []any{
					map[string]any{"index": 0, "name": "Ok"},
					map[string]any{
						"fields": []any{map[string]any{"type": r.errorID}},
						"index":  1,
						"name":   "Err",
					},
				},
			},
		},
		"params": []any{
			map[string]any{"name": "T"},
			map[string]any{"name": "E", "type": r.errorID},
		},
		"path": []any{"ink", "MessageResult"},
	}
}

// buildAddressType 构建 Address 类型定义
func (r *inkRegistry) buildAddressType() map[string]any {
	return map[string]any{
		"def": map[string]any{
			"composite": map[string]any{
				"fields": []any{
					map[string]any{"name": "T", "type": r.goTypeToID["u8"]},
					map[string]any{"name": "V", "type": r.ensureBytes()},
				},
			},
		},
		"path": []any{"AccountId"},
	}
}

// ensureBytes 确保 Bytes 类型存在
func (r *inkRegistry) ensureBytes() int {
	if id, ok := r.goTypeToID["Bytes"]; ok {
		return id
	}
	id := r.appendType(map[string]any{
		"def": map[string]any{
			"sequence": map[string]any{
				"type": r.goTypeToID["u8"],
			},
		},
		"path": []any{"Bytes"},
	})
	r.goTypeToID["Bytes"] = id
	return id
}

// typeDef 类型定义
var typeDefs = map[string]map[string]any{
	// 原语类型
	"u8":   mustJSONType(`{"def":{"primitive":"u8"}}`),
	"u16":  mustJSONType(`{"def":{"primitive":"u16"}}`),
	"u32":  mustJSONType(`{"def":{"primitive":"u32"}}`),
	"u64":  mustJSONType(`{"def":{"primitive":"u64"}}`),
	"i64":  mustJSONType(`{"def":{"primitive":"i64"}}`),
	"u128": mustJSONType(`{"def":{"primitive":"u128"}}`),
	"bool": mustJSONType(`{"def":{"primitive":"bool"}}`),
	// 复合类型
	// U256 动态生成，包含 [u64; 4] 字段
	"LangError": mustJSONType(`{"def":{"variant":{"variants":[{"index":1,"name":"CouldNotReadInput"}]}},"path":["ink_primitives","LangError"]}`),
}

func mustJSONType(s string) map[string]any {
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		panic(err)
	}
	return m
}

func (r *inkRegistry) appendType(typ map[string]any) int {
	id := len(r.types)
	r.types = append(r.types, map[string]any{"id": id, "type": typ})
	return id
}

func (r *inkRegistry) resultVariantLang(okType, errType int) map[string]any {
	return map[string]any{
		"def": map[string]any{
			"variant": map[string]any{
				"variants": []any{
					map[string]any{"fields": []any{map[string]any{"type": okType}}, "index": 0, "name": "Ok"},
					map[string]any{"fields": []any{map[string]any{"type": errType}}, "index": 1, "name": "Err"},
				},
			},
		},
		"params": []any{
			map[string]any{"name": "T", "type": okType},
			map[string]any{"name": "E", "type": errType},
		},
		"path": []any{"Result"},
	}
}

// ensureResultQueryLang 返回 Result<T, ink::LangError> 的 type id（T 为 SCALE 返回值类型）。
func (r *inkRegistry) ensureResultQueryLang(inner int) int {
	if id, ok := r.resultQLang[inner]; ok {
		return id
	}
	id := r.appendType(r.resultVariantLang(inner, r.langErrorID))
	r.resultQLang[inner] = id
	return id
}

func (r *inkRegistry) ensureInnerQueryReturn(goType string) (int, error) {
	goType = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(goType), "*"))

	// 检查缓存
	if id, ok := r.goTypeToID[goType]; ok {
		return id, nil
	}

	kind, inner := parseGoTypeRoot(goType)
	switch kind {
	case "slice":
		inner = strings.TrimSpace(inner)
		innerID, err := r.ensureInnerQueryReturn(inner)
		if err != nil {
			return 0, err
		}
		return r.ensureVec(innerID), nil
	case "array":
		// inner 格式为 "Type|N"
		parts := strings.Split(inner, "|")
		if len(parts) != 2 {
			return 0, fmt.Errorf("invalid array type format: %s", inner)
		}
		elemType := strings.TrimSpace(parts[0])
		lenStr := strings.TrimSpace(parts[1])
		len, err := strconv.Atoi(lenStr)
		if err != nil {
			return 0, fmt.Errorf("invalid array length: %s", lenStr)
		}
		elemID, err := r.ensureInnerQueryReturn(elemType)
		if err != nil {
			return 0, err
		}
		return r.ensureArray(elemID, len), nil
	case "option":
		inner = strings.TrimSpace(inner)
		innerID, err := r.ensureInnerQueryReturn(inner)
		if err != nil {
			return 0, err
		}
		return r.ensureOption(innerID), nil
	case "ptr":
		inner = strings.TrimSpace(inner)
		innerID, err := r.ensureInnerQueryReturn(inner)
		if err != nil {
			return 0, err
		}
		return r.ensureOption(innerID), nil
	case "named":
		switch goType {
		case "bool":
			return r.goTypeToID["bool"], nil
		case "byte", "uint8":
			return r.goTypeToID["u8"], nil
		case "int64", "int":
			return r.goTypeToID["i64"], nil
		case "uint32":
			return r.goTypeToID["u32"], nil
		case "uint64":
			return r.goTypeToID["u64"], nil
		case "model.Amount":
			return r.u256ID, nil
		case "model.UniAddr":
			return r.addressID, nil
		}
		// 尝试动态注册未知的命名类型
		return r.ensureNamedType(goType)
	}
	return 0, fmt.Errorf("unknown query return type %q", goType)
}

// ensureVec 确保 Vec<T> 类型存在
func (r *inkRegistry) ensureVec(innerID int) int {
	key := fmt.Sprintf("Vec<%d>", innerID)
	if id, ok := r.goTypeToID[key]; ok {
		return id
	}
	id := r.appendType(map[string]any{
		"def": map[string]any{
			"sequence": map[string]any{
				"type": innerID,
			},
		},
		"params": []any{
			map[string]any{"name": "T", "type": innerID},
		},
		"path": []any{"Vec"},
	})
	r.goTypeToID[key] = id
	return id
}

// ensureArray 确保数组类型 [N; T] 存在
func (r *inkRegistry) ensureArray(elemID int, len int) int {
	key := fmt.Sprintf("Array<%d,%d>", len, elemID)
	if id, ok := r.goTypeToID[key]; ok {
		return id
	}
	id := r.appendType(map[string]any{
		"def": map[string]any{
			"array": map[string]any{
				"len":  len,
				"type": elemID,
			},
		},
	})
	r.goTypeToID[key] = id
	return id
}

// ensureOption 确保 Option<T> 类型存在
func (r *inkRegistry) ensureOption(innerID int) int {
	key := fmt.Sprintf("Option<%d>", innerID)
	if id, ok := r.goTypeToID[key]; ok {
		return id
	}
	id := r.appendType(map[string]any{
		"def": map[string]any{
			"variant": map[string]any{
				"variants": []any{
					map[string]any{"index": 0, "name": "None"},
					map[string]any{
						"fields": []any{map[string]any{"type": innerID}},
						"index":  1,
						"name":   "Some",
					},
				},
			},
		},
		"params": []any{
			map[string]any{"name": "T", "type": innerID},
		},
		"path": []any{"Option"},
	})
	r.goTypeToID[key] = id
	return id
}

// ensureNamedType 确保命名类型存在（动态注册）
func (r *inkRegistry) ensureNamedType(goType string) (int, error) {
	key := goType
	if id, ok := r.goTypeToID[key]; ok {
		return id, nil
	}

	// 特殊处理 string 类型 -> sequence<u8>
	if goType == "string" {
		return r.ensureStringType(), nil
	}

	// 检查是否有结构体定义
	if st, ok := r.structTypes[goType]; ok {
		// 有结构体定义，生成包含字段的 composite 类型
		return r.ensureStructType(st)
	}

	// 对于未知的命名类型，创建一个简单的 composite 类型
	id := r.appendType(map[string]any{
		"def": map[string]any{
			"composite": map[string]any{},
		},
		"path": []any{goType},
	})
	r.goTypeToID[key] = id
	return id, nil
}

// ensureStringType 确保 string 类型存在（sequence<u8>）
func (r *inkRegistry) ensureStringType() int {
	key := "string"
	if id, ok := r.goTypeToID[key]; ok {
		return id
	}
	u8ID := r.goTypeToID["u8"]
	id := r.appendType(map[string]any{
		"def": map[string]any{
			"sequence": map[string]any{
				"type": u8ID,
			},
		},
		"path": []any{"string"},
	})
	r.goTypeToID[key] = id
	return id
}

// ensureStructType 根据结构体定义生成 composite 类型
func (r *inkRegistry) ensureStructType(st structType) (int, error) {
	key := st.name
	if id, ok := r.goTypeToID[key]; ok {
		return id, nil
	}

	// 检查是否是类型别名（只有一个 _alias 字段）
	if len(st.fields) == 1 && st.fields[0].name == "_alias" {
		return r.ensureTypeAlias(st.name, st.fields[0].typ)
	}

	// 先注册类型占位，避免递归循环
	placeholderID := r.appendType(map[string]any{
		"def": map[string]any{
			"composite": map[string]any{},
		},
		"path": []any{st.name},
	})
	r.goTypeToID[key] = placeholderID

	// 解析字段类型并生成字段定义
	var fields []any
	for _, f := range st.fields {
		fieldTypeID, err := r.ensureInnerQueryReturn(f.typ)
		if err != nil {
			// 如果字段类型解析失败，使用占位符
			fieldTypeID = r.goTypeToID["u8"] // 默认使用 u8
		}
		fieldDef := map[string]any{
			"name": f.name,
			"type": fieldTypeID,
		}
		fields = append(fields, fieldDef)
	}

	// 更新类型定义
	r.types[placeholderID] = map[string]any{
		"id": placeholderID,
		"type": map[string]any{
			"def": map[string]any{
				"composite": map[string]any{
					"fields": fields,
				},
			},
			"path": []any{st.name},
		},
	}

	return placeholderID, nil
}

// ensureTypeAlias 为类型别名生成 ABI 类型
// string 类型的别名生成 sequence<u8> 类型
func (r *inkRegistry) ensureTypeAlias(name string, underlyingType string) (int, error) {
	key := name
	if id, ok := r.goTypeToID[key]; ok {
		return id, nil
	}

	var typeDef map[string]any
	switch underlyingType {
	case "string":
		// string 类型生成 sequence<u8>（即 Vec<u8>）
		u8ID := r.goTypeToID["u8"]
		typeDef = map[string]any{
			"def": map[string]any{
				"sequence": map[string]any{
					"type": u8ID,
				},
			},
			"path": []any{name},
		}
	default:
		// 其他类型别名生成空的 composite
		typeDef = map[string]any{
			"def": map[string]any{
				"composite": map[string]any{},
			},
			"path": []any{name},
		}
	}

	id := r.appendType(typeDef)
	r.goTypeToID[key] = id
	return id, nil
}

func parseGoTypeRoot(s string) (kind, inner string) {
	s = strings.TrimSpace(s)
	// 数组类型 [N]Type
	if len(s) > 2 && s[0] == '[' && s[1] != ']' {
		// 找到 ]
		for i := 1; i < len(s); i++ {
			if s[i] == ']' {
				return "array", s[i+1:] + "|" + s[1:i]
			}
		}
	}
	if strings.HasPrefix(s, "[]") {
		return "slice", strings.TrimPrefix(s, "[]")
	}
	if strings.HasPrefix(s, "util.Option[") && strings.HasSuffix(s, "]") {
		return "option", s[len("util.Option[") : len(s)-1]
	}
	if strings.HasPrefix(s, "*") {
		return "ptr", strings.TrimPrefix(s, "*")
	}
	return "named", s
}

func (r *inkRegistry) ensureResultQuery(goType string) (map[string]any, error) {
	inner, err := r.ensureInnerQueryReturn(goType)
	if err != nil {
		return nil, err
	}
	rid := r.ensureResultQueryLang(inner)
	return map[string]any{"displayName": []any{"Result"}, "type": rid}, nil
}

func (r *inkRegistry) goTypeToABIRefParam(goType string) (abiTypeRef, error) {
	goType = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(goType), "*"))
	if goType == "[]byte" {
		return abiTypeRef{Display: []string{"UniAddr"}, TypeID: r.addressID}, nil
	}
	return r.goTypeToABIRefNamed(goType)
}

func (r *inkRegistry) goTypeToABIRefNamed(goType string) (abiTypeRef, error) {
	// 优先处理已知的 model 类型（避免被动态注册）
	switch goType {
	case "bool":
		return abiTypeRef{Display: []string{"bool"}, TypeID: r.goTypeToID["bool"]}, nil
	case "byte", "uint8":
		return abiTypeRef{Display: []string{"u8"}, TypeID: r.goTypeToID["u8"]}, nil
	case "int64", "int":
		return abiTypeRef{Display: []string{"i64"}, TypeID: r.goTypeToID["i64"]}, nil
	case "uint32":
		return abiTypeRef{Display: []string{"u32"}, TypeID: r.goTypeToID["u32"]}, nil
	case "uint64":
		return abiTypeRef{Display: []string{"u64"}, TypeID: r.goTypeToID["u64"]}, nil
	case "model.Amount":
		return abiTypeRef{Display: []string{"U256"}, TypeID: r.u256ID}, nil
	case "model.UniAddr":
		return abiTypeRef{Display: []string{"UniAddr"}, TypeID: r.addressID}, nil
	}

	// 处理 util.Option[T] 类型
	if strings.HasPrefix(goType, "util.Option[") && strings.HasSuffix(goType, "]") {
		innerType := goType[len("util.Option[") : len(goType)-1]
		innerID, err := r.ensureInnerQueryReturn(innerType)
		if err != nil {
			return abiTypeRef{}, err
		}
		optionID := r.ensureOption(innerID)
		return abiTypeRef{Display: []string{"Option"}, TypeID: optionID}, nil
	}

	// 处理 slice 类型 []T
	if strings.HasPrefix(goType, "[]") && len(goType) > 2 {
		innerType := strings.TrimPrefix(goType, "[]")
		innerID, err := r.ensureInnerQueryReturn(innerType)
		if err != nil {
			return abiTypeRef{}, err
		}
		vecID := r.ensureVec(innerID)
		return abiTypeRef{Display: []string{"Vec"}, TypeID: vecID}, nil
	}

	// 检查缓存
	if id, ok := r.goTypeToID[goType]; ok {
		// 使用简短的 displayName
		displayName := goType
		switch goType {
		case "model.Amount":
			displayName = "U256"
		case "model.UniAddr":
			displayName = "UniAddr"
		}
		return abiTypeRef{Display: []string{displayName}, TypeID: id}, nil
	}

	// 动态注册未知类型
	id, err := r.ensureNamedType(goType)
	if err != nil {
		return abiTypeRef{}, err
	}
	return abiTypeRef{Display: []string{goType}, TypeID: id}, nil
}

func inkEnvironment(r *inkRegistry) map[string]any {
	// 确保 Balance、BlockNumber、Hash 类型存在
	balanceID := r.ensureBalanceType()
	blockNumberID := r.goTypeToID["u32"] // BlockNumber 使用 u32
	hashID := r.ensureHashType()

	return map[string]any{
		"accountId": map[string]any{
			"displayName": []any{"AccountId"},
			"type":        r.addressID,
		},
		"balance": map[string]any{
			"displayName": []any{"Balance"},
			"type":        balanceID,
		},
		"blockNumber": map[string]any{
			"displayName": []any{"BlockNumber"},
			"type":        blockNumberID,
		},
		"hash": map[string]any{
			"displayName": []any{"Hash"},
			"type":        hashID,
		},
		"nativeToEthRatio": 100000000,
		"staticBufferSize": 16384,
		"timestamp": map[string]any{
			"displayName": []any{"Timestamp"},
			"type":        r.goTypeToID["u64"],
		},
	}
}

// ensureBalanceType 确保 Balance 类型存在
func (r *inkRegistry) ensureBalanceType() int {
	if id, ok := r.goTypeToID["Balance"]; ok {
		return id
	}
	id := r.appendType(map[string]any{
		"def": map[string]any{
			"primitive": "u128",
		},
		"path": []any{"Balance"},
	})
	r.goTypeToID["Balance"] = id
	return id
}

// ensureHashType 确保 Hash 类型存在
func (r *inkRegistry) ensureHashType() int {
	if id, ok := r.goTypeToID["Hash"]; ok {
		return id
	}
	id := r.appendType(map[string]any{
		"def": map[string]any{
			"composite": map[string]any{
				"fields": []any{
					map[string]any{"name": "0", "type": r.ensureBytes()},
				},
			},
		},
		"path": []any{"Hash"},
	})
	r.goTypeToID["Hash"] = id
	return id
}

func inkConstructor() []any {
	return []any{}
}

func buildInkRoot(contractName string, mutMethods, qMethods []*methodSig, errVariants []errorVariant, structTypes map[string]structType) (map[string]any, error) {
	reg := newInkRegistry(errVariants, structTypes)
	messages, err := buildInkMessages(reg, mutMethods, qMethods)
	if err != nil {
		return nil, err
	}
	spec := map[string]any{
		"constructors": inkConstructor(),
		"docs":         []any{},
		"environment":  inkEnvironment(reg),
		"events":       []any{},
		"lang_error": map[string]any{
			"displayName": []any{"ink", "LangError"},
			"type":        reg.langErrorID,
		},
		"messages": messages,
	}
	root := map[string]any{
		"contract": map[string]any{
			"name":    contractNameGoSafe(contractName),
			"version": "0.1.0",
		},
		"image": nil,
		"source": map[string]any{
			"compiler": "contractgen",
			"hash":     "0x0000000000000000000000000000000000000000000000000000000000000000",
			"language": "go",
		},
		"spec":    spec,
		"storage": nil,
		"types":   reg.types,
		"version": 6,
	}
	return root, nil
}

func buildInkMessages(reg *inkRegistry, mutMethods, qMethods []*methodSig) ([]any, error) {
	var messages []any

	// mutation 方法
	for _, m := range mutMethods {
		args, err := abiArgs(reg, m)
		if err != nil {
			return nil, fmt.Errorf("mutation %s: %w", m.caseName, err)
		}
		msg := map[string]any{
			"args":       args,
			"default":    false,
			"docs":       []any{},
			"label":      m.caseName,
			"mutates":    true,
			"payable":    false,
			"returnType": map[string]any{"displayName": []any{"ink", "MessageResult"}, "type": reg.messageResID},
			"selector":   selectorHex(m.selector),
		}
		messages = append(messages, msg)
	}

	// query 方法
	for _, m := range qMethods {
		args, err := abiArgs(reg, m)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", m.caseName, err)
		}
		retType, err := reg.ensureResultQuery(m.queryResult0)
		if err != nil {
			return nil, fmt.Errorf("abi %s: %w", m.caseName, err)
		}
		msg := map[string]any{
			"args":       args,
			"default":    false,
			"docs":       []any{},
			"label":      m.caseName,
			"mutates":    false,
			"payable":    false,
			"returnType": retType,
			"selector":   selectorHex(m.selector),
		}
		messages = append(messages, msg)
	}

	// 按 selector 排序以保持稳定输出
	sort.Slice(messages, func(i, j int) bool {
		return messages[i].(map[string]any)["selector"].(string) < messages[j].(map[string]any)["selector"].(string)
	})

	return messages, nil
}

func abiArgs(reg *inkRegistry, ms *methodSig) ([]any, error) {
	if ms.isInit {
		out := make([]any, 0, 3)
		for i := 0; i < 3 && i < len(ms.params); i++ {
			a, err := abiOneArg(reg, ms.params[i], i)
			if err != nil {
				return nil, err
			}
			out = append(out, a)
		}
		return out, nil
	}
	out := make([]any, 0, len(ms.params))
	for i := range ms.params {
		a, err := abiOneArg(reg, ms.params[i], i)
		if err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, nil
}

func abiOneArg(reg *inkRegistry, p param, idx int) (map[string]any, error) {
	ref, err := reg.goTypeToABIRefParam(p.underlying)
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"docs":  []any{},
		"label": paramABIName(p.name, idx),
		"type": map[string]any{
			"displayName": ref.Display,
			"type":        ref.TypeID,
		},
	}, nil
}
