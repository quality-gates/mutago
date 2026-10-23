package engine

import (
	"bytes"
	"errors"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/coverage"
	"github.com/quality-gates/mutago/v2/internal/models"
)

func resultMutant(id, mutatorName string) models.Mutant {
	return models.Mutant{ID: id, Mutator: models.Mutator{MutatorName: mutatorName}}
}

func TestCompletedResultsReportMatchesRecordedOutcomes(t *testing.T) {
	results := &completedResults{}
	results.addKilled(resultMutant("k1", "a"))
	results.addKilled(resultMutant("k2", "b"))
	results.addEscaped(resultMutant("e1", "a"))
	results.addSkipped(resultMutant("s1", "b"))
	results.addErrored(resultMutant("x1", "a"))
	results.addNotCovered(resultMutant("n1", "a"))

	report := &models.Report{HasCoverage: true}
	results.fill(report)
	report.Calculate()

	want := models.Stats{TotalMutantsCount: 6, KilledCount: 2, EscapedCount: 1, SkippedCount: 1, ErrorCount: 1, NotCoveredCount: 1}
	got := report.Stats
	got.Msi, got.CoveredCodeMsi = 0, 0
	if got != want {
		t.Fatalf("stats = %+v, want %+v", got, want)
	}
	if len(report.Killed) != 2 || len(report.Escaped) != 1 || len(report.Skipped) != 1 || len(report.Errored) != 1 || len(report.NotCovered) != 1 {
		t.Fatalf("collection lengths = %d/%d/%d/%d/%d, want 2/1/1/1/1",
			len(report.Killed), len(report.Escaped), len(report.Skipped), len(report.Errored), len(report.NotCovered))
	}
	if report.Escaped[0].ID != "e1" || report.NotCovered[0].ID != "n1" {
		t.Fatalf("recorded mutants landed in the wrong collection: %+v", report)
	}
	if math.Abs(report.Stats.Msi-4.0/6.0) > 1e-9 {
		t.Fatalf("msi = %v, want 4/6", report.Stats.Msi)
	}
	if math.Abs(report.Stats.CoveredCodeMsi-0.8) > 1e-9 {
		t.Fatalf("covered msi = %v, want 0.8", report.Stats.CoveredCodeMsi)
	}

	perMutator := append([]models.MutatorStats(nil), report.MutatorStats...)
	sort.Slice(perMutator, func(i, j int) bool { return perMutator[i].Name < perMutator[j].Name })
	wantPerMutator := []models.MutatorStats{
		{Name: "a", Killed: 2, Escaped: 1, Total: 3},
		{Name: "b", Killed: 1, Skipped: 1, Total: 2},
	}
	if len(perMutator) != len(wantPerMutator) {
		t.Fatalf("mutator stats = %+v, want %+v", perMutator, wantPerMutator)
	}
	for i := range wantPerMutator {
		if perMutator[i] != wantPerMutator[i] {
			t.Fatalf("mutator stats = %+v, want %+v", perMutator, wantPerMutator)
		}
	}
}

func TestCompletedResultsSnapshotIsIsolated(t *testing.T) {
	results := &completedResults{}
	results.addEscaped(resultMutant("e1", "a"))

	first := &models.Report{}
	results.fill(first)
	first.Escaped[0].ID = "changed"
	first.Escaped = append(first.Escaped, resultMutant("extra", "a"))

	second := &models.Report{}
	results.fill(second)
	if len(second.Escaped) != 1 || second.Escaped[0].ID != "e1" {
		t.Fatalf("escaped = %+v, want only e1", second.Escaped)
	}
	if second.Stats.EscapedCount != 1 {
		t.Fatalf("escaped count = %d, want 1", second.Stats.EscapedCount)
	}
}

func TestCompletedResultsEmptyCollectionsStayNil(t *testing.T) {
	report := &models.Report{}
	(&completedResults{}).fill(report)
	if report.Killed != nil || report.Escaped != nil || report.Errored != nil {
		t.Fatalf("empty collections should stay nil so JSON is unchanged: %+v", report)
	}
}

func TestCompletedResultsConcurrentRecordingAndSnapshots(t *testing.T) {
	const producers, perProducer = 8, 50
	results := &completedResults{}
	var wg sync.WaitGroup
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				m := resultMutant("", "a")
				switch i % 5 {
				case 0:
					results.addKilled(m)
				case 1:
					results.addEscaped(m)
				case 2:
					results.addSkipped(m)
				case 3:
					results.addErrored(m)
				default:
					results.addNotCovered(m)
				}
				if s := results.stats(); s.KilledCount+s.EscapedCount+s.SkippedCount+s.ErrorCount+s.NotCoveredCount == 0 {
					t.Errorf("stats snapshot missed this producer's own result")
				}
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < perProducer; i++ {
			report := &models.Report{}
			results.fill(report)
			if int64(len(report.Killed)) != report.Stats.KilledCount || int64(len(report.NotCovered)) != report.Stats.NotCoveredCount {
				t.Errorf("snapshot counts disagree with collections: %+v", report.Stats)
			}
		}
	}()
	wg.Wait()

	report := &models.Report{}
	results.fill(report)
	report.Calculate()
	if report.Stats.TotalMutantsCount != producers*perProducer {
		t.Fatalf("total = %d, want %d", report.Stats.TotalMutantsCount, producers*perProducer)
	}
	for _, n := range []int{len(report.Killed), len(report.Escaped), len(report.Skipped), len(report.Errored), len(report.NotCovered)} {
		if n != producers*perProducer/5 {
			t.Fatalf("each outcome should hold %d mutants, got %d", producers*perProducer/5, n)
		}
	}
}

func TestCaptureFailureIsRecordedAsErrored(t *testing.T) {
	var stdout bytes.Buffer
	results := &completedResults{}
	r := &mutationRun{exec: execConfig{mode: modeExecute}, stdout: &stdout, results: results}

	recordCaptureFailure(r, resultMutant("c1", "a"), errors.New("no stable ID"))

	report := &models.Report{}
	results.fill(report)
	if report.Stats.ErrorCount != 1 || len(report.Errored) != 1 {
		t.Fatalf("errored = %d/%d, want 1/1", report.Stats.ErrorCount, len(report.Errored))
	}
	if got := report.Errored[0].ProcessOutput; got != "INTERNAL ERROR no stable ID\n" {
		t.Fatalf("process output = %q", got)
	}
	if !strings.Contains(stdout.String(), "INTERNAL ERROR no stable ID") {
		t.Fatalf("stdout = %q, want the internal error", stdout.String())
	}
}

func TestMaterializationFailureIsRecordedAsErrored(t *testing.T) {
	var stdout bytes.Buffer
	results := &completedResults{}
	job := execJob{
		opts:   &models.Options{},
		mutant: resultMutant("m1", "a"),
		out:    jobOutput{stdout: &stdout, stderr: io.Discard},
		source: mutationSource{edit: mutationEdit{start: 5, end: 6}},
	}

	runExecJob(job, results, &sync.Mutex{})

	report := &models.Report{}
	results.fill(report)
	if report.Stats.ErrorCount != 1 || len(report.Errored) != 1 || report.Errored[0].ID != "m1" {
		t.Fatalf("errored = %+v", report.Errored)
	}
	if !strings.HasPrefix(report.Errored[0].ProcessOutput, "INTERNAL ERROR ") {
		t.Fatalf("process output = %q", report.Errored[0].ProcessOutput)
	}
}

func TestUncoveredMutantIsRecordedAsNotCovered(t *testing.T) {
	results := &completedResults{}
	mutant := resultMutant("u1", "a")
	mutant.Mutator.OriginalFilePath = "uncovered.go"
	mutant.Mutator.OriginalStartLine = 3
	opts := &models.Options{}
	opts.Config.SilentMode = true
	job := execJob{
		opts:     opts,
		mutant:   mutant,
		covScope: jobCoverageScope{profile: &coverage.Profile{}},
		out:      jobOutput{stdout: io.Discard, stderr: io.Discard},
	}

	runExecJob(job, results, &sync.Mutex{})

	report := &models.Report{HasCoverage: true}
	results.fill(report)
	report.Calculate()
	if report.Stats.NotCoveredCount != 1 || len(report.NotCovered) != 1 || report.NotCovered[0].ID != "u1" {
		t.Fatalf("not covered = %+v", report.NotCovered)
	}
	if !strings.HasPrefix(report.NotCovered[0].ProcessOutput, "NOT COVERED ") {
		t.Fatalf("process output = %q", report.NotCovered[0].ProcessOutput)
	}
	if len(report.MutatorStats) != 0 {
		t.Fatalf("not-covered mutants must not enter per-mutator totals: %+v", report.MutatorStats)
	}
}
