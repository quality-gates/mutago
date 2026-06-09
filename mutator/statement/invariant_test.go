package statement

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

	"github.com/quality-gates/mutago/v2/mutator"
)

func TestRemoveStatementPreservesASTInvariants(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		comments []string
	}{
		{
			name: "field key and multiline literal",
			source: `package example

type record struct {
	value int
}

var sink record

func mutate(outer int) {
	// keep assignment comment
	sink = record{
		// keep field comment
		value: outer,
	}
}
`,
			comments: []string{"// keep assignment comment", "// keep field comment"},
		},
		{
			name: "selector rooted in call",
			source: `package example

import "time"

func mutate(target *int) {
	// keep selector comment
	*target = time.Now().
		Nanosecond()
}
`,
			comments: []string{"// keep selector comment"},
		},
		{
			name: "multiline expression and blank line",
			source: `package example

func consume(int) {}

func mutate(outer int) {
	// keep expression comment
	consume(
		// keep argument comment
		outer,
	)

	// keep block boundary comment
	outer++
}
`,
			comments: []string{
				"// keep expression comment",
				"// keep argument comment",
				"// keep block boundary comment",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset, file, pkg, info := parseStatementSource(t, tt.source)
			original := printStatementSource(t, fset, file)
			var mutations []mutator.Mutation
			ast.Inspect(file, func(node ast.Node) bool {
				mutations = append(mutations, MutatorRemoveStatement(pkg, info, node)...)
				return true
			})
			if len(mutations) == 0 {
				t.Fatal("mutator produced no mutations")
			}

			for i, mutation := range mutations {
				mutation.Change()
				mutated := printStatementSource(t, fset, file)
				if mutated == original {
					t.Errorf("mutation %d did not change printed source", i)
				}
				parseStatementSource(t, mutated)
				for _, comment := range tt.comments {
					if count := strings.Count(mutated, comment); count != 1 {
						t.Errorf("mutation %d: comment %q occurs %d times:\n%s", i, comment, count, mutated)
					}
				}

				mutation.Reset()
				if reset := printStatementSource(t, fset, file); reset != original {
					t.Errorf("mutation %d reset did not restore original source\noriginal:\n%s\nreset:\n%s", i, original, reset)
				}
			}
		})
	}
}

func parseStatementSource(t *testing.T, source string) (*token.FileSet, *ast.File, *types.Package, *types.Info) {
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

func printStatementSource(t *testing.T, fset *token.FileSet, file *ast.File) string {
	t.Helper()

	var output bytes.Buffer
	if err := printer.Fprint(&output, fset, file); err != nil {
		t.Fatalf("print source: %v", err)
	}

	return output.String()
}
