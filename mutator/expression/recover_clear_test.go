package expression

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorRecoverClearRegistered(t *testing.T) {
	if _, err := mutator.New("expression/recover-clear"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorRecoverClear(t *testing.T) {
	test.Mutator(
		t,
		MutatorRecoverClear,
		"../../testdata/expression/recover_clear.go",
		3,
	)
}

func TestMutatorRecoverClear_Compiles(t *testing.T) {
	tests := []struct {
		name   string
		source string
	}{
		{
			name: "defer recover",
			source: `package example

func safe() {
	defer recover()
}
`,
		},
		{
			name: "go recover",
			source: `package example

func concurrent() {
	go recover()
}
`,
		},
		{
			name: "guarded recover in defer closure",
			source: `package example

func guarded() {
	defer func() {
		if r := recover(); r != nil {
		}
	}()
}
`,
		},
		{
			name: "assignment in defer closure",
			source: `package example

func assign() {
	defer func() {
		_ = recover()
	}()
}
`,
		},
		{
			name: "bare statement",
			source: `package example

func bare() {
	recover()
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, "example.go", tt.source, 0)
			if err != nil {
				t.Fatalf("parse error: %v", err)
			}
			conf := &types.Config{Importer: importer.Default()}
			info := &types.Info{
				Types: make(map[ast.Expr]types.TypeAndValue),
			}
			pkg, err := conf.Check("example", fset, []*ast.File{file}, info)
			if err != nil {
				t.Fatalf("type check error: %v", err)
			}

			var mutations []mutator.Mutation
			ast.Inspect(file, func(n ast.Node) bool {
				mutations = append(mutations, MutatorRecoverClear(pkg, info, n)...)
				return true
			})

			if len(mutations) == 0 {
				t.Fatal("expected at least one mutation")
			}

			for i, m := range mutations {
				m.Change()

				var buf bytes.Buffer
				if err := printer.Fprint(&buf, fset, file); err != nil {
					t.Fatalf("mutation %d print error: %v", i, err)
				}
				mutatedSrc := buf.String()

				mutFset := token.NewFileSet()
				mutFile, err := parser.ParseFile(mutFset, "example.go", mutatedSrc, 0)
				if err != nil {
					t.Fatalf("mutation %d mutated source failed to parse: %v\n%s", i, err, mutatedSrc)
				}
				mutInfo := &types.Info{
					Types: make(map[ast.Expr]types.TypeAndValue),
				}
				if _, err := conf.Check("example", mutFset, []*ast.File{mutFile}, mutInfo); err != nil {
					t.Fatalf("mutation %d mutated source failed type check: %v\n%s", i, err, mutatedSrc)
				}

				m.Reset()
			}
		})
	}
}
