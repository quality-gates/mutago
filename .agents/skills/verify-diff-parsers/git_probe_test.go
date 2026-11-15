package gitdiff

import (
	"strings"
	"testing"
)

type verifyEvent struct {
	text         string
	file         string
	start, count int
}

func TestVerifyChangedLines(t *testing.T) {
	events := []verifyEvent{
		{text: "+++ b/a.go\n", file: "a.go"},
		{text: "+++ b/nested/b.go\n", file: "nested/b.go"},
		{text: "@@ -1,1 +2,0 @@\n", start: 2},
		{text: "@@ -1,1 +2 @@\n", start: 2, count: 1},
		{text: "@@ -1,1 +3,2 @@\n", start: 3, count: 2},
		{text: "irrelevant\n"},
	}
	var check func([]verifyEvent, int)
	check = func(sequence []verifyEvent, depth int) {
		var input strings.Builder
		current := ""
		expected := map[string]map[int]bool{}
		for _, event := range sequence {
			input.WriteString(event.text)
			if event.file != "" {
				current = event.file
				continue
			}
			if current == "" || event.count == 0 {
				continue
			}
			if expected[current] == nil {
				expected[current] = map[int]bool{}
			}
			for line := event.start; line < event.start+event.count; line++ {
				expected[current][line] = true
			}
		}
		actual := parse(input.String())
		if len(actual) != len(expected) {
			t.Fatalf("sequence %q: files %v, want %v", input.String(), actual, expected)
		}
		for _, file := range []string{"a.go", "nested/b.go"} {
			for line := 1; line <= 5; line++ {
				if got, want := IsRelativeLineChanged(actual, file, line), expected[file][line]; got != want {
					t.Fatalf("sequence %q: %s:%d got %t, want %t (%v)", input.String(), file, line, got, want, actual)
				}
			}
		}
		if depth == 0 {
			return
		}
		for _, event := range events {
			check(append(append([]verifyEvent(nil), sequence...), event), depth-1)
		}
	}
	check(nil, 5)
}
