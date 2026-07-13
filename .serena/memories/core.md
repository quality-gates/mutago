# mutago

- Go mutation-testing CLI; module `github.com/quality-gates/mutago/v2`, entrypoint `cmd/mutago`.
- Main areas: `mutator/` implementations; `internal/engine/` orchestration; `internal/models/` reports/stats/quality gates; `internal/parser/` package loading and AST cache; `internal/filter/` and `internal/annotation/` filtering; `internal/coverage/`; `internal/gitdiff/`; `internal/reportmaker/`.
- `AGENTS.md` is a symlink to `CLAUDE.md`; project workflow and invariants live there.
- Build stack and pinned versions: `mem:tech_stack`.
- Common local commands: `mem:suggested_commands`.
- Code and test conventions: `mem:conventions`.
- Required completion and shipping checks: `mem:task_completion`.