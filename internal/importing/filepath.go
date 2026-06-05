package importing

/*

This file holds lots of code of the golint project https://github.com/golang/lint and some code of a pull request of mine https://github.com/golang/lint/pull/76
This is just temporary until I have time to clean up this code and make a more general solution for go-commands as I stated here https://github.com/kisielk/errcheck/issues/45#issuecomment-57732642

so TODO and FIXME. Heck I also give you a WORKAROUND.

*/

import (
	"fmt"
	"go/build"
	"log"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/quality-gates/mutago/v2/internal/models"
)

var (
	buildTagRegex  = regexp.MustCompile(`(?m)^//(go:build|\s*\+build)`)
	testFileSuffix = "_test.go"
)

func packagesWithFilesOfArgs(args []string, opts *models.Options) map[string]map[string]struct{} {
	filenames := resolveFileList(args)
	fileLookup := make(map[string]struct{})
	pkgs := make(map[string]map[string]struct{})

	for _, filename := range filenames {
		if _, ok := fileLookup[filename]; ok {
			continue
		}
		if shouldSkipFile(filename, opts) {
			continue
		}
		if !exists(filename) {
			fmt.Printf("%q does not exist", filename)
			continue
		}
		fileLookup[filename] = struct{}{}
		pkgName := path.Dir(filename)
		pkg, ok := pkgs[pkgName]
		if !ok {
			pkg = make(map[string]struct{})
			pkgs[pkgName] = pkg
		}
		pkg[filename] = struct{}{}
	}

	return pkgs
}

// resolveFileList expands args into a flat list of .go filenames.
func resolveFileList(args []string) []string {
	var filenames []string
	if len(args) == 0 {
		return append(filenames, checkDir(".")...)
	}
	for _, arg := range args {
		if strings.HasSuffix(arg, "/...") && isDir(arg[:len(arg)-4]) {
			for _, dirname := range allPackagesInFS(arg) {
				filenames = append(filenames, checkDir(dirname)...)
			}
			continue
		}
		if isDir(arg) {
			filenames = append(filenames, checkDir(arg)...)
			continue
		}
		if exists(arg) {
			filenames = append(filenames, arg)
			continue
		}
		for _, pkgname := range importPaths([]string{arg}) {
			filenames = append(filenames, checkPackage(pkgname)...)
		}
	}
	return filenames
}

// shouldSkipFile reports whether filename should be excluded from mutation.
func shouldSkipFile(filename string, opts *models.Options) bool {
	if strings.HasSuffix(filename, testFileSuffix) {
		return true
	}
	for _, exDir := range opts.Config.ExcludeDirs {
		if strings.HasPrefix(filename, exDir) {
			return true
		}
	}
	return skipForMissingOrTaggedTest(filename, opts)
}

// skipForMissingOrTaggedTest reports whether filename should be skipped under
// the "skip files without a test" and "skip files whose test has a build tag"
// policies.
func skipForMissingOrTaggedTest(filename string, opts *models.Options) bool {
	if !opts.Config.SkipFileWithoutTest && !opts.Config.SkipFileWithBuildTag {
		return false
	}
	nameSize := len(filename)
	if nameSize <= 3 {
		return true
	}
	testName := filename[:nameSize-3] + testFileSuffix
	if !exists(testName) {
		return true
	}
	return opts.Config.SkipFileWithBuildTag && regexpSearchInFile(testName, buildTagRegex)
}

func regexpSearchInFile(file string, re *regexp.Regexp) bool {
	contents, err := os.ReadFile(file)
	if err != nil {
		log.Fatal(err)
	}

	return re.MatchString(string(contents))
}

// FilesOfArgs returns all available Go files given a list of packages, directories and files which can embed patterns.
func FilesOfArgs(args []string, opts *models.Options) []string {
	pkgs := packagesWithFilesOfArgs(args, opts)

	pkgsNames := make([]string, 0, len(pkgs))
	for name := range pkgs {
		pkgsNames = append(pkgsNames, name)
	}
	sort.Strings(pkgsNames)

	var files []string

	for _, name := range pkgsNames {
		var filenames []string
		for name := range pkgs[name] {
			filenames = append(filenames, name)
		}
		sort.Strings(filenames)

		files = append(files, filenames...)
	}

	return files
}

// Package holds file information of a package.
type Package struct {
	Name  string
	Files []string
}

// Packages defines a list of packages.
type Packages []Package

// Len is the number of elements in the collection.
func (p Packages) Len() int { return len(p) }

// Swap swaps the elements with indexes i and j.
func (p Packages) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// PackagesByName sorts a list of packages by their name.
type PackagesByName struct{ Packages }

// Less reports whether the element with index i should sort before the element with index j.
func (p PackagesByName) Less(i, j int) bool { return p.Packages[i].Name < p.Packages[j].Name }

// PackagesWithFilesOfArgs returns all available Go files sorted by their packages given a list of packages, directories and files which can embed patterns.
func PackagesWithFilesOfArgs(args []string, opts *models.Options) []Package {
	pkgs := packagesWithFilesOfArgs(args, opts)

	r := make([]Package, 0, len(pkgs))
	for name := range pkgs {
		r = append(r, Package{
			Name: name,
		})
	}
	sort.Sort(PackagesByName{r})

	for i := range r {
		var filenames []string
		for name := range pkgs[r[i].Name] {
			filenames = append(filenames, name)
		}
		sort.Strings(filenames)

		r[i].Files = filenames
	}

	return r
}

func isDir(filename string) bool {
	fi, err := os.Stat(filename)
	return err == nil && fi.IsDir()
}

func exists(filename string) bool {
	_, err := os.Stat(filename)
	return err == nil
}

func checkDir(dirname string) []string {
	pkg, err := build.ImportDir(dirname, 0)

	return checkImportedPackage(pkg, err)
}

func checkPackage(pkgname string) []string {
	pkg, err := build.Import(pkgname, ".", 0)

	return checkImportedPackage(pkg, err)
}

func checkImportedPackage(pkg *build.Package, err error) []string {
	if err != nil {
		if _, nogo := err.(*build.NoGoError); nogo {
			// Don't complain if the failure is due to no Go source files.
			return []string{}
		}
		_, err := fmt.Fprintln(os.Stderr, err)
		if err != nil {
			fmt.Println(err)
		}

		return []string{}
	}

	var files []string

	files = append(files, pkg.GoFiles...)

	joinDirWithFilenames(pkg.Dir, files)

	return files
}

func joinDirWithFilenames(dir string, files []string) {
	if dir != "." {
		for i, f := range files {
			files[i] = filepath.Join(dir, f)
		}
	}
}
