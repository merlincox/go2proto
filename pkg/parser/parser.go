package parser

import (
	"errors"
	"fmt"
	"go/token"
	"os"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/merlincox/go2proto/pkg/protobuf"
)

// GetMessages returns a filtered and sorted slice of protobuf message representations for the input paths and filter, or an error
func GetMessages(inputPaths []string, filter string) ([]*protobuf.Message, error) {

	pkgs, err := loadPackages(inputPaths)

	if err != nil {
		return nil, err
	}

	var msgs []*protobuf.Message

	for _, msg := range protobuf.NewMessageMap(pkgs).Messages() {
		if filter == "" || strings.Contains(msg.TypeName, filter) {
			msgs = append(msgs, msg)
		}
	}

	sort.Slice(msgs, func(i, j int) bool { return msgs[i].TypeName < msgs[j].TypeName })

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
