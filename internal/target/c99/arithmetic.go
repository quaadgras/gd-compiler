package c99

import (
	"fmt"
	"go/token"
	"go/types"
	"io"
)

// Go's integer arithmetic wraps around, and shifts by any amount, where C's arithmetic on
// signed integers (and on the small unsigned integers that C promotes to int) is undefined
// when it overflows, as are shifts by the width of the type (or more).

// wrapType returns the C unsigned type in which the arithmetic of the integer type t wraps
// as it does in Go, or "" when C's arithmetic on t already does.
func wrapType(t types.Type) string {
	t = types.Default(t)
	basic, ok := t.Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsInteger == 0 {
		return ""
	}
	unsigned := basic.Info()&types.IsUnsigned != 0
	switch sizes.Sizeof(t) {
	case 8:
		switch {
		case unsigned:
			return ""
		case basic.Kind() == types.Int:
			return "go_uu"
		}
		return "go_u8"
	}
	return "go_u4" // (1, 2 and 4 byte integers are promoted to int)
}

// isIntegerOp reports whether [Target.integerOp] writes op on values of type t.
func isIntegerOp(op token.Token, t types.Type) bool {
	switch op {
	case token.ADD, token.SUB, token.MUL:
		return wrapType(t) != ""
	case token.SHL, token.SHR:
		basic, ok := types.Default(t).Underlying().(*types.Basic)
		return ok && basic.Info()&types.IsInteger != 0
	}
	return false
}

// integerOp returns a C expression for x op y (C expressions, of the integer type t, or of
// any integer type for the count of shifts), for the arithmetic operators that may overflow,
// and shifts, or false for other operators and types.
func (c99 Target) integerOp(op token.Token, x, y string, t types.Type, count types.Type, constCount bool) (string, bool) {
	t = types.Default(t)
	basic, ok := t.Underlying().(*types.Basic)
	if !ok || basic.Info()&types.IsInteger == 0 {
		return "", false
	}
	ctype := c99.TypeOf(t)
	switch op {
	case token.ADD, token.SUB, token.MUL:
		ut := wrapType(t)
		if ut == "" {
			return "", false
		}
		return fmt.Sprintf("((%s)((%s)(%s) %s (%s)(%s)))", ctype, ut, x, op, ut, y), true
	case token.SHL, token.SHR:
		n := fmt.Sprintf("(go_u8)(%s)", y)
		if b, ok := count.Underlying().(*types.Basic); ok && b.Info()&types.IsUnsigned == 0 && !constCount {
			n = fmt.Sprintf("go_shift_count((go_i8)(%s))", y)
		}
		name := "shl"
		if op == token.SHR {
			name = "shr"
		}
		symbol := "go_" + name + "_" + identifier.ReplaceAllString(ctype, "_")
		c99.Requires(symbol, c99.Generic, func(w io.Writer) error {
			bits := sizes.Sizeof(t) * 8
			ut := wrapType(t)
			if ut == "" {
				ut = ctype
			}
			switch {
			case op == token.SHL:
				fmt.Fprintf(w, "static inline %s %s(%[1]s x, go_u8 n) { return n >= %[3]d ? 0 : (%[1]s)((%[4]s)x << n); }\n", ctype, symbol, bits, ut)
			case basic.Info()&types.IsUnsigned != 0:
				fmt.Fprintf(w, "static inline %s %s(%[1]s x, go_u8 n) { return n >= %[3]d ? 0 : x >> n; }\n", ctype, symbol, bits)
			default: // (C's >> of negative integers is implementation-defined, but arithmetic)
				fmt.Fprintf(w, "static inline %s %s(%[1]s x, go_u8 n) { return n >= %[3]d ? (x < 0 ? -1 : 0) : x >> n; }\n", ctype, symbol, bits)
			}
			return nil
		})
		return fmt.Sprintf("%s(%s, %s)", symbol, x, n), true
	}
	return "", false
}

// sizes are those of the Go types that the type checker assumes.
var sizes = types.SizesFor("gc", "amd64")
