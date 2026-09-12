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
	return zeroExprForType(t, currentPkg, nil, token.NoPos)
}

// ZeroExprForTypeAt returns the zero-value AST expression for t using the
// package qualifier declared in the source file containing pos.
func ZeroExprForTypeAt(t types.Type, currentPkg *types.Package, info *types.Info, pos token.Pos) ast.Expr {
	return zeroExprForType(t, currentPkg, info, pos)
}

func zeroExprForType(t types.Type, currentPkg *types.Package, info *types.Info, pos token.Pos) ast.Expr {
	t = types.Unalias(t)
	switch u := t.(type) {
	case *types.Basic:
		return zeroExprForBasic(u)
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Interface, *types.Signature:
		return ast.NewIdent("nil")
	case *types.Named:
		return zeroExprForNamed(u, currentPkg, info, pos)
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

func zeroExprForNamed(u *types.Named, currentPkg *types.Package, info *types.Info, pos token.Pos) ast.Expr {
	if u.TypeParams() != nil {
		return nil
	}
	if _, ok := u.Underlying().(*types.Struct); !ok {
		return zeroExprForType(u.Underlying(), currentPkg, info, pos)
	}
	typeExpr := structTypeExpr(u.Obj(), currentPkg, info, pos)
	if typeExpr == nil {
		return nil
	}
	return &ast.CompositeLit{Type: typeExpr}
}

func structTypeExpr(obj *types.TypeName, currentPkg *types.Package, info *types.Info, pos token.Pos) ast.Expr {
	if obj.Pkg() == nil {
		return ast.NewIdent(obj.Name())
	}
	if currentPkg != nil && obj.Pkg().Path() == currentPkg.Path() {
		return ast.NewIdent(obj.Name())
	}
	packageName, ok := localPackageName(info, pos, obj.Pkg())
	if ok {
		if packageName == "." {
			return ast.NewIdent(obj.Name())
		}
		if packageName == "_" {
			return nil
		}
		return &ast.SelectorExpr{
			X:   ast.NewIdent(packageName),
			Sel: ast.NewIdent(obj.Name()),
		}
	}
	return &ast.SelectorExpr{
		X:   ast.NewIdent(obj.Pkg().Name()),
		Sel: ast.NewIdent(obj.Name()),
	}
}

func localPackageName(info *types.Info, pos token.Pos, imported *types.Package) (string, bool) {
	if !canResolveLocalPackageName(info, pos, imported) {
		return "", false
	}
	for node := range info.Scopes {
		file, ok := sourceFileAtPosition(node, pos)
		if !ok {
			continue
		}
		for _, imp := range file.Imports {
			pkgName := info.PkgNameOf(imp)
			if pkgName == nil || pkgName.Imported() == nil || pkgName.Imported().Path() != imported.Path() {
				continue
			}
			return pkgName.Name(), true
		}
	}
	return "", false
}

func sourceFileAtPosition(node ast.Node, pos token.Pos) (*ast.File, bool) {
	file, ok := node.(*ast.File)
	if !ok || pos < file.Pos() || pos > file.End() {
		return nil, false
	}
	return file, true
}

func canResolveLocalPackageName(info *types.Info, pos token.Pos, imported *types.Package) bool {
	return info != nil && pos.IsValid() && imported != nil
}

// ZeroReturnForSignature returns a return of zero values for sig. If a zero
// expression cannot be built and every result is named, it returns a bare
// return. It returns nil when the signature has results that cannot be zeroed.
func ZeroReturnForSignature(pkg *types.Package, sig *types.Signature) *ast.ReturnStmt {
	return zeroReturnForSignature(pkg, sig, nil, token.NoPos)
}

// ZeroReturnForSignatureAt is like ZeroReturnForSignature but resolves
// imported package qualifiers using the source file containing pos.
func ZeroReturnForSignatureAt(pkg *types.Package, sig *types.Signature, info *types.Info, pos token.Pos) *ast.ReturnStmt {
	return zeroReturnForSignature(pkg, sig, info, pos)
}

func zeroReturnForSignature(pkg *types.Package, sig *types.Signature, info *types.Info, pos token.Pos) *ast.ReturnStmt {
	if sig == nil {
		return nil
	}
	results := sig.Results()
	if results.Len() == 0 {
		return nil
	}
	zeros := zeroExprsForResults(pkg, results, info, pos)
	if zeros != nil {
		return &ast.ReturnStmt{Results: zeros}
	}
	if allResultsNamed(results) {
		return &ast.ReturnStmt{}
	}
	return nil
}

func zeroExprsForResults(pkg *types.Package, results *types.Tuple, info *types.Info, pos token.Pos) []ast.Expr {
	zeros := make([]ast.Expr, results.Len())
	for i := 0; i < results.Len(); i++ {
		zero := zeroExprForType(results.At(i).Type(), pkg, info, pos)
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
