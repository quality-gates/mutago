package gitdiff

import "testing"

// FuzzParse exercises the unified-diff parser with arbitrary git-diff output.
// The parser must never panic and must return internally-consistent ranges
// (Start <= End) for any input.
func FuzzParse(f *testing.F) {
	seeds := []string{
		"",
		"+++ b/foo.go\n@@ -1 +1 @@",
		"+++ b/foo.go\n@@ -1,0 +1,3 @@",
		"+++ b/foo.go\n@@ -1,2 +1,0 @@",
		"+++ b/a.go\n@@ -10,5 +20,7 @@ func main()",
		"+++ b/" + string(rune(0)) + "\n@@ -1 +1 @@",
		"@@ -1 +1 @@\n+++ b/x.go",
		"+++ b/x.go\n@@ -1 +99999999999999999999 @@",
		"+++ b/x.go\n@@ -1 +0 @@",
		"+++ b/x.go\n@@ -1 + @@",
	}
	for _, s := range seeds {
		f.Add(s)
	}

	f.Fuzz(func(t *testing.T, output string) {
		cl := parse(output)
		for file, ranges := range cl {
			for _, r := range ranges {
				if r.Start > r.End {
					t.Fatalf("inverted range for %q: Start=%d End=%d (input=%q)",
						file, r.Start, r.End, output)
				}
			}
		}
		// IsLineChanged must not panic on any parsed result.
		_ = IsLineChanged(cl, "/abs/path/foo.go", 1)
	})
}
