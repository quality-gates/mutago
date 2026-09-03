package astutil

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"strings"
	"testing"
)

func TestCreateNoopOfStatementsPreservesASTInvariants(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		comments []string
	}{
		{
			name: "local declarations and field names",
			source: `package example

type record struct {
	value int
}

var sink int

func mutate(enabled bool, outer int) {
	if enabled {
		// keep this branch comment
		local := record{
			// keep this field comment
			value: outer,
		}
		sink = local.value
	}
}
`,
			comments: []string{"// keep this branch comment", "// keep this field comment"},
		},
		{
			name: "multiline selector call",
			source: `package example

import "fmt"

func mutate(enabled bool, values []int) {
	if enabled {
		// keep this call comment
		fmt.Printf(
			// keep this argument comment
			"%d",
			values[0],
		)
	}
}
`,
			comments: []string{"// keep this call comment", "// keep this argument comment"},
		},
		{
			name: "nested selector and blank line",
			source: `package example

import "time"

var sink int

func mutate(enabled bool) {
	if enabled {
		// keep this nested selector comment
		value := time.Now().
			Nanosecond()

		// keep this block boundary comment
		sink = value
	}
}
`,
			comments: []string{"// keep this nested selector comment", "// keep this block boundary comment"},
		},
		{
			name: "type assertion with interface selector",
			source: `package example

import "io"

var sink any

func dummy() {
	io.WriteString(nil, "")
}

func mutate(enabled bool, r any) {
	if enabled {
		// keep this comment
		v := r.(io.Reader)
		sink = v
	}
}
`,
			comments: []string{"// keep this comment"},
		},
		{
			name: "type conversion with basic named type selector",
			source: `package example

import "time"

var sink any

func dummy() {
	time.Sleep(0)
}

func mutate(enabled bool, n int64) {
	if enabled {
		// keep this comment
		d := time.Duration(n)
		sink = d
	}
}
`,
			comments: []string{"// keep this comment"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset, file, pkg, info := parseAndTypeCheck(t, tt.source)
			original := printFile(t, fset, file)
			ifStmt := firstIfStatement(t, file)
			old := ifStmt.Body.List

			noop := CreateNoopOfStatements(pkg, info, old)
			wantPosition := old[0].Pos()
			if got := noop.Pos(); got != wantPosition {
				t.Fatalf("noop position = %v, want %v", fset.Position(got), fset.Position(wantPosition))
			}
			ast.Inspect(noop, func(node ast.Node) bool {
				if node != nil && node.Pos() != wantPosition {
					t.Errorf("%T position = %v, want %v", node, fset.Position(node.Pos()), fset.Position(wantPosition))
				}
				return true
			})

			ifStmt.Body.List = []ast.Stmt{noop}
			mutated := printFile(t, fset, file)
			parseAndTypeCheck(t, mutated)

			for _, comment := range tt.comments {
				if count := strings.Count(mutated, comment); count != 1 {
					t.Errorf("comment %q occurs %d times in mutated source:\n%s", comment, count, mutated)
				}
			}
			if comment, assignment := strings.Index(mutated, tt.comments[0]), strings.Index(mutated, "_ ="); comment < 0 || assignment < 0 || comment > assignment {
				t.Errorf("leading comment moved below noop assignment:\n%s", mutated)
			}

			ifStmt.Body.List = old
			if reset := printFile(t, fset, file); reset != original {
				t.Errorf("reset did not restore original source\noriginal:\n%s\nreset:\n%s", original, reset)
			}
		})
	}
}

func parseAndTypeCheck(t *testing.T, source string) (*token.FileSet, *ast.File, *types.Package, *types.Info) {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse source: %v\n%s", err, source)
	}

	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("example", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatalf("type-check source: %v\n%s", err, source)
	}

	return fset, file, pkg, info
}

func printFile(t *testing.T, fset *token.FileSet, file *ast.File) string {
	t.Helper()

	var output bytes.Buffer
	if err := printer.Fprint(&output, fset, file); err != nil {
		t.Fatalf("print source: %v", err)
	}

	return output.String()
}

func firstIfStatement(t *testing.T, file *ast.File) *ast.IfStmt {
	t.Helper()

	var found *ast.IfStmt
	ast.Inspect(file, func(node ast.Node) bool {
		if found != nil {
			return false
		}
		ifStmt, ok := node.(*ast.IfStmt)
		if ok {
			found = ifStmt
			return false
		}
		return true
	})
	if found == nil {
		t.Fatal("source contains no if statement")
	}

	return found
}
