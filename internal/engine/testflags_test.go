package engine

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTestBinaryFlags(t *testing.T) {
	assert.Equal(t, []string{
		"-test.short=true",
		"-test.short=true",
		"-test.count=2",
		"-test.v=true",
		"-test.v=true",
		"-test.count=3",
		"-test.count=4",
		"-test.count=5",
		"-test.failfast=true",
		"-test.failfast=true",
		"-test.parallel=4",
		"-test.parallel=8",
		"-test.shuffle=on",
		"-test.shuffle=123",
		"-test.cpu=1,2",
		"-test.cpu=4",
		"-test.timeout=10s",
		"-test.timeout=20s",
		"-custom-flag",
		"-custom=value",
	}, testBinaryFlags([]string{
		"-short",
		"--short",
		"-test.count=2",
		"-v",
		"--verbose",
		"-race",
		"--race",
		"-tags=integration",
		"-tags", "unit",
		"-vet=off",
		"-vet", "all",
		"-gcflags=all=-N",
		"-gcflags", "all=-l",
		"-asmflags=all=-trimpath=/tmp",
		"-asmflags", "all=-trimpath=/var",
		"-trimpath",
		"--trimpath",
		"-count=3",
		"-count", "4",
		"--count=5",
		"-failfast",
		"--failfast",
		"-parallel=4",
		"-parallel", "8",
		"-shuffle=on",
		"-shuffle", "123",
		"-cpu=1,2",
		"-cpu", "4",
		"-timeout=10s",
		"-timeout", "20s",
		"-custom-flag",
		"-custom=value",
	}))

	// Boolean flags with explicit values.
	assert.Equal(t, []string{
		"-test.short=false",
		"-test.v=false",
		"-test.failfast=false",
	}, testBinaryFlags([]string{
		"-short=false",
		"--verbose=false",
		"-failfast=false",
	}))

	// Value flags without following value or followed by another flag.
	assert.Equal(t, []string{
		"-test.count",
	}, testBinaryFlags([]string{
		"-count",
	}))
	assert.Equal(t, []string{
		"-test.count",
		"-test.failfast=true",
	}, testBinaryFlags([]string{
		"-count",
		"-failfast",
	}))

	// Build flag at end of args without value.
	assert.Equal(t, []string{}, testBinaryFlags([]string{
		"-tags",
	}))

	// Preserving --test. flags and non-flag tokens (including tokens matching flag names).
	assert.Equal(t, []string{
		"--test.count=2",
		"--test.v",
		"positional",
		"race",
		"-test.run=TestFoo",
		"-test.bench=BenchmarkBar",
		"-test.skip=TestBaz",
	}, testBinaryFlags([]string{
		"--test.count=2",
		"--test.v",
		"positional",
		"race",
		"-run", "TestFoo",
		"-bench=BenchmarkBar",
		"-skip", "TestBaz",
	}))
}

func TestTestBinaryFlagsDropsEveryBuildFlag(t *testing.T) {
	assert.Equal(t, []string{"-test.short=true"}, testBinaryFlags([]string{
		"-ldflags", "-s -w",
		"-mod=vendor",
		"-modfile", "alt.mod",
		"-msan",
		"-asan",
		"-short",
	}))
}

func TestBuildTestFlags(t *testing.T) {
	assert.Equal(t, []string{
		"-tags", "integration",
		"--race",
		"-gcflags=all=-N",
		"-ldflags", "-s",
		"-trimpath",
		"-mod=mod",
	}, buildTestFlags([]string{
		"-tags", "integration",
		"--race",
		"-count", "2",
		"-gcflags=all=-N",
		"-vet=off",
		"-ldflags", "-s",
		"-run", "TestX",
		"-trimpath",
		"positional",
		"-mod=mod",
		"-v",
	}))
	assert.Equal(t, []string{"-tags"}, buildTestFlags([]string{"-tags"}))
	assert.Equal(t, []string{"-tags", "-race"}, buildTestFlags([]string{"-tags", "-race"}))
	assert.Empty(t, buildTestFlags(nil))
}

func TestHasTestFlagAcceptsEveryForm(t *testing.T) {
	for _, flags := range [][]string{
		{"-count=1"}, {"--count=1"}, {"-count", "1"}, {"--count", "1"},
	} {
		assert.True(t, hasTestFlag(flags, "count"), "%q", flags)
	}
	assert.False(t, hasTestFlag([]string{"count"}, "count"), "a bare word is not a flag")
	assert.False(t, hasTestFlag([]string{"-counter=1"}, "count"))
	assert.False(t, hasTestFlag([]string{"-test.count=1"}, "count"))
}

func TestTestCountValueAcceptsEveryForm(t *testing.T) {
	tests := []struct {
		flags []string
		index int
		want  string
		found bool
	}{
		{[]string{"-count=3"}, 0, "3", true},
		{[]string{"--count=3"}, 0, "3", true},
		{[]string{"-count", "3"}, 0, "3", true},
		{[]string{"--count", "3"}, 0, "3", true},
		{[]string{"--count"}, 0, "", false},
		{[]string{"-short", "-count", "0"}, 0, "", false},
		{[]string{"-short", "-count", "0"}, 1, "0", true},
		{[]string{"count", "0"}, 0, "", false},
	}
	for _, tt := range tests {
		got, found := testCountValue(tt.flags, tt.index)
		assert.Equal(t, tt.want, got, "%q[%d]", tt.flags, tt.index)
		assert.Equal(t, tt.found, found, "%q[%d]", tt.flags, tt.index)
	}
}

func TestUncachedTestFlagsRespectsDoubleDashCount(t *testing.T) {
	assert.Equal(t, []string{"--count=2"}, uncachedTestFlags([]string{"--count=2"}))
	assert.Equal(t, []string{"-short", "-count=1"}, uncachedTestFlags([]string{"-short"}))
}
