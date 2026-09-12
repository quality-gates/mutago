package numbers

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/stretchr/testify/require"
)

func parseTyped(t *testing.T, src string) (*ast.File, *types.Info) {
	t.Helper()
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", src, 0)
	require.NoError(t, err)
	info := &types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	_, err = (&types.Config{}).Check("main", fset, []*ast.File{file}, info)
	require.NoError(t, err)
	return file, info
}

func TestMutatorNumbersDecrementer_SkipsUnsignedZero(t *testing.T) {
	src := `package main
func f() {
	var a uint = 0
	var b byte = 0
	var c uint16 = 0
	var d uint32 = 0
	var e uint64 = 0
	var f uintptr = 0
	var g int = 0
	var h uint = 1
	_ = a
	_ = b
	_ = c
	_ = d
	_ = e
	_ = f
	_ = g
	_ = h
}
`
	file, info := parseTyped(t, src)

	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return true
		}
		muts := MutatorNumbersDecrementer(nil, info, lit)
		tname := ""
		if tv, ok := info.Types[lit]; ok && tv.Type != nil {
			tname = tv.Type.String()
		}
		switch {
		case lit.Value == "0" && (tname == "uint" || tname == "byte" || tname == "uint16" ||
			tname == "uint32" || tname == "uint64" || tname == "uintptr"):
			require.Empty(t, muts, "expected skip for %s=0", tname)
		case lit.Value == "0" && tname == "int":
			require.Len(t, muts, 1, "int 0 should still decrement")
		case lit.Value == "1" && tname == "uint":
			require.Len(t, muts, 1, "uint 1 should still decrement")
		}
		return true
	})
}

func TestMutatorNumbersIncrementer_SkipsTypedMax(t *testing.T) {
	src := `package main
func f() {
	var a int8 = 127
	var b byte = 255
	var c uint8 = 255
	var d int16 = 32767
	var e uint16 = 65535
	var f int8 = 126
	var g byte = 254
	var h int = 100
	_ = a
	_ = b
	_ = c
	_ = d
	_ = e
	_ = f
	_ = g
	_ = h
}
`
	file, info := parseTyped(t, src)

	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return true
		}
		muts := MutatorNumbersIncrementer(nil, info, lit)
		tname := ""
		if tv, ok := info.Types[lit]; ok && tv.Type != nil {
			tname = tv.Type.String()
		}
		switch {
		case lit.Value == "127" && tname == "int8":
			require.Empty(t, muts, "int8 max")
		case lit.Value == "255" && (tname == "byte" || tname == "uint8"):
			require.Empty(t, muts, "%s max", tname)
		case lit.Value == "32767" && tname == "int16":
			require.Empty(t, muts, "int16 max")
		case lit.Value == "65535" && tname == "uint16":
			require.Empty(t, muts, "uint16 max")
		case lit.Value == "126" && tname == "int8":
			require.Len(t, muts, 1, "int8 below max")
		case lit.Value == "254" && tname == "byte":
			require.Len(t, muts, 1, "byte below max")
		case lit.Value == "100" && tname == "int":
			require.Len(t, muts, 1, "int mid-range")
		}
		return true
	})
}

func TestMutatorNumbers_NamedUnsignedType(t *testing.T) {
	src := `package main
type T uint8
func f() {
	var a T = 0
	var b T = 255
	_ = a
	_ = b
}
`
	file, info := parseTyped(t, src)
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok || lit.Kind != token.INT {
			return true
		}
		switch lit.Value {
		case "0":
			require.Empty(t, MutatorNumbersDecrementer(nil, info, lit))
			require.Len(t, MutatorNumbersIncrementer(nil, info, lit), 1)
		case "255":
			require.Len(t, MutatorNumbersDecrementer(nil, info, lit), 1)
			require.Empty(t, MutatorNumbersIncrementer(nil, info, lit))
		}
		return true
	})
}

func TestMaxIntOf(t *testing.T) {
	cases := []struct {
		kind types.BasicKind
		max  int64
		ok   bool
	}{
		{types.Int8, 127, true},
		{types.Uint8, 255, true},
		{types.Int16, 32767, true},
		{types.Uint16, 65535, true},
		{types.Int32, 2147483647, true},
		{types.Uint32, 4294967295, true},
		{types.Int64, 9223372036854775807, true},
		{types.Uint64, 0, false},
	}
	for _, tt := range cases {
		basic := types.Typ[tt.kind]
		got, ok := maxIntOf(basic)
		require.Equal(t, tt.ok, ok, "kind %v", tt.kind)
		if ok {
			require.Equal(t, tt.max, got, "kind %v", tt.kind)
		}
	}
}
