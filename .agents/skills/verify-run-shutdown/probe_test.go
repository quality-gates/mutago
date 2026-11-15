package engine

import (
	"github.com/quality-gates/mutago/v2/internal/models"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

type shutdownProbeListener struct {
	mu        sync.Mutex
	summaries int
}

func (*shutdownProbeListener) OnMutantCompleted(models.MutationEvent) {}
func (*shutdownProbeListener) OnProgress(models.Stats)                {}
func (l *shutdownProbeListener) OnSummary(*models.Report) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.summaries++
}
func TestShutdownVerification(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "run")
	if err := os.Mkdir(dir, 0755); err != nil {
		t.Fatal(err)
	}
	listener := &shutdownProbeListener{}
	r := &mutationRun{opts: &models.Options{}, stderr: io.Discard, tmpDir: dir, exec: execConfig{mode: modeExecute}, report: &models.Report{}, results: &completedResults{listener: listener}}
	setup := &runSetup{run: r, execution: startExecutionPool(1, r.results, dir)}
	var wg sync.WaitGroup
	for i := 0; i < 3; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); setup.close() }()
	}
	wg.Wait()
	if listener.summaries != 1 {
		t.Fatalf("summary count = %d", listener.summaries)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("directory not removed: %v", err)
	}
}
