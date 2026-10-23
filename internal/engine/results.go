package engine

import (
	"sync"

	"github.com/quality-gates/mutago/v2/internal/models"
)

// completedResults owns the run's completed mutants. Each outcome is one
// collection, and its count is that collection's length, so the report's
// counts and collections cannot disagree. It is safe for concurrent use.
type completedResults struct {
	mu         sync.Mutex
	killed     []models.Mutant
	escaped    []models.Mutant
	skipped    []models.Mutant
	errored    []models.Mutant
	notCovered []models.Mutant
}

func (c *completedResults) add(list *[]models.Mutant, mutant models.Mutant) {
	c.mu.Lock()
	defer c.mu.Unlock()
	*list = append(*list, mutant)
}

func (c *completedResults) addKilled(mutant models.Mutant)     { c.add(&c.killed, mutant) }
func (c *completedResults) addEscaped(mutant models.Mutant)    { c.add(&c.escaped, mutant) }
func (c *completedResults) addSkipped(mutant models.Mutant)    { c.add(&c.skipped, mutant) }
func (c *completedResults) addErrored(mutant models.Mutant)    { c.add(&c.errored, mutant) }
func (c *completedResults) addNotCovered(mutant models.Mutant) { c.add(&c.notCovered, mutant) }

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
