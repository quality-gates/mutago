package numbers

import (
	"go/ast"
	"go/types"
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseAndFormatIntLiteral(t *testing.T) {
	tests := []struct {
		input       string
		decExpected string
		incExpected string
	}{
		{"10", "9", "11"},
		{"0", "(-1)", "1"},
		{"1_000", "999", "1001"},
		{"0x10", "0xf", "0x11"},
		{"0X10", "0Xf", "0X11"},
		{"0x0", "(-0x1)", "0x1"},
		{"0b10", "0b1", "0b11"},
		{"0B10", "0B1", "0B11"},
		{"0b0", "(-0b1)", "0b1"},
		{"0o10", "0o7", "0o11"},
		{"0O10", "0O7", "0O11"},
		{"0o0", "(-0o1)", "0o1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			info, ok := parseIntLiteral(tt.input)
			assert.True(t, ok)
			assert.Equal(t, tt.decExpected, formatIntLiteral(info.val-1, info))
			assert.Equal(t, tt.incExpected, formatIntLiteral(info.val+1, info))
		})
	}

	_, ok := parseIntLiteral("invalid")
	assert.False(t, ok)
}

func TestShouldSkipDecrement(t *testing.T) {
	dummyExpr := &ast.BasicLit{}

	// info == nil
	assert.False(t, shouldSkipDecrement(nil, dummyExpr, 0))

	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
	}

	// expr not in info.Types
	assert.False(t, shouldSkipDecrement(info, dummyExpr, 0))

	// non-basic underlying type (e.g. struct)
	structExpr := &ast.BasicLit{}
	info.Types[structExpr] = types.TypeAndValue{
		Type: types.NewStruct(nil, nil),
	}
	assert.False(t, shouldSkipDecrement(info, structExpr, 0))

	// basic non-integer type (e.g. float)
	floatExpr := &ast.BasicLit{}
	info.Types[floatExpr] = types.TypeAndValue{
		Type: types.Typ[types.Float64],
	}
	assert.False(t, shouldSkipDecrement(info, floatExpr, 0))

	// signed integer (e.g. int): should never skip decrementing
	signedExpr := &ast.BasicLit{}
	info.Types[signedExpr] = types.TypeAndValue{
		Type: types.Typ[types.Int],
	}
	assert.False(t, shouldSkipDecrement(info, signedExpr, 0))
	assert.False(t, shouldSkipDecrement(info, signedExpr, -1))
	assert.False(t, shouldSkipDecrement(info, signedExpr, 1))

	// unsigned integer (e.g. uint): skip if val <= 0
	unsignedExpr := &ast.BasicLit{}
	info.Types[unsignedExpr] = types.TypeAndValue{
		Type: types.Typ[types.Uint],
	}
	assert.True(t, shouldSkipDecrement(info, unsignedExpr, 0))
	assert.True(t, shouldSkipDecrement(info, unsignedExpr, -1))
	assert.False(t, shouldSkipDecrement(info, unsignedExpr, 1))
	assert.False(t, shouldSkipDecrement(info, unsignedExpr, 10))

	// byte (uint8)
	byteExpr := &ast.BasicLit{}
	info.Types[byteExpr] = types.TypeAndValue{
		Type: types.Typ[types.Uint8],
	}
	assert.True(t, shouldSkipDecrement(info, byteExpr, 0))
	assert.False(t, shouldSkipDecrement(info, byteExpr, 1))
}

func TestShouldSkipIncrement(t *testing.T) {
	dummyExpr := &ast.BasicLit{}

	// info == nil
	assert.False(t, shouldSkipIncrement(nil, dummyExpr, 0))

	info := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
	}

	// expr not in info.Types
	assert.False(t, shouldSkipIncrement(info, dummyExpr, 0))

	// non-basic underlying type
	structExpr := &ast.BasicLit{}
	info.Types[structExpr] = types.TypeAndValue{
		Type: types.NewStruct(nil, nil),
	}
	assert.False(t, shouldSkipIncrement(info, structExpr, 0))

	// unbounded basic type (e.g. float)
	floatExpr := &ast.BasicLit{}
	info.Types[floatExpr] = types.TypeAndValue{
		Type: types.Typ[types.Float64],
	}
	assert.False(t, shouldSkipIncrement(info, floatExpr, math.MaxInt64))

	// test each bounded type in expectedBounds
	expectedBounds := map[types.BasicKind]int64{
		types.Int8:    math.MaxInt8,
		types.Int16:   math.MaxInt16,
		types.Int32:   math.MaxInt32,
		types.Int64:   math.MaxInt64,
		types.Int:     math.MaxInt,
		types.Uint8:   math.MaxUint8,
		types.Uint16:  math.MaxUint16,
		types.Uint32:  math.MaxUint32,
		types.Uint64:  math.MaxInt64,
		types.Uint:    math.MaxInt,
		types.Uintptr: math.MaxInt,
	}
	for kind, bound := range expectedBounds {
		expr := &ast.BasicLit{}
		info.Types[expr] = types.TypeAndValue{
			Type: types.Typ[kind],
		}
		assert.True(t, shouldSkipIncrement(info, expr, bound), "expected skip at bound for kind %v", kind)
		if bound < math.MaxInt64 {
			assert.True(t, shouldSkipIncrement(info, expr, bound+1), "expected skip above bound for kind %v", kind)
		}
		assert.False(t, shouldSkipIncrement(info, expr, bound-1), "expected no skip below bound for kind %v", kind)
	}
}
