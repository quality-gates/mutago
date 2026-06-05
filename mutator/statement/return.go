package statement

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("statement/return", MutatorReturnValue)
}

// MutatorReturnValue replaces each non-zero return value with the zero value
// for its type (false, 0, "", nil, TypeName{}).
func MutatorReturnValue(pkg *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.ReturnStmt)
	if !ok || len(n.Results) == 0 || info == nil {
		return nil
	}

	var mutations []mutator.Mutation

	for i, result := range n.Results {
		t := info.TypeOf(result)
		if t == nil {
			continue
		}

		zero := zeroExprForType(t, pkg)
		if zero == nil {
			continue
		}

		if isAlreadyZero(result) {
			continue
		}

		idx := i
		original := n.Results[idx]

		mutations = append(mutations, mutator.Mutation{
			Change: func() { n.Results[idx] = zero },
			Reset:  func() { n.Results[idx] = original },
		})
	}

	return mutations
}

func zeroBasicExpr(u *types.Basic) ast.Expr {
	if u.Kind() == types.Bool {
		return ast.NewIdent("false")
	}
	if u.Info()&types.IsString != 0 {
		return &ast.BasicLit{Kind: token.STRING, Value: `""`}
	}
	if u.Info()&types.IsNumeric != 0 {
		return &ast.BasicLit{Kind: token.INT, Value: "0"}
	}
	if u.Kind() == types.UnsafePointer {
		return ast.NewIdent("nil")
	}
	return nil
}

func zeroTypeExpr(obj *types.TypeName, currentPkg *types.Package) ast.Expr {
	if currentPkg != nil && obj.Pkg() != nil && obj.Pkg().Path() == currentPkg.Path() {
		return ast.NewIdent(obj.Name())
	}
	if obj.Pkg() != nil {
		return &ast.SelectorExpr{
			X:   ast.NewIdent(obj.Pkg().Name()),
			Sel: ast.NewIdent(obj.Name()),
		}
	}
	return ast.NewIdent(obj.Name())
}

// zeroExprForType returns the zero-value AST expression for t as seen from
// currentPkg. Named struct types produce TypeName{} (or pkg.TypeName{} for
// imported types). All other types follow the same rules as before.
func zeroExprForType(t types.Type, currentPkg *types.Package) ast.Expr {
	switch u := t.(type) {
	case *types.Basic:
		return zeroBasicExpr(u)
	case *types.Pointer, *types.Slice, *types.Map, *types.Chan, *types.Interface, *types.Signature:
		return ast.NewIdent("nil")
	case *types.Named:
		// For named struct types, generate TypeName{} or pkg.TypeName{}.
		// Skip generic types (TypeParams present) — the instantiation syntax
		// is complex and rarely worth mutating.
		if u.TypeParams() != nil {
			return nil
		}
		if _, ok := u.Underlying().(*types.Struct); ok {
			typeExpr := zeroTypeExpr(u.Obj(), currentPkg)
			return &ast.CompositeLit{Type: typeExpr}
		}
		return zeroExprForType(u.Underlying(), currentPkg)
	}
	return nil
}

// isAlreadyZero reports whether expr is already a zero-value literal,
// avoiding no-op mutations.
func isAlreadyZero(expr ast.Expr) bool {
	switch n := expr.(type) {
	case *ast.Ident:
		switch n.Name {
		case "nil", "false":
			return true
		}
	case *ast.BasicLit:
		switch n.Kind {
		case token.INT, token.FLOAT:
			switch n.Value {
			case "0", "0.0":
				return true
			}
		case token.STRING:
			switch n.Value {
			case `""`:
				return true
			}
		}
	case *ast.CompositeLit:
		// Already a zero-value struct literal if it has no field initializers.
		return len(n.Elts) == 0
	}
	return false
}
