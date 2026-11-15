package engine

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/baseline"
	"github.com/quality-gates/mutago/v2/internal/models"
)

func TestFinalizationPrecedence(t *testing.T) {
	for _, tc := range []struct {
		name            string
		id              bool
		found           bool
		update          bool
		errors          bool
		reportFailure   bool
		want            int
		baselineWritten bool
	}{
		{name: "missing id wins", id: true, update: true, want: returnError},
		{name: "baseline update wins over gate", update: true, want: returnOk, baselineWritten: true},
		{name: "errors prevent update", update: true, errors: true, want: returnError},
		{name: "report failure wins over individual id", id: true, found: true, reportFailure: true, want: returnError},
		{name: "individual id bypasses gate", id: true, found: true, want: returnOk},
		{name: "gate fails", want: returnMsiThresholdNotMet},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := models.NewOptions()
			opts.Baseline.Update = tc.update
			opts.Baseline.File = filepath.Join(t.TempDir(), "baseline.json")
			opts.Score.FailOnEscaped = true
			if tc.id {
				opts.Exec.RunMutantID = "id"
			}
			if tc.reportFailure {
				opts.Baseline.Update = false
				opts.Config.JSONOutput = true
				opts.Config.HTMLOutput = true
				t.Chdir(t.TempDir())
				if err := os.Mkdir(models.ReportFileName, 0755); err != nil {
					t.Fatal(err)
				}
			}
			report := &models.Report{Escaped: []models.Mutant{{ID: "escaped"}}}
			if tc.errors {
				report.Stats.ErrorCount = 1
			}
			var out, errout bytes.Buffer
			got := finalizeResults(&out, &errout, opts, report, baseline.Capture(nil), sourceRoots{}, tc.found)
			if got != tc.want {
				t.Fatalf("exit = %d, want %d; stderr = %s", got, tc.want, errout.String())
			}
			_, err := os.Stat(opts.Baseline.File)
			if (err == nil) != tc.baselineWritten {
				t.Fatalf("baseline written = %v", err == nil)
			}
		})
	}
}
