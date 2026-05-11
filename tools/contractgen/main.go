// Contractgen 扫描合约包目录，根据 DaoMutation / DaoQuery（可配置）上的导出方法生成 ExecCall / ExecQuery。
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/printer"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

func main() {
	dir := flag.String("dir", "", "合约包目录（含 .go 源文件，必填）")
	out := flag.String("out", "", "输出文件名，默认 <dir>/<pkg>_gen.go")
	mutation := flag.String("mutation", "DaoMutation", "mutation 接收者类型名")
	query := flag.String("query", "DaoQuery", "query 接收者类型名")
	skipMut := flag.String("skip-mutation", "Delete", "逗号分隔：不生成 dispatch 的 mutation 方法名")
	skipQ := flag.String("skip-query", "", "逗号分隔：不生成 dispatch 的 query 方法名")
	errPrefix := flag.String("err-prefix", "", "错误信息前缀，默认与包名相同")
	execCall := flag.String("exec-call", "ExecCall", "跳过自身：mutation 上的 dispatch 方法名")
	execQuery := flag.String("exec-query", "ExecQuery", "跳过自身：query 上的 dispatch 方法名")
	abiOut := flag.String("abi-out", "", "ink metadata 输出路径；默认 <dir>/../../abis/<包名>.json（contracts/<pkg> 布局时）")
	abiSkip := flag.Bool("abi-skip", false, "不生成 ABI")

	// 构造函数生成相关参数
	structName := flag.String("struct", "", "结构体名称，用于生成构造函数（如 DAO、Token等）")
	ctorFunc := flag.String("ctor-func", "", "构造函数名，默认 New<Struct>")
	ctorApiType := flag.String("ctor-api-type", "model.ContractApi", "构造函数的 API 参数类型")
	ctorApiField := flag.String("ctor-api-field", "api", "结构体中 API 字段名")
	ctorSkip := flag.Bool("ctor-skip", false, "不生成构造函数")
	flag.Parse()

	if *dir == "" {
		fmt.Fprintln(os.Stderr, "contractgen: -dir is required")
		os.Exit(1)
	}
	abs, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, abs, func(fi os.FileInfo) bool {
		if strings.HasSuffix(fi.Name(), "_test.go") {
			return false
		}
		return strings.HasSuffix(fi.Name(), ".go")
	}, parser.ParseComments)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if len(pkgs) != 1 {
		fmt.Fprintf(os.Stderr, "contractgen: expected single package in %s, got %d\n", abs, len(pkgs))
		os.Exit(1)
	}
	var pkg *ast.Package //nolint:staticcheck // parser.ParseDir 返回 map[string]*ast.Package
	for _, p := range pkgs {
		pkg = p
		break
	}
	pkgName := ""
	for _, f := range pkg.Files {
		pkgName = f.Name.Name
		break
	}

	outPath := *out
	if outPath == "" {
		outPath = filepath.Join(abs, pkgName+"_gen.go")
	}
	outBase := filepath.Base(outPath)

	// 重新解析，排除输出文件
	pkgs, err = parser.ParseDir(fset, abs, func(fi os.FileInfo) bool {
		if strings.HasSuffix(fi.Name(), "_test.go") {
			return false
		}
		if fi.Name() == outBase {
			return false
		}
		return strings.HasSuffix(fi.Name(), ".go")
	}, parser.ParseComments)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, p := range pkgs {
		pkg = p
		break
	}

	prefix := *errPrefix
	if prefix == "" {
		prefix = pkgName
	}

	skipM := parseSkip(*skipMut)
	skipQset := parseSkip(*skipQ)

	var mutMethods, qMethods []*methodSig
	var storeFields []storeField
	for _, f := range pkg.Files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil || d.Name == nil {
					continue
				}
				recvName := recvTypeName(d.Recv.List[0].Type)
				if recvName == "" {
					continue
				}
				name := d.Name.Name
				if !ast.IsExported(name) {
					continue
				}
				switch recvName {
				case *mutation:
					if name == *execCall {
						continue
					}
					if skipM[name] {
						continue
					}
					ms, err := parseMethod(d, fset, true)
					if err != nil {
						fmt.Fprintf(os.Stderr, "contractgen %s.%s: %v\n", recvName, name, err)
						os.Exit(1)
					}
					mutMethods = append(mutMethods, ms)
				case *query:
					if name == *execQuery {
						continue
					}
					if skipQset[name] {
						continue
					}
					ms, err := parseMethod(d, fset, false)
					if err != nil {
						fmt.Fprintf(os.Stderr, "contractgen %s.%s: %v\n", recvName, name, err)
						os.Exit(1)
					}
					qMethods = append(qMethods, ms)
				}
			case *ast.GenDecl:
				// 解析结构体定义（用于生成构造函数）
				if !*ctorSkip && *structName != "" && d.Tok == token.TYPE {
					for _, spec := range d.Specs {
						ts, ok := spec.(*ast.TypeSpec)
						if !ok || ts.Name == nil || ts.Name.Name != *structName {
							continue
						}
						st, ok := ts.Type.(*ast.StructType)
						if !ok {
							continue
						}
						storeFields = parseStructFields(st, fset)
					}
				}
			}
		}
	}

	sort.Slice(mutMethods, func(i, j int) bool {
		return mutMethods[i].caseName < mutMethods[j].caseName
	})
	sort.Slice(qMethods, func(i, j int) bool {
		return qMethods[i].caseName < qMethods[j].caseName
	})

	// 检查 mutation 和 query 之间是否有同名方法（会导致 ABI label 冲突）
	if dup := findDuplicateLabels(mutMethods, qMethods); len(dup) > 0 {
		fmt.Fprintf(os.Stderr, "contractgen: error: duplicate method names between %s and %s:\n", *mutation, *query)
		for _, d := range dup {
			fmt.Fprintf(os.Stderr, "  - %s (mutation: %s, query: %s)\n", d.caseName, d.mutGoName, d.queryGoName)
		}
		fmt.Fprintf(os.Stderr, "Please rename one of the methods to avoid ABI label conflict.\n")
		os.Exit(1)
	}

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "// Code generated by contractgen; DO NOT EDIT.\n\n")
	fmt.Fprintf(&buf, "package %s\n\n", pkgName)
	fmt.Fprintf(&buf, "import (\n")
	fmt.Fprintf(&buf, "\t\"fmt\"\n\n")
	fmt.Fprintf(&buf, "\t\"github.com/centrifuge/go-substrate-rpc-client/v4/types/codec\"\n")
	fmt.Fprintf(&buf, "\t\"github.com/wetee-dao/ink.go/util\"\n")
	fmt.Fprintf(&buf, "\t\"github.com/wetee-dao/tee-dsecret/pkg/model\"\n")
	fmt.Fprintf(&buf, ")\n\n")

	writeExecCall(&buf, *mutation, mutMethods, prefix, *structName)
	writeExecQuery(&buf, *query, qMethods, prefix)
	if !*ctorSkip && *structName != "" && len(storeFields) > 0 {
		ctorName := *ctorFunc
		if ctorName == "" {
			ctorName = "New" + *structName
		}
		writeConstructor(&buf, *structName, ctorName, pkgName, storeFields, *ctorApiType, *ctorApiField)
	}
	// 生成 methodMap（方法选择器到方法名的映射，仅包含 query 方法）
	writeMethodMap(&buf, qMethods)

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Fprintf(os.Stderr, "format: %v\n%s", err, buf.String())
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, formatted, 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Fprintf(os.Stderr, "contractgen: wrote %s\n", outPath)

	abiOutPath := *abiOut
	if abiOutPath == "" {
		// abs 通常为 .../side-chain/contracts/<pkg>，ABI 放在 .../side-chain/abis/<pkg>.json
		abiOutPath = filepath.Join(abs, "..", "..", "abis", pkgName+".json")
	}
	if !*abiSkip {
		// 收集错误变量定义
		errVariants := collectErrorVariants(pkg)
		// 收集结构体类型定义
		structTypes := collectStructTypes(pkg, fset)
		if err := writeABI(abiOutPath, pkgName, mutMethods, qMethods, errVariants, structTypes); err != nil {
			fmt.Fprintf(os.Stderr, "contractgen abi: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stderr, "contractgen: wrote %s\n", abiOutPath)
	}
}

func parseSkip(s string) map[string]bool {
	m := make(map[string]bool)
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			m[p] = true
		}
	}
	return m
}

func recvTypeName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return recvTypeName(t.X)
	default:
		return ""
	}
}

type methodSig struct {
	goName       string
	caseName     string
	selector     [4]byte // 4字节selector，用于客户端传入selector时匹配
	params       []param
	isInit       bool
	isMutation   bool   // 是否为mutation方法，用于计算selector
	queryResult0 string // query 第一个返回值类型字符串，用于生成 ABI returnType
}

type param struct {
	name       string
	typ        string
	underlying string
	isPtr      bool
}

// errorVariant 表示一个错误变体（用于生成 ABI Error 类型）
type errorVariant struct {
	name  string // 错误名称，如 "MustCallByGov"
	index int    // 变体索引
}

// structField 表示结构体字段
type structField struct {
	name string // 字段名
	typ  string // 字段类型
}

// structType 表示一个结构体类型定义
type structType struct {
	name   string        // 结构体名
	fields []structField // 字段列表
}

// collectTypesFromMethods 从所有方法签名中收集使用的类型
func collectTypesFromMethods(mutMethods, qMethods []*methodSig) map[string]bool {
	types := make(map[string]bool)

	// 收集 mutation 方法的参数类型
	for _, m := range mutMethods {
		for _, p := range m.params {
			collectType(types, p.underlying)
		}
	}

	// 收集 query 方法的参数类型和返回值类型
	for _, m := range qMethods {
		for _, p := range m.params {
			collectType(types, p.underlying)
		}
		if m.queryResult0 != "" {
			collectType(types, m.queryResult0)
		}
	}

	return types
}

// collectErrorVariants 从合约包中收集错误变量定义
// 查找以 "Err" 开头的变量，如 ErrMustCallByGov -> MustCallByGov
func collectErrorVariants(pkg *ast.Package) []errorVariant {
	var variants []errorVariant
	seen := make(map[string]bool)

	for _, f := range pkg.Files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, id := range vs.Names {
					if !strings.HasPrefix(id.Name, "Err") {
						continue
					}
					errName := strings.TrimPrefix(id.Name, "Err")
					if errName == "" || seen[errName] {
						continue
					}
					seen[errName] = true
					variants = append(variants, errorVariant{name: errName, index: len(variants)})
				}
			}
		}
	}

	// 按名称排序以保持稳定输出
	sort.Slice(variants, func(i, j int) bool {
		return variants[i].name < variants[j].name
	})
	// 重新分配索引
	for i := range variants {
		variants[i].index = i
	}

	return variants
}

// collectStructTypes 从合约包中收集结构体类型定义和类型别名
func collectStructTypes(pkg *ast.Package, fset *token.FileSet) map[string]structType {
	structs := make(map[string]structType)

	for _, f := range pkg.Files {
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}

				typeName := ts.Name.Name

				// 处理结构体类型
				if st, ok := ts.Type.(*ast.StructType); ok {
					var fields []structField
					if st.Fields != nil {
						for _, field := range st.Fields.List {
							typStr := typeString(fset, field.Type)
							if len(field.Names) == 0 {
								// 匿名字段
								fields = append(fields, structField{name: "", typ: typStr})
							} else {
								for _, id := range field.Names {
									fields = append(fields, structField{name: id.Name, typ: typStr})
								}
							}
						}
					}
					structs[typeName] = structType{name: typeName, fields: fields}
					continue
				}

				// 处理类型别名（如 type ProposalState string）
				if ident, ok := ts.Type.(*ast.Ident); ok {
					underlyingType := ident.Name
					// 对于 string 类型的别名，将其作为特殊的结构体处理
					// 使用一个特殊的字段名 "_alias" 来标识这是类型别名
					if underlyingType == "string" {
						structs[typeName] = structType{
							name:   typeName,
							fields: []structField{{name: "_alias", typ: "string"}},
						}
					}
				}
			}
		}
	}

	return structs
}

// collectType 从类型字符串中收集基础类型
func collectType(types map[string]bool, typ string) {
	typ = strings.TrimSpace(typ)
	if typ == "" {
		return
	}

	// 处理指针
	if strings.HasPrefix(typ, "*") {
		collectType(types, strings.TrimPrefix(typ, "*"))
		return
	}

	// 处理切片
	if strings.HasPrefix(typ, "[]") {
		inner := strings.TrimPrefix(typ, "[]")
		// 特殊处理 []Member
		if inner == "Member" {
			types["[]Member"] = true
		}
		collectType(types, inner)
		return
	}

	// 处理 Option
	if strings.HasPrefix(typ, "util.Option[") && strings.HasSuffix(typ, "]") {
		inner := typ[len("util.Option[") : len(typ)-1]
		collectType(types, inner)
		return
	}

	// 基础类型和自定义类型
	switch typ {
	case "bool", "uint32", "uint64", "[]byte",
		"model.UniAddr", "model.Amount",
		"CallContent", "TrackData", "Member":
		types[typ] = true
	}
}

func parseMethod(fd *ast.FuncDecl, fset *token.FileSet, isMutation bool) (*methodSig, error) {
	name := fd.Name.Name
	caseName := snakeCase(name)
	ms := &methodSig{
		goName:     name,
		caseName:   caseName,
		selector:   pickSelectorInkBytes(caseName, isMutation),
		isMutation: isMutation,
	}

	if fd.Type.Params != nil {
		for _, field := range fd.Type.Params.List {
			typStr := typeString(fset, field.Type)
			if len(field.Names) == 0 {
				p := param{name: "", typ: typStr}
				setPtrUnderlying(&p)
				ms.params = append(ms.params, p)
				continue
			}
			for _, id := range field.Names {
				p := param{name: id.Name, typ: typStr}
				setPtrUnderlying(&p)
				ms.params = append(ms.params, p)
			}
		}
	}

	if isMutation {
		if fd.Type.Results == nil || len(fd.Type.Results.List) != 1 {
			return nil, fmt.Errorf("mutation must return a single error")
		}
		rt := typeString(fset, fd.Type.Results.List[0].Type)
		if rt != "error" {
			return nil, fmt.Errorf("mutation must return error, got %s", rt)
		}
		if name == "Init" && len(ms.params) >= 4 {
			last := ms.params[len(ms.params)-1]
			if last.isPtr {
				ms.isInit = true
			}
		}
		return ms, nil
	}

	if fd.Type.Results == nil || len(fd.Type.Results.List) != 2 {
		return nil, fmt.Errorf("query must return (T, error)")
	}
	r1 := typeString(fset, fd.Type.Results.List[1].Type)
	if r1 != "error" {
		return nil, fmt.Errorf("query second result must be error, got %s", r1)
	}
	ms.queryResult0 = typeString(fset, fd.Type.Results.List[0].Type)
	return ms, nil
}

func setPtrUnderlying(p *param) {
	p.underlying = strings.TrimPrefix(p.typ, "*")
	p.isPtr = strings.HasPrefix(p.typ, "*")
}

func typeString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, fset, expr)
	return strings.TrimSpace(buf.String())
}

func snakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			b.WriteByte('_')
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

func writeExecCall(buf *bytes.Buffer, mutation string, methods []*methodSig, prefix string, structName string) {
	shortName := strings.TrimPrefix(mutation, "*")
	fmt.Fprintf(buf, "// ExecCall 解析 Tx.contract，再按 Method 与字符串参数列表调用对应变更。\n")
	fmt.Fprintf(buf, "func (d %s) ExecCall(call *model.ContractCall) error {\n", shortName)
	if structName != "" {
		fmt.Fprintf(buf, "\t%s := New%s(d.api)\n", strings.ToLower(structName), structName)
		fmt.Fprintf(buf, "\tm := %s{%s: *%s}\n\n", shortName, structName, strings.ToLower(structName))
	} else {
		fmt.Fprintf(buf, "\tdao := NewDAO(d.api)\n")
		fmt.Fprintf(buf, "\tm := %s{gov: *dao}\n\n", shortName)
	}
	fmt.Fprintf(buf, "\tmethodSel := [4]byte(call.Method[:4])\n")
	fmt.Fprintf(buf, "\targs := call.Args\n")
	fmt.Fprintf(buf, "\tswitch methodSel {\n")
	for _, m := range methods {
		if m.isInit {
			writeInitCase(buf, m)
			continue
		}
		writeMutationCase(buf, m)
	}
	fmt.Fprintf(buf, "\tdefault:\n")
	fmt.Fprintf(buf, "\t\treturn fmt.Errorf(%q, methodSel)\n", prefix+": unknown method %q")
	fmt.Fprintf(buf, "\t}\n")
	fmt.Fprintf(buf, "}\n\n")
}

// Init：与 SCALE 约定一致——至少 3 个参数，第 4 个可选 defaultTrack（TrackData）。
func writeInitCase(buf *bytes.Buffer, m *methodSig) {
	fmt.Fprintf(buf, "\tcase [4]byte{0x%02x, 0x%02x, 0x%02x, 0x%02x}:\n", m.selector[0], m.selector[1], m.selector[2], m.selector[3])
	fmt.Fprintf(buf, "\t\tif err := model.RequireArgLen(args, 3, %q); err != nil {\n", m.caseName)
	fmt.Fprintf(buf, "\t\t\treturn err\n")
	fmt.Fprintf(buf, "\t\t}\n")
	p0 := m.params[0]
	p1 := m.params[1]
	p2 := m.params[2]
	last := m.params[3]

	fmt.Fprintf(buf, "\t\tmembers, err := model.DecodeScaleArgBytes[%s](args[0])\n", p0.underlying)
	fmt.Fprintf(buf, "\t\tif err != nil {\n")
	fmt.Fprintf(buf, "\t\t\treturn fmt.Errorf(%q, err)\n", "init: members: %w")
	fmt.Fprintf(buf, "\t\t}\n")
	fmt.Fprintf(buf, "\t\tpub, err := model.DecodeScaleArgBytes[%s](args[1])\n", p1.underlying)
	fmt.Fprintf(buf, "\t\tif err != nil {\n")
	fmt.Fprintf(buf, "\t\t\treturn fmt.Errorf(%q, err)\n", "init: publicJoin: %w")
	fmt.Fprintf(buf, "\t\t}\n")
	fmt.Fprintf(buf, "\t\tsudo, err := model.DecodeScaleArgBytes[%s](args[2])\n", p2.underlying)
	fmt.Fprintf(buf, "\t\tif err != nil {\n")
	fmt.Fprintf(buf, "\t\t\treturn fmt.Errorf(%q, err)\n", "init: sudoAccount: %w")
	fmt.Fprintf(buf, "\t\t}\n")

	fmt.Fprintf(buf, "\t\tvar dt *%s\n", last.underlying)
	fmt.Fprintf(buf, "\t\tif len(args) >= 4 {\n")
	fmt.Fprintf(buf, "\t\t\tt, err := model.DecodeScaleArgBytes[%s](args[3])\n", last.underlying)
	fmt.Fprintf(buf, "\t\t\tif err != nil {\n")
	fmt.Fprintf(buf, "\t\t\t\treturn fmt.Errorf(%q, err)\n", "init: defaultTrack: %w")
	fmt.Fprintf(buf, "\t\t\t}\n")
	fmt.Fprintf(buf, "\t\t\tdt = &t\n")
	fmt.Fprintf(buf, "\t\t}\n")
	fmt.Fprintf(buf, "\t\treturn m.Init(members, pub, sudo, dt)\n")
}

func writeMutationCase(buf *bytes.Buffer, m *methodSig) {
	fmt.Fprintf(buf, "\tcase [4]byte{0x%02x, 0x%02x, 0x%02x, 0x%02x}:\n", m.selector[0], m.selector[1], m.selector[2], m.selector[3])
	if len(m.params) == 0 {
		fmt.Fprintf(buf, "\t\treturn m.%s()\n", m.goName)
		return
	}

	fmt.Fprintf(buf, "\t\tif err := model.RequireArgLen(args, %d, %q); err != nil {\n", len(m.params), m.caseName)
	fmt.Fprintf(buf, "\t\t\treturn err\n")
	fmt.Fprintf(buf, "\t\t}\n")

	var argNames []string
	for i := range m.params {
		p := m.params[i]
		dec := localName(p.name, i, "a")
		fmt.Fprintf(buf, "\t\t%s, err := model.DecodeScaleArgBytes[%s](args[%d])\n", dec, p.underlying, i)
		errKey := errLabel(m, p, i)
		if shortDecodeErr(m.goName, len(m.params)) {
			fmt.Fprintf(buf, "\t\tif err != nil {\n")
			fmt.Fprintf(buf, "\t\t\treturn fmt.Errorf(%q, err)\n", m.caseName+": %w")
			fmt.Fprintf(buf, "\t\t}\n")
		} else {
			fmt.Fprintf(buf, "\t\tif err != nil {\n")
			fmt.Fprintf(buf, "\t\t\treturn fmt.Errorf(%q, err)\n", m.caseName+": "+errKey+": %w")
			fmt.Fprintf(buf, "\t\t}\n")
		}

		if p.isPtr {
			ptr := dec + "p"
			fmt.Fprintf(buf, "\t\t%s := &%s\n", ptr, dec)
			argNames = append(argNames, ptr)
		} else {
			argNames = append(argNames, dec)
		}
	}

	fmt.Fprintf(buf, "\t\treturn m.%s(%s)\n", m.goName, strings.Join(argNames, ", "))
}

func shortDecodeErr(goName string, n int) bool {
	if n != 1 {
		return false
	}
	return goName == "SetPublicJoin" || goName == "AddTrack"
}

func errLabel(m *methodSig, p param, idx int) string {
	if p.name != "" {
		return p.name
	}
	return fmt.Sprintf("arg%d", idx)
}

func localName(paramName string, idx int, prefix string) string {
	if paramName == "" {
		return fmt.Sprintf("%s%d", prefix, idx)
	}
	return paramName
}

func writeExecQuery(buf *bytes.Buffer, query string, methods []*methodSig, prefix string) {
	shortName := strings.TrimPrefix(query, "*")
	fmt.Fprintf(buf, "// ExecuteQuery 按 method 将字符串参数解析为合约查询实参，返回 SCALE 编码结果。\n")
	fmt.Fprintf(buf, "func (q %s) ExecQuery(call *model.ContractCall) ([]byte, error) {\n", shortName)
	fmt.Fprintf(buf, "\targs := call.Args\n")
	fmt.Fprintf(buf, "\tmethodSel := [4]byte(call.Method[:4])\n")
	fmt.Fprintf(buf, "\tswitch methodSel {\n")

	for _, m := range methods {
		fmt.Fprintf(buf, "\tcase [4]byte{0x%02x, 0x%02x, 0x%02x, 0x%02x}:\n", m.selector[0], m.selector[1], m.selector[2], m.selector[3])
		if len(m.params) == 0 {
			fmt.Fprintf(buf, "\t\tout, err := q.%s()\n", m.goName)
			fmt.Fprintf(buf, "\t\tif err != nil {\n")
			fmt.Fprintf(buf, "\t\t\treturn nil, err\n")
			fmt.Fprintf(buf, "\t\t}\n")
			fmt.Fprintf(buf, "\t\treturn codec.Encode(out)\n")
			continue
		}

		fmt.Fprintf(buf, "\t\tif err := model.RequireArgLen(args, %d, %q); err != nil {\n", len(m.params), m.caseName)
		fmt.Fprintf(buf, "\t\t\treturn nil, err\n")
		fmt.Fprintf(buf, "\t\t}\n")

		var argNames []string
		for i := range m.params {
			p := m.params[i]
			dec := localName(p.name, i, "a")
			fmt.Fprintf(buf, "\t\t%s, err := model.DecodeScaleArgBytes[%s](args[%d])\n", dec, p.underlying, i)
			errKey := errLabel(m, p, i)
			fmt.Fprintf(buf, "\t\tif err != nil {\n")
			fmt.Fprintf(buf, "\t\t\treturn nil, fmt.Errorf(%q, err)\n", m.caseName+": "+errKey+": %w")
			fmt.Fprintf(buf, "\t\t}\n")
			if p.isPtr {
				ptr := dec + "p"
				fmt.Fprintf(buf, "\t\t%s := &%s\n", ptr, dec)
				argNames = append(argNames, ptr)
			} else {
				argNames = append(argNames, dec)
			}
		}

		fmt.Fprintf(buf, "\t\tout, err := q.%s(%s)\n", m.goName, strings.Join(argNames, ", "))
		fmt.Fprintf(buf, "\t\tif err != nil {\n")
		fmt.Fprintf(buf, "\t\t\treturn nil, err\n")
		fmt.Fprintf(buf, "\t\t}\n")
		fmt.Fprintf(buf, "\t\treturn codec.Encode(out)\n")
	}

	fmt.Fprintf(buf, "\tdefault:\n")
	fmt.Fprintf(buf, "\t\treturn nil, fmt.Errorf(%q, methodSel)\n", prefix+": unknown query method %q")
	fmt.Fprintf(buf, "\t}\n")
	fmt.Fprintf(buf, "}\n")
}

// storeMappingField 表示结构体中的一个 StoreMapping 字段
type storeMappingField struct {
	name    string // 字段名
	keyType string // StoreMapping 的 Key 类型
	valType string // StoreMapping 的 Value 类型
	keyPfx  string // KeyPrefix
}

// storeValueField 表示结构体中的一个 StoreValue 字段
type storeValueField struct {
	name    string // 字段名
	valType string // StoreValue 的 Value 类型
	key     string // Key (单值存储的 key)
}

// storeField 表示一个存储字段（StoreMapping 或 StoreValue）
type storeField struct {
	isMapping bool              // 是否为 StoreMapping
	isValue   bool              // 是否为 StoreValue
	isList    bool              // 是否为 StoreList
	isList2D  bool              // 是否为 StoreList2D
	mapping   storeMappingField // StoreMapping 字段
	value     storeValueField   // StoreValue 字段
	list      storeListField    // StoreList 字段
	list2D    storeList2DField  // StoreList2D 字段
}

type storeListField struct {
	name    string // 字段名
	keyType string // K 类型
	valType string // V 类型
	keyPfx  string // keyPfx tag 值或自动生成的
}

type storeList2DField struct {
	name      string // 字段名
	key1Type  string // K1 类型
	indexType string // Ix 类型
	valType   string // V 类型
	keyPfx    string // keyPfx tag 值或自动生成的
}

// parseStructFields 解析结构体中的 StoreMapping 和 StoreValue 字段
func parseStructFields(st *ast.StructType, fset *token.FileSet) []storeField {
	var fields []storeField
	if st.Fields == nil {
		return fields
	}
	for _, field := range st.Fields.List {
		if len(field.Names) == 0 {
			continue
		}
		fieldName := field.Names[0].Name
		typStr := typeString(fset, field.Type)

		// 尝试解析 StoreMapping 类型
		if keyType, valType, ok := parseStoreMappingType(typStr); ok {
			keyPfx := extractKeyPrefix(field, "keyPfx")
			if keyPfx == "" {
				keyPfx = storeKeyPrefix(fieldName)
			}
			fields = append(fields, storeField{
				isMapping: true,
				mapping: storeMappingField{
					name:    fieldName,
					keyType: keyType,
					valType: valType,
					keyPfx:  keyPfx,
				},
			})
			continue
		}

		// 尝试解析 StoreValue 类型
		if valType, ok := parseStoreValueType(typStr); ok {
			key := extractKeyPrefix(field, "key")
			if key == "" {
				key = storeKey(fieldName)
			}
			fields = append(fields, storeField{
				isValue: true,
				value: storeValueField{
					name:    fieldName,
					valType: valType,
					key:     key,
				},
			})
			continue
		}

		// 尝试解析 StoreList 类型
		if keyType, valType, ok := parseStoreListType(typStr); ok {
			keyPfx := extractKeyPrefix(field, "keyPfx")
			if keyPfx == "" {
				keyPfx = storeKeyPrefix(fieldName)
			}
			fields = append(fields, storeField{
				isList: true,
				list: storeListField{
					name:    fieldName,
					keyType: keyType,
					valType: valType,
					keyPfx:  keyPfx,
				},
			})
			continue
		}

		// 尝试解析 StoreList2D 类型
		if k1Type, ixType, valType, ok := parseStoreList2DType(typStr); ok {
			keyPfx := extractKeyPrefix(field, "keyPfx")
			if keyPfx == "" {
				keyPfx = storeKeyPrefix(fieldName)
			}
			fields = append(fields, storeField{
				isList2D: true,
				list2D: storeList2DField{
					name:      fieldName,
					key1Type:  k1Type,
					indexType: ixType,
					valType:   valType,
					keyPfx:    keyPfx,
				},
			})
		}
	}
	return fields
}

// reflectParseStructTag 简单解析 struct tag
func reflectParseStructTag(tag, key string) (map[string]string, bool) {
	// 查找 key:"value" 格式
	idx := strings.Index(tag, key+`:`)
	if idx == -1 {
		return nil, false
	}
	start := idx + len(key) + 2 // 跳过 key:"
	end := strings.Index(tag[start:], `"`)
	if end == -1 {
		return nil, false
	}
	value := tag[start : start+end]
	// 解析 key1:val1,key2:val2 格式
	result := make(map[string]string)
	for _, pair := range strings.Split(value, ",") {
		parts := strings.SplitN(pair, ":", 2)
		if len(parts) == 2 {
			result[parts[0]] = parts[1]
		}
	}
	return result, true
}

// parseStoreMappingType 解析 *model.StoreMapping[K, V] 类型，返回 K, V 类型字符串
func parseStoreMappingType(typ string) (key, val string, ok bool) {
	// 匹配 *model.StoreMapping[K, V] 或 model.StoreMapping[K, V]
	typ = strings.TrimSpace(typ)
	typ = strings.TrimPrefix(typ, "*")
	if !strings.HasPrefix(typ, "model.StoreMapping[") || !strings.HasSuffix(typ, "]") {
		return "", "", false
	}
	inner := typ[len("model.StoreMapping[") : len(typ)-1]
	// 解析 [K, V] 中的 K 和 V，需要处理嵌套的 [] 和 ,
	k, v, ok := parseTypePair(inner)
	if !ok {
		return "", "", false
	}
	return k, v, true
}

// parseStoreValueType 解析 *model.StoreValue[T] 类型，返回 T 类型字符串
func parseStoreValueType(typ string) (val string, ok bool) {
	// 匹配 *model.StoreValue[T] 或 model.StoreValue[T]
	typ = strings.TrimSpace(typ)
	typ = strings.TrimPrefix(typ, "*")
	if !strings.HasPrefix(typ, "model.StoreValue[") || !strings.HasSuffix(typ, "]") {
		return "", false
	}
	inner := typ[len("model.StoreValue[") : len(typ)-1]
	return strings.TrimSpace(inner), true
}

// parseStoreListType 解析 *model.StoreList[K, V] 类型，返回 K, V 类型字符串
func parseStoreListType(typ string) (key, val string, ok bool) {
	// 匹配 *model.StoreList[K, V] 或 model.StoreList[K, V]
	typ = strings.TrimSpace(typ)
	typ = strings.TrimPrefix(typ, "*")
	if !strings.HasPrefix(typ, "model.StoreList[") || !strings.HasSuffix(typ, "]") {
		return "", "", false
	}
	inner := typ[len("model.StoreList[") : len(typ)-1]
	// 解析 [K, V] 中的 K 和 V
	k, v, ok := parseTypePair(inner)
	if !ok {
		return "", "", false
	}
	return k, v, true
}

// parseStoreList2DType 解析 *model.StoreList2D[K1, Ix, V] 类型，返回 K1, Ix, V 类型字符串
func parseStoreList2DType(typ string) (k1, ix, val string, ok bool) {
	// 匹配 *model.StoreList2D[K1, Ix, V] 或 model.StoreList2D[K1, Ix, V]
	typ = strings.TrimSpace(typ)
	typ = strings.TrimPrefix(typ, "*")
	if !strings.HasPrefix(typ, "model.StoreList2D[") || !strings.HasSuffix(typ, "]") {
		return "", "", "", false
	}
	inner := typ[len("model.StoreList2D[") : len(typ)-1]
	// 解析 [K1, Ix, V] 中的三个类型参数
	// 先找到第一个逗号
	depth := 0
	firstComma := -1
	secondComma := -1
	for i, c := range inner {
		switch c {
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		case ',':
			if depth == 0 {
				if firstComma == -1 {
					firstComma = i
				} else {
					secondComma = i
					break
				}
			}
		}
	}
	if firstComma == -1 || secondComma == -1 {
		return "", "", "", false
	}
	k1 = strings.TrimSpace(inner[:firstComma])
	ix = strings.TrimSpace(inner[firstComma+1 : secondComma])
	val = strings.TrimSpace(inner[secondComma+1:])
	return k1, ix, val, true
}

// extractKeyPrefix 从字段注释或 tag 中提取 key 或 keyPfx
func extractKeyPrefix(field *ast.Field, keyName string) string {
	keyPfx := ""
	// 先检查行尾注释（Comment）
	if field.Comment != nil {
		for _, c := range field.Comment.List {
			text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			if strings.HasPrefix(text, keyName+":") {
				keyPfx = strings.TrimSpace(strings.TrimPrefix(text, keyName+":"))
				break
			}
		}
	}
	// 再检查文档注释（Doc）
	if keyPfx == "" && field.Doc != nil {
		for _, c := range field.Doc.List {
			text := strings.TrimSpace(strings.TrimPrefix(c.Text, "//"))
			if strings.HasPrefix(text, keyName+":") {
				keyPfx = strings.TrimSpace(strings.TrimPrefix(text, keyName+":"))
				break
			}
		}
	}
	if keyPfx == "" && field.Tag != nil {
		// 解析 struct tag: `store:"key:xxx"` 或 `store:"keyPfx:xxx"`
		tag := strings.Trim(field.Tag.Value, "`")
		if storeTag, ok := reflectParseStructTag(tag, "store"); ok {
			if pfx, ok := storeTag[keyName]; ok {
				keyPfx = pfx
			}
		}
	}
	return keyPfx
}

// storeKey 根据字段名生成 Key（用于 StoreValue 单值存储）
func storeKey(field string) string {
	// 去掉特定后缀
	name := strings.TrimSuffix(field, "Store")
	// 转蛇形（处理连续大写字母如 ID）
	return smartSnakeCase(name)
}

// parseTypePair 解析 "K, V" 格式的类型对，处理嵌套的 [] 和 ,
func parseTypePair(s string) (k, v string, ok bool) {
	s = strings.TrimSpace(s)
	depth := 0
	for i, c := range s {
		switch c {
		case '[', '(':
			depth++
		case ']', ')':
			depth--
		case ',':
			if depth == 0 {
				k = strings.TrimSpace(s[:i])
				v = strings.TrimSpace(s[i+1:])
				return k, v, true
			}
		}
	}
	return "", "", false
}

// storeKeyPrefix 根据字段名生成 KeyPrefix
// 规则：驼峰转蛇形，处理复数形式（去掉末尾 s），去掉特定后缀（Store），加下划线
func storeKeyPrefix(field string) string {
	// 去掉特定后缀
	name := strings.TrimSuffix(field, "Store")
	// 转蛇形（处理连续大写字母如 ID）
	snake := smartSnakeCase(name)
	// 处理复数形式（简单去掉末尾 s）
	if strings.HasSuffix(snake, "s") && !strings.HasSuffix(snake, "ss") {
		snake = snake[:len(snake)-1]
	}
	return snake + "_"
}

// smartSnakeCase 将驼峰转为蛇形，处理连续大写字母（如 ID -> id）
func smartSnakeCase(s string) string {
	var b strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			// 前一个字符是小写，或者后一个字符是小写，才添加下划线
			// 连续大写字母不添加下划线（如 ID）
			prev := []rune(s)[i-1]
			if prev >= 'a' && prev <= 'z' {
				b.WriteByte('_')
			} else if i+1 < len(s) {
				next := []rune(s)[i+1]
				if next >= 'a' && next <= 'z' {
					b.WriteByte('_')
				}
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}

// writeConstructor 生成构造函数
func writeConstructor(buf *bytes.Buffer, structName, ctorName, pkgName string, fields []storeField, apiType, apiField string) {
	fmt.Fprintf(buf, "\nfunc %s(api %s) *%s {\n", ctorName, apiType, structName)
	fmt.Fprintf(buf, "\treturn &%s{\n", structName)
	fmt.Fprintf(buf, "\t\t%s: api,\n", apiField)
	for _, f := range fields {
		if f.isMapping {
			fmt.Fprintf(buf, "\t\t%s: &model.StoreMapping[%s, %s]{Namespace: %q, KeyPrefix: %q},\n",
				f.mapping.name, f.mapping.keyType, f.mapping.valType, pkgName, f.mapping.keyPfx)
		} else if f.isValue {
			fmt.Fprintf(buf, "\t\t%s: &model.StoreValue[%s]{Namespace: %q, Key: %q},\n",
				f.value.name, f.value.valType, pkgName, f.value.key)
		} else if f.isList {
			// StoreList 使用 NewStoreList 构造函数
			// prefixNextID: "{keyPfx}next_", prefixItems: "{keyPfx}items_"
			prefixNextID := f.list.keyPfx + "next_"
			prefixItems := f.list.keyPfx + "items_"
			fmt.Fprintf(buf, "\t\t%s: model.NewStoreList[%s, %s](%q, %q, %q),\n",
				f.list.name, f.list.keyType, f.list.valType, pkgName, prefixNextID, prefixItems)
		} else if f.isList2D {
			// StoreList2D 使用 NewStoreList2D 构造函数
			// prefixK1ToID: "{keyPfx}k1_to_id_", prefixK1Length: "{keyPfx}k1_length_"
			// prefixK2NextID: "{keyPfx}k2_next_", prefixStore: "{keyPfx}store_"
			prefixK1ToID := f.list2D.keyPfx + "k1_to_id_"
			prefixK1Length := f.list2D.keyPfx + "k1_length_"
			prefixK2NextID := f.list2D.keyPfx + "k2_next_"
			prefixStore := f.list2D.keyPfx + "store_"
			fmt.Fprintf(buf, "\t\t%s: model.NewStoreList2D[%s, %s, %s](%q, %q, %q, %q, %q),\n",
				f.list2D.name, f.list2D.key1Type, f.list2D.indexType, f.list2D.valType,
				pkgName, prefixK1ToID, prefixK1Length, prefixK2NextID, prefixStore)
		}
	}
	fmt.Fprintf(buf, "\t}\n")
	fmt.Fprintf(buf, "}\n")
}

// duplicateLabel 表示重复的方法名
type duplicateLabel struct {
	caseName    string // ABI label（蛇形命名）
	mutGoName   string // mutation 的 Go 方法名
	queryGoName string // query 的 Go 方法名
}

// findDuplicateLabels 检查 mutation 和 query 之间是否有同名方法
// 返回重复的方法列表，如果无重复则返回空切片
func findDuplicateLabels(mutMethods, qMethods []*methodSig) []duplicateLabel {
	// 构建 mutation 的 caseName 集合
	mutNames := make(map[string]string) // caseName -> goName
	for _, m := range mutMethods {
		mutNames[m.caseName] = m.goName
	}

	var duplicates []duplicateLabel
	for _, q := range qMethods {
		if mutGoName, exists := mutNames[q.caseName]; exists {
			duplicates = append(duplicates, duplicateLabel{
				caseName:    q.caseName,
				mutGoName:   mutGoName,
				queryGoName: q.goName,
			})
		}
	}
	return duplicates
}

// writeMethodMap 生成 MethodMap 变量，用于方法选择器到方法名的映射
func writeMethodMap(buf *bytes.Buffer, methods []*methodSig) {
	if len(methods) == 0 {
		return
	}

	fmt.Fprintf(buf, "\nvar MethodMap = map[[4]byte]string{\n")
	for _, m := range methods {
		fmt.Fprintf(buf, "\t[4]byte{0x%02x, 0x%02x, 0x%02x, 0x%02x}: %q,\n",
			m.selector[0], m.selector[1], m.selector[2], m.selector[3], m.caseName)
	}
	fmt.Fprintf(buf, "}\n")
}
