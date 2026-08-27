package parser

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sync"

	"golang.org/x/tools/go/packages"

	"github.com/quality-gates/mutago/v2/internal/filter"
)

// pkgCacheEntry holds the result of loading a package directory via packages.Load.
type pkgCacheEntry struct {
	pkgs []*packages.Package
	fset *token.FileSet
	err  error
}

type loadedFile struct {
	pkg  *packages.Package
	file *ast.File
	fset *token.FileSet
}

var (
	pkgCacheMu    sync.Mutex
	pkgCache      = map[string]*pkgCacheEntry{}
	preparedFiles = map[string]loadedFile{}
)

// ClearPackageCache resets the directory-level package-load cache.
// Call this in tests that need a clean state between ParseAndTypeCheckFile invocations.
func ClearPackageCache() {
	pkgCacheMu.Lock()
	pkgCache = map[string]*pkgCacheEntry{}
	preparedFiles = map[string]loadedFile{}
	pkgCacheMu.Unlock()
}

// PreparePackages loads all target packages in one packages.Load call and
// indexes their syntax by absolute filename for subsequent parsing requests.
func PreparePackages(files []string) error {
	patterns, seenDirs, err := packagePatterns(files)
	if err != nil {
		return err
	}
	if len(patterns) == 0 {
		return nil
	}

	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports,
		Fset: fset,
		ParseFile: func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
			return parser.ParseFile(fset, filename, src, parser.AllErrors|parser.ParseComments)
		},
	}
	pkgs, err := packages.Load(cfg, patterns...)
	if err != nil {
		return err
	}
	index := make(map[string]loadedFile)
	for _, pkg := range pkgs {
		for _, file := range pkg.Syntax {
			filename := fset.PositionFor(file.Pos(), false).Filename
			abs, absErr := filepath.Abs(filename)
			if absErr != nil {
				continue
			}
			index[abs] = loadedFile{pkg: pkg, file: file, fset: fset}
		}
	}
	pkgCacheMu.Lock()
	preparedFiles = index
	entry := &pkgCacheEntry{pkgs: pkgs, fset: fset}
	for dir := range seenDirs {
		pkgCache[dir] = entry
	}
	pkgCacheMu.Unlock()
	return nil
}

func packagePatterns(files []string) ([]string, map[string]struct{}, error) {
	patterns := make([]string, 0, len(files))
	seenDirs := make(map[string]struct{})
	for _, file := range files {
		abs, err := filepath.Abs(file)
		if err != nil {
			return nil, nil, err
		}
		dir := filepath.Dir(abs)
		if _, seen := seenDirs[dir]; seen {
			continue
		}
		seenDirs[dir] = struct{}{}
		patterns = append(patterns, dir)
	}
	return patterns, seenDirs, nil
}

// loadPkgForDir returns the cached packages.Load result for dir, computing it on first
// access. Subsequent calls for the same directory return the cached value without
// re-invoking the type-checker. Safe for concurrent use.
func loadPkgForDir(dir string) *pkgCacheEntry {
	pkgCacheMu.Lock()
	if entry, ok := pkgCache[dir]; ok {
		pkgCacheMu.Unlock()
		return entry
	}
	pkgCacheMu.Unlock()

	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedImports,
		Dir:  dir,
		Fset: fset,
		ParseFile: func(fset *token.FileSet, filename string, src []byte) (*ast.File, error) {
			return parser.ParseFile(fset, filename, src, parser.AllErrors|parser.ParseComments)
		},
	}
	pkgs, loadErr := packages.Load(cfg, ".")
	entry := &pkgCacheEntry{pkgs: pkgs, fset: fset, err: loadErr}

	pkgCacheMu.Lock()
	// Double-check: another goroutine may have stored first.
	if existing, ok := pkgCache[dir]; ok {
		pkgCacheMu.Unlock()
		return existing
	}
	pkgCache[dir] = entry
	pkgCacheMu.Unlock()

	return entry
}

// ParseFile parses the content of the given file and returns the corresponding ast.File node and its file set for positional information.
// If a fatal error is encountered the error return argument is not nil.
func ParseFile(file string) (*ast.File, *token.FileSet, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, err
	}

	return ParseSource(data)
}

// ParseSource parses the given source and returns the corresponding ast.File node and its file set for positional information.
// If a fatal error is encountered the error return argument is not nil.
func ParseSource(data interface{}) (*ast.File, *token.FileSet, error) {
	fset := token.NewFileSet()

	src, err := parser.ParseFile(fset, "", data, parser.ParseComments|parser.AllErrors)
	if err != nil {
		return nil, nil, err
	}

	return src, fset, err
}

// ParseAndTypeCheckFile parses and type-checks the given file, and returns everything interesting about the file.
// If a fatal error is encountered the error return argument is not nil.
func ParseAndTypeCheckFile(file string, collectors []filter.NodeCollector) (*ast.File, *token.FileSet, *types.Package, *types.Info, error) {
	fileAbs, err := filepath.Abs(file)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("Could not absolute the file path of %q: %v", file, err)
	}
	pkgCacheMu.Lock()
	prepared, ok := preparedFiles[fileAbs]
	pkgCacheMu.Unlock()
	if ok {
		applyCollectors(collectors, prepared.file, prepared.fset, fileAbs)
		return prepared.file, prepared.fset, prepared.pkg.Types, prepared.pkg.TypesInfo, nil
	}
	entry := loadPkgForDir(filepath.Dir(fileAbs))
	if entry.err != nil {
		return nil, nil, nil, nil, fmt.Errorf("Could not load package of file %q: %v", file, entry.err)
	}

	if pkg, f := fileInLoadedPkg(entry, fileAbs); f != nil {
		applyCollectors(collectors, f, entry.fset, fileAbs)
		return f, entry.fset, pkg.Types, pkg.TypesInfo, nil
	}

	// The file was not found in the loaded package syntax (e.g., excluded by
	// //go:build constraints in testdata fixtures). Fall back to direct parsing
	// and standalone type-checking, bypassing build constraints.
	src, typPkg, typInfo, err := parseAndTypeCheckDirect(entry.fset, fileAbs)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	if src != nil {
		applyCollectors(collectors, src, entry.fset, fileAbs)
	}

	return src, entry.fset, typPkg, typInfo, nil
}

// fileInLoadedPkg returns the loaded package and parsed syntax for fileAbs, or
// nil when the file is not part of the loaded package (e.g. excluded by build
// constraints).
func fileInLoadedPkg(entry *pkgCacheEntry, fileAbs string) (*packages.Package, *ast.File) {
	if len(entry.pkgs) == 0 {
		return nil, nil
	}
	pkg := entry.pkgs[0]
	for _, f := range pkg.Syntax {
		if entry.fset.Position(f.Pos()).Filename == fileAbs {
			return pkg, f
		}
	}
	return nil, nil
}

// applyCollectors runs every node collector over the given file.
func applyCollectors(collectors []filter.NodeCollector, f *ast.File, fset *token.FileSet, fileAbs string) {
	for _, c := range collectors {
		c.Collect(f, fset, fileAbs)
	}
}

// parseAndTypeCheckDirect parses a file directly (ignoring build constraints)
// and type-checks it as a standalone unit. Used as a fallback for files that
// are excluded from their package by build tags.
func parseAndTypeCheckDirect(fset *token.FileSet, fileAbs string) (*ast.File, *types.Package, *types.Info, error) {
	src, err := parser.ParseFile(fset, fileAbs, nil, parser.AllErrors|parser.ParseComments)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("Could not parse file %q: %v", fileAbs, err)
	}

	info := &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Implicits:  make(map[ast.Node]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
		Scopes:     make(map[ast.Node]*types.Scope),
	}

	conf := &types.Config{
		Importer: importer.Default(),
		Error:    func(error) {}, // tolerate errors in isolated test fixtures
	}

	pkg, _ := conf.Check("", fset, []*ast.File{src}, info)

	return src, pkg, info, nil
}
