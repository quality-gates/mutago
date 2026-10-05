package mutator_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quality-gates/mutago/v2/mutator"
	_ "github.com/quality-gates/mutago/v2/mutator/branch"
	_ "github.com/quality-gates/mutago/v2/mutator/numbers"
)

func TestSelectorMatchesExactName(t *testing.T) {
	s, err := mutator.ParseSelector([]string{"numbers/incrementer"})
	require.NoError(t, err)

	assert.True(t, s.Matches("numbers/incrementer"))
	assert.False(t, s.Matches("numbers/decrementer"))
}

func TestSelectorStarMatchesEveryName(t *testing.T) {
	s, err := mutator.ParseSelector([]string{"*"})
	require.NoError(t, err)

	assert.True(t, s.Matches("numbers/incrementer"))
	assert.True(t, s.Matches("branch/if"))
}

func TestSelectorPrefixWildcardMatchesNamesWithThatPrefix(t *testing.T) {
	s, err := mutator.ParseSelector([]string{"numbers/*"})
	require.NoError(t, err)

	assert.True(t, s.Matches("numbers/incrementer"))
	assert.True(t, s.Matches("numbers/decrementer"))
	assert.False(t, s.Matches("branch/if"))
}

func TestParseSelectorReportsPatternsMatchingNoRegisteredMutator(t *testing.T) {
	s, err := mutator.ParseSelector([]string{"numbers/*", "bogus", "nope/*", "branch/if"})

	var unknown *mutator.UnknownPatternsError
	require.ErrorAs(t, err, &unknown)
	assert.Equal(t, []string{"bogus", "nope/*"}, unknown.Patterns)
	assert.Equal(t, `mutator selectors match no registered mutator: "bogus", "nope/*"`, err.Error())
	assert.True(t, s.Matches("branch/if"), "known patterns still select")
}
