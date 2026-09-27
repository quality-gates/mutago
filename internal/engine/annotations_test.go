package engine

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"
)

func TestPrintGitHubAnnotations_RepoRelativeForRelativeTargets(t *testing.T) {
	repo, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	pkgDir := filepath.Join(repo, "mod", "internal", "store")
	if err := os.MkdirAll(pkgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "-q", repo)
	var env []string
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GIT_") {
			env = append(env, e)
		}
	}
	cmd.Env = env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	t.Chdir(pkgDir)

	for _, target := range []string{"store.go", "./store.go", filepath.Join(pkgDir, "store.go")} {
		var m models.Mutant
		m.Mutator.OriginalFilePath = target
		m.Mutator.MutatorName = "statement/return"
		m.Mutator.OriginalStartLine = 4
		var out bytes.Buffer
		printGitHubAnnotations(&out, &models.Report{Escaped: []models.Mutant{m}})
		if !strings.Contains(out.String(), "file=mod/internal/store/store.go,line=4") {
			t.Errorf("target %q: annotation not repo-relative:\n%s", target, out.String())
		}
	}
}
