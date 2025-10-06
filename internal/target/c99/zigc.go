package c99

import (
	"bytes"
	"fmt"
	"go/types"
	"io"
	"reflect"
	"strconv"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
)

type Target struct {
	io.Writer

	Init    io.Writer
	Prelude io.Writer
	Exports io.Writer
	Private io.Writer
	Generic io.Writer

	Tabs int

	CurrentPackage  string
	CurrentFunction string
	CurrentClosures int

	Symbols map[string]struct{}
}

func (c99 Target) Requires(symbol string, w io.Writer, fn func(w io.Writer) error) error {
	if _, ok := c99.Symbols[symbol]; !ok {
		c99.Symbols[symbol] = struct{}{}
		var buf bytes.Buffer
		if err := fn(&buf); err != nil {
			return err
		}
		fmt.Fprint(w, buf.String())
	}
	return nil
}

func (c99 Target) Compile(node source.Node) error {
	rtype := reflect.TypeOf(node)
	method := reflect.ValueOf(&c99).MethodByName(rtype.Name())
	if !method.IsValid() {
		return fmt.Errorf("unsupported node type: %s", rtype.Name())
	}
	err := method.Call([]reflect.Value{reflect.ValueOf(node)})
	if len(err) > 0 && !err[0].IsNil() {
		return err[0].Interface().(error)
	}
	return nil
}

func (c99 Target) toString(node source.Node) string {
	var buf strings.Builder
	c99.Writer = &buf
	c99.Tabs = 0
	if err := c99.Compile(node); err != nil {
		panic(err)
	}
	return buf.String()
}

func (c99 Target) Selection(sel source.Selection) error {
	if sel.X.TypeAndValue().Type != nil {
		if err := c99.Compile(sel.X); err != nil {
			return err
		}
		for _, elem := range sel.Path {
			fmt.Fprintf(c99, ".%s", elem)
		}
		fmt.Fprintf(c99, ".")
	}
	return c99.Compile(sel.Selection)
}

func (c99 Target) Star(star source.Star) error {
	fmt.Fprintf(c99, "go_pointer_get(")
	if err := c99.Compile(star.Value); err != nil {
		return err
	}
	fmt.Fprintf(c99, ", %s)", c99.TypeOf(star.Value.TypeAndValue().Type.(*types.Pointer).Elem()))
	return nil
}

func (c99 *Target) File(file source.File) error {
	fmt.Fprintln(c99.Prelude, `#include <go.h>`)
	fmt.Fprintf(c99.Prelude, `#include <go/%s.h>`, c99.CurrentPackage)
	fmt.Fprintln(c99.Prelude)
	fmt.Fprintf(c99.Prelude, `#include <go/%s/private.h>`, c99.CurrentPackage)
	fmt.Fprintln(c99.Prelude)
	for _, pkg := range file.Imports {
		path, _ := strconv.Unquote(pkg.Path.Value)
		fmt.Fprintf(c99.Prelude, `#include <go/%s.h>`, path)
		fmt.Fprintln(c99.Prelude)
	}
	fmt.Fprintln(c99.Prelude)
	for _, decl := range file.Definitions {
		if err := c99.Compile(decl); err != nil {
			return err
		}
	}
	return nil
}

func (c99 Target) PackageOf(name string) string {
	return name
}

func (c99 Target) Definition(decl source.Definition) error {
	node, _ := decl.Get()
	return c99.Compile(node)
}

func (c99 Target) StatementDefinitions(defs source.StatementDefinitions) error {
	for _, def := range defs {
		if err := c99.Definition(def); err != nil {
			return err
		}
	}
	return nil
}

func (c99 Target) ArrayStrippedTypeOf(typ types.Type) (types.Type, string) {
	var suffix string
	for {
		if arr, ok := typ.(*types.Array); ok {
			typ = arr.Elem()
			suffix += fmt.Sprintf("[%d]", arr.Len())
			continue
		}
		break
	}
	return typ, suffix
}
