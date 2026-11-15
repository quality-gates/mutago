package engine

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"
)

func TestRunShutdownPublishesOneSummaryAndRemovesTemporaryFiles(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "run")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	listener := &recordingListener{}
	report := &models.Report{}
	run := &mutationRun{
		opts: &models.Options{}, stderr: io.Discard, tmpDir: dir,
		exec:   execConfig{mode: modeExecute},
		report: report, results: &completedResults{listener: listener},
	}
	setup := &runSetup{run: run, execution: startExecutionPool(1, run.results, dir)}
	setup.close()
	setup.close()
	if len(listener.summaries) != 1 || listener.summaries[0] != report {
		t.Fatalf("summaries = %v, want one final report", listener.summaries)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("temporary run directory still exists: %v", err)
	}
}
