package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"
)

// ── ABI JSON structures (match ink ABI v6) ───────────────────

type InkABI struct {
	Contract struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"contract"`
	Spec struct {
		Constructors []Message `json:"constructors"`
		Messages     []Message `json:"messages"`
	} `json:"spec"`
	Types   []TypeEntry `json:"types"`
	Version int         `json:"version"`
}

type Message struct {
	Label      string `json:"label"`
	Selector   string `json:"selector"`
	Mutates    bool   `json:"mutates"`
	Payable    bool   `json:"payable"`
	Args       []Arg  `json:"args"`
	ReturnType struct {
		DisplayName []string `json:"displayName"`
		Type        int      `json:"type"`
	} `json:"returnType"`
}

type Arg struct {
	Label string `json:"label"`
	Type  struct {
		DisplayName []string `json:"displayName"`
		Type        int      `json:"type"`
	} `json:"type"`
}

type TypeEntry struct {
	ID   int `json:"id"`
	Type struct {
		Def    TypeDef  `json:"def"`
		Path   []string `json:"path"`
		Params []Param  `json:"params"`
	} `json:"type"`
}

type Param struct {
	Name string `json:"name"`
	Type int    `json:"type"`
}

type TypeDef struct {
	Primitive string        `json:"primitive,omitempty"`
	Composite *CompositeDef `json:"composite,omitempty"`
	Variant   *VariantDef   `json:"variant,omitempty"`
	Sequence  *SequenceDef  `json:"sequence,omitempty"`
	Array     *ArrayDef     `json:"array,omitempty"`
	Tuple     *TupleDef     `json:"tuple,omitempty"`
}

type CompositeDef struct {
	Fields []Field `json:"fields"`
}

type VariantDef struct {
	Variants []Variant `json:"variants"`
}

type SequenceDef struct {
	Type int `json:"type"`
}

type ArrayDef struct {
	Type int `json:"type"`
	Len  int `json:"len"`
}

type TupleDef struct {
	Fields []int `json:"fields"`
}

type Field struct {
	Name string `json:"name,omitempty"`
	Type int    `json:"type"`
}

type Variant struct {
	Name   string  `json:"name"`
	Index  int     `json:"index"`
	Fields []Field `json:"fields"`
}

// ── TypeScript Generator ─────────────────────────────────────

type TSGen struct {
	abi         *InkABI
	typeMap     map[int]TypeEntry
	typeResult  map[int]string    // id -> emitted TS type name / expression
	definitions map[string]string // name -> definition body
	anonCount   int
}

func NewTSGen(abi *InkABI) *TSGen {
	tm := make(map[int]TypeEntry, len(abi.Types))
	for _, t := range abi.Types {
		tm[t.ID] = t
	}
	return &TSGen{
		abi:         abi,
		typeMap:     tm,
		typeResult:  make(map[int]string),
		definitions: make(map[string]string),
	}
}

// ── Helpers ──────────────────────────────────────────────────

func pascal(s string) string {
	parts := strings.Split(s, "_")
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		r := []rune(p)
		r[0] = unicode.ToUpper(r[0])
		b.WriteString(string(r))
	}
	return b.String()
}

func camel(s string) string {
	if s == "" {
		return s
	}
	r := []rune(s)
	r[0] = unicode.ToLower(r[0])
	return string(r)
}

func typeNameFromPath(path []string) string {
	if len(path) == 0 {
		return ""
	}
	return pascal(path[len(path)-1])
}

func primitiveToTS(p string) string {
	switch p {
	case "u8", "u16", "u32", "i8", "i16", "i32":
		return "number"
	case "u64", "i64":
		return "string"
	case "u128", "i128", "u256", "i256":
		return "string"
	case "bool":
		return "boolean"
	case "str":
		return "string"
	}
	return "unknown /* " + p + " */"
}

// findVariant 按名称查找 variant，不依赖顺序
func findVariant(variants []Variant, name string) *Variant {
	for i := range variants {
		if variants[i].Name == name {
			return &variants[i]
		}
	}
	return nil
}

// isPrimitiveU8 判断指定 type id 是否为 primitive u8
func (g *TSGen) isPrimitiveU8(ty int) bool {
	entry, ok := g.typeMap[ty]
	if !ok {
		return false
	}
	return entry.Type.Def.Primitive == "u8"
}

// ── Recursive type resolver (returns TS type expression) ─────

func (g *TSGen) RecursionTypes(ty int, hintName string, depth int) string {
	if depth > 50 {
		return "unknown /* depth exceeded */"
	}
	if cached, ok := g.typeResult[ty]; ok {
		return cached
	}

	entry, ok := g.typeMap[ty]
	if !ok {
		return "unknown /* unknown type id */"
	}

	d := entry.Type.Def

	// Primitive
	if d.Primitive != "" {
		result := primitiveToTS(d.Primitive)
		g.typeResult[ty] = result
		return result
	}

	// Composite (struct)
	if d.Composite != nil {
		name := typeNameFromPath(entry.Type.Path)
		if name == "" {
			g.anonCount++
			name = fmt.Sprintf("AnonStruct%d", g.anonCount)
		}
		g.typeResult[ty] = name
		body := g.structBody(d.Composite.Fields, depth)
		g.definitions[name] = body
		return name
	}

	// Variant (enum / Result / Option)
	if d.Variant != nil {
		return g.variantToTS(ty, entry, d.Variant, depth)
	}

	// Sequence (Vec<T>)
	if d.Sequence != nil {
		if g.isPrimitiveU8(d.Sequence.Type) {
			g.typeResult[ty] = "string"
			return "string"
		}
		inner := g.RecursionTypes(d.Sequence.Type, "", depth+1)
		result := inner + "[]"
		g.typeResult[ty] = result
		return result
	}

	// Array ([T; n])
	if d.Array != nil {
		if g.isPrimitiveU8(d.Array.Type) {
			g.typeResult[ty] = "string"
			return "string"
		}
		inner := g.RecursionTypes(d.Array.Type, "", depth+1)
		result := inner + "[]"
		g.typeResult[ty] = result
		return result
	}

	// Tuple
	if d.Tuple != nil {
		parts := make([]string, len(d.Tuple.Fields))
		for i, fid := range d.Tuple.Fields {
			parts[i] = g.RecursionTypes(fid, "", depth+1)
		}
		result := "[" + strings.Join(parts, ", ") + "]"
		g.typeResult[ty] = result
		return result
	}

	result := "unknown /* unsupported */"
	g.typeResult[ty] = result
	return result
}

func (g *TSGen) structBody(fields []Field, depth int) string {
	if len(fields) == 0 {
		return "{}"
	}
	var lines []string
	for _, f := range fields {
		fname := f.Name
		if fname == "" {
			fname = fmt.Sprintf("_%d", f.Type)
		}
		lines = append(lines, fmt.Sprintf("  %s: %s;", fname, g.RecursionTypes(f.Type, "", depth+1)))
	}
	return "{\n" + strings.Join(lines, "\n") + "\n}"
}

// variantToTS handles enums. Special-cases Result<T,E> and Option<T>.
func (g *TSGen) variantToTS(ty int, entry TypeEntry, v *VariantDef, depth int) string {
	variants := v.Variants

	// Result<T, E> —— 不依赖 variants 顺序
	var okVariant, errVariant *Variant
	if len(variants) == 2 {
		switch variants[0].Name {
		case "Ok":
			if variants[1].Name == "Err" {
				okVariant, errVariant = &variants[0], &variants[1]
			}
		case "Err":
			if variants[1].Name == "Ok" {
				okVariant, errVariant = &variants[1], &variants[0]
			}
		}
	}
	if okVariant != nil {
		okType := "void"
		if len(okVariant.Fields) > 0 {
			okType = g.RecursionTypes(okVariant.Fields[0].Type, "", depth+1)
		}
		name := typeNameFromPath(entry.Type.Path)
		if name != "" && name != "Result" {
			if _, exists := g.definitions[name]; !exists {
				errType := "unknown"
				if len(errVariant.Fields) > 0 {
					errType = g.RecursionTypes(errVariant.Fields[0].Type, "", depth+1)
				}
				g.typeResult[ty] = name
				g.definitions[name] = fmt.Sprintf("{ Ok: %s } | { Err: %s }", okType, errType)
				return name
			}
		}
		g.typeResult[ty] = okType
		return okType
	}

	// Option<T> —— 不依赖 variants 顺序，返回内联 T | null（不生成 named definition）
	var someVariant *Variant
	if len(variants) == 2 {
		switch variants[0].Name {
		case "Some":
			if variants[1].Name == "None" {
				someVariant = &variants[0]
			}
		case "None":
			if variants[1].Name == "Some" {
				someVariant = &variants[1]
			}
		}
	}
	if someVariant != nil {
		someType := "unknown"
		if len(someVariant.Fields) > 0 {
			someType = g.RecursionTypes(someVariant.Fields[0].Type, "", depth+1)
		}
		result := someType + " | null"
		g.typeResult[ty] = result
		return result
	}

	// Generic enum
	name := typeNameFromPath(entry.Type.Path)
	if name == "" {
		g.anonCount++
		name = fmt.Sprintf("AnonEnum%d", g.anonCount)
	}
	if _, exists := g.definitions[name]; !exists {
		var parts []string
		for _, vr := range variants {
			switch len(vr.Fields) {
			case 0:
				parts = append(parts, fmt.Sprintf("{ %s: null }", vr.Name))
			case 1:
				parts = append(parts, fmt.Sprintf("{ %s: %s }", vr.Name, g.RecursionTypes(vr.Fields[0].Type, "", depth+1)))
			default:
				fields := make([]string, len(vr.Fields))
				for i, f := range vr.Fields {
					fields[i] = g.RecursionTypes(f.Type, "", depth+1)
				}
				parts = append(parts, fmt.Sprintf("{ %s: [%s] }", vr.Name, strings.Join(fields, ", ")))
			}
		}
		g.definitions[name] = strings.Join(parts, " | ")
	}
	g.typeResult[ty] = name
	return name
}

// ── Return value extraction ──────────────────────────────────

func (g *TSGen) GetReturnValue(msg Message) string {
	rt := msg.ReturnType.Type
	entry, ok := g.typeMap[rt]
	if !ok {
		return "unknown"
	}
	v := entry.Type.Def.Variant
	if v == nil {
		return g.RecursionTypes(rt, "", 0)
	}
	for _, vr := range v.Variants {
		if vr.Name == "Ok" && len(vr.Fields) > 0 {
			return g.RecursionTypes(vr.Fields[0].Type, "", 0)
		}
	}
	return "void"
}

// ── Output generation ────────────────────────────────────────

func (g *TSGen) Generate() string {
	var out strings.Builder
	out.WriteString("// Auto-generated from ink! ABI\n")
	out.WriteString("// DO NOT EDIT MANUALLY\n\n")

	msgReturns := make(map[string]string)
	for _, msg := range g.abi.Spec.Messages {
		ret := g.GetReturnValue(msg)
		msgReturns[msg.Label] = ret
	}

	var names []string
	for n := range g.definitions {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		body := g.definitions[name]
		if strings.HasPrefix(body, "{") && strings.HasSuffix(body, "}") {
			out.WriteString(fmt.Sprintf("export interface %s %s\n\n", name, body))
		} else {
			out.WriteString(fmt.Sprintf("export type %s = %s;\n\n", name, body))
		}
	}

	for _, msg := range g.abi.Spec.Messages {
		funcName := pascal(msg.Label)
		ret := msgReturns[msg.Label]
		out.WriteString(fmt.Sprintf("export type %sResponse = %s;\n", funcName, ret))
	}

	return out.String()
}

// ── Main ─────────────────────────────────────────────────────

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: %s <abi.json> [out.ts]\n", os.Args[0])
		os.Exit(1)
	}
	abiPath := os.Args[1]
	outPath := ""
	if len(os.Args) >= 3 {
		outPath = os.Args[2]
	} else {
		base := strings.TrimSuffix(filepath.Base(abiPath), filepath.Ext(abiPath))
		outPath = base + ".ts"
	}

	data, err := os.ReadFile(abiPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read abi: %v\n", err)
		os.Exit(1)
	}

	var abi InkABI
	if err := json.Unmarshal(data, &abi); err != nil {
		fmt.Fprintf(os.Stderr, "parse abi: %v\n", err)
		os.Exit(1)
	}

	gen := NewTSGen(&abi)
	output := gen.Generate()

	if err := os.WriteFile(outPath, []byte(output), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write output: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Generated", outPath)
}
