package astutil

import (
	"go/ast"
	"go/token"
	"go/types"
)

// IdentifiersInStatement returns all identifiers with their found in a statement.
func IdentifiersInStatement(pkg *types.Package, info *types.Info, stmt ast.Stmt) []ast.Expr {
	w := &identifierWalker{
		pkg:  pkg,
		info: info,
	}

	ast.Walk(w, stmt)

	return w.identifiers
}

type identifierWalker struct {
	identifiers []ast.Expr
	pkg         *types.Package
	info        *types.Info
}

func checkForSelectorExpr(node ast.Expr) bool {
	switch n := node.(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return checkForSelectorExpr(n.X)
	}

	return false
}

func (w *identifierWalker) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.Ident:
		return w.visitIdent(n)
	case *ast.SelectorExpr:
		return w.visitSelector(n)
	}

	return w
}

// visitIdent records variable identifiers, skipping the blank identifier,
// keywords, and non-variable uses.
func (w *identifierWalker) visitIdent(n *ast.Ident) ast.Visitor {
	if n.Name == "_" {
		return nil
	}

	if token.Lookup(n.Name) != token.IDENT {
		return nil
	}

	if obj, ok := w.info.Uses[n]; ok {
		if _, ok := obj.(*types.Var); !ok {
			return nil
		}
	}

	// FIXME instead of manually creating a new node, clone it and trim the node from its comments and position https://github.com/zimmski/go-mutesting/issues/49
	w.identifiers = append(w.identifiers, &ast.Ident{
		Name: n.Name,
	})

	return nil
}

// visitSelector records selector expressions, wrapping composite types in a
// composite literal so they can be instantiated.
func (w *identifierWalker) visitSelector(n *ast.SelectorExpr) ast.Visitor {
	if !checkForSelectorExpr(n) {
		return nil
	}

	// FIXME we need to clone the node and trim comments and position recursively https://github.com/zimmski/go-mutesting/issues/49
	if w.shouldInitialize(n) {
		w.identifiers = append(w.identifiers, &ast.CompositeLit{
			Type: n,
		})

		return nil
	}

	w.identifiers = append(w.identifiers, n)

	return nil
}

// shouldInitialize reports whether the selector refers to a composite type
// (array, map, slice, or struct) that needs a composite literal.
func (w *identifierWalker) shouldInitialize(n *ast.SelectorExpr) bool {
	if n.Sel == nil {
		return false
	}

	obj, ok := w.info.Uses[n.Sel]
	if !ok {
		return false
	}

	switch obj.Type().Underlying().(type) {
	case *types.Array, *types.Map, *types.Slice, *types.Struct:
		return true
	}

	return false
}

// Functions returns all found functions.
func Functions(n ast.Node) []*ast.FuncDecl {
	w := &functionWalker{}

	ast.Walk(w, n)

	return w.functions
}

type functionWalker struct {
	functions []*ast.FuncDecl
}

func (w *functionWalker) Visit(node ast.Node) ast.Visitor {
	switch n := node.(type) {
	case *ast.FuncDecl:
		w.functions = append(w.functions, n)

		return nil
	}

	return w
}
