package engine

import "github.com/quality-gates/mutago/v2/internal/gitdiff"

// skipReason is why mutantScope rejected a discovered mutant.
// admitReason means the mutant is in scope.
type skipReason int

const (
	admitReason skipReason = iota
	skipGitDiff
	skipMutantID
	skipBlacklist
	skipDuplicate
)

// scopeCandidate is one discovered mutant, identified before a job is queued.
type scopeCandidate struct {
	relFile  string
	absFile  string
	line     int
	checksum string
	id       string
}

// mutantScope decides which discovered mutants are in scope.
// Discovery calls Admit for a dry run and a real run, so both runs use one rule set.
// Coverage is not a scope rule. The exec worker classifies admitted mutants.
type mutantScope struct {
	gitLines gitdiff.ChangedLines
	runID    string
	blocked  map[string]struct{}
	seen     map[string]struct{}
}

func newMutantScope(gitLines gitdiff.ChangedLines, runID string, blocked map[string]struct{}) *mutantScope {
	return &mutantScope{
		gitLines: gitLines,
		runID:    runID,
		blocked:  blocked,
		seen:     map[string]struct{}{},
	}
}

// Admit reports whether candidate is in scope.
// The check order is git diff, --run-mutant-id, blacklist, then duplicate.
// A rejected mutant is not recorded as seen, so it cannot hide a later mutant.
func (s *mutantScope) Admit(c scopeCandidate) (bool, skipReason) {
	if gitDiffExcludes(s.gitLines, c.relFile, c.absFile, c.line) {
		return false, skipGitDiff
	}
	if s.runID != "" && c.id != s.runID {
		return false, skipMutantID
	}
	if _, blocked := s.blocked[c.checksum]; blocked {
		return false, skipBlacklist
	}
	if _, seen := s.seen[c.checksum]; seen {
		return false, skipDuplicate
	}
	s.seen[c.checksum] = struct{}{}
	return true, admitReason
}

// gitDiffExcludes is the only git-diff scope rule.
// A nil line set means the filter is off. An empty relFile falls back to absFile.
// Line 0 is not excluded: the position is unknown, so the mutant stays in scope.
func gitDiffExcludes(lines gitdiff.ChangedLines, relFile, absFile string, line int) bool {
	if lines == nil {
		return false
	}
	changed := gitdiff.IsRelativeLineChanged(lines, relFile, line)
	if relFile == "" {
		changed = gitdiff.IsLineChanged(lines, absFile, line)
	}
	return !changed
}
