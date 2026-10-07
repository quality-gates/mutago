package engine

import (
	"testing"

	"github.com/quality-gates/mutago/v2/internal/gitdiff"
)

func TestMutantScopeAdmit(t *testing.T) {
	lines := gitdiff.ChangedLines{
		"pkg/foo.go": {{Start: 10, End: 15}},
	}
	absLines := gitdiff.ChangedLines{
		"pkg/foo.go": {{Start: 4, End: 4}},
	}

	tests := []struct {
		name   string
		scope  *mutantScope
		c      scopeCandidate
		want   bool
		reason skipReason
	}{
		{
			name:  "nil git lines admit every line",
			scope: newMutantScope(nil, "", nil),
			c:     scopeCandidate{relFile: "foo.go", absFile: "/repo/foo.go", line: 10, checksum: "a"},
			want:  true,
		},
		{
			name:  "changed line is in scope",
			scope: newMutantScope(lines, "", nil),
			c:     scopeCandidate{relFile: "pkg/foo.go", absFile: "/repo/pkg/foo.go", line: 12, checksum: "a"},
			want:  true,
		},
		{
			name:   "unchanged line is out of scope",
			scope:  newMutantScope(lines, "", nil),
			c:      scopeCandidate{relFile: "pkg/foo.go", absFile: "/repo/pkg/foo.go", line: 20, checksum: "a"},
			reason: skipGitDiff,
		},
		{
			name:  "empty rel path uses the absolute path",
			scope: newMutantScope(absLines, "", nil),
			c:     scopeCandidate{absFile: "/repo/pkg/foo.go", line: 4, checksum: "a"},
			want:  true,
		},
		{
			name:   "empty rel path outside the diff is out of scope",
			scope:  newMutantScope(absLines, "", nil),
			c:      scopeCandidate{absFile: "/repo/pkg/foo.go", line: 3, checksum: "a"},
			reason: skipGitDiff,
		},
		{
			name:  "unknown line stays in scope",
			scope: newMutantScope(lines, "", nil),
			c:     scopeCandidate{relFile: "pkg/foo.go", absFile: "/repo/pkg/foo.go", line: 0, checksum: "a"},
			want:  true,
		},
		{
			name:  "matching mutant id is in scope",
			scope: newMutantScope(nil, "wanted", nil),
			c:     scopeCandidate{relFile: "foo.go", line: 1, checksum: "a", id: "wanted"},
			want:  true,
		},
		{
			name:   "other mutant id is out of scope",
			scope:  newMutantScope(nil, "wanted", nil),
			c:      scopeCandidate{relFile: "foo.go", line: 1, checksum: "a", id: "other"},
			reason: skipMutantID,
		},
		{
			name:  "empty run id does not filter on id",
			scope: newMutantScope(nil, "", nil),
			c:     scopeCandidate{relFile: "foo.go", line: 1, checksum: "a", id: "other"},
			want:  true,
		},
		{
			name:   "blacklisted checksum is out of scope",
			scope:  newMutantScope(nil, "", map[string]struct{}{"blocked": {}}),
			c:      scopeCandidate{relFile: "foo.go", line: 1, checksum: "blocked", id: "wanted"},
			reason: skipBlacklist,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, reason := tt.scope.Admit(tt.c)
			if got != tt.want || reason != tt.reason {
				t.Fatalf("Admit() = (%v, %v), want (%v, %v)", got, reason, tt.want, tt.reason)
			}
		})
	}
}

func TestMutantScopeAdmitDuplicateAndOrder(t *testing.T) {
	scope := newMutantScope(gitdiff.ChangedLines{
		"foo.go": {{Start: 2, End: 2}},
	}, "wanted", map[string]struct{}{"blocked": {}})

	if ok, reason := scope.Admit(scopeCandidate{relFile: "foo.go", line: 9, checksum: "same", id: "wanted"}); ok || reason != skipGitDiff {
		t.Fatalf("out-of-diff mutant admitted or wrong reason: %v %v", ok, reason)
	}
	if ok, reason := scope.Admit(scopeCandidate{relFile: "foo.go", line: 2, checksum: "same", id: "other"}); ok || reason != skipMutantID {
		t.Fatalf("id miss should not be recorded as a duplicate: %v %v", ok, reason)
	}
	if ok, reason := scope.Admit(scopeCandidate{relFile: "foo.go", line: 2, checksum: "same", id: "wanted"}); !ok || reason != admitReason {
		t.Fatalf("matching mutant should be admitted after earlier rejections: %v %v", ok, reason)
	}
	if ok, reason := scope.Admit(scopeCandidate{relFile: "foo.go", line: 2, checksum: "same", id: "wanted"}); ok || reason != skipDuplicate {
		t.Fatalf("second identical edit = (%v, %v), want duplicate", ok, reason)
	}
	if ok, reason := scope.Admit(scopeCandidate{relFile: "foo.go", line: 2, checksum: "blocked", id: "wanted"}); ok || reason != skipBlacklist {
		t.Fatalf("blacklist = (%v, %v), want blacklist", ok, reason)
	}
	if ok, _ := scope.Admit(scopeCandidate{relFile: "foo.go", line: 2, checksum: "other", id: "wanted"}); !ok {
		t.Fatal("a different checksum should still be admitted")
	}
}
