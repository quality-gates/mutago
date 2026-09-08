package astutil

import (
	"go/ast"
	"go/token"
)

// IsTerminatingList reports whether list is a terminating statement list
// under the Go spec rules for terminating statements.
func IsTerminatingList(list []ast.Stmt) bool {
	return isTerminatingList(list, "")
}

func isTerminating(s ast.Stmt, label string) bool {
	return terminatingSimple(s, label) || terminatingCompound(s, label)
}

func terminatingSimple(s ast.Stmt, label string) bool {
	switch s := s.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return s.Tok == token.GOTO || s.Tok == token.FALLTHROUGH
	case *ast.LabeledStmt:
		return isTerminating(s.Stmt, s.Label.Name)
	case *ast.BlockStmt:
		return isTerminatingList(s.List, "")
	default:
		return false
	}
}

func terminatingCompound(s ast.Stmt, label string) bool {
	switch s := s.(type) {
	case *ast.IfStmt:
		return terminatingIf(s)
	case *ast.SwitchStmt:
		return terminatingSwitch(s.Body, label)
	case *ast.TypeSwitchStmt:
		return terminatingSwitch(s.Body, label)
	case *ast.SelectStmt:
		return terminatingSelect(s, label)
	case *ast.ForStmt:
		return s.Cond == nil && !hasBreak(s.Body, label, true)
	default:
		return false
	}
}

func terminatingIf(s *ast.IfStmt) bool {
	return s.Else != nil && isTerminating(s.Body, "") && isTerminating(s.Else, "")
}

func isTerminatingList(list []ast.Stmt, label string) bool {
	for i := len(list) - 1; i >= 0; i-- {
		if _, ok := list[i].(*ast.EmptyStmt); ok {
			continue
		}
		return isTerminating(list[i], label)
	}
	return false
}

func terminatingSwitch(body *ast.BlockStmt, label string) bool {
	if body == nil {
		return false
	}
	hasDefault := false
	for _, s := range body.List {
		cc, ok := s.(*ast.CaseClause)
		if !ok {
			return false
		}
		if cc.List == nil {
			hasDefault = true
		}
		if !isTerminatingList(cc.Body, "") || hasBreakList(cc.Body, label, true) {
			return false
		}
	}
	return hasDefault
}

func terminatingSelect(s *ast.SelectStmt, label string) bool {
	if s.Body == nil {
		return false
	}
	for _, stmt := range s.Body.List {
		cc, ok := stmt.(*ast.CommClause)
		if !ok {
			return false
		}
		if !isTerminatingList(cc.Body, "") || hasBreakList(cc.Body, label, true) {
			return false
		}
	}
	return true
}

func hasBreak(s ast.Stmt, label string, implicit bool) bool {
	return hasBreakSimple(s, label, implicit) || hasBreakNested(s, label, implicit)
}

func hasBreakSimple(s ast.Stmt, label string, implicit bool) bool {
	switch s := s.(type) {
	case *ast.BranchStmt:
		return branchBreaks(s, label, implicit)
	case *ast.LabeledStmt:
		return hasBreak(s.Stmt, label, implicit)
	case *ast.BlockStmt:
		return hasBreakList(s.List, label, implicit)
	case *ast.CaseClause:
		return hasBreakList(s.Body, label, implicit)
	case *ast.CommClause:
		return hasBreakList(s.Body, label, implicit)
	default:
		return false
	}
}

func hasBreakNested(s ast.Stmt, label string, implicit bool) bool {
	switch s := s.(type) {
	case *ast.IfStmt:
		return hasBreak(s.Body, label, implicit) || (s.Else != nil && hasBreak(s.Else, label, implicit))
	case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt, *ast.ForStmt, *ast.RangeStmt:
		return labeledHasBreak(s, label)
	default:
		return false
	}
}

func branchBreaks(s *ast.BranchStmt, label string, implicit bool) bool {
	if s.Tok != token.BREAK {
		return false
	}
	if s.Label == nil {
		return implicit
	}
	return s.Label.Name == label
}

func labeledHasBreak(s ast.Stmt, label string) bool {
	if label == "" {
		return false
	}
	switch s := s.(type) {
	case *ast.SwitchStmt:
		return hasBreak(s.Body, label, false)
	case *ast.TypeSwitchStmt:
		return hasBreak(s.Body, label, false)
	case *ast.SelectStmt:
		return hasBreak(s.Body, label, false)
	case *ast.ForStmt:
		return hasBreak(s.Body, label, false)
	case *ast.RangeStmt:
		return hasBreak(s.Body, label, false)
	default:
		return false
	}
}

func hasBreakList(list []ast.Stmt, label string, implicit bool) bool {
	for _, s := range list {
		if hasBreak(s, label, implicit) {
			return true
		}
	}
	return false
}
