package numbers

import (
	"go/ast"
	"go/token"
	"testing"

	"github.com/quality-gates/mutago/v2/mutator"
	"github.com/quality-gates/mutago/v2/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMutatorNumbersIncrementer(t *testing.T) {
	test.Mutator(
		t,
		MutatorNumbersIncrementer,
		"../../testdata/numbers/incrementer.go",
		3,
	)
}

func TestMutatorNumbersIncrementerRegistered(t *testing.T) {
	if _, err := mutator.New("numbers/incrementer"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorNumbersIncrementer_ModernLiterals(t *testing.T) {
	testCases := []struct {
		original string
		mutated  string
	}{
		{original: "1_000", mutated: "1001"},
		{original: "0x10", mutated: "0x11"},
		{original: "0b1010", mutated: "0b1011"},
		{original: "0o755", mutated: "0o756"},
	}

	for _, tt := range testCases {
		t.Run(tt.original, func(t *testing.T) {
			literal := &ast.BasicLit{Kind: token.INT, Value: tt.original}
			mutations := MutatorNumbersIncrementer(nil, nil, literal)
			require.Len(t, mutations, 1)

			mutations[0].Change()
			assert.Equal(t, tt.mutated, literal.Value)

			mutations[0].Reset()
			assert.Equal(t, tt.original, literal.Value)
		})
	}
}
