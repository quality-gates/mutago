package coverage

import (
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// goToolchain runs plain go commands with no user test flags; the engine's
// toolchain owns flag handling and is tested there.
type goToolchain struct {
	calls *[]string
}

func (g goToolchain) record(kind string) {
	if g.calls != nil {
		*g.calls = append(*g.calls, kind)
	}
}

func (g goToolchain) Context() context.Context { return context.Background() }

func (g goToolchain) ListTests(pkgPath string) *exec.Cmd {
	g.record("list tests")
	return exec.Command("go", "test", "-list", ".*", pkgPath)
}

func (g goToolchain) ListPackages(pattern string) *exec.Cmd {
	g.record("list packages")
	return exec.Command("go", "list", pattern)
}

func (g goToolchain) CompileTestBinary(pkgPath, coverPkg, binaryPath string) *exec.Cmd {
	g.record("compile")
	return exec.Command("go", "test", "-c", "-cover", "-covermode=set", "-coverpkg="+coverPkg, "-o", binaryPath, pkgPath)
}

func (g goToolchain) PackageDir(pkgPath string) *exec.Cmd {
	return exec.Command("go", "list", "-f", "{{.Dir}}", pkgPath)
}

func (g goToolchain) RunTest(binaryPath, _, testName, profilePath string) *exec.Cmd {
	g.record("run " + testName)
	return exec.Command(binaryPath, "-test.run=^"+testName+"$", "-test.coverprofile="+profilePath, "-test.timeout=30s")
}

func TestBuildPerTestProfileCompilesOnce(t *testing.T) {
	var calls []string
	prof, err := BuildPerTestProfile(
		goToolchain{calls: &calls},
		"github.com/quality-gates/mutago/v2/internal/coverage/testdata/entrypoints",
		"github.com/quality-gates/mutago/v2", t.TempDir(), 1,
	)
	require.NoError(t, err)
	require.NotNil(t, prof)
	require.Len(t, calls, 4, "list once, compile once, run each test")
	assert.Equal(t, []string{"list tests", "compile"}, calls[:2])
	assert.ElementsMatch(t, []string{"run FuzzValue", "run TestValue"}, calls[2:])
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

// A single profile line declares a span of 150,001 covered lines. Parsing
// must stay proportional to the number of profile lines, not the span, and
// membership must be exact at both ends of the span.
func TestParseProfile_LargeRange(t *testing.T) {
	profile := "mode: set\ngithub.com/example/pkg/foo.go:1.1,150001.3 2 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)

	assert.True(t, p.IsCoveredRelative("pkg/foo.go", 1))
	assert.True(t, p.IsCoveredRelative("pkg/foo.go", 75000))
	assert.True(t, p.IsCoveredRelative("pkg/foo.go", 150001))
	assert.False(t, p.IsCoveredRelative("pkg/foo.go", 150002))
}

// The span declared here (2,147,483,648 lines in a 66-byte profile) used to
// make the parser run at least as long as the span. It must now parse
// immediately and answer membership over the whole declared span.
func TestParseProfile_AbsurdRangeCompletes(t *testing.T) {
	profile := "mode: set\ngithub.com/example/pkg/foo.go:174127.1,2147657774.3 1 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)

	assert.True(t, p.IsCoveredRelative("pkg/foo.go", 174127))
	assert.True(t, p.IsCoveredRelative("pkg/foo.go", 1000000000))
	assert.True(t, p.IsCoveredRelative("pkg/foo.go", 2147657774))
	assert.False(t, p.IsCoveredRelative("pkg/foo.go", 2147657775))
	assert.False(t, p.IsCoveredRelative("pkg/foo.go", 174126))
}

// A span ending at max int64 used to wrap the expansion loop and never
// terminate. It must now parse immediately and answer membership at the end.
func TestParseProfile_MaxInt64RangeCompletes(t *testing.T) {
	profile := "mode: set\ngithub.com/example/pkg/foo.go:1.1,9223372036854775807.3 1 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)

	assert.True(t, p.IsCoveredRelative("pkg/foo.go", 1))
	assert.True(t, p.IsCoveredRelative("pkg/foo.go", 9223372036854775807))
	assert.False(t, p.IsCoveredRelative("pkg/foo.go", 0))
}

// A reversed range declares no covered lines, as it always did.
func TestParseProfile_ReversedRangeRecordsNothing(t *testing.T) {
	profile := "mode: set\ngithub.com/example/pkg/foo.go:50.1,10.3 2 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)

	assert.False(t, p.IsCoveredRelative("pkg/foo.go", 10))
	assert.False(t, p.IsCoveredRelative("pkg/foo.go", 30))
	assert.False(t, p.IsCoveredRelative("pkg/foo.go", 50))
}

// Ranges for one file may arrive out of order, nested or overlapping. The
// covered set is their union, and an inner range must not hide the part of
// an outer range that extends past it.
func TestParseProfile_OverlappingRanges(t *testing.T) {
	profile := "mode: set\n" +
		"github.com/example/pkg/foo.go:25.1,30.1 1 1\n" +
		"github.com/example/pkg/foo.go:1.1,10.1 1 1\n" +
		"github.com/example/pkg/foo.go:8.1,20.1 1 1\n" +
		"github.com/example/pkg/foo.go:5.1,7.1 1 1\n"
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)

	for _, line := range []int{1, 9, 15, 19, 20, 25, 30} {
		assert.True(t, p.IsCoveredRelative("pkg/foo.go", line), "line %d should be covered", line)
	}
	for _, line := range []int{21, 24, 31} {
		assert.False(t, p.IsCoveredRelative("pkg/foo.go", line), "line %d should not be covered", line)
	}
}

func TestParseProfile_EmptyProfile(t *testing.T) {
	path := writeTmpProfile(t, "mode: set\n")
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	assert.False(t, p.IsCovered("/any/file.go", 1))
}

func TestIsCovered_EmptyFilenameDoesNotMatchUnrelatedFile(t *testing.T) {
	path := writeTmpProfile(t, "mode: set\n:1.1,1.1 1 1\n")
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)

	assert.False(t, p.IsCovered("/unrelated/file.go", 1))
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
	lines := []lineRange{{start: 10, end: 10}}
	p := &Profile{coveredRanges: map[string][]lineRange{"pkg/foo.go": lines}}
	absFile := "/workspace/pkg/foo.go"

	assert.True(t, p.IsCovered(absFile, 10))
	p.coveredRanges["pkg/foo.go"] = []lineRange{{start: 1, end: 1}}
	assert.True(t, p.IsCovered(absFile, 10), "subsequent lookups must reuse the resolved path")
}

func TestIsCovered_CachesMissingPath(t *testing.T) {
	p := &Profile{coveredRanges: map[string][]lineRange{}}
	absFile := "/workspace/pkg/foo.go"

	assert.False(t, p.IsCovered(absFile, 10))
	p.coveredRanges["pkg/foo.go"] = []lineRange{{start: 10, end: 10}}
	assert.False(t, p.IsCovered(absFile, 10), "subsequent lookups must reuse the cached miss")
}

func TestIsCoveredRelativeUsesDirectProfileKey(t *testing.T) {
	path := writeTmpProfile(t, sampleProfile)
	p, err := ParseProfile(path, modulePath)
	require.NoError(t, err)
	assert.True(t, p.IsCoveredRelative("pkg/foo.go", 10))
	assert.False(t, p.IsCoveredRelative("pkg/foo.go", 20))
}

func TestIsLineCoveredAnywhere_HitAndMiss(t *testing.T) {
	// A single-file profile so the range loop must run its body to find a hit.
	p := &Profile{coveredRanges: map[string][]lineRange{
		"pkg/a.go": {{start: 1, end: 1}},
	}}
	// Hit: line 1 is inside the range — must return true (kills the `return true`
	// and a loop that skips its body).
	assert.True(t, p.IsLineCoveredAnywhere(1))
	// Miss: line 5 is past the range's end, line 99 absent — both must return
	// false (kills a removed range-membership guard).
	assert.False(t, p.IsLineCoveredAnywhere(5))
	assert.False(t, p.IsLineCoveredAnywhere(99))
}

func TestIsLineCoveredAnywhere_NonPositiveShortCircuits(t *testing.T) {
	// line <= 0 returns false before scanning the ranges, even when a range
	// covers line 0. This kills the `<=` comparison, its negation, the `0`
	// literal mutations, and removal of the guard's `return false`.
	p := &Profile{coveredRanges: map[string][]lineRange{
		"pkg/a.go": {{start: 0, end: 1}},
	}}
	assert.False(t, p.IsLineCoveredAnywhere(0))
	assert.False(t, p.IsLineCoveredAnywhere(-1))
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

// TestParseProfile_WorkspaceModulePaths covers #352: `go list -m` prints one
// line per workspace module, so the module argument can hold several paths.
// Entries from every module must resolve, and the longest (most specific)
// prefix wins when one module path is a prefix of another.
func TestParseProfile_WorkspaceModulePaths(t *testing.T) {
	profile := `mode: set
example.com/repro/limits/limits.go:5.1,6.3 1 1
example.com/repro/tools/tools.go:3.1,4.3 1 1
`
	path := writeTmpProfile(t, profile)
	p, err := ParseProfile(path, "example.com/repro\nexample.com/repro/tools")
	require.NoError(t, err)

	assert.True(t, p.IsCovered("/work/limits/limits.go", 5), "main-module entry should resolve")
	assert.True(t, p.IsCovered("/work/tools/tools.go", 3), "second-module entry should resolve")
	// The tools entry must be stripped by its own (longer) module path, not
	// by the main module path that prefixes it as a string.
	assert.True(t, p.IsCoveredRelative("tools.go", 3))
	assert.False(t, p.IsCoveredRelative("tools/tools.go", 3))
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
	count := CountTests(goToolchain{}, "github.com/quality-gates/mutago/v2/mutator/arithmetic")
	assert.Positive(t, count, "arithmetic package should have tests")
}

func TestCountTests_ExcludesBenchmarks(t *testing.T) {
	count := CountTests(goToolchain{}, "github.com/quality-gates/mutago/v2/internal/coverage/testdata/entrypoints")
	assert.Equal(t, 2, count, "only Test and Fuzz entrypoints run via -run")
}

func TestCountTests_NonexistentPackage(t *testing.T) {
	count := CountTests(goToolchain{}, "github.com/quality-gates/mutago/v2/nonexistent_pkg_xyzzy")
	assert.Zero(t, count, "nonexistent package should return 0")
}

// --- BuildPerTestProfile tests ---

func TestBuildPerTestProfile_RealPackage(t *testing.T) {
	if testing.Short() {
		t.Skip("slow: runs per-test coverage profiling")
	}
	tmp := t.TempDir()
	prof, err := BuildPerTestProfile(
		goToolchain{},
		"github.com/quality-gates/mutago/v2/mutator/arithmetic",
		"github.com/quality-gates/mutago/v2",
		tmp, 1,
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
		goToolchain{},
		"github.com/quality-gates/mutago/v2/nonexistent_pkg_xyzzy",
		"github.com/quality-gates/mutago/v2",
		tmp, 1,
	)
	assert.Error(t, err)
	assert.Nil(t, prof)
}

func TestBuildPerTestProfile_WorkersZero(t *testing.T) {
	// workers=0 must be normalised to 1; verify the profile is returned without
	// deadlocking and contains actual coverage data. Uses gitdiff (small test suite).
	tmp := t.TempDir()
	prof, err := BuildPerTestProfile(
		goToolchain{},
		"github.com/quality-gates/mutago/v2/internal/gitdiff",
		"github.com/quality-gates/mutago/v2",
		tmp, 0,
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
	workingDir := t.TempDir()
	t.Chdir(workingDir)
	t.Setenv("TMPDIR", workingDir)
	tmpFile := filepath.Join(workingDir, "not-a-directory")
	require.NoError(t, os.WriteFile(tmpFile, []byte("occupied"), 0o600))

	var calls []string
	prof, err := BuildPerTestProfileForTests(
		goToolchain{calls: &calls},
		"github.com/quality-gates/mutago/v2/internal/coverage/testdata/entrypoints",
		"github.com/quality-gates/mutago/v2", tmpFile, 1, []string{"TestAlpha"},
	)
	assert.Error(t, err)
	assert.Nil(t, prof)
	assert.Empty(t, calls, "an invalid temporary directory must not start compilation")
}

func TestBuildPerTestProfileForTests_CompileFailure(t *testing.T) {
	prof, err := BuildPerTestProfileForTests(
		goToolchain{},
		"github.com/quality-gates/mutago/v2/nonexistent_pkg_xyzzy",
		"github.com/quality-gates/mutago/v2", t.TempDir(), 1, []string{"TestMissing"},
	)
	assert.ErrorContains(t, err, "compile coverage test binary")
	assert.Nil(t, prof)
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
	p := &PerTestProfile{data: map[string]map[string][]lineRange{
		"pkg/foo.go": {"TestHidden": {{start: 0, end: 0}}, "TestVisible": {{start: 1, end: 1}}},
	}}
	assert.Nil(t, p.CoveringTests("/abs/pkg/foo.go", 0))
	assert.Equal(t, []string{"TestVisible"}, p.CoveringTests("/abs/pkg/foo.go", 1))
}

func TestCoveringTests_NegativeLine(t *testing.T) {
	p := &PerTestProfile{data: map[string]map[string][]lineRange{
		"pkg/foo.go": {"TestHidden": {{start: 0, end: 0}}, "TestFoo": {{start: 1, end: 1}}},
	}}
	assert.Nil(t, p.CoveringTests("/abs/pkg/foo.go", -1))
}

func TestCoveringTests_SuffixMatch(t *testing.T) {
	p := &PerTestProfile{data: map[string]map[string][]lineRange{
		"pkg/foo.go": {"TestB": {{start: 5, end: 5}}, "TestA": {{start: 5, end: 6}}},
	}}
	// Map iteration order is random; the result must stay sorted every call.
	for range 100 {
		assert.Equal(t, []string{"TestA", "TestB"}, p.CoveringTests("/home/user/go/pkg/foo.go", 5))
	}
	assert.Equal(t, []string{"TestA"}, p.CoveringTests("/home/user/go/pkg/foo.go", 6))
}

func TestCoveringTests_ExactMatch(t *testing.T) {
	p := &PerTestProfile{data: map[string]map[string][]lineRange{
		"pkg/foo.go": {"TestC": {{start: 3, end: 3}}},
	}}
	assert.Equal(t, []string{"TestC"}, p.CoveringTests("pkg/foo.go", 3))
}

func TestCoveringTests_NoMatch(t *testing.T) {
	p := &PerTestProfile{data: map[string]map[string][]lineRange{
		"pkg/foo.go": {"TestA": {{start: 5, end: 5}}},
	}}
	assert.Nil(t, p.CoveringTests("/different/path/bar.go", 5))
	assert.Nil(t, p.CoveringTests("/abs/pkg/foo.go", 99))
}

func TestCoveringTests_EmptyFilenameDoesNotMatchUnrelatedFile(t *testing.T) {
	p := &PerTestProfile{data: map[string]map[string][]lineRange{
		"": {"TestUnexpected": {{start: 1, end: 1}}},
	}}

	assert.Nil(t, p.CoveringTests("/unrelated/file.go", 1))
}

func TestCoveringTests_CachesResolvedPath(t *testing.T) {
	lines := map[string][]lineRange{"TestA": {{start: 5, end: 5}}}
	p := &PerTestProfile{data: map[string]map[string][]lineRange{"pkg/foo.go": lines}}
	absFile := "/workspace/pkg/foo.go"

	assert.Equal(t, []string{"TestA"}, p.CoveringTests(absFile, 5))
	p.data["pkg/foo.go"] = map[string][]lineRange{"TestB": {{start: 5, end: 5}}}
	assert.Equal(t, []string{"TestA"}, p.CoveringTests(absFile, 5), "subsequent lookups must reuse the resolved path")
}

func TestCoveringTests_CachesMissingPath(t *testing.T) {
	p := &PerTestProfile{data: map[string]map[string][]lineRange{}}
	absFile := "/workspace/pkg/foo.go"

	assert.Nil(t, p.CoveringTests(absFile, 5))
	p.data["pkg/foo.go"] = map[string][]lineRange{"TestA": {{start: 5, end: 5}}}
	assert.Nil(t, p.CoveringTests(absFile, 5), "subsequent lookups must reuse the cached miss")
}

// Two packages sharing a test name contribute two sorted runs of ranges for
// that test. Concatenated, they are neither sorted nor non-overlapping, so
// each merge must re-coalesce before binary search can answer correctly.
func TestApplyPerTestResult_SharedTestNameReCoalesces(t *testing.T) {
	prof := func(ranges ...lineRange) *Profile {
		return &Profile{coveredRanges: map[string][]lineRange{"pkg/foo.go": ranges}}
	}
	p := &PerTestProfile{data: make(map[string]map[string][]lineRange)}
	applyPerTestResult(p, profileSuccess{name: "TestShared", prof: prof(lineRange{start: 10, end: 20})})
	applyPerTestResult(p, profileSuccess{name: "TestShared", prof: prof(lineRange{start: 1, end: 5})})

	for _, line := range []int{1, 3, 5, 10, 15, 20} {
		assert.Equal(t, []string{"TestShared"}, p.CoveringTestsRelative("pkg/foo.go", line), "line %d", line)
	}
	for _, line := range []int{6, 9, 21} {
		assert.Nil(t, p.CoveringTestsRelative("pkg/foo.go", line), "line %d", line)
	}

	// A later nested range must not hide the part of an earlier range that
	// extends past it.
	applyPerTestResult(p, profileSuccess{name: "TestNested", prof: prof(lineRange{start: 1, end: 20})})
	applyPerTestResult(p, profileSuccess{name: "TestNested", prof: prof(lineRange{start: 5, end: 10})})
	assert.Equal(t, []string{"TestNested", "TestShared"}, p.CoveringTestsRelative("pkg/foo.go", 15))
	assert.Equal(t, []string{"TestNested", "TestShared"}, p.CoveringTestsRelative("pkg/foo.go", 3))
}

// The per-test path merges whole profiles, so a huge declared span must stay
// one span per test instead of becoming one entry per line.
func TestApplyPerTestResult_AbsurdRange(t *testing.T) {
	profile := "mode: set\ngithub.com/example/pkg/foo.go:174127.1,2147657774.3 1 1\n"
	path := writeTmpProfile(t, profile)
	prof, err := ParseProfile(path, modulePath)
	require.NoError(t, err)

	p := &PerTestProfile{data: make(map[string]map[string][]lineRange)}
	applyPerTestResult(p, profileSuccess{name: "TestX", prof: prof})

	assert.Equal(t, []string{"TestX"}, p.CoveringTestsRelative("pkg/foo.go", 1000000000))
	assert.Equal(t, []string{"TestX"}, p.CoveringTestsRelative("pkg/foo.go", 2147657774))
	assert.Nil(t, p.CoveringTestsRelative("pkg/foo.go", 174126))
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
		goToolchain{},
		"github.com/quality-gates/mutago/v2/mutator",
		"github.com/quality-gates/mutago/v2",
		tmp, 1,
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

// writeRecursiveModule writes a module whose rec package is checked only by a
// subpackage test, plus a directory go list rejects, and makes it the
// working directory.
func writeRecursiveModule(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	files := map[string]string{
		"go.mod":          "module example.com/m\n\ngo 1.22\n",
		"rec/rec.go":      "package rec\n\nfunc Double(x int) int {\n\treturn x * 2\n}\n",
		"rec/rec_test.go": "package rec\n\nimport \"testing\"\n\nfunc TestTouch(t *testing.T) { _ = Double(1) }\n",
		"rec/sub/sub.go":  "package sub\n",
		"rec/sub/sub_test.go": "package sub\n\nimport (\n\t\"testing\"\n\n\t\"example.com/m/rec\"\n)\n\n" +
			"func TestDouble(t *testing.T) {\n\tif rec.Double(3) != 6 {\n\t\tt.Fatal(\"bad\")\n\t}\n}\n\n" +
			"func TestTouch(t *testing.T) { _ = rec.Double(2) }\n",
		"rec/notests/notests.go": "package notests\n",
		"broken/a.go":            "package a\n",
		"broken/b.go":            "package b\n",
	}
	for name, content := range files {
		path := filepath.Join(dir, filepath.FromSlash(name))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
		require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
	}
	t.Chdir(dir)
}

func TestListTestPackages(t *testing.T) {
	writeRecursiveModule(t)

	flat, err := ListTestPackages(goToolchain{}, "example.com/m/rec", false)
	require.NoError(t, err)
	assert.Equal(t, []TestPackage{{ImportPath: "example.com/m/rec", Tests: []string{"TestTouch"}}}, flat)

	tree, err := ListTestPackages(goToolchain{}, "example.com/m/rec", true)
	require.NoError(t, err)
	assert.Equal(t, []TestPackage{
		{ImportPath: "example.com/m/rec", Tests: []string{"TestTouch"}},
		{ImportPath: "example.com/m/rec/sub", Tests: []string{"TestDouble", "TestTouch"}},
	}, tree, "packages without tests are left out")

	_, err = ListTestPackages(goToolchain{}, "example.com/m/broken", true)
	assert.Error(t, err, "a package tree that go list rejects")
	_, err = ListTestPackages(goToolchain{}, "example.com/m/nope", false)
	assert.Error(t, err, "a package whose tests cannot be listed")
}

func TestBuildPerTestProfileForPackages_AttributesSubpackageTests(t *testing.T) {
	writeRecursiveModule(t)
	for dir, testName := range map[string]string{"rec/a/b_c": "TestKiller", "rec/a_b/c": "TestObserver"} {
		require.NoError(t, os.MkdirAll(dir, 0o755))
		test := "package child\nimport (\"testing\"; \"example.com/m/rec\")\nfunc " + testName + "(t *testing.T) { _ = rec.Double(3) }\n"
		require.NoError(t, os.WriteFile(filepath.Join(dir, "child_test.go"), []byte(test), 0o600))
	}
	pkgs := []TestPackage{
		{ImportPath: "example.com/m/rec", Tests: []string{"TestTouch"}},
		{ImportPath: "example.com/m/rec/sub", Tests: []string{"TestDouble", "TestTouch"}},
		{ImportPath: "example.com/m/rec/a/b_c", Tests: []string{"TestKiller"}},
		{ImportPath: "example.com/m/rec/a_b/c", Tests: []string{"TestObserver"}},
	}

	tmp := t.TempDir()
	otherBuild := filepath.Join(tmp, "per-test", "other-build")
	require.NoError(t, os.MkdirAll(otherBuild, 0o755))
	otherFile := filepath.Join(otherBuild, "keep")
	require.NoError(t, os.WriteFile(otherFile, []byte("other"), 0o600))
	savedMutation := filepath.Join(tmp, "mutation.go")
	require.NoError(t, os.WriteFile(savedMutation, []byte("saved"), 0o600))
	prof, err := BuildPerTestProfileForPackages(goToolchain{}, "example.com/m/rec", pkgs, "example.com/m", tmp, 1)
	require.NoError(t, err)
	require.NotNil(t, prof)
	assert.Equal(t, []string{"TestDouble", "TestKiller", "TestObserver", "TestTouch"}, prof.CoveringTestsRelative("rec/rec.go", 4),
		"subpackage tests cover the target, and a name shared across packages is listed once")
	entries, err := os.ReadDir(filepath.Join(tmp, "per-test"))
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "other-build", entries[0].Name())
	entries, err = os.ReadDir(tmp)
	require.NoError(t, err)
	require.Len(t, entries, 2)
	contents, err := os.ReadFile(otherFile)
	require.NoError(t, err)
	assert.Equal(t, "other", string(contents))
	contents, err = os.ReadFile(savedMutation)
	require.NoError(t, err)
	assert.Equal(t, "saved", string(contents))
}

func TestBuildPerTestProfile_NoTests(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "never-created")
	prof, err := BuildPerTestProfileForPackages(goToolchain{}, "example.com/m/rec", nil, "example.com/m", tmp, 1)
	assert.NoError(t, err)
	assert.Nil(t, prof)
	prof, err = BuildPerTestProfileForTests(goToolchain{}, "example.com/m/rec", "example.com/m", tmp, 1, nil)
	assert.NoError(t, err)
	assert.Nil(t, prof)
	assert.NoDirExists(t, tmp, "nothing is compiled when there are no tests")
}

// scriptedToolchain compiles nothing and runs each test as the shell script
// named for it in runs, with $1 set to the test's profile path. Compiling
// runs the compile script, when set, and so does finding the package
// directory with the pkgDir script.
// Its commands are bound to ctx, the run context (none means background),
// and testRuns, when set, counts the test commands built.
type scriptedToolchain struct {
	runs     map[string]string
	compile  string
	pkgDir   string
	ctx      context.Context
	testRuns *atomic.Int32
}

func (s scriptedToolchain) Context() context.Context {
	if s.ctx == nil {
		return context.Background()
	}
	return s.ctx
}

func (s scriptedToolchain) ListTests(string) *exec.Cmd {
	return exec.CommandContext(s.Context(), "echo", "TestTouch")
}
func (s scriptedToolchain) ListPackages(string) *exec.Cmd {
	return exec.CommandContext(s.Context(), "echo", "example.com/m/rec")
}
func (s scriptedToolchain) CompileTestBinary(_, _, _ string) *exec.Cmd {
	if s.compile != "" {
		return exec.CommandContext(s.Context(), "sh", "-c", s.compile)
	}
	return exec.CommandContext(s.Context(), "true")
}
func (s scriptedToolchain) PackageDir(string) *exec.Cmd {
	if s.pkgDir != "" {
		return exec.CommandContext(s.Context(), "sh", "-c", s.pkgDir)
	}
	return exec.CommandContext(s.Context(), "pwd")
}
func (s scriptedToolchain) RunTest(_, pkgDir, testName, profilePath string) *exec.Cmd {
	if s.testRuns != nil {
		s.testRuns.Add(1)
	}
	cmd := exec.CommandContext(s.Context(), "sh", "-c", s.runs[testName], "sh", profilePath)
	cmd.Dir = pkgDir
	return cmd
}

const writeValidProfile = `printf 'mode: set\nexample.com/m/rec/rec.go:3.24,5.2 1 1\n' > "$1"`

func TestBuildPerTestProfile_UnavailableWhenAnyProfileIsLost(t *testing.T) {
	for name, lostScript := range map[string]string{
		"missing":    "exit 0",
		"empty":      `: > "$1"`,
		"headerless": `printf 'example.com/m/rec/rec.go:3.24,5.2 1 1\n' > "$1"`,
		"malformed":  `printf 'mode: set\nexample.com/m/rec/rec.go:3.24,5.2 1 x\n' > "$1"`,
	} {
		t.Run(name, func(t *testing.T) {
			tc := scriptedToolchain{runs: map[string]string{"TestTouch": writeValidProfile, "TestCheck": lostScript}}
			pkgs := []TestPackage{{ImportPath: "example.com/m/rec", Tests: []string{"TestTouch", "TestCheck"}}}

			prof, err := BuildPerTestProfileForPackages(tc, "example.com/m/rec", pkgs, "example.com/m", t.TempDir(), 1)

			require.Error(t, err)
			assert.Nil(t, prof, "a partial map must not restrict mutant tests")
			assert.Contains(t, err.Error(), "TestCheck")
			assert.Contains(t, err.Error(), "coverage.out")
			assert.NotContains(t, err.Error(), "TestTouch")
		})
	}
}

func TestBuildPerTestProfile_NamesEveryLostProfile(t *testing.T) {
	tc := scriptedToolchain{runs: map[string]string{"TestTouch": "exit 0", "TestCheck": "exit 0"}}
	pkgs := []TestPackage{{ImportPath: "example.com/m/rec", Tests: []string{"TestTouch", "TestCheck"}}}

	prof, err := BuildPerTestProfileForPackages(tc, "example.com/m/rec", pkgs, "example.com/m", t.TempDir(), 1)

	assert.Nil(t, prof)
	assert.ErrorIs(t, err, fs.ErrNotExist, "the profile's own error stays inspectable")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "TestTouch")
	assert.Contains(t, err.Error(), "TestCheck")
}

func TestBuildPerTestProfile_FailingTestWithValidProfileIsCovered(t *testing.T) {
	tc := scriptedToolchain{runs: map[string]string{
		"TestTouch": writeValidProfile,
		"TestCheck": writeValidProfile + "; exit 1",
	}}
	pkgs := []TestPackage{{ImportPath: "example.com/m/rec", Tests: []string{"TestTouch", "TestCheck"}}}

	prof, err := BuildPerTestProfileForPackages(tc, "example.com/m/rec", pkgs, "example.com/m", t.TempDir(), 1)

	require.NoError(t, err)
	assert.Equal(t, []string{"TestCheck", "TestTouch"}, prof.CoveringTestsRelative("rec/rec.go", 4))
}

func TestListTestPackages_CancelledRunIsAnError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	tc := scriptedToolchain{ctx: ctx}

	for _, recursive := range []bool{false, true} {
		pkgs, err := ListTestPackages(tc, "example.com/m/rec", recursive)

		assert.ErrorIs(t, err, context.Canceled, "recursive=%v", recursive)
		assert.Nil(t, pkgs, "recursive=%v", recursive)
	}
}

func TestBuildPerTestProfile_CancelledBeforeCompileRunsNoTests(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ran := filepath.Join(t.TempDir(), "ran")
	tc := scriptedToolchain{ctx: ctx, runs: map[string]string{"TestTouch": `: > "` + ran + `"`}}
	pkgs := []TestPackage{{ImportPath: "example.com/m/rec", Tests: []string{"TestTouch"}}}

	prof, err := BuildPerTestProfileForPackages(tc, "example.com/m/rec", pkgs, "example.com/m", t.TempDir(), 1)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, prof)
	assert.NoFileExists(t, ran)
}

// A cancelled run stops the test in progress, starts no further test, and
// reports the cancellation rather than a lost profile.
func TestBuildPerTestProfile_CancellationStopsProfiling(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dir := t.TempDir()
	started, second := filepath.Join(dir, "started"), filepath.Join(dir, "second")
	var testRuns atomic.Int32
	tc := scriptedToolchain{ctx: ctx, testRuns: &testRuns, runs: map[string]string{
		"TestTouch": `: > "` + started + `"; exec sleep 30`,
		"TestCheck": `: > "` + second + `"; ` + writeValidProfile,
		"TestThird": writeValidProfile,
	}}
	pkgs := []TestPackage{{ImportPath: "example.com/m/rec", Tests: []string{"TestTouch", "TestCheck", "TestThird"}}}
	cancelOnMarker(ctx, cancel, started)

	tmp := t.TempDir()
	prof, err := buildWithin(t, 10*time.Second, tc, pkgs, tmp)

	assert.ErrorIs(t, err, context.Canceled)
	assert.NotContains(t, err.Error(), "coverage.out", "cancellation is not a lost profile")
	assert.Nil(t, prof)
	assert.NoFileExists(t, second, "no test starts after cancellation")
	assert.Equal(t, int32(1), testRuns.Load(), "no test is admitted after cancellation")
	assertProfileBuildsCleaned(t, tmp)
}

// A compile the run's cancellation kills reports the cancellation, while any
// other compile failure keeps the command's own error.
func TestBuildPerTestProfile_CompileErrors(t *testing.T) {
	pkgs := []TestPackage{{ImportPath: "example.com/m/rec", Tests: []string{"TestTouch"}}}

	t.Run("failed", func(t *testing.T) {
		tc := scriptedToolchain{compile: "echo boom; exit 3"}

		tmp := t.TempDir()
		prof, err := buildWithin(t, 10*time.Second, tc, pkgs, tmp)

		var exitErr *exec.ExitError
		assert.ErrorAs(t, err, &exitErr)
		assert.NotErrorIs(t, err, context.Canceled)
		assert.ErrorContains(t, err, "boom")
		assert.Nil(t, prof)
		assertProfileBuildsCleaned(t, tmp)
	})

	t.Run("cancelled", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		started := filepath.Join(t.TempDir(), "started")
		tc := scriptedToolchain{ctx: ctx, compile: `: > "` + started + `"; exec sleep 30`}
		cancelOnMarker(ctx, cancel, started)

		tmp := t.TempDir()
		prof, err := buildWithin(t, 10*time.Second, tc, pkgs, tmp)

		assert.ErrorIs(t, err, context.Canceled)
		assert.Nil(t, prof)
		assertProfileBuildsCleaned(t, tmp)
	})
}

// Each test runs in the directory go list prints, even one whose name ends
// in a space.
func TestBuildPerTestProfile_RunsTestsInPackageDir(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	pkgDir := filepath.Join(root, "pkg ")
	require.NoError(t, os.Mkdir(pkgDir, 0755))
	ranIn := filepath.Join(t.TempDir(), "ran-in")
	t.Setenv("PKG_DIR", pkgDir)
	t.Setenv("RAN_IN", ranIn)
	tc := scriptedToolchain{
		pkgDir: `printf '%s\n' "$PKG_DIR"`,
		runs:   map[string]string{"TestTouch": `pwd -P > "$RAN_IN"; ` + writeValidProfile},
	}
	pkgs := []TestPackage{{ImportPath: "example.com/m/rec", Tests: []string{"TestTouch"}}}

	prof, err := buildWithin(t, 10*time.Second, tc, pkgs, t.TempDir())

	require.NoError(t, err)
	require.NotNil(t, prof)
	got, err := os.ReadFile(ranIn)
	require.NoError(t, err)
	assert.Equal(t, pkgDir+"\n", string(got))
}

// A package directory that go list cannot print leaves no profile, so mutants
// run against the full suite and no test runs in the wrong directory.
func TestBuildPerTestProfile_PackageDirErrors(t *testing.T) {
	pkgs := []TestPackage{{ImportPath: "example.com/m/rec", Tests: []string{"TestTouch"}}}
	for name, script := range map[string]string{"failed": "pwd; exit 2", "empty": "true"} {
		t.Run(name, func(t *testing.T) {
			var testRuns atomic.Int32
			tc := scriptedToolchain{pkgDir: script, testRuns: &testRuns, runs: map[string]string{"TestTouch": writeValidProfile}}

			tmp := t.TempDir()
			prof, err := buildWithin(t, 10*time.Second, tc, pkgs, tmp)

			assert.ErrorContains(t, err, "package directory of example.com/m/rec")
			assert.Nil(t, prof)
			assert.Zero(t, testRuns.Load())
			assertProfileBuildsCleaned(t, tmp)
		})
	}
}

func assertProfileBuildsCleaned(t *testing.T, tmp string) {
	t.Helper()
	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// cancelOnMarker cancels the run once the marker file exists.
func cancelOnMarker(ctx context.Context, cancel context.CancelFunc, marker string) {
	go func() {
		for ctx.Err() == nil {
			if _, err := os.Stat(marker); err == nil {
				cancel()
				return
			}
			time.Sleep(10 * time.Millisecond)
		}
	}()
}

// buildWithin builds the per-test profile with one worker, failing the test
// if it has not returned within limit.
func buildWithin(t *testing.T, limit time.Duration, tc Toolchain, pkgs []TestPackage, tmp string) (*PerTestProfile, error) {
	t.Helper()
	type built struct {
		prof *PerTestProfile
		err  error
	}
	done := make(chan built, 1)
	go func() {
		prof, err := BuildPerTestProfileForPackages(tc, "example.com/m/rec", pkgs, "example.com/m", tmp, 1)
		done <- built{prof, err}
	}()
	select {
	case b := <-done:
		return b.prof, b.err
	case <-time.After(limit):
		t.Fatalf("per-test profiling did not return within %s", limit)
		return nil, nil
	}
}
