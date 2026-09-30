package c99

import (
	"bytes"
	"fmt"
	"go/types"
	"io"
	"reflect"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
)

type Target struct {
	io.Writer

	Prelude io.Writer
	Exports io.Writer
	Private io.Writer
	Generic io.Writer

	Tabs int

	CurrentPackage  string
	CurrentFunction string

	Symbols map[string]struct{}

	// Results are the result types of the function being compiled.
	Results []types.Type

	// Closures of the package, and the environment (captured variables) of the closure
	// being compiled.
	Closures    *Closures
	Environment []*types.Var

	// Frame is true when the function being compiled has deferred calls (so go_fr is its
	// go_frame), and ResultVars are the C expressions for its result variables, when they
	// are named, or it has a frame.
	Frame      bool
	ResultVars []string

	// Deferred is set when compiling the call of a deferred function, with the C
	// expressions for the parts of the call that were evaluated by the defer statement.
	Deferred *DeferredCall

	// Order of evaluation of the statement being compiled, see [Order].
	Order *Order

	// Initializers of package-level variables, which are written to the package's init
	// function after all of its files are compiled, in the order given by the type
	// checker.
	Initializers *Initializers
}

// Initializers of package-level variables, see [Initializers.WriteTo].
type Initializers struct {
	order []types.Object
	code  map[types.Object]*bytes.Buffer
	funcs []string // init functions, in declaration order.

	// Prelude of the init function's file, for the helpers (and closures) that the
	// initializers require, see [Target.Requires].
	Prelude bytes.Buffer
	Symbols map[string]struct{}
}

// For returns the buffer for the initializer of the package-level variable v.
func (inits *Initializers) For(v types.Object) *bytes.Buffer {
	if inits.code == nil {
		inits.code = make(map[types.Object]*bytes.Buffer)
	}
	buf := new(bytes.Buffer)
	inits.order = append(inits.order, v)
	inits.code[v] = buf
	return buf
}

// Func returns the name to use for the next init function of the package (which may have
// many), without the package suffix.
func (inits *Initializers) Func(suffix string) string {
	name := fmt.Sprintf("go_init_%d", len(inits.funcs))
	inits.funcs = append(inits.funcs, name+suffix)
	return name
}

// WriteTo writes the initializers in the order that the spec requires: dependencies first,
// otherwise in declaration order (as computed by the type checker, see
// [types.Info.InitOrder]), followed by calls to the init functions.
func (inits *Initializers) WriteTo(w io.Writer, order []*types.Initializer) error {
	write := func(v types.Object) error {
		buf, ok := inits.code[v]
		if !ok {
			return nil
		}
		delete(inits.code, v)
		_, err := w.Write(buf.Bytes())
		return err
	}
	for _, init := range order {
		for _, v := range init.Lhs {
			if err := write(v); err != nil {
				return err
			}
		}
	}
	for _, v := range inits.order { // not in the type checker's order, keep declaration order.
		if err := write(v); err != nil {
			return err
		}
	}
	for _, fn := range inits.funcs {
		if _, err := fmt.Fprintf(w, "\n\t%s();", fn); err != nil {
			return err
		}
	}
	return nil
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
