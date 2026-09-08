package astutil

import (
	"go/ast"
	"go/token"
	"go/types"
)

// ZeroExprForType returns the zero-value AST expression for t as seen from
// currentPkg. Named struct types produce TypeName{} (or pkg.TypeName{} for
// imported types). Returns nil when no zero expression can be built.
func ZeroExprForType(t types.Type, currentPkg *types.Package) ast.Expr {
	switch u := t.(type) {
	case *types.Basic:
		return zeroExprForBasic(u)
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Interface, *types.Signature:
		return ast.NewIdent("nil")
	case *types.Named:
		return zeroExprForNamed(u, currentPkg)
	}
	return nil
}

func zeroExprForBasic(u *types.Basic) ast.Expr {
	switch {
	case u.Kind() == types.Bool:
		return ast.NewIdent("false")
	case u.Info()&types.IsString != 0:
		return &ast.BasicLit{Kind: token.STRING, Value: `""`}
	case u.Info()&types.IsNumeric != 0:
		return &ast.BasicLit{Kind: token.INT, Value: "0"}
	case u.Kind() == types.UnsafePointer:
		return ast.NewIdent("nil")
	}
	return nil
}

func zeroExprForNamed(u *types.Named, currentPkg *types.Package) ast.Expr {
	if u.TypeParams() != nil {
		return nil
	}
	if _, ok := u.Underlying().(*types.Struct); !ok {
		return ZeroExprForType(u.Underlying(), currentPkg)
	}
	return &ast.CompositeLit{Type: structTypeExpr(u.Obj(), currentPkg)}
}

func structTypeExpr(obj *types.TypeName, currentPkg *types.Package) ast.Expr {
	if obj.Pkg() == nil {
		return ast.NewIdent(obj.Name())
	}
	if currentPkg != nil && obj.Pkg().Path() == currentPkg.Path() {
		return ast.NewIdent(obj.Name())
	}
	return &ast.SelectorExpr{
		X:   ast.NewIdent(obj.Pkg().Name()),
		Sel: ast.NewIdent(obj.Name()),
	}
}

// ZeroReturnForSignature returns a return of zero values for sig. If a zero
// expression cannot be built and every result is named, it returns a bare
// return. It returns nil when the signature has results that cannot be zeroed.
func ZeroReturnForSignature(pkg *types.Package, sig *types.Signature) *ast.ReturnStmt {
	if sig == nil {
		return nil
	}
	results := sig.Results()
	if results.Len() == 0 {
		return nil
	}
	zeros := zeroExprsForResults(pkg, results)
	if zeros != nil {
		return &ast.ReturnStmt{Results: zeros}
	}
	if allResultsNamed(results) {
		return &ast.ReturnStmt{}
	}
	return nil
}

func zeroExprsForResults(pkg *types.Package, results *types.Tuple) []ast.Expr {
	zeros := make([]ast.Expr, results.Len())
	for i := 0; i < results.Len(); i++ {
		zero := ZeroExprForType(results.At(i).Type(), pkg)
		if zero == nil {
			return nil
		}
		zeros[i] = zero
	}
	return zeros
}

func allResultsNamed(results *types.Tuple) bool {
	for i := 0; i < results.Len(); i++ {
		if results.At(i).Name() == "" {
			return false
		}
	}
	return true
}
