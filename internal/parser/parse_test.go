package parser

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
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

func TestPreparePackagesLoadsTargetDirectoriesInOneGraph(t *testing.T) {
	if os.Getenv("MUTAGO_PACKAGE_LOAD_HELPER") == "1" {
		ClearPackageCache()
		buildConstrained := "../../testdata/loop/condition.go"
		err := PreparePackages([]string{
			"../importing/filepathfixtures/first.go",
			"../importing/filepathfixtures/second.go",
			"../importing/filepathfixtures/secondfixturespackage/fourth.go",
			buildConstrained,
		})
		assert.NoError(t, err)
		_, _, _, _, err = ParseAndTypeCheckFile(buildConstrained, nil)
		assert.NoError(t, err)
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run=^TestPreparePackagesLoadsTargetDirectoriesInOneGraph$")
	cmd.Env = append(os.Environ(),
		"GOPACKAGESDEBUG=true",
		"GOPACKAGESDRIVER=off",
		"MUTAGO_PACKAGE_LOAD_HELPER=1",
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	assert.NoError(t, cmd.Run())

	typedLoads := 0
	for line := range strings.Lines(stderr.String()) {
		if strings.Contains(line, "starting ") &&
			strings.Contains(line, " go list ") &&
			strings.Contains(line, "-json=") {
			typedLoads++
		}
	}
	assert.Equal(t, 1, typedLoads, stderr.String())
}
