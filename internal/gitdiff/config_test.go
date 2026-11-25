package gitdiff

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Regression tests for #353: ParseChangedLines must find the same changed
// lines whatever the repository's git display/content configuration sets,
// including when mutago runs from a subdirectory. Each case sets one
// configuration on an isolated repository and asserts the real changed line
// is reported; without the fix, every case below that the host git honours
// returns no changed lines (or shifted lines), silently producing an empty
// mutation scope.
func TestParseChangedLines_GitConfigIndependent(t *testing.T) {
	// Isolate git from the caller's environment and global/system config so
	// only the per-case repository-local settings under test apply.
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_PREFIX", "GIT_QUARANTINE_PATH"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")

	const changedLine = 3
	cases := []struct {
		name      string
		file      string // repo-relative file that changes
		configure func(t *testing.T, dir string)
		subdir    bool // run ParseChangedLines from the file's directory
	}{
		{
			name: "color.diff=always",
			configure: func(t *testing.T, dir string) {
				runGit(t, dir, "config", "color.diff", "always")
			},
		},
		{
			name: "diff.mnemonicPrefix=true",
			configure: func(t *testing.T, dir string) {
				runGit(t, dir, "config", "diff.mnemonicPrefix", "true")
			},
		},
		{
			name: "diff.noprefix=true",
			configure: func(t *testing.T, dir string) {
				runGit(t, dir, "config", "diff.noprefix", "true")
			},
		},
		{
			name: "diff.dstPrefix=w/",
			configure: func(t *testing.T, dir string) {
				runGit(t, dir, "config", "diff.dstPrefix", "w/")
			},
		},
		{
			name: "diff.external",
			configure: func(t *testing.T, dir string) {
				script := filepath.Join(dir, "ext-diff.sh")
				if err := os.WriteFile(script, []byte("#!/bin/sh\necho external diff output\n"), 0o755); err != nil {
					t.Fatal(err)
				}
				runGit(t, dir, "config", "diff.external", script)
			},
		},
		{
			name: "GIT_EXTERNAL_DIFF",
			// Set below after the script is created via configure.
			configure: func(t *testing.T, dir string) {
				script := filepath.Join(dir, "ext-diff.sh")
				if err := os.WriteFile(script, []byte("#!/bin/sh\necho external diff output\n"), 0o755); err != nil {
					t.Fatal(err)
				}
				// Communicate the script path to the case runner through
				// the environment inherited by ParseChangedLines.
				t.Setenv("GIT_EXTERNAL_DIFF", script)
			},
		},
		{
			name:   "diff.relative=true from subdirectory",
			file:   "sub/a.go",
			subdir: true,
			configure: func(t *testing.T, dir string) {
				runGit(t, dir, "config", "diff.relative", "true")
			},
		},
		{
			name: "combined display settings",
			configure: func(t *testing.T, dir string) {
				runGit(t, dir, "config", "color.diff", "always")
				runGit(t, dir, "config", "diff.mnemonicPrefix", "true")
				runGit(t, dir, "config", "diff.noprefix", "true")
				runGit(t, dir, "config", "diff.dstPrefix", "w/")
			},
		},
		{
			name: "non-ASCII filename (core.quotepath default)",
			file: "café.go",
		},
		{
			name: "textconv driver",
			configure: func(t *testing.T, dir string) {
				// The driver duplicates every line, shifting line numbers;
				// the real changed line must still be reported at line 3.
				runGit(t, dir, "config", "diff.shift.textconv", "awk '{print; print \"// extra\"}'")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			file := tc.file
			if file == "" {
				file = "a.go"
			}
			dir := setupChangedFileRepo(t, file, tc.name == "textconv driver")
			if tc.configure != nil {
				tc.configure(t, dir)
			}
			if tc.subdir {
				t.Chdir(filepath.Join(dir, filepath.Dir(file)))
			} else {
				t.Chdir(dir)
			}
			changed, err := ParseChangedLines("HEAD")
			if err != nil {
				t.Fatalf("ParseChangedLines: %v", err)
			}
			if !IsRelativeLineChanged(changed, file, changedLine) {
				t.Fatalf("changed line %d of %s not reported (got %v) with %s",
					changedLine, file, changed, tc.name)
			}
		})
	}
}

// TestParseChangedLines_InvalidBaseReturnsError pins the failure path of the
// pinned diff invocation: a base git cannot resolve must surface as a
// wrapped error, not as an empty scope. The errors.As assertion fails if
// the %w wrap is downgraded to %v.
func TestParseChangedLines_InvalidBaseReturnsError(t *testing.T) {
	for _, k := range []string{"GIT_DIR", "GIT_WORK_TREE", "GIT_INDEX_FILE", "GIT_PREFIX", "GIT_QUARANTINE_PATH"} {
		t.Setenv(k, "")
		os.Unsetenv(k)
	}
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	t.Setenv("GIT_CONFIG_GLOBAL", "/dev/null")
	t.Setenv("GIT_CONFIG_SYSTEM", "/dev/null")
	dir := setupChangedFileRepo(t, "a.go", false)
	t.Chdir(dir)
	changed, err := ParseChangedLines("no-such-ref-353")
	if err == nil {
		t.Fatalf("expected error for unknown base, got changed=%v", changed)
	}
	if changed != nil {
		t.Fatalf("expected nil changed lines on error, got %v", changed)
	}
	if !strings.Contains(err.Error(), "no-such-ref-353") {
		t.Fatalf("error should name the base ref, got %v", err)
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("error should wrap the git exit error, got %T: %v", err, err)
	}
}

// setupChangedFileRepo creates an isolated repository whose file at relPath
// changes on line 3 between the base commit and the working tree. When
// withTextconv is set, a committed .gitattributes assigns the shift driver
// to Go files (the driver itself is configured by the caller).
func setupChangedFileRepo(t *testing.T, relPath string, withTextconv bool) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q", "-b", "main")
	runGit(t, dir, "config", "user.email", "t@t.com")
	runGit(t, dir, "config", "user.name", "t")
	write := func(name, contents string) {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write(relPath, "package x\n\nfunc A() int { return 1 }\n")
	if withTextconv {
		write(".gitattributes", "*.go diff=shift\n")
	}
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-qm", "base")
	write(relPath, "package x\n\nfunc A() int { return 2 }\n")
	return dir
}
