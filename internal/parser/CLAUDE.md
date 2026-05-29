# internal/parser

## Package cache

`parse.go` keeps a package-level `pkgCache` that stores `packages.Load` results by directory. This is a performance optimization — one load per directory instead of one per file.

**Integration tests must call `parser.ClearPackageCache()` before each run.** The mutation runner's exec script writes mutations directly to the original file on disk. If the cache isn't cleared, the next call returns the old (pre-mutation) AST while `os.ReadFile` returns the mutated bytes — the two diverge silently.

## Build-constrained files

If `packages.Load` fails (e.g. the file has a build tag that excludes it from the current platform), `ParseAndTypeCheckFile` falls back to parsing directly with `go/types.Config`. This is normal — don't treat it as an error.
