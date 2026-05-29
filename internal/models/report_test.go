package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func killed(name string) Mutant {
	return Mutant{Mutator: Mutator{MutatorName: name}}
}

func escaped(name string) Mutant {
	return Mutant{Mutator: Mutator{MutatorName: name}}
}

func TestMsiScore_ZeroTotal(t *testing.T) {
	r := &Report{}
	r.Calculate()
	assert.Equal(t, 0.0, r.Stats.Msi)
}

func TestMsiScore_AllKilled(t *testing.T) {
	r := &Report{
		Stats:   Stats{KilledCount: 8},
		Killed:  []Mutant{killed("a"), killed("a"), killed("a"), killed("a"), killed("a"), killed("a"), killed("a"), killed("a")},
		Escaped: nil,
	}
	r.Stats.EscapedCount = 0
	r.Calculate()
	assert.InDelta(t, 1.0, r.Stats.Msi, 0.001)
}

func TestMsiScore_Mixed(t *testing.T) {
	r := &Report{
		Stats: Stats{
			KilledCount:  3,
			EscapedCount: 1,
		},
		Killed:  []Mutant{killed("x"), killed("x"), killed("x")},
		Escaped: []Mutant{escaped("x")},
	}
	r.Calculate()
	// 3/(3+1) = 0.75
	assert.InDelta(t, 0.75, r.Stats.Msi, 0.001)
}

func TestCoveredMsiScore_ExcludesNotCovered(t *testing.T) {
	r := &Report{
		HasCoverage: true,
		Stats: Stats{
			KilledCount:     4,
			EscapedCount:    2,
			NotCoveredCount: 10,
		},
		Killed:  []Mutant{killed("m"), killed("m"), killed("m"), killed("m")},
		Escaped: []Mutant{escaped("m"), escaped("m")},
	}
	r.Calculate()
	// total = 4+2+10 = 16; covered = 6; killed = 4; coveredMSI = 4/6
	assert.InDelta(t, 4.0/6.0, r.Stats.CoveredCodeMsi, 0.001)
}

func TestCoveredMsiScore_AllNotCovered(t *testing.T) {
	r := &Report{
		HasCoverage: true,
		Stats:       Stats{NotCoveredCount: 5},
	}
	r.Calculate()
	assert.Equal(t, 0.0, r.Stats.CoveredCodeMsi)
}

func TestCoveredMsiScore_NoCoverageData(t *testing.T) {
	r := &Report{
		Stats: Stats{
			KilledCount:  3,
			EscapedCount: 1,
		},
		Killed:  []Mutant{killed("x"), killed("x"), killed("x")},
		Escaped: []Mutant{escaped("x")},
	}
	r.Calculate()
	// No coverage analysis ran (NotCoveredCount==0) → CoveredCodeMsi stays 0.
	assert.Equal(t, 0.0, r.Stats.CoveredCodeMsi)
}

func TestTotalCount(t *testing.T) {
	r := &Report{
		Stats: Stats{
			KilledCount:     2,
			EscapedCount:    3,
			ErrorCount:      1,
			SkippedCount:    1,
			NotCoveredCount: 4,
		},
	}
	assert.Equal(t, int64(11), r.TotalCount())
}

func TestComputeMutatorStats_Basic(t *testing.T) {
	r := &Report{
		Killed:  []Mutant{killed("branch/if"), killed("branch/if"), killed("expr/cmp")},
		Escaped: []Mutant{escaped("branch/if"), escaped("expr/cmp"), escaped("expr/cmp")},
	}
	r.Stats.KilledCount = 3
	r.Stats.EscapedCount = 3
	r.Calculate()

	byName := make(map[string]MutatorStats)
	for _, ms := range r.MutatorStats {
		byName[ms.Name] = ms
	}

	assert.Equal(t, int64(2), byName["branch/if"].Killed)
	assert.Equal(t, int64(1), byName["branch/if"].Escaped)
	assert.Equal(t, int64(3), byName["branch/if"].Total)

	assert.Equal(t, int64(1), byName["expr/cmp"].Killed)
	assert.Equal(t, int64(2), byName["expr/cmp"].Escaped)
	assert.Equal(t, int64(3), byName["expr/cmp"].Total)
}

func TestComputeMutatorStats_Empty(t *testing.T) {
	r := &Report{}
	r.Calculate()
	assert.Empty(t, r.MutatorStats)
}

func TestCalculate_SetsAll(t *testing.T) {
	r := &Report{
		Stats: Stats{
			KilledCount:     5,
			EscapedCount:    5,
			NotCoveredCount: 0,
		},
		Killed:  make([]Mutant, 5),
		Escaped: make([]Mutant, 5),
	}
	r.Calculate()
	assert.Equal(t, int64(10), r.Stats.TotalMutantsCount)
	assert.InDelta(t, 0.5, r.Stats.Msi, 0.001)
	// No coverage analysis ran → CoveredCodeMsi stays 0.
	assert.Equal(t, 0.0, r.Stats.CoveredCodeMsi)
}
