package engine

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"testing"

	"github.com/quality-gates/mutago/v2"
	"github.com/quality-gates/mutago/v2/internal/coverage"
	"github.com/quality-gates/mutago/v2/internal/models"
	"github.com/quality-gates/mutago/v2/mutator"
)

func parseClauseHeadFixture(t *testing.T, src string) (*token.FileSet, *ast.File) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "sample.go", src, 0)
	if err != nil {
		t.Fatalf("failed to parse fixture: %v", err)
	}
	return fset, file
}

// TestClauseHeadAnchorsSurviveSelectClauseRemoval reads the heads, then removes
// every select clause from the tree, as select/case-remove and
// select/default-remove do before the anchor is looked up (#326).
func TestClauseHeadAnchorsSurviveSelectClauseRemoval(t *testing.T) {
	src := `package sample

func Poll(ch chan int) int {
	select {
	case n := <-ch:
		return n
	default:
		return 0
	}
}
`
	fset, file := parseClauseHeadFixture(t, src)
	heads := collectClauseHeads(file)

	var sel *ast.SelectStmt
	var keywords []token.Pos
	ast.Inspect(file, func(n ast.Node) bool {
		if s, ok := n.(*ast.SelectStmt); ok {
			sel = s
			for _, c := range s.Body.List {
				keywords = append(keywords, c.(*ast.CommClause).Case)
			}
		}
		return true
	})
	if len(keywords) != 2 {
		t.Fatalf("got %d select clauses, want 2", len(keywords))
	}
	sel.Body.List = nil

	for _, kw := range keywords {
		if got := heads.anchor(kw); got != sel.Pos() {
			t.Errorf("clause keyword on line %d: got anchor %v, want select on line %d",
				fset.Position(kw).Line, fset.Position(got), fset.Position(sel.Pos()).Line)
		}
	}
}

// TestClauseHeadAnchorsSurviveRightOperandRemoval replaces a case condition's
// right operand with a position-less identifier, as expression/remove does.
func TestClauseHeadAnchorsSurviveRightOperandRemoval(t *testing.T) {
	src := `package sample

func Open(c byte) bool {
	switch {
	case c == '{' || c == '[':
		return true
	}
	return false
}
`
	fset, file := parseClauseHeadFixture(t, src)
	heads := collectClauseHeads(file)

	var sw *ast.SwitchStmt
	var or *ast.BinaryExpr
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SwitchStmt:
			sw = x
		case *ast.BinaryExpr:
			if x.Op == token.LOR {
				or = x
			}
		}
		return true
	})
	right := or.Y.Pos()
	or.Y = ast.NewIdent("false")

	if got := heads.anchor(right); got != sw.Pos() {
		t.Errorf("right operand: got anchor %v, want switch on line %d",
			fset.Position(got), fset.Position(sw.Pos()).Line)
	}
}

// TestClauseHeadAnchorsPreferInnermostHead nests a switch inside a function
// literal in an outer case condition. The inner head belongs to the inner
// switch; the rest of the outer head, up to its colon, to the outer one.
func TestClauseHeadAnchorsPreferInnermostHead(t *testing.T) {
	src := `package sample

func Nested(x, y int) bool {
	switch {
	case func() bool {
		switch {
		case y > 0:
			return true
		}
		return false
	}() && x > 0:
		return true
	}
	return false
}
`
	fset, file := parseClauseHeadFixture(t, src)
	heads := collectClauseHeads(file)

	var switches []*ast.SwitchStmt
	var outerColon token.Pos
	conditions := map[string]token.Pos{}
	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SwitchStmt:
			switches = append(switches, x)
			if outerColon == token.NoPos {
				outerColon = x.Body.List[0].(*ast.CaseClause).Colon
			}
		case *ast.BinaryExpr:
			if x.Op == token.GTR {
				conditions[x.X.(*ast.Ident).Name] = x.OpPos
			}
		}
		return true
	})
	outer, inner := switches[0], switches[1]

	for _, tc := range []struct {
		name string
		pos  token.Pos
		want *ast.SwitchStmt
	}{
		{"inner condition", conditions["y"], inner},
		{"outer condition after the literal", conditions["x"], outer},
		{"outer colon", outerColon, outer},
	} {
		if got := heads.anchor(tc.pos); got != tc.want.Pos() {
			t.Errorf("%s: got anchor %v, want switch on line %d",
				tc.name, fset.Position(got), fset.Position(tc.want.Pos()).Line)
		}
	}
}

const clauseHeadProbe = `package probe

func Class(c byte) string {
	switch {
	case c == '{' || c == '[':
		return "open"
	case c == '}':
		return "close"
	}
	return "other"
}

func Offer(ch chan int, v int) bool {
	select {
	case ch <- v:
		return true
	default:
		return false
	}
}
`

// clauseHeadProbeProfile has the Go 1.27 shape: each clause's block starts at
// its first body statement, so clause head lines 5, 7, 15 and 17 are covered
// only through the switch or select line.
const clauseHeadProbeProfile = `mode: set
probe/probe.go:3.27,4.9 1 1
probe/probe.go:6.3,6.16 1 1
probe/probe.go:8.3,8.17 1 1
probe/probe.go:10.2,10.16 1 1
probe/probe.go:13.37,14.9 1 1
probe/probe.go:16.3,16.14 1 1
probe/probe.go:18.3,18.15 1 1
`

// TestCoverageRunsClauseHeadMutationsWithGo127Profile drives every registered
// mutator over the probe the way the engine does: the coverage scope is built
// while the mutation is applied to the tree (#326).
func TestCoverageRunsClauseHeadMutationsWithGo127Profile(t *testing.T) {
	dir := t.TempDir()
	absFile := filepath.Join(dir, "probe.go")
	profilePath := filepath.Join(dir, "coverage.out")
	if err := os.WriteFile(profilePath, []byte(clauseHeadProbeProfile), 0o600); err != nil {
		t.Fatalf("failed to write profile: %v", err)
	}
	profile, err := coverage.ParseProfile(profilePath, "probe")
	if err != nil {
		t.Fatalf("failed to parse profile: %v", err)
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, absFile, clauseHeadProbe, 0)
	if err != nil {
		t.Fatalf("failed to parse probe: %v", err)
	}
	info := &types.Info{
		Types:      map[ast.Expr]types.TypeAndValue{},
		Defs:       map[*ast.Ident]types.Object{},
		Uses:       map[*ast.Ident]types.Object{},
		Implicits:  map[ast.Node]types.Object{},
		Selections: map[*ast.SelectorExpr]*types.Selection{},
		Scopes:     map[ast.Node]*types.Scope{},
	}
	pkg, err := (&types.Config{}).Check("probe", fset, []*ast.File{file}, info)
	if err != nil {
		t.Fatalf("failed to type-check probe: %v", err)
	}

	r := &mutationRun{roots: sourceRoots{module: dir}}
	fc := &fileContext{pkg: pkg, info: info, fset: fset, src: file, absFile: absFile, coverProfile: profile, clauseHeads: collectClauseHeads(file)}

	headLines := map[int]bool{5: true, 7: true, 15: true, 17: true}
	seen := map[string]int{}
	for _, name := range mutator.List() {
		m, err := mutator.New(name)
		if err != nil {
			t.Fatalf("mutator %s: %v", name, err)
		}
		changed := mutago.MutateWalkWithPositions(pkg, info, file, m)
		for mutation := range changed {
			line := fset.Position(mutation.Position).Line
			if headLines[line] {
				seen[name]++
				if !clauseHeadMutationCovered(r, fc, mutation) {
					t.Errorf("%s on clause head line %d is NOT COVERED, want it run", name, line)
				}
			}
			changed <- mutago.PositionedMutation{}
			<-changed
			changed <- mutago.PositionedMutation{}
		}
	}
	for _, name := range []string{"select/case-remove", "select/default-remove", "expression/remove"} {
		if seen[name] == 0 {
			t.Errorf("expected %s mutants on clause head lines, got none", name)
		}
	}
}

func clauseHeadMutationCovered(r *mutationRun, fc *fileContext, mutation mutago.PositionedMutation) bool {
	pos := fc.fset.Position(mutation.Position)
	var mutant models.Mutant
	mutant.Mutator.Location = &models.MutationLocation{
		Physical: models.Coordinate{Filename: fc.absFile, Line: int64(pos.Line)},
		Coverage: models.Coordinate{Filename: pos.Filename, Line: int64(pos.Line)},
	}
	job := execJob{
		mutant:   mutant,
		covScope: buildJobCoverageScope(r, fc, mutation),
		source:   mutationSource{absFile: fc.absFile, moduleRoot: r.roots.module},
	}
	return mutationCovered(job)
}
