package importing

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"

	"github.com/stretchr/testify/assert"
)

func toSlashSlice(paths []string) []string {
	if paths == nil {
		return nil
	}
	out := make([]string, len(paths))
	for i, p := range paths {
		out[i] = filepath.ToSlash(p)
	}
	return out
}

func toSlashPackages(pkgs []Package) []Package {
	if pkgs == nil {
		return nil
	}
	out := make([]Package, len(pkgs))
	for i, p := range pkgs {
		out[i] = Package{
			Name:  filepath.ToSlash(p.Name),
			Files: toSlashSlice(p.Files),
		}
	}
	return out
}

func TestFilesOfArgs(t *testing.T) {
	for _, test := range []struct {
		args   []string
		expect []string
	}{
		{
			[]string{},
			[]string{"filepath.go", "import.go"},
		},
		{
			[]string{"./filepathfixtures/first.go"},
			[]string{"./filepathfixtures/first.go"},
		},
		{
			[]string{"./filepathfixtures"},
			[]string{"filepathfixtures/fifth.go", "filepathfixtures/first.go", "filepathfixtures/second.go", "filepathfixtures/third.go"},
		},
		{
			[]string{"../importing/filepathfixtures"},
			[]string{
				"../importing/filepathfixtures/fifth.go",
				"../importing/filepathfixtures/first.go",
				"../importing/filepathfixtures/second.go",
				"../importing/filepathfixtures/third.go",
			},
		},
	} {
		var opts = &models.Options{}
		got := toSlashSlice(FilesOfArgs(test.args, opts))
		assert.Equal(t, test.expect, got, fmt.Sprintf("With args: %#v", test.args))
	}
}

func TestFilesOfArgsIncludesCgoFiles(t *testing.T) {
	var opts = &models.Options{}

	got := toSlashSlice(FilesOfArgs([]string{"./cgofixture"}, opts))

	assert.Equal(t, []string{"cgofixture/cgo.go"}, got)
}

func TestPackagesWithFilesOfArgs(t *testing.T) {
	for _, test := range []struct {
		args   []string
		expect []Package
	}{
		{
			[]string{},
			[]Package{{Name: ".", Files: []string{"filepath.go", "import.go"}}},
		},
		{
			[]string{"./filepathfixtures/first.go"},
			[]Package{{Name: "filepathfixtures", Files: []string{"./filepathfixtures/first.go"}}},
		},
		{
			[]string{"./filepathfixtures"},
			[]Package{{Name: "filepathfixtures", Files: []string{
				"filepathfixtures/fifth.go",
				"filepathfixtures/first.go",
				"filepathfixtures/second.go",
				"filepathfixtures/third.go",
			}}},
		},
		{
			[]string{"../importing/filepathfixtures"},
			[]Package{{Name: "../importing/filepathfixtures", Files: []string{
				"../importing/filepathfixtures/fifth.go",
				"../importing/filepathfixtures/first.go",
				"../importing/filepathfixtures/second.go",
				"../importing/filepathfixtures/third.go",
			}}},
		},
		{
			[]string{"./filepathfixtures/..."},
			[]Package{
				{Name: "filepathfixtures", Files: []string{
					"filepathfixtures/fifth.go",
					"filepathfixtures/first.go",
					"filepathfixtures/second.go",
					"filepathfixtures/third.go",
				}},
				{Name: "filepathfixtures/secondfixturespackage", Files: []string{
					"filepathfixtures/secondfixturespackage/fourth.go",
				}},
			},
		},
	} {
		var opts = &models.Options{}
		got := toSlashPackages(PackagesWithFilesOfArgs(test.args, opts))
		assert.Equal(t, test.expect, got, fmt.Sprintf("With args: %#v", test.args))
	}
}

func TestFilesWithSkipWithoutTests(t *testing.T) {
	for _, test := range []struct {
		args   []string
		expect []string
	}{
		{
			[]string{"./filepathfixtures/first.go"},
			[]string(nil),
		},
		{
			[]string{"./filepathfixtures/second.go"},
			[]string{"./filepathfixtures/second.go"},
		},
		{
			[]string{"./filepathfixtures"},
			[]string{"filepathfixtures/fifth.go", "filepathfixtures/second.go", "filepathfixtures/third.go"},
		},
	} {
		var opts = &models.Options{}
		opts.Config.SkipFileWithoutTest = true
		got := toSlashSlice(FilesOfArgs(test.args, opts))
		assert.Equal(t, test.expect, got, fmt.Sprintf("With args: %#v", test.args))
	}
}

func TestFilesWithSkipWithBuildTagsTests(t *testing.T) {
	for _, test := range []struct {
		args   []string
		expect []string
	}{
		{
			[]string{"./filepathfixtures/first.go"},
			[]string(nil),
		},
		{
			[]string{"./filepathfixtures/third.go"},
			[]string(nil),
		},
		{
			[]string{"./filepathfixtures/fifth.go"},
			[]string(nil),
		},
		{
			[]string{"./filepathfixtures/second.go"},
			[]string{"./filepathfixtures/second.go"},
		},
		{
			[]string{"./filepathfixtures"},
			[]string{"filepathfixtures/second.go"},
		},
	} {
		var opts = models.NewOptions()
		opts.Config.SkipFileWithBuildTag = true
		got := toSlashSlice(FilesOfArgs(test.args, opts))
		assert.Equal(t, test.expect, got, fmt.Sprintf("With args: %#v", test.args))
	}
}

func TestFilesWithBuildTagsOnlyDoesNotSkipUntestedFiles(t *testing.T) {
	var opts = &models.Options{}
	opts.Config.SkipFileWithoutTest = false
	opts.Config.SkipFileWithBuildTag = true

	got := toSlashSlice(FilesOfArgs([]string{"./filepathfixtures"}, opts))
	assert.Equal(t, []string{"filepathfixtures/first.go", "filepathfixtures/second.go"}, got)
}

func TestFilesWithExcludedDirs(t *testing.T) {
	for _, test := range []struct {
		args   []string
		expect []string
		config []string
	}{
		{
			[]string{"./filepathfixtures/first.go"},
			[]string{"./filepathfixtures/first.go"},
			[]string(nil),
		},
		{
			[]string{"./filepathfixtures/second.go"},
			[]string{"./filepathfixtures/second.go"},
			[]string{"filepathfixtures"},
		},
		{
			[]string{"filepathfixtures/second.go"},
			[]string(nil),
			[]string{"filepathfixtures"},
		},
		{
			[]string{"./filepathfixtures/second.go"},
			[]string(nil),
			[]string{"./filepathfixtures"},
		},
		{
			[]string{"./filepathfixtures/..."},
			[]string{
				"filepathfixtures/fifth.go",
				"filepathfixtures/first.go",
				"filepathfixtures/second.go",
				"filepathfixtures/third.go",
			},
			[]string{"filepathfixtures/secondfixturespackage"},
		},
		{
			[]string{"./filepathfixtures/..."},
			[]string(nil),
			[]string{"filepathfixtures"},
		},
		{
			[]string{"./filepathfixtures"},
			[]string(nil),
			[]string{"filepathfixtures"},
		},
		{
			[]string{"./filepathfixtures"},
			[]string{
				"filepathfixtures/fifth.go",
				"filepathfixtures/first.go",
				"filepathfixtures/second.go",
				"filepathfixtures/third.go",
			},
			[]string(nil),
		},
	} {
		var opts = &models.Options{}
		opts.Config.ExcludeDirs = test.config
		got := toSlashSlice(FilesOfArgs(test.args, opts))
		assert.Equal(t, test.expect, got, fmt.Sprintf("With args: %#v", test.args))
	}
}

func TestShouldSkipFileBackslashExcludesDir(t *testing.T) {
	opts := &models.Options{}
	opts.Config.ExcludeDirs = []string{"filepathfixtures/secondfixturespackage"}
	filename := `filepathfixtures\secondfixturespackage\fourth.go`
	assert.True(t, shouldSkipFile(filename, opts), "expected backslash-separated filename to be excluded by slash-written exclude_dirs")
}
