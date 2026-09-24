// 类型生成：ABI 类型 → Go 类型（沿用 go-ink-gen 的类型映射与 centrifuge scale 编解码），
// 每个合约一个包，类型全部落在该包的 types.go。
package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/wetee-dao/ink.go/util"
)

// ReviveGen 生成器：解析 ink metadata v6，产出 types.go 正文与调用列表。
type ReviveGen struct {
	Abi        *util.InkAbi
	TypeMap    map[int]util.AbiSubType
	TypeResult map[int]string  // type id → 类型声明
	Names      map[string]bool // 已生成的类型名（与客户端类型名查重、同名类型查重）

	TypeIDByName map[string]int // 类型名 → 首次登记的 type id
	conflict     error          // ABI 内同名不同类型布局（Parse 返回错误）
}

func NewReviveGen(abiRaw []byte) (*ReviveGen, error) {
	abi, err := util.InitAbi(abiRaw)
	if err != nil {
		return nil, err
	}

	typeMap := map[int]util.AbiSubType{}
	for _, t := range abi.Types {
		typeMap[t.Id] = t.Type
	}

	return &ReviveGen{
		Abi:          abi,
		TypeMap:      typeMap,
		TypeResult:   map[int]string{},
		Names:        map[string]bool{},
		TypeIDByName: map[string]int{},
	}, nil
}

// Parse 解析全部 message，递归生成涉及的类型（TypeResult）并返回调用列表。
// 只读方法解析返回类型（链上已解包 Result：查询返回裸值）；变更方法不解析返回类型
// （提交只确认交易，Err 侧枚举无需生成）。
func (r *ReviveGen) Parse() ([]Func, error) {
	funcs := make([]Func, 0, len(r.Abi.Spec.Messages))
	for i := range r.Abi.Spec.Messages {
		msg := r.Abi.Spec.Messages[i]

		args := make([]string, 0, len(msg.Args))
		argNames := make([]string, 0, len(msg.Args))
		for j, arg := range msg.Args {
			typeName := ""
			if len(arg.Type.DisplayName) > 0 {
				typeName = arg.Type.DisplayName[len(arg.Type.DisplayName)-1]
			}
			rtype := r.RecursionTypes(arg.Type.Type, typeName, 1)
			name := goParamName(arg.Label, j)
			args = append(args, name+" "+rtype[1])
			argNames = append(argNames, name)
		}

		ret := "util.NullTuple"
		if !msg.Mutates && msg.ReturnType.Type != 0 {
			returnName := ""
			if len(msg.ReturnType.DisplayName) > 0 {
				returnName = msg.ReturnType.DisplayName[len(msg.ReturnType.DisplayName)-1]
			}
			if returnType, ok := r.GetReturnValue(msg.ReturnType.Type); ok {
				ret = r.RecursionTypes(returnType, returnName, 1)[1]
			}
		}

		funcs = append(funcs, Func{
			Selector:   msg.Selector,
			FuncName:   msg.Label,
			ArgStr:     joinCommaSuffix(argNames),
			ArgTypeStr: joinCommaSuffix(args),
			Return:     ret,
			IsMut:      msg.Mutates,
		})
	}
	return funcs, r.conflict
}

// TypesBody 渲染 types.go 正文（按 type id 排序，与 ABI 中的定义顺序一致）。
func (r *ReviveGen) TypesBody() string {
	keys := make([]int, 0, len(r.TypeResult))
	for k := range r.TypeResult {
		keys = append(keys, k)
	}
	sort.Ints(keys)

	var sb strings.Builder
	for _, t := range keys {
		sb.WriteString(r.TypeResult[t])
		sb.WriteString("\n")
	}
	return sb.String()
}

// GetReturnValue 取 Result 的 Ok 内部类型 id（链上查询已解包 Result，返回裸值）。
func (r *ReviveGen) GetReturnValue(ty int) (int, bool) {
	def, ok := r.TypeMap[ty]
	if !ok || def.Def.Variant == nil {
		return 0, false
	}
	for _, v := range def.Def.Variant.Variants {
		for i := 0; i < len(v.Fields); i++ {
			if i == 0 {
				return v.Fields[i].Type, true
			}
		}
	}
	return 0, false
}

// RecursionTypes 递归解析类型 → [字段名, Go 类型, ABI 类型种类]；顺带把新类型写入 TypeResult。
func (r *ReviveGen) RecursionTypes(ty int, name string, level int) []string {
	path := r.TypeMap[ty].Path
	fields := [][]string{}
	def := r.TypeMap[ty].Def
	curtype := r.GetType(r.TypeMap[ty])

	name = UnderscoreToCamelCase(name)

	returnType := ""
	returnTraits := ""
	if def.Composite != nil {
		for _, v := range def.Composite.Fields {
			f := r.RecursionTypes(v.Type, v.Name, level+1)
			fields = append(fields, f)
		}
	} else if def.Variant != nil {
		enums := [][][]string{}
		for _, v := range def.Variant.Variants {
			enumItem := [][]string{}
			for i := 0; i < len(v.Fields); i++ {
				subfield := v.Fields[i]
				f := r.RecursionTypes(subfield.Type, subfield.Name, level+2)
				enumItem = append(enumItem, f)
			}
			enums = append(enums, enumItem)
		}

		var enumName = name
		if len(path) > 0 {
			enumName = path[len(path)-1]
		}
		returnTraits = r.EnumGen(ty, enumName, def.Variant.Variants, enums)
	} else if def.Sequence != nil {
		f := r.RecursionTypes(def.Sequence.Type, "", level+1)
		f[0] = name
		if isInkString(path) {
			// ink 的 String：ABI 里是 path=["string"] 的 Vec<u8>（seq<u8>），
			// wire 格式与 []byte 相同（compact 长度 + UTF-8 字节），Go 侧用 string 更贴合语义。
			f[1] = "string"
		} else {
			f[1] = "[]" + f[1]
		}
		return f
	} else if def.Array != nil {
		f := r.RecursionTypes(def.Array.Type, fmt.Sprintf("Length%d", def.Array.Len), level+1)
		f[0] = name
		f[1] = fmt.Sprintf("[%d]%s", def.Array.Len, f[1])
		return f
	} else if def.Tuple != nil {
		for i, v := range *def.Tuple {
			f := r.RecursionTypes(v, fmt.Sprintf("F%d", i), level+1)
			fields = append(fields, f)
		}
	} else if def.Primitive != nil {
		returnType = primitiveMapping[fmt.Sprint(*def.Primitive)]
	} else {
		// Range / Compact / BitSequence 等：不支持，明确报错而非静默生成错误类型。
		panic(fmt.Sprintf("RecursionTypes: unsupported def kind for type %d (%s)", ty, curtype))
	}

	typeName := ""
	if len(path) > 0 {
		typeName = path[len(path)-1]
	}
	if typeName == "" && len(fields) > 0 {
		// 无 path 的匿名复合类型 / 元组：用 Tuple_<id> 命名，保证包内唯一且可被引用
		typeName = "Tuple_" + fmt.Sprint(ty)
	}

	switch {
	case len(fields) > 0 && typeName != "" && !r.CheckTypeIsSkip(ty, typeName):
		var typeStr strings.Builder
		switch {
		case len(fields) > 1:
			typeStr.WriteString("type " + typeName + " struct {  // " + curtype + "\n")
			for _, v := range fields {
				typeStr.WriteString("  " + v[0] + "   " + v[1] + "\n")
			}
			typeStr.WriteString("}\n")
		case fields[0][0] == "":
			typeStr.WriteString("type " + typeName + " = " + fields[0][1] + "  // " + curtype + "\n")
		default:
			typeStr.WriteString("type " + typeName + " struct {  // " + curtype + "\n")
			for _, v := range fields {
				typeStr.WriteString("  " + v[0] + "   " + v[1] + "\n")
			}
			typeStr.WriteString("}\n")
		}
		r.registerType(ty, typeName, typeStr.String())
	case len(fields) == 0 && curtype == "Tuple":
		// ink 的 () （空元组）→ util.NullTuple（命名空元组生成别名）
		if typeName == "" || r.CheckTypeIsSkip(ty, typeName) {
			returnType = "util.NullTuple"
		} else {
			r.registerType(ty, typeName, "type "+typeName+" = util.NullTuple  // Tuple\n")
			returnType = typeName
		}
	}

	if returnType == "" && typeName != "" {
		returnType = typeName
	}

	return []string{
		name, addTypePrefix(returnType) + returnTraits, curtype,
	}
}

// GetType ABI 类型定义 → 种类名（注释与分支用）。
func (r *ReviveGen) GetType(data util.AbiSubType) string {
	def := data.Def
	switch {
	case def.Composite != nil:
		return "Composite"
	case def.Variant != nil:
		return "Variant|Enum"
	case def.Sequence != nil:
		return "Sequence|Vec"
	case def.Array != nil:
		return "Array"
	case def.Tuple != nil:
		return "Tuple"
	case def.Range != nil:
		return "Range"
	case def.Compact != nil:
		return "Compact"
	case def.BitSequence != nil:
		return "BitSequence"
	case def.Primitive != nil:
		return "Primitive"
	default:
		return "Unknown"
	}
}

// CheckTypeIsSkip 已生成 / 映射到外部包（types、util）的类型不再生成。
func (r *ReviveGen) CheckTypeIsSkip(ty int, name string) bool {
	if _, ok := r.TypeResult[ty]; ok {
		return true
	}
	_, skip := skipTypes[strings.ReplaceAll(name, " ", "")]
	return skip
}

// registerType 按类型名登记生成结果：contractgen 会把同一命名类型重复注册成不同 id
// （如 gateway 的 UniAddr type 13 / 72，布局相同），同名同布局只生成一次；
// 同名不同布局无法在同一包内共存，记录下来由 Parse 报错，让人工介入。
func (r *ReviveGen) registerType(ty int, name, src string) {
	if prev, ok := r.TypeIDByName[name]; ok {
		if r.TypeResult[prev] != src {
			r.conflict = fmt.Errorf(
				"ABI 中类型名 %s 存在多个不同布局（type %d / type %d），无法生成到同一个包，请重命名其中之一",
				name, prev, ty,
			)
		}
		return
	}
	r.TypeIDByName[name] = ty
	r.TypeResult[ty] = src
	r.Names[name] = true
}

// isInkString ABI 里以 Vec<u8> 表示的字符串（path 末段为 string / String / str / &str）。
func isInkString(path []string) bool {
	if len(path) == 0 {
		return false
	}
	switch path[len(path)-1] {
	case "string", "String", "str", "&str":
		return true
	}
	return false
}

// addTypePrefix 外部包类型补包名（types.H160 / util.Option 等）。
func addTypePrefix(ty string) string {
	if t, ok := typePrefix[ty]; ok {
		ty = t + "." + ty
	}
	return ty
}

// joinCommaSuffix 以逗号连接，并在非空时补尾逗号（模板里直接拼进参数列表）。
func joinCommaSuffix(items []string) string {
	if len(items) == 0 {
		return ""
	}
	return strings.Join(items, ",") + ","
}

// ink type to go type
var primitiveMapping = map[string]string{
	"bool":   "bool",
	"i8":     "int8",
	"i16":    "int16",
	"i32":    "int32",
	"i64":    "int64",
	"i128":   "types.I128",
	"u8":     "byte",
	"u16":    "uint16",
	"u32":    "uint32",
	"u64":    "uint64",
	"u128":   "types.U128",
	"String": "string",
	"&str":   "string",
}

var skipTypes = map[string]bool{
	"U256":      true,
	"H256":      true,
	"H160":      true,
	"AccountId": true,
	"LangError": true,
}

var typePrefix = map[string]string{
	"U256":        "types",
	"H256":        "types",
	"H160":        "types",
	"Result":      "util",
	"Option":      "util",
	"AccountId":   "util",
	"BlockNumber": "util",
	"Bytes":       "util",
}
