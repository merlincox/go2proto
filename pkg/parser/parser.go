package parser

import (
	"errors"
	"fmt"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/merlincox/go2proto/pkg/protobuf"
)

// GetMessages returns a sorted slice of protobuf message representations for the input paths and filter, or an error
func GetMessages(inputPaths []string, filter string) ([]*protobuf.Message, error) {

	pkgs, err := loadPackages(inputPaths)

	if err != nil {
		return nil, err
	}

	var msgs []*protobuf.Message
	seen := map[string]struct{}{}
	for _, p := range pkgs {
		for _, t := range p.TypesInfo.Defs {
			if t == nil {
				continue
			}
			if !t.Exported() {
				continue
			}
			if _, ok := seen[t.Name()]; ok {
				continue
			}
			if s, ok := t.Type().Underlying().(*types.Struct); ok {
				seen[t.Name()] = struct{}{}
				if filter == "" || strings.Contains(t.Name(), filter) {
					msgs = appendMessage(msgs, t, s)
				}
			}
		}
	}
	sort.Slice(msgs, func(i, j int) bool { return msgs[i].Name < msgs[j].Name })
	return msgs, nil
}

// attempt to load all packages
func loadPackages(inputPaths []string) ([]*packages.Package, error) {

	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting working directory: %s", err)
	}

	fset := token.NewFileSet()
	cfg := &packages.Config{
		Dir:  pwd,
		Mode: packages.LoadSyntax,
		Fset: fset,
	}
	packages, err := packages.Load(cfg, inputPaths...)
	if err != nil {
		return nil, err
	}

	var errs []string
	//check each loaded package for errors during loading
	for _, p := range packages {
		if len(p.Errors) > 0 {
			var perrs []string
			for _, e := range p.Errors {
				perrs = append(perrs, e.Error())
			}
			err := fmt.Sprintf("package %s: %s", p.String(), strings.Join(perrs, ", "))
			errs = append(errs, err)
		}
	}
	if len(errs) != 0 {
		return nil, errors.New(strings.Join(errs, "; "))
	}
	return packages, nil
}

func appendMessage(out []*protobuf.Message, t types.Object, s *types.Struct) []*protobuf.Message {

	msg := &protobuf.Message{
		Name: t.Name(),
	}

	order := 0
	for i := 0; i < s.NumFields(); i++ {
		goField := s.Field(i)
		if !goField.Exported() {
			continue
		}
		order++
		msg.Fields = append(msg.Fields, protobuf.NewField(goField, order, s.Tag(i)))
	}
	out = append(out, msg)
	return out
}
