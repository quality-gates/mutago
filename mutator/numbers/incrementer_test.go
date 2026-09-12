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
		{original: "0x7fffffffffffffff", mutated: "(-0x8000000000000000)"},
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

func TestMutatorNumbersIncrementer_SkipsMaxBoundaries(t *testing.T) {
	src := `package main

type CustomByte byte

func takeByte(b byte) {}

func testFunc() byte {
	var i8 int8 = 127
	var i16 int16 = 32767
	var i32 int32 = 2147483647
	var i64 int64 = 9223372036854775807
	var u8 uint8 = 255
	var u16 uint16 = 65535
	var u32 uint32 = 4294967295
	var b byte = 255
	var cb CustomByte = 255

	var safeInt8 int8 = 100
	var safeByte byte = 200

	takeByte(255)
	_ = byte(255)
	var bAssign byte
	bAssign = 255
	if bAssign == 255 {}

	type S struct {
		B byte
	}
	_ = S{B: 255}
	_ = []byte{255}
	_ = map[byte]byte{255: 255}
	ch := make(chan byte, 1)
	ch <- 255

	_ = i8; _ = i16; _ = i32; _ = i64; _ = u8; _ = u16; _ = u32; _ = b; _ = cb
	_ = safeInt8; _ = safeByte; _ = bAssign; _ = ch
	return 255
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

	var maxBoundaryMutations int
	var safeMutations int

	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return true
		}
		muts := MutatorNumbersIncrementer(pkg, info, lit)
		if lit.Value == "100" || lit.Value == "200" || lit.Value == "1" {
			safeMutations += len(muts)
			return true
		}
		maxBoundaryMutations += len(muts)
		return true
	})

	assert.Equal(t, 0, maxBoundaryMutations, "expected 0 mutations on max boundary literals")
	assert.Greater(t, safeMutations, 0, "expected mutations on safe literals")
}

func TestMutatorNumbersIncrementer_MutantsCompileCleanly(t *testing.T) {
	src := `package main

type CustomByte byte

func takeByte(b byte) {}

func testFunc() byte {
	var i8 int8 = 127
	var i16 int16 = 32767
	var i32 int32 = 2147483647
	var i64 int64 = 9223372036854775807
	var u8 uint8 = 255
	var u16 uint16 = 65535
	var u32 uint32 = 4294967295
	var b byte = 255
	var cb CustomByte = 255

	var safeInt8 int8 = 100
	var safeByte byte = 200

	takeByte(255)
	_ = byte(255)
	var bAssign byte
	bAssign = 255
	if bAssign == 255 {}

	type S struct {
		B byte
	}
	_ = S{B: 255}
	_ = []byte{255}
	_ = map[byte]byte{255: 255}
	ch := make(chan byte, 1)
	ch <- 255

	_ = i8; _ = i16; _ = i32; _ = i64; _ = u8; _ = u16; _ = u32; _ = b; _ = cb
	_ = safeInt8; _ = safeByte; _ = bAssign; _ = ch
	return 255
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
		muts := MutatorNumbersIncrementer(pkg, info, n)
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
