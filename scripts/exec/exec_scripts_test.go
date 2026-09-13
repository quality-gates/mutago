// Package exec_test drives the shipped --exec helper scripts end to end and
// asserts the exit codes mutago's engine relies on: 0 KILLED, 1 ESCAPED,
// 2 SKIP.
package exec_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// execScripts are the helper scripts under test, keyed by file name.
var execScripts = []string{"test-mutated-package.sh", "test-current-directory.sh"}

const (
	killed  = 0
	escaped = 1
	skipped = 2
)

// writeModule lays out a single-package module whose test calls foo, and
// returns the module directory plus the path of a mutation file holding
// mutantBody as the body of foo.
func writeModule(t *testing.T, mutantBody string) (modDir, mutationFile string) {
	t.Helper()
	modDir = filepath.Join(t.TempDir(), "mod")
	require.NoError(t, os.MkdirAll(modDir, 0o755))

	write := func(dir, name, content string) string {
		path := filepath.Join(dir, name)
		require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
		return path
	}

	write(modDir, "go.mod", "module example.com/testmod\n\ngo 1.21\n")
	write(modDir, "foo.go", "package testmod\n\nfunc foo() int { return 1 }\n")
	write(modDir, "foo_test.go", `package testmod

import "testing"

func TestFoo(t *testing.T) {
	if foo() != 1 {
		t.Fatal("foo changed")
	}
}
`)

	// The mutation lives outside the package directory, the way mutago writes
	// it, so the only build error is the mutation itself.
	mutationFile = write(filepath.Dir(modDir), "mutant.go", "package testmod\n\nfunc foo() int { "+mutantBody+" }\n")
	return modDir, mutationFile
}

// runExecScript runs script against the mutation and returns its exit code.
func runExecScript(t *testing.T, script, modDir, mutationFile string) int {
	t.Helper()
	abs, err := filepath.Abs(script)
	require.NoError(t, err)

	cmd := exec.Command("bash", abs)
	cmd.Dir = modDir
	cmd.Env = append(os.Environ(),
		"MUTATE_ORIGINAL=foo.go",
		"MUTATE_CHANGED="+mutationFile,
		"MUTATE_PACKAGE=example.com/testmod",
		"MUTATE_TIMEOUT=30",
	)
	out, err := cmd.CombinedOutput()
	t.Logf("%s output:\n%s", script, out)

	var exitErr *exec.ExitError
	if err == nil {
		return 0
	}
	require.ErrorAs(t, err, &exitErr, "script must exit with a status, not fail to run")
	return exitErr.ExitCode()
}

func TestExecScriptsClassifyMutants(t *testing.T) {
	for _, script := range execScripts {
		t.Run(script, func(t *testing.T) {
			cases := []struct {
				name       string
				mutantBody string
				want       int
			}{
				// go test exits 1 on a build failure just as it does on a test
				// failure, so an uncompilable mutant must not be read as killed.
				{"uncompilable mutant is skipped", "var s string; return s", skipped},
				{"failing test kills mutant", "return 2", killed},
				{"passing test lets mutant escape", "return 1", escaped},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					modDir, mutationFile := writeModule(t, tc.mutantBody)
					assert.Equal(t, tc.want, runExecScript(t, script, modDir, mutationFile))

					original, err := os.ReadFile(filepath.Join(modDir, "foo.go"))
					require.NoError(t, err)
					assert.Contains(t, string(original), "return 1", "script must restore the original file")
				})
			}
		})
	}
}
