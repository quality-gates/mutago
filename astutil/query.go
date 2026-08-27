package astutil

import (
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"sync"
)

// IdentifiersInStatement returns all identifiers with their found in a statement.
func IdentifiersInStatement(pkg *types.Package, info *types.Info, stmt ast.Stmt) []ast.Expr {
	return identifiersInStatements(pkg, info, []ast.Stmt{stmt})
}

func identifiersInStatements(pkg *types.Package, info *types.Info, stmts []ast.Stmt) []ast.Expr {
	_ = pkg
	index, _ := identifierIndexes.LoadOrStore(info, &identifierIndex{
		seenNodes: make(map[ast.Node]struct{}),
		seenDefs:  make(map[*ast.Ident]struct{}),
	})
	return index.(*identifierIndex).query(info, stmts)
}

type identifierEvent struct {
	pos    token.Pos
	expr   ast.Expr
	object types.Object
}

type definitionEvent struct {
	pos    token.Pos
	object types.Object
}

type positionRange struct{ start, end token.Pos }

type identifierIndex struct {
	mu        sync.Mutex
	events    []identifierEvent
	defs      []definitionEvent
	covered   []positionRange
	seenNodes map[ast.Node]struct{}
	seenDefs  map[*ast.Ident]struct{}
}

var identifierIndexes sync.Map

// ClearIdentifierCache releases per-type-check identifier indexes between runs.
func ClearIdentifierCache() {
	identifierIndexes = sync.Map{}
}

func (idx *identifierIndex) query(info *types.Info, stmts []ast.Stmt) []ast.Expr {
	if len(stmts) == 0 {
		return nil
	}
	start, end := stmts[0].Pos(), stmts[len(stmts)-1].End()
	idx.mu.Lock()
	defer idx.mu.Unlock()
	idx.ensure(info, stmts, start, end)
	return idx.results(info, start, end)
}

func (idx *identifierIndex) ensure(info *types.Info, stmts []ast.Stmt, start, end token.Pos) {
	if !idx.covers(start, end) {
		for _, stmt := range stmts {
			idx.collect(info, stmt)
		}
		idx.covered = append(idx.covered, positionRange{start: start, end: end})
		sort.Slice(idx.events, func(i, j int) bool { return idx.events[i].pos < idx.events[j].pos })
		sort.Slice(idx.defs, func(i, j int) bool { return idx.defs[i].pos < idx.defs[j].pos })
	}
}

func (idx *identifierIndex) results(info *types.Info, start, end token.Pos) []ast.Expr {
	excluded := make(map[types.Object]struct{})
	defStart := sort.Search(len(idx.defs), func(i int) bool { return idx.defs[i].pos >= start })
	for i := defStart; i < len(idx.defs) && idx.defs[i].pos <= end; i++ {
		excluded[idx.defs[i].object] = struct{}{}
	}
	eventStart := sort.Search(len(idx.events), func(i int) bool { return idx.events[i].pos >= start })
	result := make([]ast.Expr, 0)
	for i := eventStart; i < len(idx.events) && idx.events[i].pos <= end; i++ {
		event := idx.events[i]
		if _, skip := excluded[event.object]; skip {
			continue
		}
		if expr := cloneIdentifierEvent(info, event.expr); expr != nil {
			result = append(result, expr)
		}
	}
	return result
}

func cloneIdentifierEvent(info *types.Info, expr ast.Expr) ast.Expr {
	switch expr := expr.(type) {
	case *ast.Ident:
		return &ast.Ident{Name: expr.Name}
	case *ast.SelectorExpr:
		selector := cloneSelector(expr)
		if (&identifierWalker{info: info}).shouldInitialize(expr) {
			return &ast.CompositeLit{Type: selector}
		}
		return selector
	default:
		return nil
	}
}

func (idx *identifierIndex) covers(start, end token.Pos) bool {
	for _, covered := range idx.covered {
		if covered.start <= start && covered.end >= end {
			return true
		}
	}
	return false
}

func (idx *identifierIndex) collect(info *types.Info, root ast.Node) {
	ast.Inspect(root, func(node ast.Node) bool {
		return idx.collectNode(info, node)
	})
}

func (idx *identifierIndex) collectNode(info *types.Info, node ast.Node) bool {
	switch node := node.(type) {
	case *ast.SelectorExpr:
		return idx.collectSelector(info, node)
	case *ast.Ident:
		idx.collectDefinition(info, node)
		idx.collectIdentifier(info, node)
		return false
	default:
		return true
	}
}

func (idx *identifierIndex) collectSelector(info *types.Info, selector *ast.SelectorExpr) bool {
	if selector.Sel == nil || !checkForSelectorExpr(selector) {
		return true
	}
	if _, seen := idx.seenNodes[selector]; seen {
		return false
	}
	var object types.Object
	if root := selectorRoot(selector); root != nil {
		object = info.Uses[root]
	}
	idx.events = append(idx.events, identifierEvent{pos: selector.Pos(), expr: selector, object: object})
	idx.seenNodes[selector] = struct{}{}
	return false
}

func (idx *identifierIndex) collectDefinition(info *types.Info, ident *ast.Ident) {
	object, ok := info.Defs[ident]
	if !ok {
		return
	}
	if _, variable := object.(*types.Var); !variable {
		return
	}
	if _, seen := idx.seenDefs[ident]; seen {
		return
	}
	idx.defs = append(idx.defs, definitionEvent{pos: ident.Pos(), object: object})
	idx.seenDefs[ident] = struct{}{}
}

func (idx *identifierIndex) collectIdentifier(info *types.Info, ident *ast.Ident) {
	if ident.Name == "_" || token.Lookup(ident.Name) != token.IDENT {
		return
	}
	object, ok := info.Uses[ident]
	variable, variableUse := object.(*types.Var)
	if !ok || !variableUse || variable.IsField() {
		return
	}
	if _, seen := idx.seenNodes[ident]; seen {
		return
	}
	idx.events = append(idx.events, identifierEvent{pos: ident.Pos(), expr: ident, object: object})
	idx.seenNodes[ident] = struct{}{}
}

type identifierWalker struct {
	identifiers []ast.Expr
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
