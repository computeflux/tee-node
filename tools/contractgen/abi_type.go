package main

import (
	"encoding/json"
)

type Abi struct {
	Contract Contract  `json:"contract"`
	Spec     Spec      `json:"spec"`
	Types    []AbiType `json:"types"`
	Version  int       `json:"version,omitempty"`
}

type Contract struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type Spec struct {
	Constructors []Message           `json:"constructors"`
	Docs         []any               `json:"docs"`
	LangError    TypeWithDisplayName `json:"lang_error"`
	Events       []SpecEvent         `json:"events"`
	Messages     []Message           `json:"messages"`
}

type SpecEvent struct {
	Args  []EventArg `json:"args"`
	Docs  []string   `json:"docs"`
	Label string     `json:"label"`
}
type EventArg struct {
	Docs    []string            `json:"docs"`
	Indexed bool                `json:"indexed"`
	Payable bool                `json:"payable"`
	Type    TypeWithDisplayName `json:"type"`
}

type TypeWithDisplayName struct {
	DisplayName []string `json:"displayName"`
	Type        int      `json:"type"`
}

type Message struct {
	Args       []MessageArg        `json:"args"`
	Docs       []string            `json:"docs"`
	Label      string              `json:"label"`
	Mutates    bool                `json:"mutates"`
	Payable    bool                `json:"payable"`
	ReturnType TypeWithDisplayName `json:"returnType"`
	Selector   string              `json:"selector"`
}

type MessageArg struct {
	Label string              `json:"label"`
	Type  TypeWithDisplayName `json:"type"`
}

type AbiType struct {
	Id   int        `json:"id"`
	Type AbiSubType `json:"type"`
}

type AbiSubType struct {
	Path []string `json:"path,omitempty"`
	Def  Def      `json:"def"`
	Docs []string `json:"docs,omitempty"`
}

type Def struct {
	Composite   *DefComposite   `json:"composite,omitempty"` // 复合类型 struct
	Variant     *DefVariant     `json:"variant,omitempty"`   // 枚举或变体 enum
	Sequence    *DefSequence    `json:"sequence,omitempty"`  // 序列，通常指 Vec 或 BoundedVec
	Array       *DefArray       `json:"array,omitempty"`     // 数组
	Tuple       *DefTuple       `json:"tuple,omitempty"`     // 元组
	Primitive   *DefPrimitive   `json:"primitive,omitempty"` // 原始类型 (Primitives)
	Compact     *DefCompact     `json:"compact,omitempty"`   // Compact<T> 节省存储空间和 gas 成本
	BitSequence *DefBitSequence `json:"bitSequence,omitempty"`
	Range       *DefRange       `json:"range,omitempty"` // 表示一个半开区间 [start, end)
}

type DefComposite struct {
	Fields []SubField `json:"fields"`
}

type DefRange struct {
	Start     int  `json:"start"`
	End       int  `json:"end"`
	Inclusive bool `json:"inclusive"`
}

type DefVariant struct {
	Variants []SubVariant `json:"variants"`
}

type SubVariant struct {
	Name   string     `json:"name"`
	Fields []SubField `json:"fields"`
	Index  int        `json:"index"`
	Docs   []string   `json:"docs,omitempty"`
}

type DefSequence struct {
	Type int `json:"type"`
}

type DefArray struct {
	Len  int `json:"len"`
	Type int `json:"type"`
}

type SubField struct {
	Name     string   `json:"name,omitempty"`
	Type     int      `json:"type"`
	TypeName string   `json:"typeName"`
	Docs     []string `json:"docs,omitempty"`
}

type DefTuple []int

type DefPrimitive string

type DefCompact struct {
	Type int `json:"type"`
}

type DefBitSequence struct {
	BitStoreType int `json:"bitStoreType"`
	BitOrderType int `json:"bitOrderType"`
}

func InitAbi(raw []byte) (*Abi, error) {
	var abi Abi
	err := json.Unmarshal(raw, &abi)
	if err != nil {
		return nil, err
	}
	return &abi, nil
}
