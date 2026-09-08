package astutil

import (
	"go/ast"
	"go/token"
	"go/types"
)

// EnclosingFunc returns the innermost function body and signature that contain pos.
func EnclosingFunc(info *types.Info, pos token.Pos) (*ast.BlockStmt, *types.Signature) {
	if info == nil || !pos.IsValid() {
		return nil, nil
	}
	bestBody, bestSig := enclosingFuncDecl(info, pos)
	litBody, litSig := enclosingFuncLit(info, pos)
	if closer(litBody, bestBody) {
		return litBody, litSig
	}
	return bestBody, bestSig
}

func enclosingFuncDecl(info *types.Info, pos token.Pos) (*ast.BlockStmt, *types.Signature) {
	var bestBody *ast.BlockStmt
	var bestSig *types.Signature
	for n := range info.Scopes {
		file, ok := n.(*ast.File)
		if !ok {
			continue
		}
		for _, decl := range file.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Body == nil || !containsPos(fd.Body, pos) {
				continue
			}
			if closer(fd.Body, bestBody) {
				bestBody = fd.Body
				bestSig = funcDeclSignature(info, fd)
			}
		}
	}
	return bestBody, bestSig
}

func enclosingFuncLit(info *types.Info, pos token.Pos) (*ast.BlockStmt, *types.Signature) {
	if info.Types == nil {
		return nil, nil
	}
	var bestBody *ast.BlockStmt
	var bestSig *types.Signature
	for expr, tv := range info.Types {
		lit, ok := expr.(*ast.FuncLit)
		if !ok || lit.Body == nil || !containsPos(lit.Body, pos) {
			continue
		}
		sig, _ := tv.Type.(*types.Signature)
		if closer(lit.Body, bestBody) {
			bestBody = lit.Body
			bestSig = sig
		}
	}
	return bestBody, bestSig
}

func funcDeclSignature(info *types.Info, fd *ast.FuncDecl) *types.Signature {
	if fd.Name == nil || info.Defs == nil {
		return nil
	}
	obj := info.Defs[fd.Name]
	fn, ok := obj.(*types.Func)
	if !ok {
		return nil
	}
	sig, _ := fn.Type().(*types.Signature)
	return sig
}

func containsPos(body *ast.BlockStmt, pos token.Pos) bool {
	return body.Pos() <= pos && pos <= body.End()
}

func closer(candidate, current *ast.BlockStmt) bool {
	if candidate == nil {
		return false
	}
	if current == nil {
		return true
	}
	return (candidate.End() - candidate.Pos()) < (current.End() - current.Pos())
}
