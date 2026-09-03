package loop

import (
	"go/ast"
	"go/token"
	"go/types"

	"github.com/quality-gates/mutago/v2/mutator"
)

func init() {
	mutator.Register("loop/break", MutatorLoopBreak)
}

// MutatorLoopBreak implements a mutator to change continue to break and break to continue in loops.
func MutatorLoopBreak(_ *types.Package, _ *types.Info, node ast.Node) []mutator.Mutation {
	var body *ast.BlockStmt
	switch n := node.(type) {
	case *ast.ForStmt:
		body = n.Body
	case *ast.RangeStmt:
		body = n.Body
	default:
		return nil
	}

	if body == nil {
		return nil
	}

	c := &loopBranchCollector{}
	ast.Walk(c, body)
	return c.mutations
}

type loopBranchCollector struct {
	mutations           []mutator.Mutation
	switchOrSelectDepth int
}

func (c *loopBranchCollector) Visit(node ast.Node) ast.Visitor {
	if node == nil {
		return nil
	}
	switch n := node.(type) {
	case *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit:
		return nil
	case *ast.SwitchStmt:
		c.visitSwitch(n)
		return nil
	case *ast.TypeSwitchStmt:
		c.visitTypeSwitch(n)
		return nil
	case *ast.SelectStmt:
		c.visitSelect(n)
		return nil
	case *ast.BranchStmt:
		c.visitBranch(n)
		return nil
	}
	return c
}

func (c *loopBranchCollector) visitSwitch(n *ast.SwitchStmt) {
	if n.Init != nil {
		ast.Walk(c, n.Init)
	}
	if n.Tag != nil {
		ast.Walk(c, n.Tag)
	}
	c.walkScoped(n.Body)
}

func (c *loopBranchCollector) visitTypeSwitch(n *ast.TypeSwitchStmt) {
	if n.Init != nil {
		ast.Walk(c, n.Init)
	}
	if n.Assign != nil {
		ast.Walk(c, n.Assign)
	}
	c.walkScoped(n.Body)
}

func (c *loopBranchCollector) visitSelect(n *ast.SelectStmt) {
	c.walkScoped(n.Body)
}

func (c *loopBranchCollector) walkScoped(body ast.Node) {
	if body != nil {
		c.switchOrSelectDepth++
		ast.Walk(c, body)
		c.switchOrSelectDepth--
	}
}

func (c *loopBranchCollector) visitBranch(n *ast.BranchStmt) {
	if n.Label != nil {
		return
	}
	if n.Tok == token.CONTINUE {
		c.addMutation(n, token.BREAK)
	} else if n.Tok == token.BREAK && c.switchOrSelectDepth == 0 {
		c.addMutation(n, token.CONTINUE)
	}
}

func (c *loopBranchCollector) addMutation(n *ast.BranchStmt, target token.Token) {
	original := n.Tok
	c.mutations = append(c.mutations, mutator.Mutation{
		Position: n.TokPos,
		Change:   func() { n.Tok = target },
		Reset:    func() { n.Tok = original },
	})
}
