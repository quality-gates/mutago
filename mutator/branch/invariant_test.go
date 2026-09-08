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

func TestMutatorIfWithEmptyInfoStillMutates(t *testing.T) {
	node := &ast.IfStmt{
		Body: &ast.BlockStmt{List: []ast.Stmt{&ast.ExprStmt{X: ast.NewIdent("x")}}},
	}
	got := MutatorIf(nil, &types.Info{}, node)
	if len(got) != 1 {
		t.Fatalf("got %d mutations, want 1", len(got))
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

func TestBranchMutatorsTerminatingBranchesCompile(t *testing.T) {
	tests := []struct {
		name    string
		mutator mutator.Mutator
		source  string
	}{
		{
			name:    "if",
			mutator: MutatorIf,
			source: `package example

func IfElse(c bool) int {
	if c {
		return 1
	} else {
		return 2
	}
}
`,
		},
		{
			name:    "else",
			mutator: MutatorElse,
			source: `package example

func IfElse(c bool) int {
	if c {
		return 1
	} else {
		return 2
	}
}
`,
		},
		{
			name:    "case",
			mutator: MutatorCase,
			source: `package example

func Switch(n int) string {
	switch n {
	case 1:
		return "one"
	default:
		return "other"
	}
}
`,
		},
		{
			name:    "if named results",
			mutator: MutatorIf,
			source: `package example

func Named(c bool) (n int) {
	if c {
		return 1
	} else {
		return 2
	}
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset, file, pkg, info := parseBranchSource(t, tt.source)
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
				mutation.Reset()
				if err := typeCheckSource(t, mutated); err != nil {
					t.Errorf("mutation %d does not compile:\n%s\n%v", i, mutated, err)
				}
			}
		})
	}
}

func TestBranchMutatorsLeaveNonTerminatingAndNoResultBodiesUnchanged(t *testing.T) {
	tests := []struct {
		name    string
		mutator mutator.Mutator
		source  string
	}{
		{
			name:    "if without results",
			mutator: MutatorIf,
			source: `package example

func NoResult(c bool) {
	if c {
		sink = 1
	} else {
		sink = 2
	}
}

var sink int
`,
		},
		{
			name:    "else without results",
			mutator: MutatorElse,
			source: `package example

func NoResult(c bool) {
	if c {
		sink = 1
	} else {
		sink = 2
	}
}

var sink int
`,
		},
		{
			name:    "case without results",
			mutator: MutatorCase,
			source: `package example

func NoResult(n int) {
	switch n {
	case 1:
		sink = 1
	default:
		sink = 2
	}
}

var sink int
`,
		},
		{
			name:    "if non-terminating",
			mutator: MutatorIf,
			source: `package example

func NonTerm(c bool) int {
	if c {
		sink = 1
	}
	return 2
}

var sink int
`,
		},
		{
			name:    "else non-terminating",
			mutator: MutatorElse,
			source: `package example

func NonTerm(c bool) int {
	if c {
		sink = 1
	} else {
		sink = 2
	}
	return 3
}

var sink int
`,
		},
		{
			name:    "case non-terminating",
			mutator: MutatorCase,
			source: `package example

func NonTerm(n int) int {
	switch n {
	case 1:
		sink = 1
	default:
		sink = 2
	}
	return 3
}

var sink int
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset, file, pkg, info := parseBranchSource(t, tt.source)
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
				mutation.Reset()
				if strings.Count(mutated, "return") != strings.Count(tt.source, "return") {
					t.Errorf("mutation %d added or removed a return:\n%s", i, mutated)
				}
				if err := typeCheckSource(t, mutated); err != nil {
					t.Errorf("mutation %d does not compile:\n%s\n%v", i, mutated, err)
				}
			}
		})
	}
}

func typeCheckSource(t *testing.T, source string) error {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "example.go", source, 0)
	if err != nil {
		return err
	}
	_, err = (&types.Config{Importer: importer.Default()}).Check("example", fset, []*ast.File{file}, nil)
	return err
}

func BenchmarkNestedIfMutationAnalysis(b *testing.B) {
	var statement ast.Stmt = &ast.ExprStmt{X: ast.NewIdent("external")}
	for range 500 {
		statement = &ast.IfStmt{Cond: ast.NewIdent("condition"), Body: &ast.BlockStmt{List: []ast.Stmt{statement}}}
	}
	info := &types.Info{Defs: make(map[*ast.Ident]types.Object), Uses: make(map[*ast.Ident]types.Object)}
	var mutations []mutator.Mutation
	ast.Inspect(statement, func(node ast.Node) bool {
		mutations = append(mutations, MutatorIf(nil, info, node)...)
		return true
	})
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		for _, mutation := range mutations {
			mutation.Change()
			mutation.Reset()
		}
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
