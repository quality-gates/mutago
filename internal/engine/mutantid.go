package engine

import (
	"strings"

	"github.com/quality-gates/mutago/v2/internal/baseline"
)

// mutantIDForEdit returns the stable ID of an edit without writing a file.
// The ID matches baseline.MutantID of a diff -u between the original file and
// the materialized edit, so an id copied from a report still selects that mutant.
func mutantIDForEdit(relFile, mutatorName string, original []byte, edit mutationEdit) (string, error) {
	mutated, err := edit.materialize(original)
	if err != nil {
		return "", err
	}
	removed, added := changedLines(original, mutated)
	return baseline.MutantID(relFile, mutatorName, formatChangedLines(removed, added)), nil
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
func changedLines(original, mutated []byte) (removed, added []string) {
	before := splitDiffLines(original)
	after := splitDiffLines(mutated)
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
