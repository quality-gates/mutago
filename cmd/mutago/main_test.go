package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"
	"github.com/quality-gates/mutago/v2/internal/parser"

	"github.com/stretchr/testify/assert"
)

func TestMainSimple(t *testing.T) {
	testMain(
		t,
		"../../example",
		[]string{"--debug", "--exec-timeout", "1"},
		returnOk,
		"mutation score",
	)
}

func TestMainRecursive(t *testing.T) {
	testMain(
		t,
		"../../example",
		[]string{"--debug", "--exec-timeout", "1", "./..."},
		returnOk,
		"mutation score",
	)
}

func TestMainFromOtherDirectory(t *testing.T) {
	testMain(
		t,
		"../..",
		[]string{"--debug", "--exec-timeout", "1", "github.com/quality-gates/mutago/v2/example"},
		returnOk,
		"mutation score",
	)
}

func TestMainMatch(t *testing.T) {
	testMain(
		t,
		"../../example",
		[]string{"--debug", "--exec", "../scripts/exec/test-mutated-package.sh", "--exec-timeout", "1", "--match", "baz", "./..."},
		returnOk,
		"mutation score",
	)
}

func TestMainUnknownConfigField(t *testing.T) {
	testMain(
		t,
		"../../example",
		[]string{"--exec-timeout", "1", "--config", "../testdata/configs/configUnknownField.yml.test"},
		returnError,
		"Could not parse config file",
	)
}

func TestMainSkipWithoutTest(t *testing.T) {
	testMain(
		t,
		"../../example",
		[]string{"--debug", "--exec-timeout", "1", "--config", "../testdata/configs/configSkipWithoutTest.yml.test"},
		returnOk,
		"mutation score",
	)
}

func TestMainMinMsiPass(t *testing.T) {
	testMain(
		t,
		"../../example",
		[]string{"--debug", "--exec-timeout", "1", "--min-msi", "1"},
		returnOk,
		"mutation score",
	)
}

func TestMainMinMsiFail(t *testing.T) {
	// 101 exceeds the maximum possible MSI (100%) so the gate always fires.
	testMain(
		t,
		"../../example",
		[]string{"--debug", "--exec-timeout", "1", "--min-msi", "101"},
		returnMsiThresholdNotMet,
		"MSI",
	)
}

func TestMainMinCoveredMsiNoProfile(t *testing.T) {
	// Without --coverage the gate cannot be evaluated and must fail with a
	// clear message rather than silently comparing against 0.
	testMain(
		t,
		"../../example",
		[]string{"--debug", "--exec-timeout", "1", "--min-covered-msi", "90"},
		returnMsiThresholdNotMet,
		"Covered MSI cannot be checked",
	)
}

func TestMainJSONReport(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "mutago-main-test-")
	assert.NoError(t, err)

	reportFileName := "reportTestMainJSONReport.json"
	jsonFile := tmpDir + "/" + reportFileName
	if _, err := os.Stat(jsonFile); err == nil {
		err = os.Remove(jsonFile)
		assert.NoError(t, err)
	}

	models.ReportFileName = jsonFile

	testMain(
		t,
		"../../example",
		[]string{"--debug", "--exec-timeout", "1", "--config", "../testdata/configs/configForJson.yml.test"},
		returnOk,
		"mutation score",
	)

	info, err := os.Stat(jsonFile)
	assert.NoError(t, err)
	assert.NotNil(t, info)

	defer func() {
		err = os.Remove(jsonFile)
		if err != nil {
			fmt.Println("Error while deleting temp file")
		}
	}()

	jsonData, err := os.ReadFile(jsonFile)
	assert.NoError(t, err)

	var mutationReport models.Report
	err = json.Unmarshal(jsonData, &mutationReport)
	assert.NoError(t, err)

	s := mutationReport.Stats
	// Totals must be internally consistent.
	assert.Equal(t, s.TotalMutantsCount, s.KilledCount+s.EscapedCount+s.ErrorCount+s.SkippedCount+s.NotCoveredCount)
	// Something must have been mutated and tested.
	assert.Greater(t, s.KilledCount+s.EscapedCount, int64(0))
	// MSI must be a valid ratio.
	assert.GreaterOrEqual(t, s.Msi, 0.0)
	assert.LessOrEqual(t, s.Msi, 1.0)
	// Collection lengths must match the stat fields.
	assert.Equal(t, int(s.EscapedCount), len(mutationReport.Escaped))
	assert.Equal(t, int(s.KilledCount), len(mutationReport.Killed))
	assert.Nil(t, mutationReport.Errored)

	for i := 0; i < len(mutationReport.Escaped); i++ {
		assert.Contains(t, mutationReport.Escaped[i].ProcessOutput, "ESCAPED")
	}
	for i := 0; i < len(mutationReport.Killed); i++ {
		assert.Contains(t, mutationReport.Killed[i].ProcessOutput, "KILLED")
	}
}

func TestMainTestFlagsPassthrough(t *testing.T) {
	// --test-flags is passed to each go test invocation.
	// Using -count=1 forces real test runs (no cache) and should not break anything.
	// The flag value must be passed with = to prevent go-flags from mis-parsing
	// values that start with a hyphen.
	testMain(
		t,
		"../../example",
		[]string{"--exec-timeout", "1", "--test-flags=-count=1"},
		returnOk,
		"mutation score",
	)
}

func TestMainPerTestFlag(t *testing.T) {
	// --per-test builds a per-test coverage map and runs only covering tests
	// for each mutation. Results must be identical to running without --per-test.
	testMain(
		t,
		"../../example",
		[]string{"--exec-timeout", "5", "--coverage", "--per-test"},
		returnOk,
		"mutation score",
	)
}

func TestMainDryRun(t *testing.T) {
	// --dry-run must exit 0 and report how many mutations would be generated
	// without writing any files or running any tests.
	testMain(
		t,
		"../../example",
		[]string{"--dry-run"},
		returnOk,
		"would be generated",
	)
}

func TestMainNoDiffs(t *testing.T) {
	// --no-diffs must complete normally without crashing; the summary line
	// confirms the run finished.
	testMain(
		t,
		"../../example",
		[]string{"--exec-timeout", "1", "--no-diffs"},
		returnOk,
		"mutation score",
	)
}

func TestMainOutputStatuses(t *testing.T) {
	// --output-statuses filters terminal output but must not affect the summary.
	testMain(
		t,
		"../../example",
		[]string{"--exec-timeout", "1", "--output-statuses", "k"},
		returnOk,
		"mutation score",
	)
}

func TestMainConfigDisableMutators(t *testing.T) {
	// disable_mutators in config silently drops the arithmetic category;
	// the run still completes and reports a mutation score.
	testMain(
		t,
		"../../example",
		[]string{"--exec-timeout", "1", "--config", "../testdata/configs/configDisableMutators.yml.test"},
		returnOk,
		"mutation score",
	)
}

func TestMainConfigEnableMutators(t *testing.T) {
	// enable_mutators allowlist restricts to branch/if only;
	// the run still completes and reports a mutation score.
	testMain(
		t,
		"../../example",
		[]string{"--exec-timeout", "1", "--config", "../testdata/configs/configEnableMutators.yml.test"},
		returnOk,
		"mutation score",
	)
}

func TestMainGenericsTypeUnionNoInternalError(t *testing.T) {
	// Regression test for: running mutago against a package that contains
	// a generics type union constraint (e.g. `*A | *B | *C` in an interface body)
	// previously produced "INTERNAL ERROR … expected ')', found '|'" because the
	// arithmetic/bitwise mutator treated the type-level | as a bitwise operator.
	parser.ClearPackageCache()

	saveStderr := os.Stderr
	saveStdout := os.Stdout
	saveCwd, err := os.Getwd()
	assert.Nil(t, err)

	r, w, err := os.Pipe()
	assert.Nil(t, err)

	os.Stderr = w
	os.Stdout = w
	assert.Nil(t, os.Chdir("../../testdata/genericsexample"))

	bufChannel := make(chan string)
	go func() {
		buf := new(bytes.Buffer)
		_, _ = io.Copy(buf, r)
		_ = r.Close()
		bufChannel <- buf.String()
	}()

	exitCode := mainCmd([]string{"--exec-timeout", "10"})

	assert.Nil(t, w.Close())
	os.Stderr = saveStderr
	os.Stdout = saveStdout
	assert.Nil(t, os.Chdir(saveCwd))

	out := <-bufChannel

	assert.Equal(t, returnOk, exitCode)
	assert.Contains(t, out, "mutation score")
	assert.NotContains(t, out, "INTERNAL ERROR")
}

func testMain(t *testing.T, root string, exec []string, expectedExitCode int, contains string) {
	// Clear the parser cache so each test loads files fresh from disk.
	// Without this, TestMainMatch's exec script (which writes to the original
	// file on disk) can leave the cache holding a stale AST for later tests.
	parser.ClearPackageCache()

	saveStderr := os.Stderr
	saveStdout := os.Stdout
	saveCwd, err := os.Getwd()
	assert.Nil(t, err)

	r, w, err := os.Pipe()
	assert.Nil(t, err)

	os.Stderr = w
	os.Stdout = w
	assert.Nil(t, os.Chdir(root))

	bufChannel := make(chan string)

	go func() {
		buf := new(bytes.Buffer)
		_, err = io.Copy(buf, r)
		assert.Nil(t, err)
		assert.Nil(t, r.Close())

		bufChannel <- buf.String()
	}()

	exitCode := mainCmd(exec)

	assert.Nil(t, w.Close())

	os.Stderr = saveStderr
	os.Stdout = saveStdout
	assert.Nil(t, os.Chdir(saveCwd))

	out := <-bufChannel

	assert.Equal(t, expectedExitCode, exitCode)
	assert.Contains(t, out, contains)
}
