package importing

import (
	"fmt"
	"testing"

	"github.com/quality-gates/mutago/v2/internal/models"

	"github.com/stretchr/testify/assert"
)

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
		got := FilesOfArgs(test.args, opts)
		assert.Equal(t, test.expect, got, fmt.Sprintf("With args: %#v", test.args))
	}
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
	} {
		var opts = &models.Options{}
		got := PackagesWithFilesOfArgs(test.args, opts)
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
		got := FilesOfArgs(test.args, opts)
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
		var opts = &models.Options{}
		opts.Config.SkipFileWithBuildTag = true
		got := FilesOfArgs(test.args, opts)
		assert.Equal(t, test.expect, got, fmt.Sprintf("With args: %#v", test.args))
	}
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
		got := FilesOfArgs(test.args, opts)
		assert.Equal(t, test.expect, got, fmt.Sprintf("With args: %#v", test.args))
	}
}
