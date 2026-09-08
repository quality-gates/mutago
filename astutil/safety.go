package astutil

import (
	"go/ast"
	"go/token"
	"go/types"
	"sync"
)

type safetyIndex struct {
	useCounts   map[types.Object]int
	filePkgUses map[*types.PkgName]map[*ast.File]int
	files       []*ast.File
	params      map[types.Object]bool
}

var safetyIndexes sync.Map

// ClearSafetyCache releases per-type-check safety indexes between runs.
func ClearSafetyCache() {
	safetyIndexes = sync.Map{}
}

func getSafetyIndex(info *types.Info) *safetyIndex {
	if info == nil {
		return nil
	}
	if val, ok := safetyIndexes.Load(info); ok {
		return val.(*safetyIndex)
	}
	idx := buildSafetyIndex(info)
	safetyIndexes.Store(info, idx)
	return idx
}

func buildSafetyIndex(info *types.Info) *safetyIndex {
	idx := &safetyIndex{
		useCounts:   countObjectUses(info),
		filePkgUses: make(map[*types.PkgName]map[*ast.File]int),
		params:      make(map[types.Object]bool),
		files:       collectScopeFiles(info),
	}
	collectParamsAndPkgUses(info, idx)
	return idx
}

func countObjectUses(info *types.Info) map[types.Object]int {
	counts := make(map[types.Object]int)
	for _, obj := range info.Uses {
		if obj != nil {
			counts[obj]++
		}
	}
	return counts
}

func collectScopeFiles(info *types.Info) []*ast.File {
	var files []*ast.File
	for node := range info.Scopes {
		if file, ok := node.(*ast.File); ok {
			files = append(files, file)
		}
	}
	return files
}

func collectParamsAndPkgUses(info *types.Info, idx *safetyIndex) {
	for _, file := range idx.files {
		collectFileParams(info, file, idx.params)
		collectFilePkgUses(info, file, idx.filePkgUses)
	}
}

func collectFileParams(info *types.Info, file *ast.File, params map[types.Object]bool) {
	ast.Inspect(file, func(n ast.Node) bool {
		if ft, ok := n.(*ast.FuncType); ok {
			collectFieldListDefs(info, ft.Params, params)
			collectFieldListDefs(info, ft.Results, params)
		}
		return true
	})
}

func collectFieldListDefs(info *types.Info, fl *ast.FieldList, defs map[types.Object]bool) {
	if fl == nil {
		return
	}
	for _, f := range fl.List {
		for _, name := range f.Names {
			if obj := info.Defs[name]; obj != nil {
				defs[obj] = true
			}
		}
	}
}

func collectFilePkgUses(info *types.Info, file *ast.File, filePkgUses map[*types.PkgName]map[*ast.File]int) {
	for id, obj := range info.Uses {
		pkgName, ok := obj.(*types.PkgName)
		if !ok || id.Pos() < file.Pos() || id.Pos() > file.End() {
			continue
		}
		if filePkgUses[pkgName] == nil {
			filePkgUses[pkgName] = make(map[*ast.File]int)
		}
		filePkgUses[pkgName][file]++
	}
}

func (idx *safetyIndex) fileForPos(pos token.Pos) *ast.File {
	for _, f := range idx.files {
		if f.Pos() <= pos && pos <= f.End() {
			return f
		}
	}
	return nil
}

// IsSafeToRemove reports whether removing node will leave no local variable
// or imported package unused in the file.
func IsSafeToRemove(info *types.Info, node ast.Node) bool {
	if info == nil || node == nil {
		return true
	}
	if HasUnsafeImport(info, node) {
		return false
	}
	if len(UnsafeLocalVars(info, node)) > 0 {
		return false
	}
	return true
}

// HasUnsafeImport reports whether removing node would remove the only use of
// an imported package in the enclosing file.
func HasUnsafeImport(info *types.Info, node ast.Node) bool {
	if info == nil || node == nil {
		return false
	}
	idx := getSafetyIndex(info)
	if idx == nil {
		return false
	}

	file := idx.fileForPos(node.Pos())
	if file == nil {
		return false
	}

	nodePkgUses := make(map[*types.PkgName]int)
	ast.Inspect(node, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if pkgName, ok := info.Uses[id].(*types.PkgName); ok {
				nodePkgUses[pkgName]++
			}
		}
		return true
	})

	for pkgName, count := range nodePkgUses {
		if count >= idx.filePkgUses[pkgName][file] {
			return true
		}
	}
	return false
}

// UnsafeLocalVars returns identifiers of local variables whose only uses are
// inside node, and which would therefore become declared and not used if node
// is removed.
func UnsafeLocalVars(info *types.Info, node ast.Node) []ast.Expr {
	if info == nil || node == nil {
		return nil
	}
	idx := getSafetyIndex(info)
	if idx == nil {
		return nil
	}

	nodeUses, varFirstIdent := collectNodeVarUses(info, idx, node)
	var unsafe []ast.Expr
	for v, count := range nodeUses {
		if count >= idx.useCounts[v] {
			id := varFirstIdent[v]
			unsafe = append(unsafe, &ast.Ident{Name: id.Name})
		}
	}
	return unsafe
}

func isEnclosingLocalVar(idx *safetyIndex, obj types.Object, node ast.Node) (*types.Var, bool) {
	v, ok := obj.(*types.Var)
	if !ok || v.IsField() || idx.params[v] {
		return nil, false
	}
	if v.Pkg() != nil && v.Parent() == v.Pkg().Scope() {
		return nil, false
	}
	if v.Pos() >= node.Pos() && v.Pos() < node.End() {
		return nil, false
	}
	return v, true
}

func collectNodeVarUses(info *types.Info, idx *safetyIndex, node ast.Node) (map[*types.Var]int, map[*types.Var]*ast.Ident) {
	nodeUses := make(map[*types.Var]int)
	varFirstIdent := make(map[*types.Var]*ast.Ident)

	ast.Inspect(node, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || id.Name == "_" || token.Lookup(id.Name) != token.IDENT {
			return true
		}
		obj := info.Uses[id]
		if obj == nil {
			return true
		}
		if v, ok := isEnclosingLocalVar(idx, obj, node); ok {
			nodeUses[v]++
			if _, seen := varFirstIdent[v]; !seen {
				varFirstIdent[v] = id
			}
		}
		return true
	})

	return nodeUses, varFirstIdent
}

// CreateNoopOfExpressions creates a blank assignment statement for the given expressions
// anchored at pos.
func CreateNoopOfExpressions(exprs []ast.Expr, pos token.Pos) ast.Stmt {
	if len(exprs) == 0 {
		return &ast.EmptyStmt{Semicolon: pos}
	}
	for _, e := range exprs {
		anchorExpression(e, pos)
	}
	lhs := make([]ast.Expr, len(exprs))
	for i := range exprs {
		lhs[i] = &ast.Ident{Name: "_", NamePos: pos}
	}
	return &ast.AssignStmt{
		Lhs:    lhs,
		Rhs:    exprs,
		Tok:    token.ASSIGN,
		TokPos: pos,
	}
}
