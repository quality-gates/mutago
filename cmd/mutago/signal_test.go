package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMainHelperProcess(t *testing.T) {
	if os.Getenv("MUTAGO_TEST_HELPER_PROCESS") != "1" {
		return
	}
	args := strings.Fields(os.Getenv("MUTAGO_TEST_ARGS"))
	os.Exit(mainCmd(args))
}

func TestSignalInterruptCleansUpTmpDir(t *testing.T) {
	signals := []struct {
		name string
		sig  syscall.Signal
	}{
		{name: "SIGINT", sig: syscall.SIGINT},
		{name: "SIGTERM", sig: syscall.SIGTERM},
	}

	for _, tc := range signals {
		t.Run(tc.name, func(t *testing.T) {
			isolatedTmp := t.TempDir()

			// Script that signals it has started by creating a file, then sleeps.
			execScript := filepath.Join(isolatedTmp, "exec_sleep.sh")
			readyFile := filepath.Join(isolatedTmp, "ready.txt")
			scriptContent := "#!/bin/sh\ntouch \"" + readyFile + "\"\nsleep 5\n"
			err := os.WriteFile(execScript, []byte(scriptContent), 0755)
			require.NoError(t, err)

			cmd := exec.Command(os.Args[0], "-test.run=^TestMainHelperProcess$")
			cmd.Env = append(os.Environ(),
				"MUTAGO_TEST_HELPER_PROCESS=1",
				"TMPDIR="+isolatedTmp,
				"MUTAGO_TEST_ARGS=--exec "+execScript+" ../../example",
			)

			err = cmd.Start()
			require.NoError(t, err)

			// Wait for the helper process to reach the exec stage (readyFile created)
			require.Eventually(t, func() bool {
				_, err := os.Stat(readyFile)
				return err == nil
			}, 3*time.Second, 20*time.Millisecond, "timed out waiting for child process to start exec")

			// Check that at least one mutago temporary folder exists before interrupting
			entries, err := os.ReadDir(isolatedTmp)
			require.NoError(t, err)
			var tmpDirsBefore []string
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), "mutago-") {
					tmpDirsBefore = append(tmpDirsBefore, e.Name())
				}
			}
			require.NotEmpty(t, tmpDirsBefore, "expected mutago-* tmp dir to exist before signal")

			// Send the interruption signal
			err = cmd.Process.Signal(tc.sig)
			require.NoError(t, err)

			// Wait for child process to exit
			waitErr := cmd.Wait()
			// Acceptance criteria: Signal handling exits with a non-zero exit status.
			assert.Error(t, waitErr, "expected non-zero exit status on signal termination")

			// Acceptance criteria: Terminating a running mutago process with SIGINT or SIGTERM cleans up its temporary directory and overlay files.
			entriesAfter, err := os.ReadDir(isolatedTmp)
			require.NoError(t, err)
			var leaked []string
			for _, e := range entriesAfter {
				if strings.HasPrefix(e.Name(), "mutago-") {
					leaked = append(leaked, e.Name())
				}
			}
			assert.Empty(t, leaked, "expected temporary directories/overlay files to be cleaned up, but found leaked files: %v", leaked)
		})
	}
}

func TestSignalInterruptRetainsTmpDirWithFlag(t *testing.T) {
	isolatedTmp := t.TempDir()

	execScript := filepath.Join(isolatedTmp, "exec_sleep.sh")
	readyFile := filepath.Join(isolatedTmp, "ready.txt")
	scriptContent := "#!/bin/sh\ntouch \"" + readyFile + "\"\nsleep 5\n"
	err := os.WriteFile(execScript, []byte(scriptContent), 0755)
	require.NoError(t, err)

	cmd := exec.Command(os.Args[0], "-test.run=^TestMainHelperProcess$")
	cmd.Env = append(os.Environ(),
		"MUTAGO_TEST_HELPER_PROCESS=1",
		"TMPDIR="+isolatedTmp,
		"MUTAGO_TEST_ARGS=--do-not-remove-tmp-folder --exec "+execScript+" ../../example",
	)

	err = cmd.Start()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, err := os.Stat(readyFile)
		return err == nil
	}, 3*time.Second, 20*time.Millisecond, "timed out waiting for child process to start exec")

	err = cmd.Process.Signal(syscall.SIGINT)
	require.NoError(t, err)

	waitErr := cmd.Wait()
	assert.Error(t, waitErr)

	// Acceptance criteria: When --do-not-remove-tmp-folder is provided, the temporary folder is retained even on exit.
	entriesAfter, err := os.ReadDir(isolatedTmp)
	require.NoError(t, err)
	var retained []string
	for _, e := range entriesAfter {
		if strings.HasPrefix(e.Name(), "mutago-") {
			retained = append(retained, e.Name())
		}
	}
	assert.NotEmpty(t, retained, "expected temporary folder to be retained when --do-not-remove-tmp-folder is set")
}

func TestNormalRunCleansUpTmpDir(t *testing.T) {
	isolatedTmp := t.TempDir()

	cmd := exec.Command(os.Args[0], "-test.run=^TestMainHelperProcess$")
	cmd.Env = append(os.Environ(),
		"MUTAGO_TEST_HELPER_PROCESS=1",
		"TMPDIR="+isolatedTmp,
		"MUTAGO_TEST_ARGS=--match foo --exec-timeout 5 ../../example",
	)

	err := cmd.Run()
	assert.NoError(t, err)

	// Acceptance criteria: Normal successful and failed runs continue to clean up their temporary files.
	entriesAfter, err := os.ReadDir(isolatedTmp)
	require.NoError(t, err)
	var leaked []string
	for _, e := range entriesAfter {
		if strings.HasPrefix(e.Name(), "mutago-") {
			leaked = append(leaked, e.Name())
		}
	}
	assert.Empty(t, leaked, "expected temporary files to be cleaned up after normal run: %v", leaked)
}

func TestSignalInterruptCleansUpBuiltinExec(t *testing.T) {
	isolatedTmp := t.TempDir()
	pkgDir := t.TempDir()

	goMod := "module builtinpkg\n\ngo 1.22\n"
	err := os.WriteFile(filepath.Join(pkgDir, "go.mod"), []byte(goMod), 0644)
	require.NoError(t, err)

	calcGo := "package builtinpkg\n\nfunc Calc(a, b int) int {\n\treturn a + b\n}\n"
	err = os.WriteFile(filepath.Join(pkgDir, "calc.go"), []byte(calcGo), 0644)
	require.NoError(t, err)

	flagFile := filepath.Join(isolatedTmp, "baseline_passed.txt")
	mutantRunningFile := filepath.Join(isolatedTmp, "mutant_running.txt")

	calcTestGo := `package builtinpkg

import (
	"os"
	"testing"
	"time"
)

func TestCalc(t *testing.T) {
	if _, err := os.Stat("` + flagFile + `"); os.IsNotExist(err) {
		_ = os.WriteFile("` + flagFile + `", []byte("ok"), 0644)
		if Calc(1, 2) != 3 {
			t.Fail()
		}
		return
	}
	_ = os.WriteFile("` + mutantRunningFile + `", []byte("running"), 0644)
	time.Sleep(5 * time.Second)
	if Calc(1, 2) != 3 {
		t.Fail()
	}
}
`
	err = os.WriteFile(filepath.Join(pkgDir, "calc_test.go"), []byte(calcTestGo), 0644)
	require.NoError(t, err)

	cmd := exec.Command(os.Args[0], "-test.run=^TestMainHelperProcess$")
	cmd.Dir = pkgDir
	cmd.Env = append(os.Environ(),
		"MUTAGO_TEST_HELPER_PROCESS=1",
		"TMPDIR="+isolatedTmp,
		"MUTAGO_TEST_ARGS=--exec-timeout 10 .",
	)

	err = cmd.Start()
	require.NoError(t, err)

	require.Eventually(t, func() bool {
		_, err := os.Stat(mutantRunningFile)
		return err == nil
	}, 15*time.Second, 20*time.Millisecond, "timed out waiting for mutant test to begin execution")

	err = cmd.Process.Signal(syscall.SIGINT)
	require.NoError(t, err)

	waitErr := cmd.Wait()
	assert.Error(t, waitErr)

	// Verify that all temporary folders and overlay files in TMPDIR were cleaned up
	entriesAfter, err := os.ReadDir(isolatedTmp)
	require.NoError(t, err)
	var leaked []string
	for _, e := range entriesAfter {
		if strings.HasPrefix(e.Name(), "mutago-") {
			leaked = append(leaked, e.Name())
		}
	}
	assert.Empty(t, leaked, "expected built-in overlay files and tmp dirs to be cleaned up: %v", leaked)
}
