# Suggested Commands

- Build CLI: `go build ./cmd/mutago`; whole tree: `go build ./...`.
- Test all packages: `go test ./...`; afterward restore integration fixture with `git restore example/example.go`.
- Run CLI smoke test: `go build -o /tmp/mutago ./cmd/mutago` then `/tmp/mutago <real-package>`.
- Enable committed CI-mirroring hooks once per clone: `git config core.hooksPath githooks`.
- Fast whole-tree checks: `githooks/pre-commit`.
- Diff-scoped mutation check is run by `githooks/pre-push` when pushing an updated ref.
- Inspect work: `git status --short --branch`, `git diff`, `git log --oneline -10`.
- PR checks: `gh pr checks <number>`.