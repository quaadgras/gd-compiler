package c

import (
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
)

type Target struct {
	io.Writer

	Prelude io.Writer

	Tabs int

	CurrentPackage string

	Symbols map[string]struct{}
}

func (cc Target) Requires(symbol string, fn func()) {
	if _, ok := cc.Symbols[symbol]; !ok {
		cc.Symbols[symbol] = struct{}{}
		fn()
	}
}

func (cc Target) Compile(node source.Node) error {
	rtype := reflect.TypeOf(node)
	method := reflect.ValueOf(&cc).MethodByName(rtype.Name())
	if !method.IsValid() {
		return fmt.Errorf("unsupported node type: %s", rtype.Name())
	}
	err := method.Call([]reflect.Value{reflect.ValueOf(node)})
	if len(err) > 0 && !err[0].IsNil() {
		return err[0].Interface().(error)
	}
	return nil
}

func (cc Target) toString(node source.Node) string {
	var buf strings.Builder
	cc.Writer = &buf
	cc.Tabs = 0
	if err := cc.Compile(node); err != nil {
		panic(err)
	}
	return buf.String()
}

func (cc Target) Selection(sel source.Selection) error {
	if err := cc.Compile(sel.X); err != nil {
		return err
	}
	for _, elem := range sel.Path {
		fmt.Fprintf(cc, ".%s", elem)
	}
	fmt.Fprintf(cc, ".")
	return cc.Compile(sel.Selection)
}

func (cc Target) Star(star source.Star) error {
	fmt.Fprintf(cc, "*")
	if err := cc.Compile(star.Value); err != nil {
		return err
	}
	return nil
}

func (cc *Target) File(file source.File) error {
	fmt.Fprintln(cc.Prelude, `#include <go.h>`)
	fmt.Fprintln(cc.Prelude)
	for _, decl := range file.Definitions {
		if err := cc.Compile(decl); err != nil {
			return err
		}
	}
	return nil
}

func (cc Target) PackageOf(name string) string {
	if name == "testing" {
		return "go.testing"
	}
	if name == "math" {
		return "go.math"
	}
	return name
}

func (cc Target) Definition(decl source.Definition) error {
	node, _ := decl.Get()
	return cc.Compile(node)
}

func (cc Target) StatementDefinitions(defs source.StatementDefinitions) error {
	for _, def := range defs {
		if err := cc.Definition(def); err != nil {
			return err
		}
	}
	return nil
}
