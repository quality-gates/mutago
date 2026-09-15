package arithmetic

import (
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"math"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("arithmetic/assignment", MutatorArithmeticAssignment)
}

var assignmentMutations = map[token.Token]token.Token{
	token.ADD_ASSIGN:     token.ASSIGN,
	token.SUB_ASSIGN:     token.ASSIGN,
	token.MUL_ASSIGN:     token.ASSIGN,
	token.QUO_ASSIGN:     token.ASSIGN,
	token.REM_ASSIGN:     token.ASSIGN,
	token.AND_ASSIGN:     token.ASSIGN,
	token.OR_ASSIGN:      token.ASSIGN,
	token.XOR_ASSIGN:     token.ASSIGN,
	token.SHL_ASSIGN:     token.ASSIGN,
	token.SHR_ASSIGN:     token.ASSIGN,
	token.AND_NOT_ASSIGN: token.ASSIGN,
}

// MutatorArithmeticAssignment implements a mutator to change base assign logic.
func MutatorArithmeticAssignment(_ *types.Package, info *types.Info, node ast.Node) []mutator.Mutation {
	n, ok := node.(*ast.AssignStmt)
	if !ok {
		return nil
	}

	original := n.Tok
	mutated, ok := assignmentMutations[n.Tok]
	if !ok {
		return nil
	}

	if skipUnassignableShift(info, n) {
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

func skipUnassignableShift(info *types.Info, n *ast.AssignStmt) bool {
	if n.Tok != token.SHL_ASSIGN && n.Tok != token.SHR_ASSIGN {
		return false
	}
	if info == nil || len(n.Lhs) == 0 || len(n.Rhs) == 0 {
		return false
	}
	lhsType := info.TypeOf(n.Lhs[0])
	rhsType := info.TypeOf(n.Rhs[0])
	if lhsType == nil || rhsType == nil {
		return false
	}
	if !types.AssignableTo(rhsType, lhsType) {
		return true
	}
	return constantOverflowsType(info.Types[n.Rhs[0]].Value, lhsType)
}

var integerMax = map[types.BasicKind]uint64{
	types.Int8:   math.MaxInt8,
	types.Int16:  math.MaxInt16,
	types.Int32:  math.MaxInt32,
	types.Int64:  math.MaxInt64,
	types.Int:    math.MaxInt,
	types.Uint8:  math.MaxUint8,
	types.Uint16: math.MaxUint16,
	types.Uint32: math.MaxUint32,
}

func constantOverflowsType(val constant.Value, t types.Type) bool {
	if val == nil {
		return false
	}
	max, ok := maxOfIntegerType(t)
	if !ok {
		return false
	}
	x, _ := constant.Uint64Val(val)
	return x > max
}

func maxOfIntegerType(t types.Type) (uint64, bool) {
	basic, ok := t.Underlying().(*types.Basic)
	if !ok {
		return 0, false
	}
	max, ok := integerMax[basic.Kind()]
	return max, ok
}
