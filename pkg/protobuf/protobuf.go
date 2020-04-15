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

// isAnonymous returns true if the object type was declared anonymously
func isAnonymous(object types.Object) bool {
	return strings.Contains(object.Type().String(), "struct{")
}

// baseTypeName returns the name of an object type shorn of package, pointer and slice prefixes
func baseTypeName(object types.Object) string {
	typeName := object.Type().String()
	if mapObject, ok := object.Type().(*types.Map); ok {
		typeName = mapObject.Elem().String()
	}
	return dePrefix(typeName, object)
}

func dePrefix(typeName string, object types.Object) string {
	return strings.TrimPrefix(strings.TrimLeft(typeName, "[]*"), object.Pkg().Path()+".")
}

// toStruct given a type object, return the underlying struct type it is based upon or nil.
func toStruct(object types.Object) *types.Struct {
	return recurseToStruct(object.Type().Underlying())
}

func recurseToStruct(objectType types.Type) *types.Struct {

	switch specificType := objectType.(type) {
	case *types.Struct:
		return specificType
	case *types.Pointer:
		return recurseToStruct(specificType.Elem())
	case *types.Slice:
		return recurseToStruct(specificType.Elem())
	case *types.Map:
		return recurseToStruct(specificType.Elem())
	case *types.Named:
		return recurseToStruct(specificType.Underlying())
	default:
		return nil
	}
}

// MessageMap stores a collection of Messages with type mapping to resolve relationships
type MessageMap map[string]*Message

// NewMessageMap creates a MessageMap from a slice of types.Infos
func NewMessageMap(infos []*types.Info) MessageMap {

	msgMap := make(map[string]*Message)

	for _, info := range infos {
		for _, object := range info.Defs {
			if object == nil {
				continue
			}
			if !object.Exported() {
				continue
			}
			if _, already := msgMap[baseTypeName(object)]; !already {
				msg := NewMessage(object)
				if msg != nil {
					msgMap[msg.TypeName] = msg
				}
			}
		}
	}

	return msgMap
}

// Messages derives a slice of Messages from the map, naming anonymous types as necessary
func (msgMap MessageMap) Messages() []*Message {

	var msgs []*Message

	for uType := range msgMap {
		msg := msgMap[uType]
		msg.linkToParent(msgMap)
	}

	for uType := range msgMap {
		msg := msgMap[uType]
		msg.resolveTypeName(msgMap)
	}

	for uType := range msgMap {

		msg := msgMap[uType]
		msg.resolveFieldTypes(msgMap)

		msgs = append(msgs, msg)
	}

	return msgs
}

// Message represents a protobuf message
type Message struct {
	TypeName string
	Fields   []*Field

	parent            *Message
	parentalFieldName string
}

// linkToParent links anonymous messages to the message in which they were declared
func (m *Message) linkToParent(msgMap MessageMap) {

	for i := range m.Fields {
		// in reverse order so that earlier field names are used for anonymous types
		f := m.Fields[len(m.Fields)-1-i]
		if f.isAnonymous {
			msg, ok := msgMap[f.nativeTypeName]
			if ok {
				msg.parent = m
				msg.parentalFieldName = f.nativeFieldName
			}
		}
	}
}

// resolveTypeName recursively resets the type name of anonymously defined messages to the name of the parent in which
// they were declared underscore-appended with the field name under which they were defined
func (m *Message) resolveTypeName(msgMap MessageMap) {

	if m.parent != nil {
		m.parent.resolveTypeName(msgMap)
		m.TypeName = m.parent.TypeName + "_" + m.parentalFieldName
	}
}

// resolveFieldTypes sets the type names of isAnonymous fields to the name of their message type
func (m *Message) resolveFieldTypes(msgMap MessageMap) {

	for i := range m.Fields {
		f := m.Fields[i]
		if f.isAnonymous {
			msg, ok := msgMap[f.nativeTypeName]
			if ok {
				f.TypeName = msg.TypeName
			}
		}
	}
}

// NewMessage attempts to generate a Message from a types.Object.
// If the object passed does not have an underlying struct type NewMessage will return nil.
func NewMessage(object types.Object) *Message {

	strct := toStruct(object)

	if strct == nil {
		return nil
	}

	msg := Message{
		TypeName: baseTypeName(object),
	}

	order := 0
	for i := 0; i < strct.NumFields(); i++ {
		goField := strct.Field(i)
		if !goField.Exported() {
			continue
		}
		order++
		msg.Fields = append(msg.Fields, NewField(goField, order, strct.Tag(i)))
	}

	return &msg
}

// Field represents a protobuf message field
type Field struct {
	nativeTypeName  string
	nativeFieldName string
	isAnonymous     bool

	TypeName   string
	FieldName  string
	IsRepeated bool
	IsMap      bool
	MapKey     string
	Order      int
	Tags       string
}

// NewField derives a Field from a object with the supplied order and tags
func NewField(object types.Object, order int, tags string) *Field {

	_, isRepeated := object.Type().Underlying().(*types.Slice)

	mp, isAMap := object.Type().Underlying().(*types.Map)

	fieldName := object.Name()

	r, n := utf8.DecodeRuneInString(fieldName)
	fieldName = string(unicode.ToLower(r)) + fieldName[n:]

	field := Field{

		nativeFieldName: object.Name(),
		isAnonymous:     isAnonymous(object),

		FieldName:  fieldName,
		IsRepeated: isRepeated,
		IsMap:      isAMap,
		Order:      order,
		Tags:       tags,
	}

	if isAMap {
		field.MapKey = protobufType(mp.Key().String())
		field.TypeName = dePrefix(mp.Elem().String(), object)
		field.nativeTypeName = field.TypeName
	} else {
		field.TypeName = protobufType(baseTypeName(object))
		field.nativeTypeName = baseTypeName(object)
	}

	return &field
}

func protobufType(goType string) string {
	switch goType {
	case "int":
		return "int64"
	case "float32":
		return "float"
	case "float64":
		return "double"
	case "interface{}":
		return "google.protobuf.Any"
	default:
		return goType
	}
}

// WriteOutput writes out a slice of protobuf message representations in protobuf 3 file format
func WriteOutput(out io.Writer, msgs []*Message, useTags bool) error {

	protobufTemplate := `{{- define "fieldType" }}{{ if .IsMap }}map<{{ .MapKey }}, {{ .TypeName }}>{{ else }}{{ .TypeName }}{{ end }}{{ end -}}
{{- define "field" }}{{ template "fieldType" .}} {{ .FieldName }} = {{.Order}}{{if writeTags . }} [(tagger.tags) = "{{escapeQuotes .Tags}}"]{{ end }};{{ end -}}
syntax = "proto3";

package proto;
{{- if importTagger}}

import "tagger/tagger.proto";{{end}}
{{- if importAny}}

import "google/protobuf/any.proto";{{end}}
{{range .}}
message {{.TypeName}} {
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
		"importAny": func() bool {
			for _, msg := range msgs {
				for _, field := range msg.Fields {
					if field.TypeName == "google.protobuf.Any" {
						return true
					}
				}
			}
			return false
		},
	}

	tmpl, err := template.New("protobuf").Funcs(customFuncMap).Parse(protobufTemplate)
	if err != nil {
		return fmt.Errorf("Unable to parse template: %s", err)
	}

	return tmpl.Execute(out, msgs)
}
