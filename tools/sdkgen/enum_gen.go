package main

import (
	"bytes"
	"fmt"
	"html/template"

	"github.com/wetee-dao/ink.go/util"
)

// EnumBox for code generator
type EnumBox struct {
	Name  string
	Items []EnumItem
}

// EnumItem of enum
type EnumItem struct {
	Name       string
	Type       string //Inline Base Tuple Struct
	InlineName string
	Fields     []EnumItemField
	Index      int
}

// EnumItemField of enum
type EnumItemField struct {
	Name string
	Type string
}

// Enum generator
func (r *ReviveGen) EnumGen(ty int, name string, items []util.SubVariant, subs [][][]string) string {
	// 为Result和Option类型添加泛型
	if name == "Result" || name == "Option" {
		switch name {
		case "Result":
			return "[" + subs[0][0][1] + "," + subs[1][0][1] + "]"
		case "Option":
			return "[" + subs[1][0][1] + "]"
		}
		return ""
	}

	var traits = ""
	if r.CheckTypeIsSkip(ty, name) {
		return traits
	}

	typeStr := ("type " + name + " struct { // Enum" + "\n")
	tempItems := make([]EnumItem, 0, len(items))
	for i, v := range items {
		typeStr += ("  " + v.Name)
		if len(v.Fields) == 0 { // enum base type
			typeStr += (" *bool // " + fmt.Sprint(v.Index) + "\n")
			tempItems = append(tempItems, EnumItem{
				Name:   v.Name,
				Type:   "Base",
				Fields: []EnumItemField{},
				Index:  v.Index,
			})
			continue
		} else if len(v.Fields) == 1 && subs[i][0][2] == "Primitive" { // inline type
			typeStr += (" *" + subs[i][0][1] + " // " + fmt.Sprint(v.Index) + "\n")
			tempItems = append(tempItems, EnumItem{
				Name:       v.Name,
				Type:       "Inline",
				InlineName: subs[i][0][1],
				Fields:     []EnumItemField{},
				Index:      v.Index,
			})
		} else { // multiple fields
			typeStr += (" *struct{ // " + fmt.Sprint(v.Index) + "\n")
			itemType := "Struct"
			itemFields := make([]EnumItemField, 0, len(v.Fields))
			for j, subfield := range v.Fields {
				typeName := UnderscoreToCamelCase(subfield.Name)
				if subfield.Name == "" { // Tuple
					typeName = "F" + fmt.Sprint(j)
					itemType = "Tuple"
				}
				typeStr += ("    " + typeName + " " + subs[i][j][1] + "\n")
				itemFields = append(itemFields, EnumItemField{
					Name: typeName,
					Type: subs[i][j][1],
				})
			}
			typeStr += ("  }\n")
			tempItems = append(tempItems, EnumItem{
				Name:   v.Name,
				Type:   itemType,
				Fields: itemFields,
				Index:  v.Index,
			})
		}
	}
	typeStr += ("}\n")

	p := EnumBox{
		Name:  name,
		Items: tempItems,
	}
	t := template.Must(template.New("scale").Parse(enumScaleTemp))

	var result bytes.Buffer
	t.Execute(&result, p)

	r.registerType(ty, name, typeStr+result.String())

	return ""
}

var enumScaleTemp = `func (ty {{.Name}}) Encode(encoder scale.Encoder) (err error) {
{{- range $outerIndex, $outerItem := .Items -}}
	{{- if eq .Type "Base"}}
	if ty.{{.Name}} != nil {
		err = encoder.PushByte({{.Index}})
		if err != nil {
			return err
		}
		return nil
	}
	{{end}}
	{{- if eq .Type "Tuple"}}
	if ty.{{.Name}} != nil {
		err = encoder.PushByte({{.Index}})
		if err != nil {
			return err
		}
		{{range .Fields}}
		err = encoder.Encode(ty.{{$outerItem.Name}}.{{.Name}})
		if err != nil {
			return err
		}
		{{end}}
		return nil
	}
	{{end}}
	{{- if eq .Type "Struct"}}
	if ty.{{.Name}} != nil {
		err = encoder.PushByte({{.Index}})
		if err != nil {
			return err
		}
		{{range .Fields}}
		err = encoder.Encode(ty.{{$outerItem.Name}}.{{.Name}})
		if err != nil {
			return err
		}
		{{end}}
		return nil
	}
	{{end}}
	{{- if eq .Type "Inline"}}
	if ty.{{.Name}} != nil {
		err = encoder.PushByte({{.Index}})
		if err != nil {
			return err
		}
		err = encoder.Encode(*ty.{{.Name}})
		if err != nil {
			return err
		}
		return nil
	}
	{{end}}
{{- end -}}
	return fmt.Errorf("unrecognized enum")
}

func (ty *{{.Name}}) Decode(decoder scale.Decoder) (err error) {
	variant, err := decoder.ReadOneByte()
	if err != nil {
		return err
	}
	switch variant {
{{- range $outerIndex, $outerItem := .Items -}}
	{{- if eq .Type "Base" -}}
	case {{.Index}}: // Base
		t := true
		ty.{{.Name}} = &t
		return;
	{{- end -}}
	{{- if eq .Type "Tuple" -}}
	case {{.Index}}: // Tuple
		ty.{{.Name}} = &struct {
			{{range .Fields}}{{.Name}} {{.Type}}
			{{end}}
		}{}

		{{ range .Fields }}
		err = decoder.Decode(&ty.{{$outerItem.Name}}.{{.Name}})
		if err != nil {
			return err
		}
		{{ end }}
		return;
	{{- end -}}
	{{- if eq .Type "Struct" -}}
	case {{.Index}}: // Struct
		ty.{{.Name}} = &struct {
			{{range .Fields}}{{.Name}} {{.Type}}
			{{end}}
		}{}

		{{range .Fields}}
		err = decoder.Decode(&ty.{{$outerItem.Name}}.{{.Name}})
		if err != nil {
			return err
		}
		{{end}}
		return;
	{{- end -}}
	{{- if eq .Type "Inline" -}}
	case {{.Index}}: // Inline
		ty.{{.Name}} = new({{.InlineName}})
		err = decoder.Decode(ty.{{.Name}})
		if err != nil {
			return err
		}
		return;
	{{- end -}}
{{- end }}
	default:
		return fmt.Errorf("unrecognized enum")
	}
}
{{- if eq .Name "Error" }}
func (ty *{{.Name}}) Error() string {
	{{- range .Items }}
	if ty.{{.Name}} != nil {
		return "{{.Name}}"
	}
	{{ end -}}
	return "Unknown"
}
{{ end}}

`
