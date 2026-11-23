package gitdiff

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/quality-gates/mutago/v2/internal/sourcepath"
)

// LineRange is an inclusive [Start, End] range of line numbers.
type LineRange struct{ Start, End int }

// ChangedLines maps relative file paths (from repo root, forward-slash separated)
// to the line ranges that were added or modified in the diff.
// A nil ChangedLines means "no filter" — all mutations are run.
type ChangedLines map[string][]LineRange

// ParseChangedLines diffs the working tree against the merge-base of <base> and
// HEAD, and returns each modified file's changed line ranges. Deleted-only hunks
// (count == 0) are excluded — there is nothing to mutate on a removed line.
//
// Diffing against the merge-base (rather than the tip of <base>) reports only
// the changes introduced on the current branch — exactly what a pull request
// shows — while still including uncommitted working-tree changes. A plain
// two-dot `git diff <base>` would also report commits that landed on <base>
// after the branch point, wrongly attributing those unrelated changes to the
// feature branch when it is behind its target.
//
// If no merge-base can be found (e.g. unrelated histories), it falls back to
// diffing against <base> directly.
func ParseChangedLines(base string) (ChangedLines, error) {
	diffBase := base
	if mb, err := exec.Command("git", "merge-base", base, "HEAD").Output(); err == nil {
		if s := strings.TrimSpace(string(mb)); s != "" {
			diffBase = s
		}
	}
	out, err := exec.Command("git", "diff", "--unified=0", diffBase).Output()
	if err != nil {
		return nil, fmt.Errorf("git diff --unified=0 %s: %w", diffBase, err)
	}
	return parse(string(out)), nil
}

var (
	newFileRe    = regexp.MustCompile(`^\+\+\+ b/(.+)$`)
	hunkHeaderRe = regexp.MustCompile(`^@@ -\S+ \+(\d+)(?:,(\d+))? @@`)
)

type changedLinesParser struct {
	currentFile string
	lines       ChangedLines
}

func (p *changedLinesParser) consume(line string) {
	if m := newFileRe.FindStringSubmatch(line); m != nil {
		p.currentFile = filepath.ToSlash(m[1])
		return
	}
	if p.currentFile == "" {
		return
	}
	m := hunkHeaderRe.FindStringSubmatch(line)
	if m == nil {
		return
	}
	start, err := strconv.Atoi(m[1])
	if err != nil {
		return
	}
	count := 1
	if m[2] != "" {
		count, err = strconv.Atoi(m[2])
		if err != nil {
			return
		}
	}
	if count == 0 {
		return
	}
	p.lines[p.currentFile] = append(p.lines[p.currentFile], LineRange{Start: start, End: start + count - 1})
}

func parse(output string) ChangedLines {
	state := changedLinesParser{lines: make(ChangedLines)}
	for line := range strings.Lines(output) {
		line = strings.TrimSuffix(line, "\n")
		state.consume(strings.TrimSuffix(line, "\r"))
	}
	state.lines.Coalesce()
	return state.lines
}

// Coalesce sorts and merges overlapping or adjacent ranges for each file.
func (cl ChangedLines) Coalesce() {
	for file, ranges := range cl {
		if len(ranges) < 2 {
			continue
		}
		sort.Slice(ranges, func(i, j int) bool { return ranges[i].Start < ranges[j].Start })
		merged := ranges[:1]
		for _, current := range ranges[1:] {
			last := &merged[len(merged)-1]
			if current.Start <= last.End+1 {
				if current.End > last.End {
					last.End = current.End
				}
				continue
			}
			merged = append(merged, current)
		}
		cl[file] = merged
	}
}

// IsRelativeLineChanged checks a normalized repository-relative file path
// using direct map lookup and binary search over coalesced ranges.
func IsRelativeLineChanged(cl ChangedLines, relFile string, line int) bool {
	if line == 0 {
		return true
	}
	ranges := cl[filepath.ToSlash(relFile)]
	i := sort.Search(len(ranges), func(i int) bool { return ranges[i].End >= line })
	return i < len(ranges) && ranges[i].Start <= line
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
//
// When more than one key matches (one relPath is a path-suffix of another,
// e.g. "foo/bar.go" and "bar.go"), the longest — most specific — match wins,
// so resolution is deterministic regardless of map iteration order.
func IsLineChanged(cl ChangedLines, absFile string, line int) bool {
	if line == 0 {
		return true
	}
	absFile = filepath.ToSlash(absFile)
	best, found := sourcepath.LongestSuffixKey(absFile, cl)
	if !found {
		return false // file not in diff → unchanged → skip
	}
	return IsRelativeLineChanged(ChangedLines{best: cl[best]}, best, line)
}
