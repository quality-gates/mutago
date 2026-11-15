package engine

import "github.com/quality-gates/mutago/v2/internal/gitdiff"

type admissionDecision uint8

const (
	admitReason admissionDecision = iota
	skipGitDiff
	skipMutantID
	skipBlacklist
	skipDuplicate
)

// scopeCandidate is one discovered mutant, identified before a job is queued.
type scopeCandidate struct {
	// repoFile is the repository-relative path, the key git diff reports.
	repoFile string
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
func (s *mutantScope) Admit(c scopeCandidate) admissionDecision {
	if gitDiffExcludes(s.gitLines, c.repoFile, c.absFile, c.line) {
		return skipGitDiff
	}
	if s.runID != "" && c.id != s.runID {
		return skipMutantID
	}
	if _, blocked := s.blocked[c.checksum]; blocked {
		return skipBlacklist
	}
	if _, seen := s.seen[c.checksum]; seen {
		return skipDuplicate
	}
	s.seen[c.checksum] = struct{}{}
	return admitReason
}

// gitDiffExcludes is the only git-diff scope rule.
// A nil line set means the filter is off. An empty repoFile falls back to absFile.
// Line 0 is not excluded: the position is unknown, so the mutant stays in scope.
func gitDiffExcludes(lines gitdiff.ChangedLines, repoFile, absFile string, line int) bool {
	if lines == nil {
		return false
	}
	changed := gitdiff.IsRelativeLineChanged(lines, repoFile, line)
	if repoFile == "" {
		changed = gitdiff.IsLineChanged(lines, absFile, line)
	}
	return !changed
}
