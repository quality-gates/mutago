package engine

import (
	"context"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/quality-gates/mutago/v2/internal/models"
)

// TestCustomExecRestoresSource is compiled into the real engine package through
// go test -overlay. Its fake go command pauses only after the script has copied
// the mutant into the source file; runCustomExec and the scripts are real.
func TestCustomExecRestoresSource(t *testing.T) {
	root := os.Getenv("MUTAGO_VERIFY_ROOT")
	if root == "" {
		t.Fatal("MUTAGO_VERIFY_ROOT is not set")
	}
	for _, script := range []string{"test-mutated-package.sh", "test-current-directory.sh"} {
		for _, scenario := range []string{"normal", "cancel", "timeout"} {
			t.Run(script+"/"+scenario, func(t *testing.T) {
				probeCustomExec(t, root, script, scenario)
			})
		}
	}
}

func probeCustomExec(t *testing.T, root, script, scenario string) {
	t.Helper()
	dir := t.TempDir()
	original := filepath.Join(dir, "source.go")
	mutant := filepath.Join(dir, "mutant.go")
	want := []byte("package sample\nvar n = 1\n")
	changed := []byte("package sample\nvar n = 2\n")
	for path, data := range map[string][]byte{original: want, mutant: changed} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	started := filepath.Join(dir, "started")
	release := filepath.Join(dir, "release")
	finished := filepath.Join(dir, "finished")
	shellPID := filepath.Join(dir, "shell-pid")
	fakeGo := "#!/bin/sh\n: > \"$MUTAGO_PROBE_STARTED\"\n" +
		"while [ ! -e \"$MUTAGO_PROBE_RELEASE\" ]; do sleep 0.02; done\n" +
		": > \"$MUTAGO_PROBE_FINISHED\"\n"
	if err := os.WriteFile(filepath.Join(dir, "go"), []byte(fakeGo), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("MUTAGO_PROBE_STARTED", started)
	t.Setenv("MUTAGO_PROBE_RELEASE", release)
	t.Setenv("MUTAGO_PROBE_FINISHED", finished)
	t.Setenv("MUTAGO_PROBE_SHELL_PID", shellPID)
	t.Cleanup(func() { _ = os.WriteFile(release, nil, 0600) })

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	opts := &models.Options{}
	opts.Exec.Timeout = 30
	if scenario == "timeout" {
		opts.Exec.Timeout = 1
	}
	stdout, stderr := synchronizedWriters(io.Discard, io.Discard)
	job := execJob{
		ctx: ctx, opts: opts, pkg: types.NewPackage("sample", "sample"),
		runner: runnerPolicy{command: []string{"bash", "-c", `echo "$$" > "$MUTAGO_PROBE_SHELL_PID"; exec bash "$1"`, "probe", filepath.Join(root, "scripts", "exec", script)}},
		out:    jobOutput{stdout: stdout, stderr: stderr},
		source: mutationSource{originalFile: original, mutationFile: mutant},
	}
	done := make(chan int, 1)
	go func() { done <- runCustomExec(job, &models.Mutant{}) }()
	waitForProbeFile(t, started)
	assertProbeFile(t, original, changed, "precondition: the script installed the mutant")
	assertProbeFile(t, original+".tmp", want, "precondition: the original is in the backup")

	if scenario == "cancel" {
		cancel()
	}
	if scenario != "normal" {
		// Go may wait for the test child to close inherited pipes. Wait for the
		// script to be reaped, then release the child; cap the wait so a fixed
		// implementation that waits for its child can still complete the probe.
		waitForScriptExit(t, shellPID)
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	var code int
	select {
	case code = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("runCustomExec did not return")
	}
	if scenario != "normal" && code != 3 {
		t.Errorf("%s exec returned %d; want tool error 3", scenario, code)
	}
	if scenario == "normal" && code != 1 {
		t.Errorf("passing fake go test returned %d; want escaped 1", code)
	}
	if err := os.WriteFile(release, nil, 0600); err != nil {
		t.Fatal(err)
	}
	waitForProbeFile(t, finished)
	assertProbeFile(t, original, want, "source was not restored after "+scenario)
}

func waitForProbeFile(t *testing.T, path string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", filepath.Base(path))
}

func waitForScriptExit(t *testing.T, path string) {
	t.Helper()
	waitForProbeFile(t, path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func assertProbeFile(t *testing.T, path string, want []byte, context string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != string(want) {
		backup, backupErr := os.ReadFile(path + ".tmp")
		t.Errorf("%s: file=%q err=%v; backup=%q backupErr=%v", context, got, err, backup, backupErr)
	}
}
