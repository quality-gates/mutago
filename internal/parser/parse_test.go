package parser

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/quality-gates/mutago/v2/internal/annotation"
	"github.com/quality-gates/mutago/v2/internal/filter"
)

func TestParseAndTypeCheckFileTypeCheckWholePackage(t *testing.T) {
	annotationProcessor := annotation.NewProcessor()
	skipFilterProcessor := filter.NewSkipMakeArgsFilter()

	collectors := []filter.NodeCollector{
		annotationProcessor,
		skipFilterProcessor,
	}
	_, _, _, _, err := ParseAndTypeCheckFile("../../astutil/create.go", collectors)
	assert.Nil(t, err)
}

func TestPreparePackagesIndexesMultipleTargetPackages(t *testing.T) {
	ClearPackageCache()
	files := []string{
		"../importing/filepathfixtures/first.go",
		"../importing/filepathfixtures/secondfixturespackage/fourth.go",
	}
	assert.NoError(t, PreparePackages(files))

	_, _, firstPkg, _, err := ParseAndTypeCheckFile(files[0], nil)
	assert.NoError(t, err)
	_, _, secondPkg, _, err := ParseAndTypeCheckFile(files[1], nil)
	assert.NoError(t, err)
	assert.NotEqual(t, firstPkg.Path(), secondPkg.Path())
}
