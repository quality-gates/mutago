package annotation

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func BenchmarkProcessorBroadRegex(b *testing.B) {
	for _, lines := range []int{100, 200, 400} {
		b.Run(fmt.Sprintf("lines=%d", lines), func(b *testing.B) {
			var source strings.Builder
			source.WriteString("package sample\n// mutator-disable-regexp .* numbers/incrementer\nfunc value() int {\n")
			for range lines {
				source.WriteString("_ = 1 + 1\n")
			}
			source.WriteString("return 1\n}\n")

			path := filepath.Join(b.TempDir(), "sample.go")
			if err := os.WriteFile(path, []byte(source.String()), 0o600); err != nil {
				b.Fatal(err)
			}

			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, source.String(), parser.ParseComments)
			if err != nil {
				b.Fatal(err)
			}

			b.ResetTimer()
			for range b.N {
				processor := NewProcessor()
				processor.Collect(file, fset, path)
				ast.Inspect(file, func(node ast.Node) bool {
					if node != nil {
						processor.ShouldSkip(node, "unmatched")
					}
					return true
				})
			}
		})
	}
}
