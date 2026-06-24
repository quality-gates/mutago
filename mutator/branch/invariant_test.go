package branch

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

func TestBranchMutationsPreserveASTInvariants(t *testing.T) {
	const source = `package example

import "time"

type record struct {
	value int
}

var sink int

func mutate(enabled bool, outer int) {
	if enabled {
		// keep outer if comment
		if outer > 0 {
			// keep nested if comment
			local := record{
				value: outer,
			}
			sink = local.value + time.Now().
				Nanosecond()
		}
	} else {
		// keep else comment
		sink = outer
	}

	switch {
	case enabled:
		// keep case comment
		local := record{value: outer}
		sink = local.value
	default:
		// keep default comment
		sink = time.Now().Nanosecond()
	}
}
`
	comments := []string{
		"// keep outer if comment",
		"// keep nested if comment",
		"// keep else comment",
		"// keep case comment",
		"// keep default comment",
	}
	tests := []struct {
		name    string
		mutator mutator.Mutator
	}{
		{name: "if", mutator: MutatorIf},
		{name: "else", mutator: MutatorElse},
		{name: "case", mutator: MutatorCase},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset, file, pkg, info := parseBranchSource(t, source)
			original := printBranchSource(t, fset, file)
			var mutations []mutator.Mutation
			ast.Inspect(file, func(node ast.Node) bool {
				mutations = append(mutations, tt.mutator(pkg, info, node)...)
				return true
			})
			if len(mutations) == 0 {
				t.Fatal("mutator produced no mutations")
			}

			for i, mutation := range mutations {
				mutation.Change()
				mutated := printBranchSource(t, fset, file)
				if mutated == original {
					t.Errorf("mutation %d did not change printed source", i)
				}
				parseBranchSource(t, mutated)
				for _, comment := range comments {
					if count := strings.Count(mutated, comment); count != 1 {
						t.Errorf("mutation %d: comment %q occurs %d times:\n%s", i, comment, count, mutated)
					}
				}

				mutation.Reset()
				if reset := printBranchSource(t, fset, file); reset != original {
					t.Errorf("mutation %d reset did not restore original source\noriginal:\n%s\nreset:\n%s", i, original, reset)
				}
			}
		})
	}
}

func TestBranchMutatorsSkipEmptyBodies(t *testing.T) {
	tests := []struct {
		name    string
		mutator mutator.Mutator
		node    ast.Node
	}{
		{
			name:    "if",
			mutator: MutatorIf,
			node:    &ast.IfStmt{Body: &ast.BlockStmt{}},
		},
		{
			name:    "else",
			mutator: MutatorElse,
			node: &ast.IfStmt{
				Body: &ast.BlockStmt{},
				Else: &ast.BlockStmt{},
			},
		},
		{
			name:    "case",
			mutator: MutatorCase,
			node:    &ast.CaseClause{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if mutations := tt.mutator(nil, &types.Info{}, tt.node); len(mutations) != 0 {
				t.Fatalf("empty body produced %d mutations", len(mutations))
			}
		})
	}
}

func parseBranchSource(t *testing.T, source string) (*token.FileSet, *ast.File, *types.Package, *types.Info) {
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

func printBranchSource(t *testing.T, fset *token.FileSet, file *ast.File) string {
	t.Helper()

	var output bytes.Buffer
	if err := printer.Fprint(&output, fset, file); err != nil {
		t.Fatalf("print source: %v", err)
	}

	return output.String()
}
