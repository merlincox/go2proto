package protobuf

import (
	"fmt"
	"go/types"
	"io"
	"strings"
	"text/template"
	"unicode"
	"unicode/utf8"
)

// Message represents a protobuf message
type Message struct {
	Name   string
	Fields []*Field
}

// Field represents a protobuf message field
type Field struct {
	goField *types.Var
	Order   int
	Tags    string
}

// Name returns the name of the message field
func (f Field) Name() string {
	name := f.goField.Name()
	if len(name) == 2 {
		return strings.ToLower(name)
	}
	r, n := utf8.DecodeRuneInString(name)
	return string(unicode.ToLower(r)) + name[n:]
}

// TypeName returns the protobuf type of the message field
func (f Field) TypeName() string {
	switch f.goField.Type().Underlying().(type) {
	case *types.Basic:
		goType := f.goField.Type().String()
		return protobufType(goType)
	case *types.Slice, *types.Pointer, *types.Struct:
		goType := strings.TrimPrefix(strings.TrimLeft(f.goField.Type().String(), "[]*"), f.goField.Pkg().Path()+".")
		return protobufType(goType)
	default:
		return f.goField.Type().String()
	}
}

// IsRepeated returns true if the field derives from a go slice
func (f Field) IsRepeated() bool {
	_, ok := f.goField.Type().Underlying().(*types.Slice)
	return ok
}

// NewField derives a Field from a types.Var with the supplied order and tags
func NewField(goField *types.Var, order int, tags string) *Field {

	return &Field{
		goField: goField,
		Order:   order,
		Tags:    tags,
	}
}

// WriteOutput writes out a slice of protobuf message representations in protobuf 3 file format
func WriteOutput(out io.Writer, msgs []*Message, useTags bool) error {

	protobufTemplate := `{{- define "field" }}{{.TypeName}} {{.Name}} = {{.Order}}{{if writeTags . }} [(tagger.tags) = "{{escapeQuotes .Tags}}"]{{ end }};{{ end -}}
syntax = "proto3";

package proto;
{{- if importTagger}}

import "tagger/tagger.proto";{{end}}
{{range .}}
message {{.Name}} {
{{- range .Fields}}
  {{ if .IsRepeated}}repeated {{ end }}{{ template "field" . }}
{{- end}}
}
{{end}}
`
	customFuncMap := template.FuncMap{
		"escapeQuotes": func(tags string) string {
			return strings.Replace(tags, `"`, `\"`, -1)
		},
		"writeTags": func(f Field) bool {
			return useTags && f.Tags != ""
		},
		"importTagger": func() bool {
			if !useTags {
				return false
			}
			for _, msg := range msgs {
				for _, field := range msg.Fields {
					if field.Tags != "" {
						return true
					}
				}
			}
			return false
		},
	}

	tmpl, err := template.New("protobuf").Funcs(customFuncMap).Parse(protobufTemplate)
	if err != nil {
		return fmt.Errorf("unable to parse template: %s", err)
	}

	return tmpl.Execute(out, msgs)
}

func protobufType(goType string) string {
	switch goType {
	case "int":
		return "int64"
	case "float32":
		return "float"
	case "float64":
		return "double"
	default:
		return goType
	}
}
