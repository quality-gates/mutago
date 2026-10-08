package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckChangeExcludesUnchangedSurvivors(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	if os.Getenv("GOCACHE") == "" {
		t.Setenv("GOCACHE", filepath.Join(t.TempDir(), "cache"))
	}
	binary := filepath.Join(t.TempDir(), "mutago")
	runCommand(t, root, "go", "build", "-o", binary, "./cmd/mutago")
	workspace, base := mutationWorkspace(t)
	output, err := runGate(root, workspace, binary, base)
	if err != nil {
		t.Fatalf("changed-code gate failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "mutation score is 100.00%") {
		t.Fatalf("changed-code mutants were not all killed:\n%s", output)
	}

	output, err = runGate(root, workspace, binary, "--full")
	requireGateFailure(t, output, err)
	if !strings.Contains(string(output), "ESCAPED") {
		t.Fatalf("full-tree gate did not detect unchanged survivors:\n%s", output)
	}

	writeFixture(t, workspace, "mutator/numbers/probe.go", "package probe\n\nfunc IsPositive(n int) bool { return n > 0 }\nfunc Unchecked(n int) bool { return n > 101 }\n")
	output, err = runGate(root, workspace, binary, base)
	requireGateFailure(t, output, err)

	runCommand(t, workspace, "git", "add", ".")
	runCommand(t, workspace, "git", "commit", "-qm", "no remaining change")
	output, err = runGate(root, workspace, binary, "HEAD")
	if err != nil || !strings.Contains(string(output), "0 total)") {
		t.Fatalf("empty scope did not pass with zero mutants: %v\n%s", err, output)
	}
}

func runGate(root, workspace, binary, base string) ([]byte, error) {
	cmd := exec.Command(filepath.Join(root, "scripts", "check-change.sh"), base)
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), "MUTAGO_BIN="+binary, "GOMAXPROCS=1", "MUTAGO_WORKERS=1")
	return cmd.CombinedOutput()
}

func requireGateFailure(t *testing.T, output []byte, err error) {
	t.Helper()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 4 || !strings.Contains(string(output), "below minimum required") {
		t.Fatalf("expected quality gate exit 4: %v\n%s", err, output)
	}
}

func mutationWorkspace(t *testing.T) (string, string) {
	t.Helper()
	workspace := t.TempDir()
	writeFixture(t, workspace, "go.mod", "module github.com/quality-gates/mutago/v2\n\ngo 1.26.6\n")
	for _, pkg := range []string{
		"mutator/arithmetic", "mutator/branch", "mutator/composite", "mutator/concurrency",
		"mutator/conditional", "mutator/expression", "mutator/loop", "mutator/numbers",
		"mutator/select", "mutator/statement", "internal/filter", "internal/coverage",
		"internal/gitdiff", "internal/models",
	} {
		writeFixture(t, workspace, pkg+"/probe.go", "package probe\n\nfunc Stub() {}\n")
	}
	source := "package probe\n\nfunc IsPositive(n int) bool { return n >= 0 }\nfunc Unchecked(n int) bool { return n > 100 }\n"
	writeFixture(t, workspace, "mutator/numbers/probe.go", source)
	writeFixture(t, workspace, "mutator/numbers/probe_test.go", `package probe

import "testing"

func TestBoundaries(t *testing.T) {
	for _, n := range []int{-1, 0, 1} {
		if IsPositive(n) != (n == 1) {
			t.Fatalf("wrong result for %d", n)
		}
		Unchecked(n)
	}
}
`)
	runCommand(t, workspace, "git", "init", "-q")
	runCommand(t, workspace, "git", "config", "user.email", "mutation-gate@example.invalid")
	runCommand(t, workspace, "git", "config", "user.name", "Mutation Gate Test")
	runCommand(t, workspace, "git", "config", "core.hooksPath", "/dev/null")
	runCommand(t, workspace, "git", "add", ".")
	runCommand(t, workspace, "git", "commit", "-qm", "baseline")
	base := strings.TrimSpace(runCommand(t, workspace, "git", "rev-parse", "HEAD"))
	writeFixture(t, workspace, "mutator/numbers/probe.go", strings.Replace(source, "n >= 0", "n > 0", 1))
	return workspace, base
}

func writeFixture(t *testing.T, root, name, content string) {
	t.Helper()
	path := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func runCommand(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, output)
	}
	return string(output)
}
