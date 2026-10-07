package annotation_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/quality-gates/mutago/v2"
	"github.com/quality-gates/mutago/v2/internal/annotation"
	"github.com/quality-gates/mutago/v2/mutator"
	_ "github.com/quality-gates/mutago/v2/mutator/arithmetic"
	_ "github.com/quality-gates/mutago/v2/mutator/composite"
	_ "github.com/quality-gates/mutago/v2/mutator/concurrency"
	_ "github.com/quality-gates/mutago/v2/mutator/select"
	_ "github.com/quality-gates/mutago/v2/mutator/statement"
)

// containerSource has one target line per container-level mutator. Each
// target line ends with a "t:<mutator>" marker, and the line before it holds a
// "{<mutator>}" placeholder for a next-line directive.
const containerSource = `package pkg

type Config struct{ A, B int }

func Run(cleanup func(), ch chan int, x int) Config {
	{statement/defer-remove}
	defer cleanup() // t:statement/defer-remove
	{concurrency/goroutine-remove}
	go cleanup() // t:concurrency/goroutine-remove
	{statement/remove-self-assign}
	x = x // t:statement/remove-self-assign
	_ = ch
	select {
	{select/case-remove}
	case <-ch: // t:select/case-remove
	{select/default-remove}
	default: // t:select/default-remove
	}
	return Config{
		A: 0,
		{composite/field-clear}
		B: 2, // t:composite/field-clear
	}
}
`

var containerMutators = []string{
	"statement/defer-remove",
	"concurrency/goroutine-remove",
	"statement/remove-self-assign",
	"select/case-remove",
	"select/default-remove",
	"composite/field-clear",
}

// countFilteredMutations counts the mutations the named mutator yields after
// annotation filtering. nextLine is placed above that mutator's target line,
// and header is placed at the top of the file.
func countFilteredMutations(t *testing.T, mutatorName, nextLine, header string) int {
	t.Helper()
	pairs := []string{}
	for _, name := range containerMutators {
		directive := "// unrelated"
		if name == mutatorName && nextLine != "" {
			directive = nextLine
		}
		pairs = append(pairs, "{"+name+"}", directive)
	}
	src := header + "\n" + strings.NewReplacer(pairs...).Replace(containerSource)
	return countFilteredMutationsIn(t, src, mutatorName)
}

// countFilteredMutationsIn counts the mutations the named mutator yields for
// src after annotation filtering.
func countFilteredMutationsIn(t *testing.T, src, mutatorName string) int {
	t.Helper()
	path := filepath.Join(t.TempDir(), "sample.go")
	require.NoError(t, os.WriteFile(path, []byte(src), 0o644))
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	require.NoError(t, err)
	info := &types.Info{
		Types: map[ast.Expr]types.TypeAndValue{},
		Defs:  map[*ast.Ident]types.Object{},
		Uses:  map[*ast.Ident]types.Object{},
	}
	pkg, err := (&types.Config{Importer: importer.Default()}).Check("pkg", fset, []*ast.File{file}, info)
	require.NoError(t, err)

	processor := annotation.NewProcessor()
	processor.Collect(file, fset, path)

	m, err := mutator.New(mutatorName)
	require.NoError(t, err)
	return mutago.CountWalk(pkg, info, file, annotation.DecoratorFilter(m, mutatorName, processor))
}

func TestContainerMutatorsAreNotSuppressedWithoutAnnotation(t *testing.T) {
	for _, name := range containerMutators {
		assert.Equal(t, 1, countFilteredMutations(t, name, "", ""), name)
	}
}

func TestNextLineAnnotationSuppressesContainerMutators(t *testing.T) {
	for _, name := range containerMutators {
		for _, directive := range []string{
			"// mutator-disable-next-line " + name,
			"// mutator-disable-next-line *",
			"// mutator-disable-next-line",
		} {
			assert.Zero(t, countFilteredMutations(t, name, directive, ""), "%q should suppress %s", directive, name)
		}
	}
}

func TestNextLineAnnotationForAnotherMutatorKeepsContainerMutation(t *testing.T) {
	for _, name := range containerMutators {
		assert.Equal(t, 1, countFilteredMutations(t, name, "// mutator-disable-next-line arithmetic/base", ""), name)
	}
}

func TestRegexAnnotationSuppressesContainerMutators(t *testing.T) {
	for _, name := range containerMutators {
		header := "// mutator-disable-regexp t:" + name + " " + name
		assert.Zero(t, countFilteredMutations(t, name, "", header), name)
	}
}

func TestRegexAnnotationForAnotherMutatorKeepsContainerMutation(t *testing.T) {
	for _, name := range containerMutators {
		header := "// mutator-disable-regexp t:" + name + " arithmetic/base"
		assert.Equal(t, 1, countFilteredMutations(t, name, "", header), name)
	}
}

func TestNextLineAnnotationStillSuppressesOperatorPositionedMutations(t *testing.T) {
	const plain = "package pkg\n\nfunc Add(a int) int {\n\treturn a + 1\n}\n"
	const annotated = "package pkg\n\nfunc Add(a int) int {\n\t// mutator-disable-next-line arithmetic/base\n\treturn a + 1\n}\n"

	assert.Equal(t, 1, countFilteredMutationsIn(t, plain, "arithmetic/base"))
	assert.Zero(t, countFilteredMutationsIn(t, annotated, "arithmetic/base"))
}
