package c99

import (
	"fmt"
	"go/token"
	"math/big"
	"strconv"
	"strings"

	"github.com/quaadgras/go-compiler/internal/source"
)

func (c99 Target) Literal(lit source.Literal) error {
	if len(lit.Value) > 2 {
		if lit.Value[0] == '0' && lit.Value[1] == 'o' {
			lit.Value = "0" + strings.TrimPrefix(lit.Value[2:], "_")
		}
	}
	if (lit.Kind == token.IMAG || lit.Kind == token.FLOAT) && len(lit.Value) > 1 {
		lit.Value = strings.TrimSuffix(lit.Value, "i")
		if lit.Value == "0" {
			lit.Value = "0.0"
		}
		if lit.Value == "." {
			lit.Value = "0.0"
		}
		if lit.Value[0] == '.' {
			lit.Value = "0" + lit.Value
		}
		if lit.Value[len(lit.Value)-1] == '.' {
			lit.Value = lit.Value + "0"
		}
	}
	if lit.Kind == token.IMAG {
		fmt.Fprintf(c99, "go_complex128(0,%s)", lit.Value)
		return nil
	}
	if lit.Kind == token.CHAR {
		// we just convert runes into integer values.
		value, _, _, err := strconv.UnquoteChar(lit.Value[1:], '\'')
		if err != nil {
			return err
		}
		fmt.Fprintf(c99, "%d", value)
		return nil
	}
	if lit.Kind == token.STRING {
		// normalize string literals, as zig has a different format for
		// unicode escape sequences.
		val, err := strconv.Unquote(lit.Value)
		if err != nil {
			return err
		}
		fmt.Fprintf(c99, "go_string_new(%q)", val)
		return nil
	}
	if lit.Kind == token.INT {
		val, _ := new(big.Int).SetString(strings.ReplaceAll(lit.Value, "_", ""), 0)
		if !val.IsInt64() && !val.IsUint64() {
			fmt.Fprintf(c99, "%d", val.Int64())
			return nil
		}
	}
	_, err := c99.Write([]byte(strings.ReplaceAll(lit.Value, "_", "")))
	return err
}
