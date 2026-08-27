package gitdiff

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// sampleDiff covers: added lines, modified lines, pure deletion, second file.
const sampleDiff = `diff --git a/pkg/foo/foo.go b/pkg/foo/foo.go
index abc..def 100644
--- a/pkg/foo/foo.go
+++ b/pkg/foo/foo.go
@@ -3,0 +4,2 @@
+	if err != nil {
+		return err
@@ -20,1 +22,1 @@
-	x := 1
+	x := 2
diff --git a/pkg/bar/bar.go b/pkg/bar/bar.go
index 111..222 100644
--- a/pkg/bar/bar.go
+++ b/pkg/bar/bar.go
@@ -10,3 +10,0 @@
-	y := 1
-	z := 2
-	w := 3
diff --git a/pkg/baz/baz.go b/pkg/baz/baz.go
index aaa..bbb 100644
--- a/pkg/baz/baz.go
+++ b/pkg/baz/baz.go
@@ -1,0 +2,1 @@
+	return nil
`

func TestParse_HunkRanges(t *testing.T) {
	cl := parse(sampleDiff)
	// foo: +4,2 → [4,5] and +22,1 → [22,22]
	assert.Equal(t, []LineRange{{4, 5}, {22, 22}}, cl["pkg/foo/foo.go"])
	// bar: +10,0 (pure deletion) → no ranges stored
	assert.Nil(t, cl["pkg/bar/bar.go"])
	// baz: +2,1 → [2,2]
	assert.Equal(t, []LineRange{{2, 2}}, cl["pkg/baz/baz.go"])
}

func TestParse_Empty(t *testing.T) {
	assert.Empty(t, parse(""))
}

func TestParse_NoCommaCountDefaultsToOne(t *testing.T) {
	// @@ -5 +5 @@ (no comma → count implicitly 1)
	diff := `+++ b/pkg/foo/foo.go
@@ -5 +5 @@
-old
+new
`
	cl := parse(diff)
	assert.Equal(t, []LineRange{{5, 5}}, cl["pkg/foo/foo.go"])
}

func TestParse_NewFile(t *testing.T) {
	diff := `+++ b/pkg/new/new.go
@@ -0,0 +1,10 @@
+package new
`
	cl := parse(diff)
	assert.Equal(t, []LineRange{{1, 10}}, cl["pkg/new/new.go"])
}

func TestIsLineChanged_InRange(t *testing.T) {
	cl := parse(sampleDiff)
	abs := "/home/user/project/pkg/foo/foo.go"
	assert.True(t, IsLineChanged(cl, abs, 4))
	assert.True(t, IsLineChanged(cl, abs, 5))
	assert.True(t, IsLineChanged(cl, abs, 22))
}

func TestIsLineChanged_OutOfRange(t *testing.T) {
	cl := parse(sampleDiff)
	abs := "/home/user/project/pkg/foo/foo.go"
	assert.False(t, IsLineChanged(cl, abs, 3))
	assert.False(t, IsLineChanged(cl, abs, 6))
	assert.False(t, IsLineChanged(cl, abs, 21))
	assert.False(t, IsLineChanged(cl, abs, 23))
}

func TestIsLineChanged_FileNotInDiff(t *testing.T) {
	cl := parse(sampleDiff)
	assert.False(t, IsLineChanged(cl, "/home/user/project/pkg/other/other.go", 1))
}

func TestIsLineChanged_PureDeletion(t *testing.T) {
	cl := parse(sampleDiff)
	// bar had only deletions → no ranges → false for all lines
	assert.False(t, IsLineChanged(cl, "/home/user/project/pkg/bar/bar.go", 10))
}

func TestIsLineChanged_ZeroLineBypassesFilter(t *testing.T) {
	cl := parse(sampleDiff)
	// line 0 means "unknown position" — never filter it out
	assert.True(t, IsLineChanged(cl, "/home/user/project/pkg/foo/foo.go", 0))
	assert.True(t, IsLineChanged(cl, "/home/user/project/pkg/other/other.go", 0))
}

func TestIsLineChanged_NilChangedLines(t *testing.T) {
	// nil map (filter not active) should not panic
	assert.True(t, IsLineChanged(nil, "/some/file.go", 0))
	// A non-zero line on nil map: file not found → false
	assert.False(t, IsLineChanged(nil, "/some/file.go", 5))
}

func TestIsLineChanged_ExactMatch(t *testing.T) {
	// When absFile equals relPath exactly (no prefix)
	cl := ChangedLines{"pkg/foo/foo.go": {{1, 10}}}
	assert.True(t, IsLineChanged(cl, "pkg/foo/foo.go", 5))
	assert.False(t, IsLineChanged(cl, "pkg/foo/foo.go", 11))
}

func TestIsRelativeLineChangedCoalescesAndSearchesRanges(t *testing.T) {
	cl := ChangedLines{"pkg/foo.go": {{10, 12}, {4, 6}, {6, 9}, {20, 20}}}
	cl.Coalesce()
	assert.Equal(t, []LineRange{{4, 12}, {20, 20}}, cl["pkg/foo.go"])
	assert.True(t, IsRelativeLineChanged(cl, "pkg/foo.go", 11))
	assert.False(t, IsRelativeLineChanged(cl, "pkg/foo.go", 13))
}
