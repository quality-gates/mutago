package models

// MutantOutcome is the result of testing one mutant.
type MutantOutcome int

const (
	// OutcomeKilled means the test suite caught the mutant.
	OutcomeKilled MutantOutcome = iota
	// OutcomeEscaped means the test suite missed the mutant.
	OutcomeEscaped
	// OutcomeSkipped means the mutant did not compile or was not tested.
	OutcomeSkipped
	// OutcomeErrored means testing the mutant failed for another reason.
	OutcomeErrored
	// OutcomeNotCovered means no test covers the mutated line.
	OutcomeNotCovered
)

// MutationEvent reports one completed mutant.
type MutationEvent struct {
	// Mutant is the completed mutant. Its ProcessOutput is the status line.
	Mutant Mutant
	// Outcome is how the mutant was classified.
	Outcome MutantOutcome
	// Stats holds the run's outcome counts, including this mutant.
	Stats Stats
}

// RunListener receives a mutation run's progress. Workers call it
// concurrently, so implementations must be safe for concurrent use, and
// events may arrive in a different order from their Stats counts.
type RunListener interface {
	// OnMutantCompleted is called once for each mutant with an outcome.
	OnMutantCompleted(event MutationEvent)
	// OnProgress is called periodically while mutants run.
	OnProgress(stats Stats)
	// OnSummary is called once after mutant execution stops, before the
	// run's results are reported.
	OnSummary(report *Report)
}
