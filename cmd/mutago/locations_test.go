package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"
	"github.com/quality-gates/mutago/v2/internal/reportmaker"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainDirectiveLocationsUsePhysicalSource(t *testing.T) {
	repo := t.TempDir()
	writeFixtureFile(t, filepath.Join(repo, "go.mod"), "module example.com/locations\n\ngo 1.26.3\n")
	writeFixtureFile(t, filepath.Join(repo, "calc_test.go"), "package locations\n")
	const source = "package locations\n\nfunc Add(a, b int) int {\n//line virtual.go:200\n return a + b\n}\n"
	writeFixtureFile(t, filepath.Join(repo, "calc.go"), strings.Replace(source, "a + b", "a - b", 1))
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "mutago@example.com")
	runGit(t, repo, "config", "user.name", "mutago test")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-q", "-m", "base")
	writeFixtureFile(t, filepath.Join(repo, "calc.go"), source)
	chdirLikeShell(t, repo)
	reportPath := filepath.Join(t.TempDir(), "report.json")
	agenticPath := filepath.Join(t.TempDir(), "agentic.json")
	gitlabPath := filepath.Join(t.TempDir(), "gitlab.json")
	useReportFile(t, &models.ReportFileName, reportPath)
	useReportFile(t, &models.ReportAgenticJSONFileName, agenticPath)
	useReportFile(t, &models.ReportGitLabJSONFileName, gitlabPath)
	common := []string{"--workers", "1", "--exec-timeout", "30", "--config", writeJSONConfig(t)}
	testMain(t, repo, append(common, "./"), returnOk, "mutation score")
	unscoped := readReport(t, reportPath)
	ids := map[string]bool{}
	for _, m := range unscoped.Escaped {
		ids[m.ID] = true
	}
	out := testMain(t, repo, append(common, "--git-diff-lines", "--git-diff-base", "HEAD", "--logger-agentic-json", "--logger-github", "--logger-gitlab", "./"), returnOk, "mutation score")
	report := readReport(t, reportPath)
	require.NotEmpty(t, report.Escaped, "the changed physical statement must stay in scope")
	assert.Contains(t, out, "calc.go:5")
	assert.Contains(t, out, "::warning file=calc.go,line=5")
	for _, m := range report.Escaped {
		assert.True(t, ids[m.ID], "scope must not change IDs")
		assert.Equal(t, int64(200), m.Mutator.OriginalStartLine)
		assert.Equal(t, "calc.go", filepath.Base(m.Mutator.OriginalFilePath))
		require.NotNil(t, m.Mutator.Location)
		assert.Equal(t, models.Coordinate{Filename: m.Mutator.OriginalFilePath, Line: 5}, m.Mutator.Location.Physical)
		assert.Equal(t, models.Coordinate{Filename: filepath.Join(repo, "virtual.go"), Line: 200}, m.Mutator.Location.Coverage)
	}
	data, err := os.ReadFile(agenticPath)
	require.NoError(t, err)
	var agentic struct {
		Mutants []reportmaker.AgenticMutant `json:"mutants"`
	}
	require.NoError(t, json.Unmarshal(data, &agentic))
	require.NotEmpty(t, agentic.Mutants)
	for _, m := range agentic.Mutants {
		assert.Equal(t, "calc.go", m.File)
		assert.Equal(t, int64(5), m.Line)
		assert.Contains(t, strings.Join(m.ContextLines, "\n"), "return a + b")
	}
	data, err = os.ReadFile(gitlabPath)
	require.NoError(t, err)
	var issues []struct {
		Location struct {
			Path  string `json:"path"`
			Lines struct {
				Begin int `json:"begin"`
			} `json:"lines"`
		} `json:"location"`
	}
	require.NoError(t, json.Unmarshal(data, &issues))
	require.NotEmpty(t, issues)
	for _, issue := range issues {
		assert.Equal(t, "calc.go", issue.Location.Path)
		assert.Equal(t, 5, issue.Location.Lines.Begin)
	}
}
