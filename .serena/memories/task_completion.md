# Task Completion

For production changes, follow `CLAUDE.md` in order:

1. Run `go build ./...` and `go test ./...`; then `git restore example/example.go`.
2. Build `/tmp/mutago` and run the full self-mutation command/package list in `CLAUDE.md`; both MSI gates must pass (exit `0`; exit `4` is a gate failure).
3. Manually run the built binary against a real package and inspect user-facing output.
4. Update README/docs when user-facing behavior changes.
5. Add a concise `CHANGELOG.md` entry under `[Unreleased]`.
6. Inspect status/diff/log, commit only intended files, push a branch, open a PR, and wait for all `gh pr checks <number>` checks to pass.
7. Merge to `main`, tag the next semver version, and create a succinct GitHub release.
8. Sync the corresponding change to `../../jonbaldie/go-mutesting` and follow that repository's own shipping workflow.