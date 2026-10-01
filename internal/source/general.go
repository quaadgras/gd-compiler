package source

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"
	"unicode"

	"runtime.link/xyz"
)

// File within a Go [Package].
type File struct {
	Location

	MinimumGoVersion string

	Documentation  xyz.Maybe[CommentGroup]
	PackageKeyword Location
	PackageName    ImportedPackage
	Imports        []Import
	Definitions    []Definition // functions, variables and types.
	Unresolved     []Identifier
	Comments       []CommentGroup
}

type EscapeInformation struct {
	Block       func() EscapeFeasibility // true if the variable escapes the block it was defined in.
	Function    func() EscapeFeasibility // true if the variable escapes the function it was defined in.
	Goroutine   func() EscapeFeasibility // true if the variable escapes the goroutine it was defined in.
	Containment func() EscapeFeasibility // true if the variable escapes into the global scope.
}

type EscapeFeasibility struct {
	Possible bool
	Together []Identifier // escape is only-possible when at least one of these identifiers can escape.
	WithBits []Expression // escape is only-possible when these escape bits resolve to one (func type).
}

type Import struct {
	Location

	Rename  xyz.Maybe[ImportedPackage]
	Path    Literal
	Comment xyz.Maybe[CommentGroup]
	End     Location
}

// Location within a set of files.
type Location struct {
	FileSet *token.FileSet
	Node    ast.Node
	Open    token.Pos
	Shut    token.Pos
}

func (loc Location) sources() Location { return loc }

func (loc Location) String() string {
	return loc.FileSet.Position(loc.Open).String()
}

type Node interface {
	sources() Location
}

type WithLocation[T any] struct {
	Value          T
	SourceLocation Location
}

func LocationOf(node Node) Location {
	if node == nil {
		return Location{}
	}
	return node.sources()
}

type Bad Location

type Parenthesized struct {
	Typed

	Location

	Opening Location
	X       Expression
	Closing Location
}

type Selection struct {
	Typed
	Location
	X         Expression
	Selection Expression

	Path []string
}

type Star struct {
	Typed
	Location
	WithLocation[Expression]
}

type Comment struct {
	Location

	Slash Location
	Text  string
}

type CommentGroup struct {
	Location
	List []Comment
}

type Field struct {
	Location

	Documentation xyz.Maybe[CommentGroup]
	Names         xyz.Maybe[[]DefinedVariable]
	Type          Type
	Tag           xyz.Maybe[Literal]
	Comment       xyz.Maybe[CommentGroup]
}

type FieldList struct {
	Location

	Opening Location
	Fields  []Field
	Closing Location
}

type Unique interface {
	types.Object
}

type Identifier struct {
	Typed
	Location

	Unique Unique

	String string

	Method bool // identifier is a method

	Shadow int // number of shadowed identifiers

	Mutable  bool              // mutability analysis result
	Escapes  EscapeInformation // escape analysis result
	IsGlobal bool              // identifier is global to the package and not defined within a sub-scope.

	Package   string // package where the identifier is defined.
	IsPackage bool
}

type Package struct {
	types.Info

	Name    string   // Go package name.
	Path    string   // import path.
	Ident   string   // C identifier, see [PackageIdent].
	Imports []string // import paths.
	Test    bool
	Files   []File

	// Embeds are the files (slash-separated paths, relative to Dir) of the variables with
	// //go:embed directives, sorted as package embed sorts them, see [parser.Load].
	Embeds map[*types.Var][]string
	Dir    string

	FileSet *token.FileSet
}

// PackageIdent returns the identifier of a package for C names (unique, unlike package
// names): its import path, with other characters than letters and digits replaced by _.
func PackageIdent(pkg *types.Package) string {
	if pkg == nil {
		return ""
	}
	if pkg.Name() == "main" {
		return "main"
	}
	var b strings.Builder
	for _, r := range pkg.Path() {
		if r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
			b.WriteRune(r)
		} else {
			b.WriteByte('_')
		}
	}
	return b.String()
}

func (location Location) Errorf(format string, args ...interface{}) error {
	return fmt.Errorf(location.String()+": "+format, args...)
}

type DataComposite struct {
	Typed

	Location

	Type       xyz.Maybe[Type]
	OpenBrace  Location
	Elements   []Expression
	CloseBrace Location
	Incomplete bool
}

type Literal struct {
	Typed

	Location

	WithLocation[string]
	Kind token.Token
}

// reserved are C keywords, and names defined by C's standard headers (or go.h) as macros,
// which Go identifiers may be.
var reserved = map[string]bool{
	"auto": true, "break": true, "case": true, "char": true, "const": true, "continue": true,
	"default": true, "do": true, "double": true, "else": true, "enum": true, "extern": true,
	"float": true, "for": true, "goto": true, "if": true, "inline": true, "int": true, "long": true,
	"register": true, "restrict": true, "return": true, "short": true, "signed": true,
	"sizeof": true, "static": true, "struct": true, "switch": true, "typedef": true, "union": true,
	"unsigned": true, "void": true, "volatile": true, "while": true,
	"true": true, "false": true, "nil": true, "NULL": true, "bool": true, "errno": true,
	"stdin": true, "stdout": true, "stderr": true, "EOF": true, "assert": true, "offsetof": true,
	"signbit": true, "isnan": true, "isinf": true, "isfinite": true, "fpclassify": true,
	"alignas": true, "alignof": true, "noreturn": true, "complex": true, "imaginary": true,
	"static_assert": true, "thread_local": true, "setjmp": true, "longjmp": true, "jmp_buf": true,
	"va_start": true, "va_arg": true, "va_end": true, "va_copy": true, "va_list": true,
	"INFINITY": true, "NAN": true, "HUGE_VAL": true,
}

// CIdent returns the C name for a Go identifier: those that are C keywords or macros (or
// that could clash with gd's runtime, which starts with go_) are suffixed with _.
func CIdent(name string) string {
	if reserved[name] || strings.HasPrefix(name, "go_") || strings.HasPrefix(name, "_") && name != "_" {
		return name + "_"
	}
	return name
}
