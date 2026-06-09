package astutil

import (
	"go/ast"
	"go/token"
	"go/types"
)

// IdentifiersInStatement returns all identifiers with their found in a statement.
func IdentifiersInStatement(pkg *types.Package, info *types.Info, stmt ast.Stmt) []ast.Expr {
	return identifiersInStatements(pkg, info, []ast.Stmt{stmt})
}

func identifiersInStatements(pkg *types.Package, info *types.Info, stmts []ast.Stmt) []ast.Expr {
	w := &identifierWalker{
		pkg:      pkg,
		info:     info,
		excluded: variableDefinitions(info, stmts),
	}

	for _, stmt := range stmts {
		ast.Walk(w, stmt)
	}

	return w.identifiers
}

type identifierWalker struct {
	identifiers []ast.Expr
	pkg         *types.Package
	info        *types.Info
	excluded    map[types.Object]struct{}
}

func variableDefinitions(info *types.Info, stmts []ast.Stmt) map[types.Object]struct{} {
	definitions := make(map[types.Object]struct{})
	for _, stmt := range stmts {
		ast.Inspect(stmt, func(node ast.Node) bool {
			ident, ok := node.(*ast.Ident)
			if !ok {
				return true
			}

			object, ok := info.Defs[ident]
			if !ok {
				return true
			}
			if _, ok := object.(*types.Var); ok {
				definitions[object] = struct{}{}
			}

			return true
		})
	}

	return definitions
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

	if _, ok := w.info.Defs[n]; ok {
		return nil
	}

	obj, ok := w.info.Uses[n]
	if !ok {
		return nil
	}
	variable, ok := obj.(*types.Var)
	if !ok || variable.IsField() {
		return nil
	}
	if _, ok := w.excluded[obj]; ok {
		return nil
	}

	w.identifiers = append(w.identifiers, &ast.Ident{
		Name: n.Name,
	})

	return nil
}

// visitSelector records selector expressions, wrapping composite types in a
// composite literal so they can be instantiated.
func (w *identifierWalker) visitSelector(n *ast.SelectorExpr) ast.Visitor {
	if n.Sel == nil || !checkForSelectorExpr(n) {
		return w
	}
	if root := selectorRoot(n); root != nil {
		if obj, ok := w.info.Uses[root]; ok {
			if _, excluded := w.excluded[obj]; excluded {
				return nil
			}
		}
	}

	selector := cloneSelector(n)
	if w.shouldInitialize(n) {
		w.identifiers = append(w.identifiers, &ast.CompositeLit{
			Type: selector,
		})

		return nil
	}

	w.identifiers = append(w.identifiers, selector)

	return nil
}

func selectorRoot(expr ast.Expr) *ast.Ident {
	switch n := expr.(type) {
	case *ast.Ident:
		return n
	case *ast.SelectorExpr:
		return selectorRoot(n.X)
	default:
		return nil
	}
}

func cloneSelector(n *ast.SelectorExpr) *ast.SelectorExpr {
	cloned := &ast.SelectorExpr{
		Sel: &ast.Ident{Name: n.Sel.Name},
	}
	switch x := n.X.(type) {
	case *ast.Ident:
		cloned.X = &ast.Ident{Name: x.Name}
	case *ast.SelectorExpr:
		cloned.X = cloneSelector(x)
	}

	return cloned
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
