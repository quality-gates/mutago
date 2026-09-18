package astutil

import (
	"bytes"
	"go/ast"
	"go/importer"
	"go/parser"
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

func TestZeroExprForTypeAt_UnexportedStructInImportedPackage(t *testing.T) {
	dependency := `package dep

type secret struct{ Val int }

func New() secret { return secret{1} }
`
	source := `package example

import "example.com/dep"

func Get() any {
	return dep.New()
}
`
	assertCrossPackageZeroExpr(t, dependency, source, "")
}

func TestZeroExprForTypeAt_ExportedStructInImportedPackage(t *testing.T) {
	dependency := `package dep

type Public struct{ Val int }

func New() Public { return Public{1} }
`
	source := `package example

import "example.com/dep"

func Get() any {
	return dep.New()
}
`
	assertCrossPackageZeroExpr(t, dependency, source, "dep.Public{}")
}

func TestZeroExprForTypeAt_ExportedStructInTransitivelyImportedPackage(t *testing.T) {
	// dep2.Public is only reachable through dep's return type; example.go
	// never imports dep2 directly, so a synthesized dep2.Public{} would be
	// uncompilable.
	dep2 := `package dep2

type Public struct{ Val int }
`
	dependency := `package dep

import "example.com/dep2"

func New() dep2.Public { return dep2.Public{Val: 1} }
`
	source := `package example

import "example.com/dep"

func Get() any {
	return dep.New()
}
`
	assertTransitiveCrossPackageZeroExpr(t, dep2, dependency, source, "")
}

// assertTransitiveCrossPackageZeroExpr type-checks dep2 as "example.com/dep2",
// then dependency as "example.com/dep" (importing dep2), then source as a
// package importing dep only, and compares the zero expression built for the
// first return result against want. An empty want means the zero expression
// must be nil.
func assertTransitiveCrossPackageZeroExpr(t *testing.T, dep2Src, dependencySrc, source, want string) {
	t.Helper()

	fset := token.NewFileSet()
	dep2File, err := parser.ParseFile(fset, "dep2.go", dep2Src, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse dep2: %v\n%s", err, dep2Src)
	}
	dep2Pkg, err := (&types.Config{Importer: importer.Default()}).Check("example.com/dep2", fset, []*ast.File{dep2File}, nil)
	if err != nil {
		t.Fatalf("type-check dep2: %v\n%s", err, dep2Src)
	}

	depFile, err := parser.ParseFile(fset, "dep.go", dependencySrc, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse dependency: %v\n%s", err, dependencySrc)
	}
	depPkg, err := (&types.Config{Importer: fixedImporter{pkg: dep2Pkg}}).Check("example.com/dep", fset, []*ast.File{depFile}, nil)
	if err != nil {
		t.Fatalf("type-check dependency: %v\n%s", err, dependencySrc)
	}

	file, err := parser.ParseFile(fset, "example.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse source: %v\n%s", err, source)
	}
	info := &types.Info{
		Types:     make(map[ast.Expr]types.TypeAndValue),
		Defs:      make(map[*ast.Ident]types.Object),
		Uses:      make(map[*ast.Ident]types.Object),
		Implicits: make(map[ast.Node]types.Object),
		Scopes:    make(map[ast.Node]*types.Scope),
	}
	config := &types.Config{Importer: multiImporter{pkgs: map[string]*types.Package{
		depPkg.Path():  depPkg,
		dep2Pkg.Path(): dep2Pkg,
	}}}
	pkg, err := config.Check("example.com/example", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatalf("type-check source: %v\n%s", err, source)
	}

	ret := firstReturn(t, file)
	result := ret.Results[0]
	typ := info.TypeOf(result)
	if typ == nil {
		t.Fatal("no type for return expression")
	}

	got := ZeroExprForTypeAt(typ, pkg, info, result.Pos())
	if want == "" {
		if got != nil {
			t.Fatalf("ZeroExprForTypeAt(%s) = %s, want nil", typ, printExpr(t, got))
		}
		return
	}
	if got == nil {
		t.Fatalf("ZeroExprForTypeAt(%s) = nil, want %s", typ, want)
	}
	if printed := printExpr(t, got); printed != want {
		t.Fatalf("ZeroExprForTypeAt(%s) = %s, want %s", typ, printed, want)
	}
}

// multiImporter resolves import paths present in pkgs, which is enough for
// the three-package sources used in these tests.
type multiImporter struct {
	pkgs map[string]*types.Package
}

func (i multiImporter) Import(path string) (*types.Package, error) {
	if pkg, ok := i.pkgs[path]; ok {
		return pkg, nil
	}
	return importer.Default().Import(path)
}

// assertCrossPackageZeroExpr type-checks dependency as "example.com/dep",
// then source as a package importing it, and compares the zero expression
// built for the first return result against want. An empty want means the
// zero expression must be nil.
func assertCrossPackageZeroExpr(t *testing.T, dependency, source, want string) {
	t.Helper()

	fset := token.NewFileSet()
	depFile, err := parser.ParseFile(fset, "dep.go", dependency, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse dependency: %v\n%s", err, dependency)
	}
	depPkg, err := (&types.Config{Importer: importer.Default()}).Check("example.com/dep", fset, []*ast.File{depFile}, nil)
	if err != nil {
		t.Fatalf("type-check dependency: %v\n%s", err, dependency)
	}

	file, err := parser.ParseFile(fset, "example.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse source: %v\n%s", err, source)
	}
	info := &types.Info{
		Types:     make(map[ast.Expr]types.TypeAndValue),
		Defs:      make(map[*ast.Ident]types.Object),
		Uses:      make(map[*ast.Ident]types.Object),
		Implicits: make(map[ast.Node]types.Object),
		Scopes:    make(map[ast.Node]*types.Scope),
	}
	config := &types.Config{Importer: fixedImporter{pkg: depPkg}}
	pkg, err := config.Check("example.com/example", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatalf("type-check source: %v\n%s", err, source)
	}

	ret := firstReturn(t, file)
	result := ret.Results[0]
	typ := info.TypeOf(result)
	if typ == nil {
		t.Fatal("no type for return expression")
	}

	got := ZeroExprForTypeAt(typ, pkg, info, result.Pos())
	if want == "" {
		if got != nil {
			t.Fatalf("ZeroExprForTypeAt(%s) = %s, want nil", typ, printExpr(t, got))
		}
		return
	}
	if got == nil {
		t.Fatalf("ZeroExprForTypeAt(%s) = nil, want %s", typ, want)
	}
	if printed := printExpr(t, got); printed != want {
		t.Fatalf("ZeroExprForTypeAt(%s) = %s, want %s", typ, printed, want)
	}
}

// fixedImporter resolves every import path to the same package, which is
// enough for the two-package sources used in these tests.
type fixedImporter struct {
	pkg *types.Package
}

func (i fixedImporter) Import(path string) (*types.Package, error) {
	if path == i.pkg.Path() {
		return i.pkg, nil
	}
	return importer.Default().Import(path)
}
