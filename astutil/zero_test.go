package astutil

import (
	"bytes"
	"go/ast"
	"go/printer"
	"go/token"
	"go/types"
	"testing"
)

func TestZeroExprForType_NamedStructControl(t *testing.T) {
	source := `package example

type Config struct {
	Timeout int
}

func Get() Config {
	return Config{Timeout: 1}
}
`
	assertZeroExpr(t, source, false, "Config{}")
}

func TestZeroExprForType_TypeAliasToInt(t *testing.T) {
	source := `package example

type ID = int

func Get() ID {
	return 1
}
`
	assertZeroExpr(t, source, true, "0")
}

func TestZeroExprForType_TypeAliasToNamedStruct(t *testing.T) {
	source := `package example

type Config struct {
	Timeout int
}
type MyConfig = Config

func Get() MyConfig {
	return MyConfig{Timeout: 1}
}
`
	assertZeroExpr(t, source, true, "Config{}")
}

func TestZeroExprForType_TypeAliasToString(t *testing.T) {
	source := `package example

type ID = string

func Get() ID {
	return "x"
}
`
	assertZeroExpr(t, source, true, `""`)
}

func TestZeroExprForType_TypeAliasToFunc(t *testing.T) {
	source := `package example

type Handler = func()

func Get(h Handler) Handler {
	return h
}
`
	assertZeroExpr(t, source, true, "nil")
}

func TestZeroReturnForSignature_TypeAliasToNamedStruct(t *testing.T) {
	source := `package example

type Config struct {
	Timeout int
}
type MyConfig = Config

func Get() MyConfig {
	return MyConfig{Timeout: 1}
}
`
	assertZeroReturn(t, source, "return Config{}")
}

func TestZeroReturnForSignature_TypeAliasToInt(t *testing.T) {
	source := `package example

type ID = int

func Get() ID {
	return 1
}
`
	assertZeroReturn(t, source, "return 0")
}

func assertZeroExpr(t *testing.T, source string, wantAlias bool, want string) {
	t.Helper()
	_, file, pkg, info := parseAndTypeCheck(t, source)
	ret := firstReturn(t, file)
	typ := info.TypeOf(ret.Results[0])
	if typ == nil {
		t.Fatal("no type for return expression")
	}
	_, isAlias := typ.(*types.Alias)
	if isAlias != wantAlias {
		t.Fatalf("alias=%v, want %v (type %T)", isAlias, wantAlias, typ)
	}
	got := ZeroExprForType(typ, pkg)
	if got == nil {
		t.Fatalf("ZeroExprForType returned nil for type %s (%T)", typ, typ)
	}
	if printed := printExpr(t, got); printed != want {
		t.Fatalf("ZeroExprForType(%s) = %s, want %s", typ, printed, want)
	}
}

func assertZeroReturn(t *testing.T, source, want string) {
	t.Helper()
	_, file, pkg, info := parseAndTypeCheck(t, source)
	fn := firstFunc(t, file)
	obj := info.Defs[fn.Name]
	if obj == nil {
		t.Fatal("no type object for function")
	}
	sig, ok := obj.Type().(*types.Signature)
	if !ok {
		t.Fatalf("function type is %T, want *types.Signature", obj.Type())
	}
	got := ZeroReturnForSignature(pkg, sig)
	if got == nil {
		t.Fatal("ZeroReturnForSignature returned nil for alias result")
	}
	if printed := printNode(t, got); printed != want {
		t.Fatalf("ZeroReturnForSignature = %s, want %s", printed, want)
	}
}

func printExpr(t *testing.T, expr ast.Expr) string {
	t.Helper()
	return printNode(t, expr)
}

func printNode(t *testing.T, node ast.Node) string {
	t.Helper()
	var buf bytes.Buffer
	if err := printer.Fprint(&buf, token.NewFileSet(), node); err != nil {
		t.Fatalf("print node: %v", err)
	}
	return buf.String()
}

func firstFunc(t *testing.T, file *ast.File) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok {
			return fn
		}
	}
	t.Fatal("source contains no function")
	return nil
}

func firstReturn(t *testing.T, file *ast.File) *ast.ReturnStmt {
	t.Helper()

	var found *ast.ReturnStmt
	ast.Inspect(file, func(node ast.Node) bool {
		if found != nil {
			return false
		}
		ret, ok := node.(*ast.ReturnStmt)
		if ok {
			found = ret
			return false
		}
		return true
	})
	if found == nil {
		t.Fatal("source contains no return statement")
	}
	return found
}
