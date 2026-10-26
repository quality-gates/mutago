package engine

import (
	"bytes"
	"context"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"
)

// recordingListener collects every event the engine sends.
type recordingListener struct {
	mu        sync.Mutex
	completed []models.MutationEvent
	summaries []*models.Report
}

func (l *recordingListener) OnMutantCompleted(event models.MutationEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.completed = append(l.completed, event)
}

func (l *recordingListener) OnProgress(models.Stats) {}

func (l *recordingListener) OnSummary(report *models.Report) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.summaries = append(l.summaries, report)
}

// captureProcessStdout replaces os.Stdout while fn runs and returns what was
// written to it.
func captureProcessStdout(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		done <- buf.String()
	}()
	defer func() { os.Stdout = orig }()
	fn()
	_ = w.Close()
	return <-done
}

func TestEngineWritesMutantStatusLinesToInjectedStdout(t *testing.T) {
	opts := escapingPackageOptions(t)

	var stdout, stderr bytes.Buffer
	var res Result
	leaked := captureProcessStdout(t, func() {
		var err error
		res, err = (&Engine{Stdout: &stdout, Stderr: &stderr}).Run(context.Background(), opts, nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	if res.Report == nil || res.Report.Stats.TotalMutantsCount == 0 {
		t.Fatalf("expected mutants to run; stderr: %s", stderr.String())
	}
	if got, want := int64(strings.Count(stdout.String(), "ESCAPED")), res.Report.Stats.EscapedCount; got != want {
		t.Fatalf("injected stdout has %d ESCAPED lines, want %d:\n%s", got, want, stdout.String())
	}
	if strings.Contains(leaked, "ESCAPED") || strings.Contains(leaked, "KILLED") || strings.Contains(leaked, "SKIP") {
		t.Fatalf("status lines leaked to process stdout:\n%s", leaked)
	}
}

func TestEngineSendsOneEventPerMutantToListener(t *testing.T) {
	opts := escapingPackageOptions(t)
	listener := &recordingListener{}
	var stdout, stderr bytes.Buffer

	res, err := (&Engine{Stdout: &stdout, Stderr: &stderr, Listener: listener}).Run(context.Background(), opts, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	report := res.Report
	if report == nil || report.Stats.TotalMutantsCount == 0 {
		t.Fatalf("expected mutants to run; stderr: %s", stderr.String())
	}

	if int64(len(listener.completed)) != report.Stats.TotalMutantsCount {
		t.Fatalf("got %d events for %d mutants", len(listener.completed), report.Stats.TotalMutantsCount)
	}
	counts := map[models.MutantOutcome]int64{}
	for _, ev := range listener.completed {
		counts[ev.Outcome]++
		if ev.Mutant.ProcessOutput == "" {
			t.Errorf("event for %s has no process output", ev.Mutant.ID)
		}
	}
	want := map[models.MutantOutcome]int64{
		models.OutcomeKilled:     report.Stats.KilledCount,
		models.OutcomeEscaped:    report.Stats.EscapedCount,
		models.OutcomeSkipped:    report.Stats.SkippedCount,
		models.OutcomeErrored:    report.Stats.ErrorCount,
		models.OutcomeNotCovered: report.Stats.NotCoveredCount,
	}
	for outcome, n := range want {
		if counts[outcome] != n {
			t.Errorf("outcome %v: %d events, report counts %d", outcome, counts[outcome], n)
		}
	}
	if len(listener.summaries) != 1 || listener.summaries[0] != report {
		t.Fatalf("expected one summary with the final report, got %d", len(listener.summaries))
	}
	if strings.Contains(stdout.String(), "ESCAPED") {
		t.Fatalf("a custom listener replaces terminal status lines, got:\n%s", stdout.String())
	}
}
