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

func TestMutatorNumbersDecrementer(t *testing.T) {
	test.Mutator(
		t,
		MutatorNumbersDecrementer,
		"../../testdata/numbers/decrementer.go",
		3,
	)
}

func TestMutatorNumbersDecrementerRegistered(t *testing.T) {
	if _, err := mutator.New("numbers/decrementer"); err != nil {
		t.Fatalf("mutator not registered: %v", err)
	}
}

func TestMutatorNumbersDecrementerParenthesizesNegativeValues(t *testing.T) {
	testCases := []struct {
		name     string
		kind     token.Token
		original string
		mutated  string
	}{
		{name: "integer becomes negative", kind: token.INT, original: "0", mutated: "(-1)"},
		{name: "integer reaches zero", kind: token.INT, original: "1", mutated: "0"},
		{name: "float becomes negative", kind: token.FLOAT, original: "0.5", mutated: "(-0.5)"},
		{name: "float reaches zero", kind: token.FLOAT, original: "1.0", mutated: "0"},
	}

	for _, tt := range testCases {
		t.Run(tt.name, func(t *testing.T) {
			literal := &ast.BasicLit{Kind: tt.kind, Value: tt.original}
			mutations := MutatorNumbersDecrementer(nil, nil, literal)
			require.Len(t, mutations, 1)

			mutations[0].Change()
			assert.Equal(t, tt.mutated, literal.Value)

			mutations[0].Reset()
			assert.Equal(t, tt.original, literal.Value)
		})
	}
}

func TestMutatorNumbersDecrementer_ModernLiterals(t *testing.T) {
	testCases := []struct {
		original string
		mutated  string
	}{
		{original: "1_000", mutated: "999"},
		{original: "0x10", mutated: "0xf"},
		{original: "0b1010", mutated: "0b1001"},
		{original: "0o755", mutated: "0o754"},
	}

	for _, tt := range testCases {
		t.Run(tt.original, func(t *testing.T) {
			literal := &ast.BasicLit{Kind: token.INT, Value: tt.original}
			mutations := MutatorNumbersDecrementer(nil, nil, literal)
			require.Len(t, mutations, 1)

			mutations[0].Change()
			assert.Equal(t, tt.mutated, literal.Value)

			mutations[0].Reset()
			assert.Equal(t, tt.original, literal.Value)
		})
	}
}
