package gitdiff

import (
	"bufio"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// LineRange is an inclusive [Start, End] range of line numbers.
type LineRange struct{ Start, End int }

// ChangedLines maps relative file paths (from repo root, forward-slash separated)
// to the line ranges that were added or modified in the diff.
// A nil ChangedLines means "no filter" — all mutations are run.
type ChangedLines map[string][]LineRange

// ParseChangedLines runs `git diff --unified=0 <base>` and returns each
// modified file's changed line ranges. Deleted-only hunks (count == 0) are
// excluded — there is nothing to mutate on a removed line.
func ParseChangedLines(base string) (ChangedLines, error) {
	out, err := exec.Command("git", "diff", "--unified=0", base).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --unified=0 %s: %w", base, err)
	}
	return parse(string(out)), nil
}

var (
	newFileRe    = regexp.MustCompile(`^\+\+\+ b/(.+)$`)
	hunkHeaderRe = regexp.MustCompile(`^@@ -\S+ \+(\d+)(?:,(\d+))? @@`)
)

func parse(output string) ChangedLines {
	cl := make(ChangedLines)
	var cur string

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		if m := newFileRe.FindStringSubmatch(line); m != nil {
			cur = filepath.ToSlash(m[1])
			continue
		}
		if cur == "" {
			continue
		}
		m := hunkHeaderRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, _ := strconv.Atoi(m[1])
		count := 1
		if m[2] != "" {
			count, _ = strconv.Atoi(m[2])
		}
		if count == 0 {
			continue // pure deletion — nothing to mutate on a removed line
		}
		cl[cur] = append(cl[cur], LineRange{Start: start, End: start + count - 1})
	}
	return cl
}

// IsLineChanged reports whether absFile at the given line falls within any
// changed range in cl.
//
// When line == 0 (position could not be determined), the function returns true
// to avoid filtering mutations at unknown positions.
//
// When a file is present in cl but line falls outside every changed range,
// false is returned — the mutation targets an unchanged line.
// When a file is absent from cl entirely, all of its mutations are filtered.
func IsLineChanged(cl ChangedLines, absFile string, line int) bool {
	if line == 0 {
		return true
	}
	absFile = filepath.ToSlash(absFile)
	for relPath, ranges := range cl {
		if !strings.HasSuffix(absFile, "/"+relPath) && absFile != relPath {
			continue
		}
		for _, r := range ranges {
			if line >= r.Start && line <= r.End {
				return true
			}
		}
		return false // file is in diff but this line was not changed
	}
	return false // file not in diff → unchanged → skip
}
