# Conventions

- Follow idiomatic Go and preserve existing package structure; production changes use test-first development.
- Quality-gate exit code is `4`; tool errors use other nonzero codes.
- CLI quality flags default to `-1` as the sentinel for config-or-skip; config zero means no gate.
- Do not assert hardcoded mutation counts. Assert summary behavior, exit codes, stat/collection consistency, process labels, and MSI range.
- Gate-failure tests use impossible thresholds such as `--min-msi 101`.
- Integration tests invoke the mutator against `example/example.go` and may leave it modified; always restore it after tests.
- User-facing flags, mutators, or behavior require matching README/docs updates.
- `main` is protected; changes land through PRs, with no force-push or commit amendment after publishing.