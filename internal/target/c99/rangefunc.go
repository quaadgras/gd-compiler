package c99

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io"
	"strings"

	"github.com/quaadgras/gd-compiler/internal/source"
	"runtime.link/xyz"
)

// Yield is the context of the body of a range over a function, which is compiled as the
// yield function that the function calls for each iteration: continue returns true, break
// returns false, and return stores the results (in the function with the range statement),
// then returns false, and the range statement returns them after the function returns. The
// state (a C expression) is set by return (to 1), and by break and continue to the labels of
// statements outside of the body (to their codes, see [Target.jumpCode]), for the range
// statement to do after the function returns.
type Yield struct {
	label   string // of the range statement.
	state   string
	results string
}

// jumpCode returns the code of the jump (break, continue or goto) to label, for the state
// of [Yield].
func (c99 Target) jumpCode(label string, jump token.Token) int {
	key := label + "/" + jump.String()
	code, ok := c99.Closures.jumps[key]
	if !ok {
		code = 2 + len(c99.Closures.jumps)
		c99.Closures.jumps[key] = code
	}
	return code
}

// innerLabels returns the labels of the statements of body (outside of function literals).
func innerLabels(body *ast.BlockStmt) map[string]bool {
	inner := make(map[string]bool)
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.LabeledStmt:
			inner[n.Label.Name] = true
		}
		return true
	})
	return inner
}

// outerJumps returns the break, continue and goto statements of body (outside of function
// literals) to statements outside of it, once per label and kind.
func outerJumps(body *ast.BlockStmt) []*ast.BranchStmt {
	inner := innerLabels(body)
	var jumps []*ast.BranchStmt
	seen := make(map[string]bool)
	ast.Inspect(body, func(node ast.Node) bool {
		switch n := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.BranchStmt:
			if n.Label == nil || inner[n.Label.Name] || seen[n.Tok.String()+n.Label.Name] {
				return true
			}
			seen[n.Tok.String()+n.Label.Name] = true
			jumps = append(jumps, n)
		}
		return true
	})
	return jumps
}

// rangeFunc writes a range over a function (of type sig).
func (c99 Target) rangeFunc(stmt source.StatementRange, sig *types.Signature) error {
	if sig.Params().Len() != 1 {
		return stmt.Errorf("unsupported range over %s", stmt.X.TypeAndValue().Type)
	}
	ysig, ok := sig.Params().At(0).Type().Underlying().(*types.Signature)
	if !ok {
		return stmt.Errorf("unsupported range over %s", stmt.X.TypeAndValue().Type)
	}
	// Calls deferred in the body are deferred by the function with the range statement.
	defers := hasDefer(stmt.Location.Node.(*ast.RangeStmt).Body)
	frame := c99.DeferFrame
	if frame == "" {
		frame = "go_fr"
	}
	n := c99.Closures.count
	c99.Closures.count++
	symbol := fmt.Sprintf("go_yield_%s_%d", c99.CurrentFunction, n)
	state, results := fmt.Sprintf("go_ystate_%d", n), fmt.Sprintf("go_yresults_%d", n)
	captures := c99.Closures.rangeCaptures[stmt.Location.Node.(*ast.RangeStmt)]
	result := c99.TupleOf(c99.Results)
	if err := c99.Requires(symbol, c99.Prelude, func(w io.Writer) error {
		fmt.Fprintf(w, "typedef struct { ")
		for _, v := range captures {
			fmt.Fprintf(w, "%s* %s; ", c99.TypeOf(subst(v.Type())), source.CIdent(v.Name()))
		}
		fmt.Fprintf(w, "go_ii* go_state; ")
		if defers {
			fmt.Fprintf(w, "go_frame* go_fr; ")
		}
		if result != "void" {
			fmt.Fprintf(w, "%s* go_results; ", result)
		}
		fmt.Fprintf(w, "} go_env_%s;\n", symbol)
		var params []string
		for i := range ysig.Params().Len() {
			params = append(params, fmt.Sprintf("%s go_y%d", c99.TypeOf(ysig.Params().At(i).Type()), i))
		}
		fmt.Fprintf(w, "static go_tf %s(%s) {\n\tgo_split();", symbol, strings.Join(append([]string{"void* go_env"}, params...), ", "))
		for _, v := range captures {
			fmt.Fprintf(w, "\n\t%s* %s = ((go_env_%s*)go_env)->%[2]s;", c99.TypeOf(subst(v.Type())), source.CIdent(v.Name()), symbol)
		}
		cc := c99
		cc.Writer = w
		cc.Tabs = 1
		cc.Environment = captures
		cc.Order, cc.Deferred, cc.BreakLabel, cc.Header, cc.Frame = nil, nil, "", false, false
		cc.Yield = &Yield{label: stmt.Label, state: fmt.Sprintf("(*((go_env_%s*)go_env)->go_state)", symbol)}
		if result != "void" {
			cc.Yield.results = fmt.Sprintf("(*((go_env_%s*)go_env)->go_results)", symbol)
		}
		cc.YieldLoop = true
		cc.DeferFrame = ""
		if defers {
			cc.DeferFrame = fmt.Sprintf("((go_env_%s*)go_env)->go_fr", symbol)
		}
		cc.Labels = innerLabels(stmt.Location.Node.(*ast.RangeStmt).Body)
		if key, ok := stmt.Key.Get(); ok && key.String != "_" && ysig.Params().Len() > 0 {
			fmt.Fprintf(w, "\n\t%s", cc.bind(key, ysig.Params().At(0).Type(), "go_y0"))
		}
		if val, ok := stmt.Value.Get(); ok && val.String != "_" && ysig.Params().Len() > 1 {
			fmt.Fprintf(w, "\n\t%s", cc.bind(val, ysig.Params().At(1).Type(), "go_y1"))
		}
		fmt.Fprintf(w, "\n\t{") // (a scope of its own, which may redeclare the range's variables)
		for _, s := range stmt.Body.Statements {
			if err := cc.Statement(s); err != nil {
				return err
			}
		}
		fmt.Fprintf(w, "\n\t}\n\treturn true;\n}\n")
		return nil
	}); err != nil {
		return err
	}
	invoke, err := c99.InvokerOf(sig)
	if err != nil {
		return stmt.Errorf("%w", err)
	}
	var env []string
	for _, v := range captures {
		env = append(env, source.CIdent(v.Name()))
	}
	env = append(env, "&"+state)
	if defers {
		env = append(env, frame)
	}
	fmt.Fprintf(c99, "{ go_ii %s = 0; ", state)
	if result != "void" {
		fmt.Fprintf(c99, "%s %s; ", result, results)
		env = append(env, "&"+results)
	}
	fmt.Fprintf(c99, "%s(%s, go_make_closure(%s, go_new(sizeof(go_env_%[3]s), &(go_env_%[3]s){ %s }).ptr)); ",
		invoke, c99.toString(stmt.X), symbol, strings.Join(env, ", "))
	fmt.Fprintf(c99, "if (%s == 1) { ", state) // the body returned.
	ret := source.StatementReturn{Location: stmt.Location}
	for i, t := range c99.Results {
		value := results
		if len(c99.Results) > 1 {
			value = fmt.Sprintf("%s.r%d", results, i)
		}
		ret.Results = append(ret.Results, source.Expressions.DefinedVariable.New(source.DefinedVariable{
			Typed:    source.Typed{TV: types.TypeAndValue{Type: t}},
			Location: source.Location{Node: &ast.Ident{Name: value}},
			String:   value,
		}))
	}
	if err := c99.StatementReturn(ret); err != nil {
		return err
	}
	for _, jump := range outerJumps(stmt.Location.Node.(*ast.RangeStmt).Body) {
		label := jump.Label.Name
		if label == stmt.Label && jump.Tok != token.GOTO {
			continue
		}
		fmt.Fprintf(c99, "; } else if (%s == %d) { ", state, c99.jumpCode(label, jump.Tok))
		var err error
		switch jump.Tok {
		case token.BREAK:
			err = c99.StatementBreak(source.StatementBreak{Location: stmt.Location, Label: xyz.New(source.Identifier{String: label})})
		case token.CONTINUE:
			err = c99.StatementContinue(source.StatementContinue{Location: stmt.Location, Label: xyz.New(source.Identifier{String: label})})
		case token.GOTO:
			err = c99.StatementGoto(source.StatementGoto{Location: stmt.Location, Label: xyz.New(source.Identifier{String: label})})
		}
		if err != nil {
			return err
		}
	}
	fmt.Fprintf(c99, "; } }")
	return nil
}

// yieldReturn writes a return statement in the body of a range over a function, see [Yield].
func (c99 Target) yieldReturn(stmt source.StatementReturn) error {
	y := c99.Yield
	fmt.Fprintf(c99, "{ ")
	switch {
	case len(c99.Results) == 0:
	case len(stmt.Results) == 0: // named results.
		fmt.Fprintf(c99, "%s = ", y.results)
		if len(c99.Results) == 1 {
			fmt.Fprintf(c99, "%s; ", c99.ResultVars[0])
		} else {
			fmt.Fprintf(c99, "(%s){ %s }; ", c99.TupleOf(c99.Results), strings.Join(c99.ResultVars, ", "))
		}
	case len(stmt.Results) == 1 && len(c99.Results) > 1: // return f(), with several results.
		value, _, err := c99.tupleValue(stmt.Results[0], len(c99.Results))
		if err != nil {
			return stmt.Location.Errorf("%w", err)
		}
		fmt.Fprintf(c99, "%s = %s; ", y.results, value)
	case len(stmt.Results) == 1:
		fmt.Fprintf(c99, "%s = ", y.results)
		if err := c99.ExpressionAs(stmt.Results[0], c99.Results[0]); err != nil {
			return err
		}
		fmt.Fprintf(c99, "; ")
	case len(stmt.Results) > 1:
		fmt.Fprintf(c99, "%s = (%s){ ", y.results, c99.TupleOf(c99.Results))
		for i, result := range stmt.Results {
			if i > 0 {
				fmt.Fprintf(c99, ", ")
			}
			if err := c99.ExpressionAs(result, c99.Results[i]); err != nil {
				return err
			}
		}
		fmt.Fprintf(c99, " }; ")
	}
	fmt.Fprintf(c99, "%s = 1; return false; }", y.state)
	return nil
}
