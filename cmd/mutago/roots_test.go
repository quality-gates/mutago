package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const subModuleBaseSource = `package svc

func Add(a, b int) int { return a + b }
`

const subModuleChangedSource = `package svc

func Add(a, b int) int { return a + b }

func Neg(a int) int {
	if a > 0 {
		return -a
	}
	return a * -1
}
`

// newSubModuleRepo commits a module in <repo>/svc, then adds Neg to calc.go
// without committing it. It returns the repository directory spelled through
// a symlink, the way a checkout under macOS /var or a linked workspace looks.
func newSubModuleRepo(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	repo := filepath.Join(parent, "repo")
	svc := filepath.Join(repo, "svc")
	require.NoError(t, os.MkdirAll(svc, 0o755))
	writeFixtureFile(t, filepath.Join(svc, "go.mod"), "module example.com/svc\n\ngo 1.26.3\n")
	writeFixtureFile(t, filepath.Join(svc, "calc_test.go"), `package svc

import "testing"

func TestAdd(t *testing.T) {
	if Add(1, 2) != 3 {
		t.Fatal("Add(1, 2) != 3")
	}
}
`)
	writeFixtureFile(t, filepath.Join(svc, "calc.go"), subModuleBaseSource)
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "mutago@example.com")
	runGit(t, repo, "config", "user.name", "mutago test")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-q", "-m", "base")
	writeFixtureFile(t, filepath.Join(svc, "calc.go"), subModuleChangedSource)
	link := filepath.Join(parent, "link")
	require.NoError(t, os.Symlink(repo, link))
	return link
}

// chdirLikeShell makes the run start in dir the way a shell does: PWD names
// dir without resolving symlinks, so os.Getwd and go env keep that spelling.
func chdirLikeShell(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PWD", dir)
}

func useReportFile(t *testing.T, name *string, path string) {
	t.Helper()
	previous := *name
	*name = path
	t.Cleanup(func() { *name = previous })
}

func TestMainGitDiffLinesAdmitsChangedLinesWhenModuleIsBelowRepoRoot(t *testing.T) {
	repo := newSubModuleRepo(t)
	svc := filepath.Join(repo, "svc")
	chdirLikeShell(t, svc)
	reportPath := filepath.Join(t.TempDir(), "report.json")
	useReportFile(t, &models.ReportFileName, reportPath)

	testMain(t, svc, []string{
		"--workers", "1", "--exec-timeout", "30",
		"--git-diff-lines", "--git-diff-base", "HEAD", "--config", writeJSONConfig(t),
		"./",
	}, returnOk, "mutation score")

	report := readReport(t, reportPath)
	mutants := append(report.Escaped, report.Killed...)
	require.NotEmpty(t, mutants, "changed lines in svc/calc.go must produce mutants")
	for _, m := range mutants {
		assert.GreaterOrEqual(t, m.Mutator.OriginalStartLine, int64(5), "mutant outside the changed lines: %+v", m.Mutator)
		require.NotNil(t, m.Mutator.Location)
		assert.Equal(t, models.Coordinate{Filename: m.Mutator.OriginalFilePath, Line: m.Mutator.OriginalStartLine}, m.Mutator.SourceLocation())
		assert.Equal(t, m.Mutator.OriginalStartLine, m.Mutator.CoverageLocation().Line)
		assert.Equal(t, "calc.go", filepath.Base(m.Mutator.CoverageLocation().Filename))
	}
}

func writeJSONConfig(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "mutago.yml")
	writeFixtureFile(t, path, "json_output: true\n")
	return path
}

func readReport(t *testing.T, path string) models.Report {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var report models.Report
	require.NoError(t, json.Unmarshal(data, &report))
	return report
}

func TestMainCIReportPathsAreRepoRelativeWhenModuleIsBelowRepoRoot(t *testing.T) {
	repo := newSubModuleRepo(t)
	svc := filepath.Join(repo, "svc")
	chdirLikeShell(t, svc)
	gitLabPath := filepath.Join(t.TempDir(), "mutago-gitlab.json")
	useReportFile(t, &models.ReportGitLabJSONFileName, gitLabPath)

	out := testMain(t, svc, []string{
		"--workers", "1", "--exec-timeout", "30",
		"--logger-github", "--logger-gitlab", "--config", writeJSONConfig(t),
		"./",
	}, returnOk, "::warning file=svc/calc.go,")

	assert.NotContains(t, out, "::warning file=calc.go,")
	assert.NotContains(t, out, "file=../")
	assert.Equal(t, []string{"svc/calc.go"}, gitLabPaths(t, gitLabPath))
}

func TestMainCIReportPathsAreModuleRelativeOutsideGitRepo(t *testing.T) {
	mod := t.TempDir()
	writeFixtureFile(t, filepath.Join(mod, "go.mod"), "module example.com/svc\n\ngo 1.26.3\n")
	writeFixtureFile(t, filepath.Join(mod, "calc.go"), subModuleChangedSource)
	writeFixtureFile(t, filepath.Join(mod, "calc_test.go"), "package svc\n")
	t.Setenv("GIT_CEILING_DIRECTORIES", filepath.Dir(mod))
	chdirLikeShell(t, mod)
	gitLabPath := filepath.Join(t.TempDir(), "mutago-gitlab.json")
	useReportFile(t, &models.ReportGitLabJSONFileName, gitLabPath)

	testMain(t, mod, []string{
		"--workers", "1", "--exec-timeout", "30",
		"--logger-github", "--logger-gitlab", "--config", writeJSONConfig(t),
		"./",
	}, returnOk, "::warning file=calc.go,")

	assert.Equal(t, []string{"calc.go"}, gitLabPaths(t, gitLabPath))
}

// gitLabPaths returns the distinct location paths in a GitLab report.
func gitLabPaths(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var issues []struct {
		Location struct {
			Path string `json:"path"`
		} `json:"location"`
	}
	require.NoError(t, json.Unmarshal(data, &issues))
	require.NotEmpty(t, issues)
	var paths []string
	seen := map[string]bool{}
	for _, issue := range issues {
		if !seen[issue.Location.Path] {
			seen[issue.Location.Path] = true
			paths = append(paths, issue.Location.Path)
		}
	}
	return paths
}
