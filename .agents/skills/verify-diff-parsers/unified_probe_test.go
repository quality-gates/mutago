package parser

import (
	"fmt"
	"strings"
	"testing"
)

func TestVerifyUnifiedDiff(t *testing.T) {
	alphabet := []byte{' ', '-', '+'}
	var check func([]byte, int)
	check = func(script []byte, depth int) {
		oldCount, newCount := 0, 0
		first := -1
		for i, op := range script {
			if op != '+' {
				oldCount++
			}
			if op != '-' {
				newCount++
			}
			if op != ' ' && first == -1 {
				first = i
			}
		}
		oldStart := 3
		if oldCount == 0 {
			oldStart = 2
		}
		expected := int64(0)
		if first >= 0 {
			expected = 3
			for _, op := range script[:first] {
				if op != '+' {
					expected++
				}
			}
		}
		var body, markedBody strings.Builder
		for _, op := range script {
			fmt.Fprintf(&body, "%cvalue\n", op)
			fmt.Fprintf(&markedBody, "%cvalue\n\\ No newline at end of file\n", op)
		}
		header := fmt.Sprintf("@@ -%d,%d +4,%d @@\n", oldStart, oldCount, newCount)
		for _, bodyText := range []string{body.String(), markedBody.String()} {
			for _, newline := range []string{"\n", "\r\n"} {
				diff := strings.ReplaceAll("--- a/x\n+++ b/x\n"+header+bodyText, "\n", newline)
				if got := FindOriginalStartLine([]byte(diff)); got != expected {
					t.Fatalf("script %q: got %d, want %d", script, got, expected)
				}
			}
		}
		if len(script) > 0 {
			badHeader := fmt.Sprintf("@@ -%d,%d +4,%d @@\n", oldStart, oldCount, newCount+1)
			if got := FindOriginalStartLine([]byte(badHeader + body.String())); got != 0 {
				t.Fatalf("script %q with incomplete declared range: got %d, want 0", script, got)
			}
		}
		if depth == 0 {
			return
		}
		for _, op := range alphabet {
			check(append(append([]byte(nil), script...), op), depth-1)
		}
	}
	check(nil, 5)
}
