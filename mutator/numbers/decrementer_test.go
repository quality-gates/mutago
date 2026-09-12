package numbers

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"go/types"
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

func TestMutatorNumbersDecrementer_SkipsUnsignedZero(t *testing.T) {
	src := `package main

type MyUint uint

func takeUint(u uint) {}

func testFunc() uint {
	var u uint = 0
	var u8 uint8 = 0
	var u16 uint16 = 0
	var u32 uint32 = 0
	var u64 uint64 = 0
	var uptr uintptr = 0
	var b byte = 0
	var mu MyUint = 0

	var safeUint uint = 10
	var safeInt int = 0
	var safeInt8 int8 = 0

	takeUint(0)
	_ = uint(0)
	var uAssign uint
	uAssign = 0
	if uAssign == 0 {}

	type S struct {
		U uint
	}
	_ = S{U: 0}
	_ = []uint{0}
	_ = map[uint]uint{0: 0}
	ch := make(chan uint, 1)
	ch <- 0

	_ = u; _ = u8; _ = u16; _ = u32; _ = u64; _ = uptr; _ = b; _ = mu
	_ = safeUint; _ = safeInt; _ = safeInt8; _ = uAssign; _ = ch
	return 0
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)

	conf := types.Config{}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	pkg, err := conf.Check("main", fset, []*ast.File{file}, info)
	require.NoError(t, err)

	var unsignedZeroMutations int
	var safeMutations int

	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return true
		}
		muts := MutatorNumbersDecrementer(pkg, info, lit)
		tv := info.Types[lit]
		if tv.Type != nil {
			if basic, ok := tv.Type.Underlying().(*types.Basic); ok && basic.Info()&types.IsUnsigned != 0 && lit.Value == "0" {
				unsignedZeroMutations += len(muts)
				return true
			}
		}
		safeMutations += len(muts)
		return true
	})

	assert.Equal(t, 0, unsignedZeroMutations, "expected 0 mutations on unsigned 0 literals")
	assert.Greater(t, safeMutations, 0, "expected mutations on safe/signed literals")
}

func TestMutatorNumbersDecrementer_MutantsCompileCleanly(t *testing.T) {
	src := `package main

type MyUint uint

func takeUint(u uint) {}

func testFunc() uint {
	var u uint = 0
	var u8 uint8 = 0
	var u16 uint16 = 0
	var u32 uint32 = 0
	var u64 uint64 = 0
	var uptr uintptr = 0
	var b byte = 0
	var mu MyUint = 0

	var safeUint uint = 10
	var safeInt int = 0

	takeUint(0)
	_ = uint(0)
	var uAssign uint
	uAssign = 0
	if uAssign == 0 {}

	type S struct {
		U uint
	}
	_ = S{U: 0}
	_ = []uint{0}
	_ = map[uint]uint{0: 0}
	ch := make(chan uint, 1)
	ch <- 0

	_ = u; _ = u8; _ = u16; _ = u32; _ = u64; _ = uptr; _ = b; _ = mu
	_ = safeUint; _ = safeInt; _ = uAssign; _ = ch
	return 0
}
`
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)

	conf := types.Config{}
	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	pkg, err := conf.Check("main", fset, []*ast.File{file}, info)
	require.NoError(t, err)

	var allMutations []mutator.Mutation
	ast.Inspect(file, func(n ast.Node) bool {
		muts := MutatorNumbersDecrementer(pkg, info, n)
		allMutations = append(allMutations, muts...)
		return true
	})

	for i, m := range allMutations {
		m.Change()
		buf := new(bytes.Buffer)
		err := printer.Fprint(buf, fset, file)
		require.NoError(t, err)

		mutantFset := token.NewFileSet()
		mutantFile, err := parser.ParseFile(mutantFset, "mutant.go", buf.String(), 0)
		require.NoError(t, err)

		checkInfo := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
		_, compileErr := conf.Check("main", mutantFset, []*ast.File{mutantFile}, checkInfo)
		m.Reset()
		assert.NoError(t, compileErr, "mutant %d failed to compile: %v\nsource:\n%s", i, compileErr, buf.String())
	}
}
