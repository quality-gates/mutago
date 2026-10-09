package engine

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"github.com/quality-gates/mutago/v2"
	"github.com/quality-gates/mutago/v2/internal/baseline"
	"github.com/quality-gates/mutago/v2/mutator"
)

// mutantIdentities returns the stable ID of every edit mutator m makes in file,
// keyed by mutationEdit.key.
//
// An ID hashes the changed line text (baseline.MutantIDAt), so it survives
// line shifts. Edits with the same text are told apart by their occurrence
// index in source order. The index counts every edit of m in the whole file,
// before annotations, --match, scope rules, and cross-mutator dedup drop any,
// so a filtered or single-mutator run gives an edit the same ID as a full run.
func mutantIdentities(pkg *types.Package, info *types.Info, fset *token.FileSet, file ast.Node, original []byte, relFile, mutatorName string, m mutator.Mutator) map[string]string {
	ids := make(map[string]string)
	occurrences := make(map[string]int)
	changed := mutago.MutateWalkWithPositions(pkg, info, file, m)
	for mutation := range changed {
		if key, text, ok := identityText(fset, mutation, original); ok {
			if _, dup := ids[key]; !dup {
				ids[key] = baseline.MutantIDAt(relFile, mutatorName, text, occurrences[text])
				occurrences[text]++
			}
		}
		changed <- mutago.PositionedMutation{}
		<-changed
		changed <- mutago.PositionedMutation{}
	}
	return ids
}

// identityText returns the edit key and changed line text of a pending mutation.
// An edit that cannot be captured gets no ID; the filtered walk reports it.
func identityText(fset *token.FileSet, mutation mutago.PositionedMutation, original []byte) (key, text string, ok bool) {
	edit, err := captureMutationEdit(fset, mutation.Node, mutation.Start, mutation.End, original)
	if err != nil {
		return "", "", false
	}
	mutated, err := edit.materialize(original)
	if err != nil {
		return "", "", false
	}
	return edit.key(), formatChangedLines(changedLines(original, mutated)), true
}

// key identifies one edit within one file.
func (e mutationEdit) key() string {
	return fmt.Sprintf("%d:%d:%s", e.start, e.end, e.replacement)
}

func formatChangedLines(removed, added []string) string {
	var b strings.Builder
	writeDiffLines(&b, "-", removed)
	writeDiffLines(&b, "+", added)
	return b.String()
}

func writeDiffLines(b *strings.Builder, prefix string, lines []string) {
	for _, line := range lines {
		b.WriteString(prefix)
		b.WriteString(line)
		b.WriteByte('\n')
	}
}

// changedLines returns removed and added line text.
// The sequences match the "-" and "+" body lines of diff -u. MutantID ignores
// headers and context, so only these sequences affect the stable ID.
// Like diff, it drops the common leading and trailing lines first, so the
// quadratic LCS covers only the edited region.
func changedLines(original, mutated []byte) (removed, added []string) {
	before := splitDiffLines(original)
	after := splitDiffLines(mutated)
	for len(before) > 0 && len(after) > 0 && before[0] == after[0] {
		before, after = before[1:], after[1:]
	}
	for len(before) > 0 && len(after) > 0 && before[len(before)-1] == after[len(after)-1] {
		before, after = before[:len(before)-1], after[:len(after)-1]
	}
	return walkLineDiff(before, after, lineLCS(before, after))
}

func splitDiffLines(data []byte) []string {
	if len(data) == 0 {
		return nil
	}
	text := string(data)
	parts := strings.Split(text, "\n")
	if strings.HasSuffix(text, "\n") {
		parts = parts[:len(parts)-1]
	}
	return parts
}

func lineLCS(before, after []string) [][]int {
	n, k := len(before), len(after)
	dp := make([][]int, n+1)
	for i := range dp {
		dp[i] = make([]int, k+1)
	}
	for i := n - 1; i >= 0; i-- {
		fillLCSRow(dp, before, after, i)
	}
	return dp
}

func fillLCSRow(dp [][]int, before, after []string, i int) {
	for j := len(after) - 1; j >= 0; j-- {
		if before[i] == after[j] {
			dp[i][j] = dp[i+1][j+1] + 1
			continue
		}
		dp[i][j] = dp[i+1][j]
		if dp[i][j+1] > dp[i][j] {
			dp[i][j] = dp[i][j+1]
		}
	}
}

func walkLineDiff(before, after []string, dp [][]int) (removed, added []string) {
	i, j := 0, 0
	for i < len(before) && j < len(after) {
		if before[i] == after[j] {
			i++
			j++
			continue
		}
		if dp[i+1][j] >= dp[i][j+1] {
			removed = append(removed, before[i])
			i++
			continue
		}
		added = append(added, after[j])
		j++
	}
	removed = append(removed, before[i:]...)
	added = append(added, after[j:]...)
	return removed, added
}
