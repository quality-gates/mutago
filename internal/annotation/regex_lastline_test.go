package annotation

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// Regression for issue #75: findLinesMatchingRegex dropped the final line of a
// file that has no trailing newline, because bufio.Reader.ReadString returns
// that line together with io.EOF and the loop broke on any error. A regex
// matching only the last line was therefore silently ignored.
func TestFindLinesMatchingRegex_NoTrailingNewline(t *testing.T) {
	dir := t.TempDir()
	src := "package p\n\nvar secret = 42" // line 3 matches "secret"; NO trailing newline
	path := filepath.Join(dir, "p.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	r := &RegexAnnotation{}
	re := regexp.MustCompile(`secret`)
	lines, _ := r.findLinesMatchingRegex(path, re)

	found := false
	for _, ln := range lines {
		if ln == 3 {
			found = true
		}
	}
	if !found {
		t.Errorf("BUG: line 3 (matching regex, last line without trailing newline) not reported; got %v", lines)
	}

	// Control: same content WITH a trailing newline is reported correctly.
	path2 := filepath.Join(dir, "p2.go")
	if err := os.WriteFile(path2, []byte(src+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	lines2, _ := r.findLinesMatchingRegex(path2, re)
	found2 := false
	for _, ln := range lines2 {
		if ln == 3 {
			found2 = true
		}
	}
	if !found2 {
		t.Errorf("control case (trailing newline) unexpectedly failed; got %v", lines2)
	}
}
