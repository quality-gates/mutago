package coverage

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPerTestProfileCompilesOnce(t *testing.T) {
	realGo, err := exec.LookPath("go")
	require.NoError(t, err)
	dir := t.TempDir()
	logPath := filepath.Join(dir, "go-calls.log")
	wrapper := filepath.Join(dir, "go")
	script := "#!/bin/sh\nprintf '%s\\n' \"$*\" >> " + logPath + "\nexec " + realGo + " \"$@\"\n"
	require.NoError(t, os.WriteFile(wrapper, []byte(script), 0o700))
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))

	_, err = BuildPerTestProfile(
		"github.com/quality-gates/mutago/v2/internal/coverage/testdata/entrypoints",
		"github.com/quality-gates/mutago/v2", dir, 30, 1, []string{"-trimpath"},
	)
	require.NoError(t, err)
	calls, err := os.ReadFile(logPath)
	require.NoError(t, err)
	callLines := strings.Split(strings.TrimSpace(string(calls)), "\n")
	require.Len(t, callLines, 2, "list once and compile once")
	assert.NotContains(t, callLines[0], "-trimpath", "listing tests must not receive build flags")
	assert.Contains(t, callLines[1], "-trimpath", "compilation must receive build flags")
}

// modulePath is the module root — shorter than the package path so that stripping
// it from coverage entries leaves multi-component relative keys.
const modulePath = "github.com/example"

// profile with covered lines 10-15 in foo.go, uncovered 20-25, and bar.go 5-8.
// All entries live inside the "pkg" sub-package of the module.
const sampleProfile = `mode: set
github.com/example/pkg/foo.go:10.5,15.3 3 1
github.com/example/pkg/foo.go:20.1,25.5 2 0
github.com/example/pkg/bar.go:5.1,8.3 1 2
`

func writeTmpProfile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "cover*.out")
	require.NoError(t, err)
	_, err = f.WriteString(content)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	return f.Name()
}

func TestParseProfile_CoveredLines(t *testing.T) {
	path := writeTmpProfile(t, sampleProfile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)

	absFile := "/home/user/src/github.com/example/pkg/foo.go"

	for l := 10; l <= 15; l++ {
		assert.True(t, p.IsCovered(absFile, l), "line %d should be covered", l)
	}
	// hitCount=0 → uncovered
	for l := 20; l <= 25; l++ {
		assert.False(t, p.IsCovered(absFile, l), "line %d should not be covered", l)
	}
}

func TestParseProfile_UncoveredLines(t *testing.T) {
	path := writeTmpProfile(t, sampleProfile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)

	absFile := "/home/user/src/github.com/example/pkg/foo.go"
	assert.False(t, p.IsCovered(absFile, 1))
	assert.False(t, p.IsCovered(absFile, 100))
}

func TestParseProfile_SecondFile(t *testing.T) {
	path := writeTmpProfile(t, sampleProfile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)

	absFile := "/home/user/src/github.com/example/pkg/bar.go"
	assert.True(t, p.IsCovered(absFile, 5))
	assert.True(t, p.IsCovered(absFile, 7))
	assert.False(t, p.IsCovered(absFile, 9))
}

func TestParseProfile_MissingFile(t *testing.T) {
	p, err := ParseProfile("/nonexistent/file.out", modulePath)
	assert.Error(t, err)
	assert.Nil(t, p)
}

func TestParseProfile_MalformedHitCount(t *testing.T) {
	profile := "mode: set\ngithub.com/example/pkg/foo.go:1.1,5.3 2 notanint\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	assert.Error(t, err)
	assert.Nil(t, p)
}

func TestParseProfile_MalformedStartLine(t *testing.T) {
	profile := "mode: set\ngithub.com/example/pkg/foo.go:abc.1,5.3 2 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	assert.Error(t, err)
	assert.Nil(t, p)
}

func TestParseProfile_MalformedEndLine(t *testing.T) {
	profile := "mode: set\ngithub.com/example/pkg/foo.go:1.1,xyz.3 2 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	assert.Error(t, err)
	assert.Nil(t, p)
}

func TestParseProfile_EmptyProfile(t *testing.T) {
	path := writeTmpProfile(t, "mode: set\n")
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	assert.False(t, p.IsCovered("/any/file.go", 1))
}

func TestParseProfile_ModeOnlyLine(t *testing.T) {
	path := writeTmpProfile(t, "mode: atomic\n")
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	assert.NotNil(t, p)
}

func TestIsCovered_UnknownFile(t *testing.T) {
	path := writeTmpProfile(t, sampleProfile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	assert.False(t, p.IsCovered("/some/unknown/file.go", 10))
}

func TestIsCovered_CachesResolvedPath(t *testing.T) {
	lines := map[int]bool{10: true}
	p := &Profile{coveredLines: map[string]map[int]bool{"pkg/foo.go": lines}}
	absFile := "/workspace/pkg/foo.go"

	assert.True(t, p.IsCovered(absFile, 10))
	p.coveredLines["pkg/foo.go"] = map[int]bool{}
	assert.True(t, p.IsCovered(absFile, 10), "subsequent lookups must reuse the resolved path")
}

func TestIsCovered_CachesMissingPath(t *testing.T) {
	p := &Profile{coveredLines: map[string]map[int]bool{}}
	absFile := "/workspace/pkg/foo.go"

	assert.False(t, p.IsCovered(absFile, 10))
	p.coveredLines["pkg/foo.go"] = map[int]bool{10: true}
	assert.False(t, p.IsCovered(absFile, 10), "subsequent lookups must reuse the cached miss")
}

func TestIsCoveredRelativeUsesDirectProfileKey(t *testing.T) {
	path := writeTmpProfile(t, sampleProfile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	assert.True(t, p.IsCoveredRelative("pkg/foo.go", 10))
	assert.False(t, p.IsCoveredRelative("pkg/foo.go", 20))
}

func TestIsCovered_DifferentPackageSameFilename(t *testing.T) {
	// A file in a different package with the same name must NOT match.
	path := writeTmpProfile(t, sampleProfile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)

	// The profile has "foo.go" relative key; this abs path resolves to a
	// different module's foo.go.
	absFile := "/home/user/src/github.com/other/module/foo.go"
	assert.False(t, p.IsCovered(absFile, 10))
}

func TestParseProfile_MultipleHits(t *testing.T) {
	profile := "mode: count\ngithub.com/example/pkg/a.go:1.1,3.5 1 5\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)

	absFile := "/src/github.com/example/pkg/a.go"
	assert.True(t, p.IsCovered(absFile, 1))
	assert.True(t, p.IsCovered(absFile, 2))
	assert.True(t, p.IsCovered(absFile, 3))
	assert.False(t, p.IsCovered(absFile, 4))
}

func TestParseProfile_NoModulePrefix(t *testing.T) {
	// Coverage entry without the module prefix should be stored as-is.
	profile := "mode: set\nfoo.go:1.1,5.3 2 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, "github.com/some/module")
	require.NoError(t, err)
	assert.True(t, p.IsCovered("/abs/path/to/foo.go", 1))
}

// --- Tests targeting specific escaped mutations in ParseProfile / parseLine ---

// TestParseProfile_ModeLine_SkippedNotParsed verifies that "mode:" lines are
// never treated as coverage data.  If the continue were removed or the if
// condition negated, a valid coverage entry on the same scanner pass would be
// silently dropped (condition negated) or a spurious error would occur.
func TestParseProfile_ModeLine_SkippedNotParsed(t *testing.T) {
	// Has a mode line followed by a real coverage entry.
	profile := "mode: set\ngithub.com/example/pkg/foo.go:5.1,10.3 1 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	// If mode line is NOT skipped and coverage line IS skipped, IsCovered returns false.
	assert.True(t, p.IsCovered("/x/github.com/example/pkg/foo.go", 5))
	assert.True(t, p.IsCovered("/x/github.com/example/pkg/foo.go", 10))
	assert.False(t, p.IsCovered("/x/github.com/example/pkg/foo.go", 11))
}

// TestParseProfile_ScannerErr exercises the scanner.Err() return on EOF with no error.
func TestParseProfile_ScannerErr(t *testing.T) {
	path := writeTmpProfile(t, "mode: set\n")
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	require.NotNil(t, p)
}

// TestIsCovered_ExactPathMatch exercises the absFile == relPath branch of IsCovered.
func TestIsCovered_ExactPathMatch(t *testing.T) {
	profile := "mode: set\npkg/foo.go:1.1,5.3 1 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, "")
	require.NoError(t, err)
	// relPath stored = "pkg/foo.go"; absFile must equal it for the == branch.
	assert.True(t, p.IsCovered("pkg/foo.go", 1))
}

// TestIsCovered_SlashNormalization ensures ToSlash is applied before matching.
func TestIsCovered_SlashNormalization(t *testing.T) {
	profile := "mode: set\ngithub.com/example/pkg/foo.go:3.1,6.3 1 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	// Path with backslashes should still match after ToSlash.
	assert.True(t, p.IsCovered("/x/github.com/example/pkg/foo.go", 3))
}

// TestParseLine_ColonAtPosition0 exercises colonIdx == 0 (not < 0).
// A line starting with ":" has a colon at index 0; parseLine should handle it gracefully
// (wrong field count → return nil, no coverage recorded).
func TestParseLine_ColonAtPosition0(t *testing.T) {
	profile := "mode: set\n:1.1,3.3 1 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err) // silent skip, not an error
	require.NotNil(t, p)
}

// TestParseLine_NoColon exercises colonIdx < 0 (no colon in line).
func TestParseLine_NoColon(t *testing.T) {
	profile := "mode: set\nnocol\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	require.NotNil(t, p)
}

// TestParseLine_WrongFieldCount exercises len(fields) != 3.
func TestParseLine_WrongFieldCount(t *testing.T) {
	// Only 2 fields after the colon instead of 3.
	profile := "mode: set\ngithub.com/example/pkg/foo.go:1.1,5.3 2\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err) // silent skip
	require.NotNil(t, p)
}

// TestParseLine_NoModulePrefixStripped verifies the relFile == rawFile branch when
// the module prefix is absent: file is stored using the raw path.
func TestParseLine_NoModulePrefixStripped(t *testing.T) {
	// File path does NOT start with modulePath, so relFile == rawFile branch executes.
	profile := "mode: set\nother/module/pkg/x.go:1.1,3.3 1 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	// relFile is "other/module/pkg/x.go" (unchanged).
	assert.True(t, p.IsCovered("/abs/other/module/pkg/x.go", 1))
	assert.False(t, p.IsCovered("/abs/other/module/pkg/x.go", 4))
}

// TestParseLine_RelFileToSlash ensures filepath.ToSlash is applied to relFile.
func TestParseLine_RelFileToSlash(t *testing.T) {
	// On all platforms the parser should normalise slashes.
	profile := "mode: set\ngithub.com/example/pkg/sub/y.go:2.1,4.3 1 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	assert.True(t, p.IsCovered("/root/github.com/example/pkg/sub/y.go", 2))
	assert.False(t, p.IsCovered("/root/github.com/example/pkg/sub/y.go", 5))
}

// TestParseProfile_MultiRange verifies multiple lines within a range are all covered.
// This exercises the l <= endLine boundary in the loop (kills numbers/incrementer).
func TestParseProfile_MultiRange(t *testing.T) {
	profile := "mode: set\ngithub.com/example/pkg/z.go:10.1,12.3 1 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	assert.True(t, p.IsCovered("/x/github.com/example/pkg/z.go", 10))
	assert.True(t, p.IsCovered("/x/github.com/example/pkg/z.go", 11))
	assert.True(t, p.IsCovered("/x/github.com/example/pkg/z.go", 12))
	assert.False(t, p.IsCovered("/x/github.com/example/pkg/z.go", 13))
}

// TestParseProfile_SingleLine verifies a range where startLine == endLine.
// Kills numbers/incrementer on the endLine value.
func TestParseProfile_SingleLine(t *testing.T) {
	profile := "mode: set\ngithub.com/example/pkg/w.go:7.1,7.3 1 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	assert.True(t, p.IsCovered("/x/github.com/example/pkg/w.go", 7))
	assert.False(t, p.IsCovered("/x/github.com/example/pkg/w.go", 6))
	assert.False(t, p.IsCovered("/x/github.com/example/pkg/w.go", 8))
}

// --- CountTests tests ---

func TestCountTests_RealPackage(t *testing.T) {
	// arithmetic has multiple test functions; expect a positive count.
	count := CountTests("github.com/quality-gates/mutago/v2/mutator/arithmetic")
	assert.Positive(t, count, "arithmetic package should have tests")
}

func TestCountTests_ExcludesBenchmarks(t *testing.T) {
	count := CountTests("github.com/quality-gates/mutago/v2/internal/coverage/testdata/entrypoints")
	assert.Equal(t, 2, count, "only Test and Fuzz entrypoints run via -run")
}

func TestCountTests_NonexistentPackage(t *testing.T) {
	count := CountTests("github.com/quality-gates/mutago/v2/nonexistent_pkg_xyzzy")
	assert.Zero(t, count, "nonexistent package should return 0")
}

// --- BuildPerTestProfile tests ---

func TestBuildPerTestProfile_RealPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: runs per-test coverage profiling")
	}
	tmp := t.TempDir()
	prof, err := BuildPerTestProfile(
		"github.com/quality-gates/mutago/v2/mutator/arithmetic",
		"github.com/quality-gates/mutago/v2",
		tmp, 30, 1, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, prof, "arithmetic package should produce a per-test profile")

	// Coverage data present for assignment.go.
	var found bool
	for l := 1; l <= 100; l++ {
		if len(prof.CoveringTests("/abs/mutator/arithmetic/assignment.go", l)) > 0 {
			found = true
			break
		}
	}
	assert.True(t, found, "profile should contain coverage data for arithmetic/assignment.go")

	// All covering-test lists across two source files must be sorted.  A
	// range_break mutation on the outer sort loop (line 223) only sorts the
	// first file; one on the inner loop (line 224) only sorts the first line
	// per file.  Checking two files with multiple multi-covered lines kills both.
	for _, file := range []string{
		"/abs/mutator/arithmetic/assignment.go",
		"/abs/mutator/arithmetic/base.go",
	} {
		for l := 1; l <= 150; l++ {
			names := prof.CoveringTests(file, l)
			for i := 1; i < len(names); i++ {
				assert.LessOrEqualf(t, names[i-1], names[i],
					"test names must be sorted at %s:%d", file, l)
			}
		}
	}
}

func TestBuildPerTestProfile_EmptyPackage(t *testing.T) {
	// A package with no tests returns nil, nil.
	tmp := t.TempDir()
	prof, err := BuildPerTestProfile(
		"github.com/quality-gates/mutago/v2/nonexistent_pkg_xyzzy",
		"github.com/quality-gates/mutago/v2",
		tmp, 30, 1, nil,
	)
	assert.Error(t, err)
	assert.Nil(t, prof)
}

func TestBuildPerTestProfile_WorkersZero(t *testing.T) {
	// workers=0 must be normalised to 1; verify the profile is returned without
	// deadlocking and contains actual coverage data. Uses gitdiff (small test suite).
	tmp := t.TempDir()
	prof, err := BuildPerTestProfile(
		"github.com/quality-gates/mutago/v2/internal/gitdiff",
		"github.com/quality-gates/mutago/v2",
		tmp, 30, 0, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, prof)
	var found bool
	for l := 1; l <= 100; l++ {
		if len(prof.CoveringTests("/abs/internal/gitdiff/gitdiff.go", l)) > 0 {
			found = true
			break
		}
	}
	assert.True(t, found, "profile should contain coverage data for gitdiff.go")
}

func TestBuildPerTestProfileForTests_InvalidTempDir(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "not-a-directory")
	require.NoError(t, os.WriteFile(tmpFile, []byte("occupied"), 0o600))

	prof, err := BuildPerTestProfileForTests(
		"github.com/quality-gates/mutago/v2/internal/coverage/testdata/entrypoints",
		"github.com/quality-gates/mutago/v2", tmpFile, 30, 1, nil, []string{"TestAlpha"},
	)
	assert.Error(t, err)
	assert.Nil(t, prof)
}

func TestBuildPerTestProfileForTests_CompileFailure(t *testing.T) {
	prof, err := BuildPerTestProfileForTests(
		"github.com/quality-gates/mutago/v2/nonexistent_pkg_xyzzy",
		"github.com/quality-gates/mutago/v2", t.TempDir(), 30, 1, nil, []string{"TestMissing"},
	)
	assert.ErrorContains(t, err, "compile coverage test binary")
	assert.Nil(t, prof)
}

func TestTestBinaryFlags(t *testing.T) {
	assert.Equal(t, []string{
		"-test.short=true",
		"-test.short=true",
		"-test.count=2",
		"-test.v=true",
		"-test.v=true",
		"-count=3",
	}, testBinaryFlags([]string{
		"-short",
		"--short",
		"-test.count=2",
		"-v",
		"--verbose",
		"-race",
		"-tags=integration",
		"-vet=off",
		"-gcflags=all=-N",
		"-asmflags=all=-trimpath=/tmp",
		"-trimpath",
		"-count=3",
	}))
}

// --- PerTestProfile tests ---

func TestCoveringTests_NilReceiver(t *testing.T) {
	var p *PerTestProfile
	assert.Nil(t, p.CoveringTests("/some/file.go", 10))
}

func TestCoveringTests_ZeroLine(t *testing.T) {
	// Line-0 entry ensures that removing the guard entirely would return
	// non-nil for lineNum=0 instead of nil (map key 0 exists).
	// lineNum=1 assertion kills the numbers/incrementer mutation that
	// widens the guard from lineNum <= 0 to lineNum <= 1.
	p := &PerTestProfile{data: map[string]map[int][]string{
		"pkg/foo.go": {0: {"TestHidden"}, 1: {"TestVisible"}},
	}}
	assert.Nil(t, p.CoveringTests("/abs/pkg/foo.go", 0))
	assert.Equal(t, []string{"TestVisible"}, p.CoveringTests("/abs/pkg/foo.go", 1))
}

func TestCoveringTests_NegativeLine(t *testing.T) {
	p := &PerTestProfile{data: map[string]map[int][]string{
		"pkg/foo.go": {0: {"TestHidden"}, 1: {"TestFoo"}},
	}}
	assert.Nil(t, p.CoveringTests("/abs/pkg/foo.go", -1))
}

func TestCoveringTests_SuffixMatch(t *testing.T) {
	p := &PerTestProfile{data: map[string]map[int][]string{
		"pkg/foo.go": {5: {"TestA", "TestB"}, 6: {"TestA"}},
	}}
	tests := p.CoveringTests("/home/user/go/pkg/foo.go", 5)
	assert.Equal(t, []string{"TestA", "TestB"}, tests)
	assert.Equal(t, []string{"TestA"}, p.CoveringTests("/home/user/go/pkg/foo.go", 6))
}

func TestCoveringTests_ExactMatch(t *testing.T) {
	p := &PerTestProfile{data: map[string]map[int][]string{
		"pkg/foo.go": {3: {"TestC"}},
	}}
	assert.Equal(t, []string{"TestC"}, p.CoveringTests("pkg/foo.go", 3))
}

func TestCoveringTests_NoMatch(t *testing.T) {
	p := &PerTestProfile{data: map[string]map[int][]string{
		"pkg/foo.go": {5: {"TestA"}},
	}}
	assert.Nil(t, p.CoveringTests("/different/path/bar.go", 5))
	assert.Nil(t, p.CoveringTests("/abs/pkg/foo.go", 99))
}

func TestCoveringTests_CachesResolvedPath(t *testing.T) {
	lines := map[int][]string{5: {"TestA"}}
	p := &PerTestProfile{data: map[string]map[int][]string{"pkg/foo.go": lines}}
	absFile := "/workspace/pkg/foo.go"

	assert.Equal(t, []string{"TestA"}, p.CoveringTests(absFile, 5))
	p.data["pkg/foo.go"] = map[int][]string{5: {"TestB"}}
	assert.Equal(t, []string{"TestA"}, p.CoveringTests(absFile, 5), "subsequent lookups must reuse the resolved path")
}

func TestCoveringTests_CachesMissingPath(t *testing.T) {
	p := &PerTestProfile{data: map[string]map[int][]string{}}
	absFile := "/workspace/pkg/foo.go"

	assert.Nil(t, p.CoveringTests(absFile, 5))
	p.data["pkg/foo.go"] = map[int][]string{5: {"TestA"}}
	assert.Nil(t, p.CoveringTests(absFile, 5), "subsequent lookups must reuse the cached miss")
}

// TestBuildPerTestProfile_SingleTestPackage uses a package with exactly one test
// function (mutator.TestMockMutator).  The len==0 guard mutant (line 194) would
// return nil,nil for a 1-element slice; the i:=1 incrementer mutant (line 217)
// would skip reading the only result and leave an empty profile.  Both are caught
// by asserting the profile is non-nil and contains coverage data.
func TestBuildPerTestProfile_SingleTestPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: runs per-test coverage profiling")
	}
	tmp := t.TempDir()
	prof, err := BuildPerTestProfile(
		"github.com/quality-gates/mutago/v2/mutator",
		"github.com/quality-gates/mutago/v2",
		tmp, 30, 1, nil,
	)
	require.NoError(t, err)
	require.NotNil(t, prof, "single-test package must produce a non-nil profile")
	var found bool
	for l := 1; l <= 100; l++ {
		if len(prof.CoveringTests("/abs/mutator/mutator.go", l)) > 0 {
			found = true
			break
		}
	}
	assert.True(t, found, "profile must contain coverage data for mutator.go")
}
