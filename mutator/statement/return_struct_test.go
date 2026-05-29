package statement

import (
	"testing"

	"github.com/quality-gates/mutago/v2/test"
)

func TestMutatorReturnValueStruct(t *testing.T) {
	test.Mutator(
		t,
		MutatorReturnValue,
		"../../testdata/statement/return_struct.go",
		1,
	)
}

func TestMutatorReturnValueZeroStruct(t *testing.T) {
	test.Mutator(
		t,
		MutatorReturnValue,
		"../../testdata/statement/return_zero_struct.go",
		0,
	)
}

func TestMutatorReturnValueImportedStruct(t *testing.T) {
	test.Mutator(
		t,
		MutatorReturnValue,
		"../../testdata/statement/return_imported_struct.go",
		1,
	)
}

func TestMutatorReturnValueAlreadyZero(t *testing.T) {
	test.Mutator(
		t,
		MutatorReturnValue,
		"../../testdata/statement/return_already_zero.go",
		0,
	)
}

func TestMutatorReturnValueUnsafe(t *testing.T) {
	test.Mutator(
		t,
		MutatorReturnValue,
		"../../testdata/statement/return_unsafe.go",
		1,
	)
}
