package expression

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
	"github.com/stretchr/testify/assert"
)

func TestMutatorErrorGuard(t *testing.T) {
	test.Mutator(
		t,
		MutatorErrorGuard,
		"../../testdata/expression/error_guard.go",
		2,
	)
}

func TestMutatorErrorGuard_SkipsNonIfStmt(t *testing.T) {
	assert.Nil(t, MutatorErrorGuard(nil, nil, &ast.BasicLit{}))
}

func TestMutatorErrorGuard_SkipsNilInfo(t *testing.T) {
	ifStmt := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			Op: token.NEQ,
			X:  ast.NewIdent("err"),
			Y:  ast.NewIdent("nil"),
		},
	}
	assert.Nil(t, MutatorErrorGuard(nil, nil, ifStmt))
}

func TestMutatorErrorGuard_SkipsNonBinaryCond(t *testing.T) {
	ifStmt := &ast.IfStmt{Cond: ast.NewIdent("ok")}
	assert.Nil(t, MutatorErrorGuard(nil, nil, ifStmt))
}

func TestMutatorErrorGuard_SkipsBothNonNil(t *testing.T) {
	// if err1 != err2 — neither side is the nil identifier → should return nil
	ifStmt := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			Op: token.NEQ,
			X:  ast.NewIdent("err1"),
			Y:  ast.NewIdent("err2"),
		},
	}
	assert.Nil(t, MutatorErrorGuard(nil, &types.Info{}, ifStmt))
}

func TestMutatorErrorGuard_SkipsNonIdentNilSide(t *testing.T) {
	// if someCall() != nil — non-Ident on X, covers isNilIdent(!ok) and isErrorExpr(t==nil)
	ifStmt := &ast.IfStmt{
		Cond: &ast.BinaryExpr{
			Op: token.NEQ,
			X:  &ast.CallExpr{Fun: ast.NewIdent("f")},
			Y:  ast.NewIdent("nil"),
		},
	}
	assert.Nil(t, MutatorErrorGuard(nil, &types.Info{}, ifStmt))
}

func TestMutatorErrorGuard_Registered(t *testing.T) {
	_, err := mutator.New("expression/error-guard")
	assert.Nil(t, err)
}

func TestMutatorErrorGuard_Safety(t *testing.T) {
	src := `package example

import (
	"errors"
)

type S struct {
	Err error
}

func getErr() error { return errors.New("err") }

func Cases(paramErr error) {
	onlyErr := getErr()
	if onlyErr != nil { // onlyErr has no other use -> skipped
		println("onlyErr")
	}

	multiErr := getErr()
	if multiErr != nil { // multiErr has another use -> mutated
		println(multiErr)
	}

	var s S
	s.Err = getErr()
	if s.Err != nil { // field selector, receiver has another use -> mutated
		println("s.Err")
	}

	if paramErr != nil { // param -> mutated
		println("paramErr")
	}
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Scopes:     make(map[ast.Node]*types.Scope),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("example", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatal(err)
	}

	fn := file.Decls[3].(*ast.FuncDecl)
	// 0: onlyErr := getErr()
	// 1: if onlyErr != nil -> should be skipped!
	ifStmt1 := fn.Body.List[1].(*ast.IfStmt)
	muts1 := MutatorErrorGuard(pkg, info, ifStmt1)
	assert.Empty(t, muts1, "expected onlyErr guard to be skipped")

	// 2: multiErr := getErr()
	// 3: if multiErr != nil -> should be mutated!
	ifStmt2 := fn.Body.List[3].(*ast.IfStmt)
	muts2 := MutatorErrorGuard(pkg, info, ifStmt2)
	assert.Len(t, muts2, 1, "expected multiErr guard to be mutated")

	// 4: var s S
	// 5: s.Err = getErr()
	// 6: if s.Err != nil -> should be mutated!
	ifStmt3 := fn.Body.List[6].(*ast.IfStmt)
	muts3 := MutatorErrorGuard(pkg, info, ifStmt3)
	assert.Len(t, muts3, 1, "expected s.Err guard to be mutated")

	// 7: if paramErr != nil -> should be mutated!
	ifStmt4 := fn.Body.List[7].(*ast.IfStmt)
	muts4 := MutatorErrorGuard(pkg, info, ifStmt4)
	assert.Len(t, muts4, 1, "expected paramErr guard to be mutated")
}
