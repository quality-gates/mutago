package arithmetic

import (
	"go/ast"
	"go/token"
	"go/types"
	"strconv"
	"strings"

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

	// Skip *= 0; inverting to /= 0 does not compile.
	if n.Tok == token.MUL_ASSIGN && len(n.Rhs) > 0 && isConstantZero(n.Rhs[0]) {
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

func isConstantZero(expr ast.Expr) bool {
	for {
		p, ok := expr.(*ast.ParenExpr)
		if !ok {
			break
		}
		expr = p.X
	}
	lit, ok := expr.(*ast.BasicLit)
	if !ok {
		return false
	}
	if lit.Kind != token.INT && lit.Kind != token.FLOAT {
		return false
	}
	v := strings.ReplaceAll(lit.Value, "_", "")
	switch v {
	case "0", "0.0", "0.", ".0", "0x0", "0X0", "0o0", "0O0", "0b0", "0B0":
		return true
	}
	if lit.Kind == token.INT {
		n, err := strconv.ParseUint(v, 0, 64)
		return err == nil && n == 0
	}
	f, err := strconv.ParseFloat(v, 64)
	return err == nil && f == 0
}
