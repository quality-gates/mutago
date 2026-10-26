package engine

import (
	"sync"

	"github.com/quality-gates/mutago/v2/internal/models"
)

// completedResults owns the run's completed mutants. Each outcome is one
// collection, and its count is that collection's length, so the report's
// counts and collections cannot disagree. It tells listener about each
// recorded mutant, the progress, and the final report. It is safe for
// concurrent use.
type completedResults struct {
	listener models.RunListener

	mu         sync.Mutex
	killed     []models.Mutant
	escaped    []models.Mutant
	skipped    []models.Mutant
	errored    []models.Mutant
	notCovered []models.Mutant
}

// record stores mutant under outcome, then tells the listener.
func (c *completedResults) record(outcome models.MutantOutcome, mutant models.Mutant) {
	stats := c.add(outcome, mutant)
	c.listener.OnMutantCompleted(models.MutationEvent{Mutant: mutant, Outcome: outcome, Stats: stats})
}

// reportProgress sends the listener the counts recorded so far.
func (c *completedResults) reportProgress() {
	c.listener.OnProgress(c.stats())
}

// summarize fills and calculates report, then sends it to the listener.
func (c *completedResults) summarize(report *models.Report) {
	c.fill(report)
	report.Calculate()
	c.listener.OnSummary(report)
}

// add records mutant under outcome and returns the counts including it.
func (c *completedResults) add(outcome models.MutantOutcome, mutant models.Mutant) models.Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	list := c.list(outcome)
	*list = append(*list, mutant)
	return c.countsLocked()
}

func (c *completedResults) list(outcome models.MutantOutcome) *[]models.Mutant {
	switch outcome {
	case models.OutcomeKilled:
		return &c.killed
	case models.OutcomeEscaped:
		return &c.escaped
	case models.OutcomeSkipped:
		return &c.skipped
	case models.OutcomeNotCovered:
		return &c.notCovered
	default:
		return &c.errored
	}
}

// stats returns the outcome counts recorded so far.
func (c *completedResults) stats() models.Stats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.countsLocked()
}

func (c *completedResults) countsLocked() models.Stats {
	return models.Stats{
		KilledCount:     int64(len(c.killed)),
		EscapedCount:    int64(len(c.escaped)),
		SkippedCount:    int64(len(c.skipped)),
		ErrorCount:      int64(len(c.errored)),
		NotCoveredCount: int64(len(c.notCovered)),
	}
}

// fill copies the recorded outcomes and their counts into report. Other
// report fields are left alone. Changing the copied collections does not
// change the recorded ones.
func (c *completedResults) fill(report *models.Report) {
	c.mu.Lock()
	defer c.mu.Unlock()
	stats := c.countsLocked()
	report.Killed = cloneMutants(c.killed)
	report.Escaped = cloneMutants(c.escaped)
	report.Skipped = cloneMutants(c.skipped)
	report.Errored = cloneMutants(c.errored)
	report.NotCovered = cloneMutants(c.notCovered)
	report.Stats.KilledCount = stats.KilledCount
	report.Stats.EscapedCount = stats.EscapedCount
	report.Stats.SkippedCount = stats.SkippedCount
	report.Stats.ErrorCount = stats.ErrorCount
	report.Stats.NotCoveredCount = stats.NotCoveredCount
}

// cloneMutants copies list, keeping nil for an empty list so the JSON report
// is unchanged.
func cloneMutants(list []models.Mutant) []models.Mutant {
	return append([]models.Mutant(nil), list...)
}
