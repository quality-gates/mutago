package models

// ReportFileName File name for json report
var ReportFileName string = "report.json"

// ReportHTMLFileName File name for html report
var ReportHTMLFileName string = "mutago-report.html"

// ReportSummaryJSONFileName File name for the compact stats-only JSON summary
var ReportSummaryJSONFileName string = "mutago-summary.json"

// ReportAgenticJSONFileName File name for the LLM-optimised escaped-mutant report
var ReportAgenticJSONFileName string = "mutago-agentic.json"

// ReportGitLabJSONFileName File name for the GitLab Code Quality report
var ReportGitLabJSONFileName string = "mutago-gitlab.json"

// Report holds the complete mutation testing result.
type Report struct {
	Stats        Stats          `json:"stats"`
	MutatorStats []MutatorStats `json:"mutatorStats,omitempty"`
	Escaped      []Mutant       `json:"escaped"`
	Killed       []Mutant       `json:"killed"`
	Skipped      []Mutant       `json:"skipped,omitempty"`
	Errored      []Mutant       `json:"errored"`
	NotCovered   []Mutant       `json:"notCovered,omitempty"`
	// HasCoverage is true when a coverage profile was loaded before mutation.
	// It distinguishes "coverage was run and all code is covered" (NotCoveredCount==0
	// but CoveredCodeMsi is meaningful) from "coverage was never run".
	HasCoverage bool `json:"-"`
}

// Stats holds aggregate mutation metrics.
type Stats struct {
	TotalMutantsCount    int64   `json:"totalMutantsCount"`
	KilledCount          int64   `json:"killedCount"`
	NotCoveredCount      int64   `json:"notCoveredCount"`
	EscapedCount         int64   `json:"escapedCount"`
	ErrorCount           int64   `json:"errorCount"`
	SkippedCount   int64   `json:"skippedCount"`
	Msi            float64 `json:"msi"`
	CoveredCodeMsi float64 `json:"coveredCodeMsi"`
	DuplicatedCount      int64   `json:"-"`
}

// MutatorStats holds per-mutator kill/escape counts for tested mutants only.
// Not-covered mutants are excluded from all counts here.
type MutatorStats struct {
	Name    string `json:"name"`
	Killed  int64  `json:"killed"`
	Escaped int64  `json:"escaped"`
	Skipped int64  `json:"skipped"`
	Total   int64  `json:"total"`
}

// Mutant is the result of one mutation attempt.
type Mutant struct {
	Mutator       Mutator `json:"mutator"`
	Diff          string  `json:"diff"`
	ProcessOutput string  `json:"processOutput,omitempty"`
}

// Mutator describes what was mutated.
type Mutator struct {
	MutatorName        string `json:"mutatorName"`
	OriginalSourceCode string `json:"originalSourceCode"`
	MutatedSourceCode  string `json:"mutatedSourceCode"`
	OriginalFilePath   string `json:"originalFilePath"`
	OriginalStartLine  int64  `json:"originalStartLine"`
}

// Calculate computes derived metrics and per-mutator breakdowns.
func (report *Report) Calculate() {
	report.Stats.TotalMutantsCount = report.TotalCount()
	report.Stats.Msi = report.MsiScore()
	report.Stats.CoveredCodeMsi = report.CoveredMsiScore()
	report.MutatorStats = report.computeMutatorStats()
}

// MsiScore returns killed / total (skipped and errors count as killed).
func (report *Report) MsiScore() float64 {
	total := report.TotalCount()
	if total == 0 {
		return 0.0
	}
	return float64(report.Stats.KilledCount+report.Stats.ErrorCount+report.Stats.SkippedCount) / float64(total)
}

// CoveredMsiScore returns killed / (total - notCovered).
// Returns 0 when coverage was never enabled (HasCoverage==false).
// When coverage IS enabled and NotCoveredCount==0, every mutant is covered
// and CoveredMsiScore equals MsiScore.
func (report *Report) CoveredMsiScore() float64 {
	if !report.HasCoverage {
		return 0.0
	}
	covered := report.TotalCount() - report.Stats.NotCoveredCount
	if covered <= 0 {
		return 0.0
	}
	return float64(report.Stats.KilledCount+report.Stats.ErrorCount+report.Stats.SkippedCount) / float64(covered)
}

// TotalCount returns all mutants: killed, escaped, errored, skipped, and not-covered.
func (report *Report) TotalCount() int64 {
	return report.Stats.KilledCount +
		report.Stats.EscapedCount +
		report.Stats.ErrorCount +
		report.Stats.SkippedCount +
		report.Stats.NotCoveredCount
}

// computeMutatorStats aggregates kill/escape/error counts per mutator for mutants
// that were actually executed (not-covered mutants are excluded).
func (report *Report) computeMutatorStats() []MutatorStats {
	counts := make(map[string]*MutatorStats)
	add := func(ms []Mutant, inc func(*MutatorStats)) {
		for _, m := range ms {
			name := m.Mutator.MutatorName
			if _, ok := counts[name]; !ok {
				counts[name] = &MutatorStats{Name: name}
			}
			inc(counts[name])
			counts[name].Total++
		}
	}
	add(report.Killed, func(s *MutatorStats) { s.Killed++ })
	add(report.Escaped, func(s *MutatorStats) { s.Escaped++ })
	add(report.Skipped, func(s *MutatorStats) { s.Skipped++ })
	add(report.Errored, func(s *MutatorStats) { s.Killed++ }) // errors count as kills

	result := make([]MutatorStats, 0, len(counts))
	for _, s := range counts {
		result = append(result, *s)
	}
	return result
}
