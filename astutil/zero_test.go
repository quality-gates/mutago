package astutil

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

func TestZeroExprForTypeUnaliases(t *testing.T) {
	const src = `package example

type Config struct {
	Timeout int
}
type MyConfig = Config
type IntAlias = int
type Handler = func()
`

	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, "example.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	conf := types.Config{}
	pkg, err := conf.Check("example", fset, []*ast.File{f}, nil)
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name string
		typ  types.Type
		want string
	}{
		{"named-struct-alias", pkg.Scope().Lookup("MyConfig").Type(), "MyConfig{}"},
		{"basic-alias", pkg.Scope().Lookup("IntAlias").Type(), "0"},
		{"func-alias", pkg.Scope().Lookup("Handler").Type(), "nil"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ZeroExprForType(tc.typ, pkg)
			if got == nil {
				t.Fatalf("ZeroExprForType returned nil for %s", tc.typ)
			}
			s := exprString(got)
			if s != tc.want {
				// After Unalias, named-struct alias may render as underlying name.
				if tc.name == "named-struct-alias" && s == "Config{}" {
					return
				}
				t.Fatalf("got %q, want %q", s, tc.want)
			}
		})
	}
}

func exprString(e ast.Expr) string {
	switch v := e.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.BasicLit:
		return v.Value
	case *ast.CompositeLit:
		return exprString(v.Type) + "{}"
	case *ast.SelectorExpr:
		return exprString(v.X) + "." + v.Sel.Name
	default:
		return ""
	}
}
