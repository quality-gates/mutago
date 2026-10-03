package filter

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNonNegativeDecrementFilter(t *testing.T) {
	testCases := []struct {
		name            string
		statement       string
		wantSkippedZero int
	}{
		{name: "index", statement: "_ = xs[0]", wantSkippedZero: 1},
		{name: "hexadecimal index", statement: "_ = xs[0x0]", wantSkippedZero: 1},
		{name: "slice low bound", statement: "_ = xs[0:]", wantSkippedZero: 1},
		{name: "slice high bound", statement: "_ = xs[:0]", wantSkippedZero: 1},
		{name: "slice max bound", statement: "_ = xs[:0:0]", wantSkippedZero: 2},
		{name: "array length", statement: "var a [0]int", wantSkippedZero: 1},
		{name: "left shift", statement: "_ = n << 0", wantSkippedZero: 1},
		{name: "right shift", statement: "_ = n >> 0", wantSkippedZero: 1},
		{name: "left shift assignment", statement: "n <<= 0", wantSkippedZero: 1},
		{name: "right shift assignment", statement: "n >>= 0", wantSkippedZero: 1},
		{name: "unrelated zero", statement: "_ = 0", wantSkippedZero: 0},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			fset := token.NewFileSet()
			source := "package main\nfunc f(xs []int, n uint) {\n" + tt.statement + "\n}\n"
			file, err := parser.ParseFile(fset, "test.go", source, 0)
			require.NoError(t, err)

			f := NewNonNegativeDecrementFilter()
			f.Collect(file, fset, "")

			var zeroLiterals []*ast.BasicLit
			ast.Inspect(file, func(node ast.Node) bool {
				lit, ok := node.(*ast.BasicLit)
				if ok && lit.Kind == token.INT && (lit.Value == "0" || lit.Value == "0x0") {
					zeroLiterals = append(zeroLiterals, lit)
				}
				return true
			})
			var skipped int
			for _, lit := range zeroLiterals {
				if f.ShouldSkip(lit, numbersDecrementer) {
					skipped++
				}
				assert.False(t, f.ShouldSkip(lit, "numbers/incrementer"))
			}
			assert.Equal(t, tt.wantSkippedZero, skipped)
		})
	}
}
