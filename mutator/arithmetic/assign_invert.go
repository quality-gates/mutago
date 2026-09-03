package arithmetic

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("arithmetic/assign_invert", MutatorArithmeticAssignInvert)
}

var assignInvertMutations = map[token.Token]token.Token{
	token.ADD_ASSIGN:     token.SUB_ASSIGN,
	token.SUB_ASSIGN:     token.ADD_ASSIGN,
	token.MUL_ASSIGN:     token.QUO_ASSIGN,
	token.QUO_ASSIGN:     token.MUL_ASSIGN,
	token.REM_ASSIGN:     token.MUL_ASSIGN,
	token.AND_ASSIGN:     token.OR_ASSIGN,
	token.OR_ASSIGN:      token.AND_ASSIGN,
	token.XOR_ASSIGN:     token.AND_ASSIGN,
	token.SHL_ASSIGN:     token.SHR_ASSIGN,
	token.SHR_ASSIGN:     token.SHL_ASSIGN,
	token.AND_NOT_ASSIGN: token.AND_ASSIGN,
}

// MutatorArithmeticAssignInvert implements a mutator to invert change assign statements.
func MutatorArithmeticAssignInvert(_ *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.AssignStmt)
	if !ok {
		return nil
	}

	if n.Tok == token.ADD_ASSIGN && isStringAssign(info, n) {
		return nil
	}

	original := n.Tok
	mutated, ok := assignInvertMutations[n.Tok]
	if !ok {
		return nil
	}

	return []mutator.Mutation{
		{
			Position: n.TokPos,
			Change: func() {
				n.Tok = mutated
			},
			Reset: func() {
				n.Tok = original
			},
		},
	}
}

func isStringAssign(info *types.Info, n *ast.AssignStmt) bool {
	if len(n.Rhs) > 0 && isStringLit(n.Rhs[0]) {
		return true
	}
	if info != nil && len(n.Lhs) > 0 {
		if t := info.TypeOf(n.Lhs[0]); t != nil {
			basic, ok := t.Underlying().(*types.Basic)
			return ok && basic.Info()&types.IsString != 0
		}
	}
	return false
}
