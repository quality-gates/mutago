package engine

import (
	"reflect"
	"runtime"
	"strings"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"
)

func TestRunnerPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, command                       string
		mode                                runMode
		coverage                            bool
		workers                             int
		wantWorkers                         int
		baseline, goTest, adaptive, perTest bool
		warnings                            []string
	}{
		{name: "builtin", mode: modeExecute, workers: 4, wantWorkers: 4, baseline: true, goTest: true, adaptive: true, perTest: true},
		{name: "whitespace", command: " \t\n", mode: modeExecute, wantWorkers: runtime.NumCPU(), baseline: true, goTest: true, adaptive: true, perTest: true},
		{name: "builtin coverage", mode: modeExecute, coverage: true, workers: 2, wantWorkers: 2, goTest: true, adaptive: true, perTest: true},
		{name: "custom", command: " sh ok.sh ", mode: modeExecute, workers: 4, wantWorkers: 1, warnings: []string{"--per-test", "--timeout-coefficient", "--test-flags"}},
		{name: "custom coverage", command: "sh ok.sh", mode: modeExecute, coverage: true, wantWorkers: 1, goTest: true, warnings: []string{"--per-test", "--timeout-coefficient"}},
		{name: "dry builtin", mode: modeDryRun, coverage: true, workers: 3, wantWorkers: 3},
		{name: "generate builtin", mode: modeGenerate, coverage: true, workers: 3, wantWorkers: 3},
		{name: "dry custom", command: "sh ok.sh", mode: modeDryRun, coverage: true, workers: 3, wantWorkers: 1},
		{name: "generate custom", command: "sh ok.sh", mode: modeGenerate, coverage: true, workers: 3, wantWorkers: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := &models.Options{}
			opts.Exec.Exec = tc.command
			opts.Exec.Coverage = tc.coverage
			opts.Exec.TestFlags = "-tags=integ -count=2"
			opts.Exec.PerTest = true
			opts.Exec.TimeoutCoefficient = 3
			opts.General.Workers = tc.workers
			r, warnings := resolveRunner(opts, tc.mode)
			if r.workers != tc.wantWorkers || r.baseline != tc.baseline || r.goTest != tc.goTest || r.adaptive != tc.adaptive || r.perTest != tc.perTest {
				t.Fatalf("unexpected capabilities: %+v", r)
			}
			if r.coverage != (tc.coverage && tc.mode == modeExecute) {
				t.Fatalf("unexpected coverage: %v", r.coverage)
			}
			if !reflect.DeepEqual(r.testFlags, []string{"-tags=integ", "-count=2"}) {
				t.Fatalf("Go test flags lost: %v", r.testFlags)
			}
			if len(warnings) != len(tc.warnings) {
				t.Fatalf("warnings = %v, want %v", warnings, tc.warnings)
			}
			for i, flag := range tc.warnings {
				if !strings.Contains(warnings[i], flag) {
					t.Errorf("warning %q does not name %s", warnings[i], flag)
				}
			}
		})
	}
}

func TestRunnerDefaultCapabilities(t *testing.T) {
	for _, command := range []string{"", "sh ok.sh"} {
		opts := &models.Options{}
		opts.Exec.Exec = command
		r, warnings := resolveRunner(opts, modeExecute)
		if r.adaptive || r.perTest || len(warnings) != 0 {
			t.Fatalf("unrequested capabilities or warnings: %+v, %v", r, warnings)
		}
		if err := r.validateTestCount(); err != nil {
			t.Fatal(err)
		}
	}
}
