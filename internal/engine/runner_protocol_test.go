package engine

import (
	"context"
	"fmt"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"
)

func TestRunnerOutcomes(t *testing.T) {
	dir := t.TempDir()
	command := "#!/bin/sh\nprintf '%s\\n' \"$RUNNER_OUTPUT\"\nexit \"$RUNNER_EXIT\"\n"
	for _, name := range []string{"go", "custom-runner"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(command), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	original := []byte("package sample\nvar n = 1\n")
	file := filepath.Join(dir, "sample.go")
	if err := os.WriteFile(file, original, 0o600); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name   string
		custom bool
		code   int
		output string
		want   models.MutantOutcome
	}{
		{"builtin pass escapes", false, 0, "ok", models.OutcomeEscaped},
		{"builtin test failure kills", false, 1, "FAIL test", models.OutcomeKilled},
		{"builtin build failure skips", false, 1, "FAIL [build failed]", models.OutcomeSkipped},
		{"builtin setup failure skips", false, 1, "FAIL [setup failed]", models.OutcomeSkipped},
		{"builtin driver error", false, 3, "error", models.OutcomeErrored},
		{"custom zero kills", true, 0, "", models.OutcomeKilled},
		{"custom one escapes", true, 1, "", models.OutcomeEscaped},
		{"custom two skips", true, 2, "", models.OutcomeSkipped},
		{"custom other errors", true, 7, "", models.OutcomeErrored},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("RUNNER_EXIT", fmt.Sprint(tc.code))
			t.Setenv("RUNNER_OUTPUT", tc.output)
			opts := &models.Options{}
			if tc.custom {
				opts.Exec.Exec = filepath.Join(dir, "custom-runner")
			}
			policy, _ := resolveRunner(opts, modeExecute)
			listener := &recordingListener{}
			results := &completedResults{listener: listener}
			job := execJob{
				ctx: context.Background(), opts: opts, pkg: types.NewPackage("sample", "sample"),
				test: goTestTarget{pkg: "sample", dir: dir}, runner: policy, tmpDir: dir,
				mutant: resultMutant("candidate", "mutator"), out: jobOutput{stdout: io.Discard, stderr: io.Discard},
				source: mutationSource{originalFile: file, mutationFile: filepath.Join(dir, "mutant.go"), original: original, edit: mutationEdit{start: len(original) - 2, end: len(original) - 1, replacement: []byte("2")}},
			}
			runExecJob(job, results)
			report := &models.Report{}
			results.fill(report)
			got := map[models.MutantOutcome][]models.Mutant{models.OutcomeKilled: report.Killed, models.OutcomeEscaped: report.Escaped, models.OutcomeSkipped: report.Skipped, models.OutcomeErrored: report.Errored}
			for outcome, mutants := range got {
				if (len(mutants) == 1) != (outcome == tc.want) {
					t.Fatalf("outcome %v: got %d mutants; expected %v", outcome, len(mutants), tc.want)
				}
			}
		})
	}
}
