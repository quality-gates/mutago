package parser

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
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
	_, err := ParseAndTypeCheckFile("../../astutil/create.go", collectors)
	assert.Nil(t, err)
}

func TestPreparePackagesRejectsTypeErrors(t *testing.T) {
	ClearPackageCache()
	t.Cleanup(ClearPackageCache)
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module broken\n\ngo 1.24\n"), 0600))
	file := filepath.Join(dir, "broken.go")
	assert.NoError(t, os.WriteFile(file, []byte("package broken\nvar X = undeclaredVar\n"), 0600))

	err := PreparePackages([]string{file}, dir)
	if assert.Error(t, err) {
		assert.Contains(t, err.Error(), "package load error(s)")
		assert.Contains(t, err.Error(), "broken:")
		assert.Contains(t, err.Error(), "undefined: undeclaredVar")
	}
}

func TestParseAndTypeCheckFileRejectsTypeErrors(t *testing.T) {
	ClearPackageCache()
	t.Cleanup(ClearPackageCache)
	dir := t.TempDir()
	assert.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module broken\n\ngo 1.24\n"), 0600))
	file := filepath.Join(dir, "broken.go")
	assert.NoError(t, os.WriteFile(file, []byte("package broken\nvar X = undeclaredVar\n"), 0600))

	for range 2 { // Cached failures must not yield incomplete types either.
		checked, err := ParseAndTypeCheckFile(file, nil)
		assert.Nil(t, checked)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), "broken:")
			assert.Contains(t, err.Error(), "undefined: undeclaredVar")
		}
	}
}

func TestPreparePackagesIndexesMultipleTargetPackages(t *testing.T) {
	ClearPackageCache()
	files := []string{
		"../importing/filepathfixtures/first.go",
		"../importing/filepathfixtures/secondfixturespackage/fourth.go",
	}
	assert.NoError(t, PreparePackages(files, ""))

	first, err := ParseAndTypeCheckFile(files[0], nil)
	assert.NoError(t, err)
	second, err := ParseAndTypeCheckFile(files[1], nil)
	assert.NoError(t, err)
	assert.NotEqual(t, first.Pkg.Path(), second.Pkg.Path())
}

func TestPreparePackagesLoadsTargetDirectoriesInOneGraph(t *testing.T) {
	if os.Getenv("MUTAGO_PACKAGE_LOAD_HELPER") == "1" {
		ClearPackageCache()
		err := PreparePackages([]string{
			"../importing/filepathfixtures/first.go",
			"../importing/filepathfixtures/second.go",
			"../importing/filepathfixtures/secondfixturespackage/fourth.go",
		}, "")
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
