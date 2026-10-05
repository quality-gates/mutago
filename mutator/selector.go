package mutator

import (
	"fmt"
	"strings"
)

// Selector matches registered mutator names against user-supplied patterns.
// A pattern is an exact mutator name, "*" for every mutator, or a prefix
// followed by a trailing "*" (for example "numbers/*").
type Selector struct {
	patterns []string
}

// UnknownPatternsError lists selector patterns that match no registered
// mutator, which usually means a typo.
type UnknownPatternsError struct {
	Patterns []string
}

func (e *UnknownPatternsError) Error() string {
	quoted := make([]string, len(e.Patterns))
	for i, pattern := range e.Patterns {
		quoted[i] = fmt.Sprintf("%q", pattern)
	}
	return "mutator selectors match no registered mutator: " + strings.Join(quoted, ", ")
}

// ParseSelector builds a Selector from patterns. The Selector is always usable;
// a non-nil error is an *UnknownPatternsError listing the patterns that match
// no registered mutator.
func ParseSelector(patterns []string) (Selector, error) {
	s := Selector{patterns: patterns}
	var unknown []string
	for _, pattern := range patterns {
		if !matchesAnyRegistered(pattern) {
			unknown = append(unknown, pattern)
		}
	}
	if len(unknown) > 0 {
		return s, &UnknownPatternsError{Patterns: unknown}
	}
	return s, nil
}

func matchesAnyRegistered(pattern string) bool {
	for name := range mutatorLookup {
		if matchesPattern(pattern, name) {
			return true
		}
	}
	return false
}

// Matches reports whether name is selected by any of the patterns.
func (s Selector) Matches(name string) bool {
	for _, pattern := range s.patterns {
		if matchesPattern(pattern, name) {
			return true
		}
	}
	return false
}

func matchesPattern(pattern, name string) bool {
	if prefix, ok := strings.CutSuffix(pattern, "*"); ok {
		return strings.HasPrefix(name, prefix)
	}
	return name == pattern
}
