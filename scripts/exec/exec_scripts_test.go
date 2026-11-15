// Package exec_test drives the shipped --exec helper scripts end to end and
// asserts the exit codes mutago's engine relies on: 0 KILLED, 1 ESCAPED,
// 2 SKIP.
package exec_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

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
func runExecScript(t *testing.T, script, modDir, originalFile, mutationFile string) int {
	t.Helper()
	abs, err := filepath.Abs(script)
	require.NoError(t, err)

	cmd := exec.Command("bash", abs)
	cmd.Dir = modDir
	cmd.Env = append(os.Environ(),
		"MUTATE_ORIGINAL="+originalFile,
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
					assert.Equal(t, tc.want, runExecScript(t, script, modDir, "foo.go", mutationFile))

					original, err := os.ReadFile(filepath.Join(modDir, "foo.go"))
					require.NoError(t, err)
					assert.Contains(t, string(original), "return 1", "script must restore the original file")
				})
			}
		})
	}
}

func TestExecScriptsHandleSpecialPaths(t *testing.T) {
	for _, script := range execScripts {
		t.Run(script, func(t *testing.T) {
			modDir, mutationFile := writeModule(t, "return 1")
			parent := filepath.Dir(modDir)
			specialMutant := filepath.Join(parent, "mutant \"\\\t\n.go")
			require.NoError(t, os.Rename(mutationFile, specialMutant))
			original := filepath.Join(modDir, "foo.go")

			assert.Equal(t, escaped, runExecScript(t, script, modDir, original, specialMutant))
			require.NoError(t, os.WriteFile(specialMutant, []byte("package testmod\nfunc foo() int { return 2 }\n"), 0o644))
			assert.Equal(t, killed, runExecScript(t, script, modDir, original, specialMutant))
			after, err := os.ReadFile(original)
			require.NoError(t, err)
			assert.Contains(t, string(after), "return 1")
		})
	}
}

func TestExecScriptsHandleRelativeMutant(t *testing.T) {
	for _, script := range execScripts {
		t.Run(script, func(t *testing.T) {
			for _, mode := range []string{"logical relative", "logical absolute", "physical relative", "physical absolute"} {
				t.Run(mode, func(t *testing.T) {
					modDir, _ := writeModule(t, "return 2")
					physical, err := filepath.EvalSymlinks(modDir)
					require.NoError(t, err)
					logical := filepath.Join(filepath.Dir(modDir), "alias")
					require.NoError(t, os.Symlink(modDir, logical))
					dir, original := physical, "foo.go"
					if mode == "logical relative" || mode == "logical absolute" {
						dir = logical
					}
					if mode == "logical absolute" || mode == "physical absolute" {
						original = filepath.Join(physical, "foo.go")
					}
					t.Setenv("PWD", dir)
					assert.Equal(t, killed, runExecScript(t, script, dir, original, "../mutant.go"))
				})
			}
		})
	}
}

func TestExecScriptsRunFromDirectoryEndingInNewline(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "scripts\n")
	require.NoError(t, os.Mkdir(dir, 0o755))
	for _, name := range append([]string{"overlay.sh"}, execScripts...) {
		content, err := os.ReadFile(name)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), content, 0o755))
	}
	for _, script := range execScripts {
		t.Run(script, func(t *testing.T) {
			modDir, mutant := writeModule(t, "return 2")
			assert.Equal(t, killed, runExecScript(t, filepath.Join(dir, script), modDir, "foo.go", mutant))
		})
	}
}

func TestExecScriptsPassSpecialPathsToGo(t *testing.T) {
	for _, script := range execScripts {
		t.Run(script, func(t *testing.T) {
			modDir, _ := writeModule(t, "return 1")
			control := make([]byte, 31)
			for i := range control {
				control[i] = byte(i + 1)
			}
			specialDir := filepath.Join(t.TempDir(), "source \"\\"+string(control)+"\n")
			require.NoError(t, os.Mkdir(specialDir, 0o755))
			original := filepath.Join(specialDir, "source.go")
			mutantDir := filepath.Join(t.TempDir(), "mutant directory \"\\"+string(control)+"\n")
			require.NoError(t, os.Mkdir(mutantDir, 0o755))
			mutant := filepath.Join(mutantDir, "mutant \"\\"+string(control))
			require.NoError(t, os.WriteFile(original, []byte("same\n"), 0o644))
			require.NoError(t, os.WriteFile(mutant, []byte("same\n"), 0o644))

			fakeDir := t.TempDir()
			capture := filepath.Join(fakeDir, "overlay.json")
			fakeGo := "#!/bin/sh\nwhile [ \"$#\" -gt 0 ]; do\n  if [ \"$1\" = -overlay ]; then cat \"$2\" > \"$CAPTURE\"; exit $?; fi\n  shift\ndone\nexit 3\n"
			require.NoError(t, os.WriteFile(filepath.Join(fakeDir, "go"), []byte(fakeGo), 0o755))
			t.Setenv("PATH", fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("CAPTURE", capture)
			assert.Equal(t, escaped, runExecScript(t, script, modDir, original, mutant))

			content, err := os.ReadFile(capture)
			require.NoError(t, err)
			var overlay struct {
				Replace map[string]string
			}
			require.NoError(t, json.Unmarshal(content, &overlay))
			physicalDir, err := filepath.EvalSymlinks(specialDir)
			require.NoError(t, err)
			want := map[string]string{filepath.Join(physicalDir, "source.go"): mutant, original: mutant}
			assert.Equal(t, want, overlay.Replace)
		})
	}
}

func TestExecScriptsRejectExecutorSetupErrors(t *testing.T) {
	for _, script := range execScripts {
		t.Run(script, func(t *testing.T) {
			t.Run("missing mutant", func(t *testing.T) {
				modDir, mutationFile := writeModule(t, "return 2")
				require.NoError(t, os.Remove(mutationFile))
				assert.Equal(t, 3, runExecScript(t, script, modDir, "foo.go", mutationFile))
			})
			t.Run("Go cannot read overlay", func(t *testing.T) {
				modDir, mutationFile := writeModule(t, "return 2")
				fakeDir := t.TempDir()
				fakeGo := "#!/bin/sh\nprintf 'go: cannot read overlay\\n' >&2\nexit 1\n"
				require.NoError(t, os.WriteFile(filepath.Join(fakeDir, "go"), []byte(fakeGo), 0o755))
				t.Setenv("PATH", fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"))
				assert.Equal(t, 3, runExecScript(t, script, modDir, "foo.go", mutationFile))
			})
			t.Run("test output resembles a Go diagnostic", func(t *testing.T) {
				modDir, mutationFile := writeModule(t, "return 2")
				mainTest := `package testmod
import (
	"fmt"
	"os"
	"testing"
)
func TestMain(m *testing.M) {
	fmt.Println("go: diagnostic from the test suite")
	os.Exit(m.Run())
}
`
				require.NoError(t, os.WriteFile(filepath.Join(modDir, "main_test.go"), []byte(mainTest), 0o644))
				assert.Equal(t, killed, runExecScript(t, script, modDir, "foo.go", mutationFile))
			})
		})
	}
}

func TestExecScriptsPreserveSourceWhenKilled(t *testing.T) {
	for _, script := range execScripts {
		t.Run(script, func(t *testing.T) {
			modDir, mutationFile := writeModule(t, "return 2")
			originalPath := filepath.Join(modDir, "foo.go")
			original, err := os.ReadFile(originalPath)
			require.NoError(t, err)
			backupPath := originalPath + ".tmp"
			backup := []byte("existing backup\n")
			require.NoError(t, os.WriteFile(backupPath, backup, 0o644))

			fakeDir := t.TempDir()
			ready := filepath.Join(fakeDir, "ready")
			release := filepath.Join(fakeDir, "release")
			finished := filepath.Join(fakeDir, "finished")
			fakeGo := "#!/bin/sh\n: > \"$READY\"\nwhile [ ! -e \"$RELEASE\" ]; do sleep 0.02; done\n: > \"$FINISHED\"\n"
			require.NoError(t, os.WriteFile(filepath.Join(fakeDir, "go"), []byte(fakeGo), 0o755))

			abs, err := filepath.Abs(script)
			require.NoError(t, err)
			cmd := exec.Command("bash", abs)
			cmd.Dir = modDir
			cmd.Env = append(os.Environ(),
				"PATH="+fakeDir+string(os.PathListSeparator)+os.Getenv("PATH"),
				"READY="+ready,
				"RELEASE="+release,
				"FINISHED="+finished,
				"MUTATE_ORIGINAL="+originalPath,
				"MUTATE_CHANGED="+mutationFile,
				"MUTATE_PACKAGE=example.com/testmod",
				"MUTATE_TIMEOUT=30",
			)
			require.NoError(t, cmd.Start())
			t.Cleanup(func() {
				_ = os.WriteFile(release, nil, 0o644)
				if cmd.ProcessState == nil {
					_ = cmd.Process.Kill()
					_ = cmd.Wait()
				}
			})
			exists := func(path string) bool {
				_, err := os.Stat(path)
				return err == nil
			}
			require.Eventually(t, func() bool { return exists(ready) }, 5*time.Second, 10*time.Millisecond)
			require.NoError(t, cmd.Process.Kill())
			require.NoError(t, os.WriteFile(release, nil, 0o644))
			require.Error(t, cmd.Wait())
			require.Eventually(t, func() bool { return exists(finished) }, 5*time.Second, 10*time.Millisecond)

			after, err := os.ReadFile(originalPath)
			require.NoError(t, err)
			assert.Equal(t, original, after, "killing the executor must leave the source unchanged")
			afterBackup, err := os.ReadFile(backupPath)
			require.NoError(t, err)
			assert.Equal(t, backup, afterBackup, "killing the executor must leave an existing backup unchanged")
		})
	}
}
