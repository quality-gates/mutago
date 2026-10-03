# Exploratory testing pass — mutago CLI, 2026-10-03

Scope: three CLI journeys not covered by the [2026-09-26 pass](../2026-09-26-cli-afk/README.md):
the baseline workflow for brownfield code, triage of escaped mutants (agentic
JSON, `--run-mutant-id`, annotations), and running through a config file and a
custom `--exec` script.

## Build and setup

- Built `mutago dev` from [`59702f7`](https://github.com/quality-gates/mutago/commit/59702f7), the `origin/main` head used for this pass.
- Go 1.26.6 on macOS arm64. See [setup](evidence/00-setup.txt).
- Used four disposable Go modules under `/tmp`. Each has its own copy under [`evidence/`](evidence/):
  - [`fixture-shop`](evidence/fixture-shop/): weakly tested pricing code, used for the baseline and config journeys.
  - [`fixture-ann`](evidence/fixture-ann/): annotations.
  - [`fixture-ship`](evidence/fixture-ship/): the minimal `ignore_source_lines` reproducer.
  - [`fixture-embed`](evidence/fixture-embed/): `//go:embed`.
- Every mutation run used `GOMAXPROCS=1`, `--workers=1`, `--exec-timeout=10`, and a disposable `GOCACHE`.

## Confirmed bugs

| Issue | Summary |
| :---- | :------ |
| [#269](https://github.com/quality-gates/mutago/issues/269) | `ignore_source_lines` does not suppress `statement/return` or `statement/remove` mutants |
| [#270](https://github.com/quality-gates/mutago/issues/270) | Unknown mutator names and invalid `ignore_source_lines` regexes are ignored with no warning; the README annotation example uses the non-existent name `increment` |

### #269 — `ignore_source_lines` misses statement mutators

**Impact:** a team uses `ignore_source_lines` to skip boilerplate, but `statement/return` and `statement/remove` still mutate those lines. Those mutants count toward MSI and the gates. `docs/config.md` says a matching line "is skipped entirely".

**Replay:** in [`fixture-ship`](evidence/fixture-ship/), run `mutago --config ignore.yml --workers=1 --exec-timeout=10 --no-diffs ./ship`. The config has `ignore_source_lines: ['return 5']`.

**Expected:** no mutant on `ship.go:7` (`return 5`).

**Actual:** the `numbers/*` mutants on line 7 are skipped, but `KILLED ship/ship.go:7 (statement/return)` still runs. Two runs gave the same result: [run 1](evidence/21-ignore-run-1.txt), [run 2](evidence/21-ignore-run-2.txt). Compare the [run without config](evidence/21-no-config-run.txt). A `// mutator-disable-next-line *` on the same line does skip it: [annotation comparison](evidence/22-annotation-comparison.txt). `statement/remove` behaves the same way: [dry-run counts](evidence/23-statement-remove-dry-run.txt).

The first sighting was in the config journey ([config run](evidence/20-config-run.txt), [report](evidence/20-report.json): a `statement/return` mutant on `price.go:19`).

### #270 — misconfiguration is silent

**Impact:** a user copies the README annotation example, or makes a typo in a mutator name or regex. The mutants they meant to skip still run, and nothing tells them.

**Replay:**

1. [`fixture-ann/ann.go`](evidence/fixture-ann/ann.go) uses the README form `// mutator-disable-next-line branch/if, increment`. The `numbers/incrementer` mutant on the next line still runs. Seen in the [first run](evidence/12-annotation-run.txt) and a [clean replay](evidence/16-annotation-replay.txt). If you use the full name `numbers/incrementer`, the dry-run count drops from 5 to 4.
2. `--disable bogus` exits 0 with no warning: [output](evidence/13-disable-unknown.txt).
3. `enable_mutators: [branch/iff]` produces `0 total` mutants with no hint why: [run](evidence/14-enable-typo-run.txt), [config](evidence/14-typo-config.yml). The `min_msi` gate in that config still fails safely with exit 4.
4. An invalid regex in `ignore_source_lines` (`return (5`) exits 0 with no warning: [output](evidence/24-bad-regex.txt).

**Expected:** an error or a warning. `docs/config.md` already treats unknown config keys as an error.

## Journeys

### 1. Baseline for brownfield code

**Goal:** accept today's survivors, then fail only on new escapes. Expected: `--update-baseline` records the survivors and exits 0. Gating against the baseline passes, still passes after lines shift, and fails (exit 4) only when a new survivor appears.

- [First run](evidence/01-first-run.txt): 33 mutants, 17 escaped, exit 0.
- [`--update-baseline`](evidence/02-update-baseline.txt) wrote [17 unique IDs](evidence/02-mutago-baseline.json) and exited 0.
- [`--fail-on-escaped --baseline`](evidence/03-baseline-gate.txt) exited 0.
- Added three comment lines above the package clause. The gate [still passed](evidence/04-baseline-shifted.txt), so IDs survived the line shift.
- Added `Tax` with a weak test ([patch](evidence/05-new-code.patch)). The gate [returned exit 4](evidence/05-baseline-new-escape.txt) with "6 new mutant(s) escaped".
- After strengthening the `Clamp` test, [`--update-baseline`](evidence/08-update-after-kill.txt) rewrote the file with 20 survivors (17 − 3 killed + 6 new).

Variations:

- With [a missing baseline path](evidence/06-missing-baseline.txt), mutago treats the baseline as empty and reports all 23 survivors as new (exit 4). This fails safe.
- With [an output directory that does not exist](evidence/07-update-custom-path.txt), mutago exits 3, but only after running every mutant.

### 2. Escaped-mutant triage

**Goal:** pick a survivor from the agentic JSON, replay just that mutant, then suppress the equivalent mutants. Expected: `--run-mutant-id` replays the same mutant after code edits, and annotations skip the named mutators.

- [Agentic JSON](evidence/01-agentic.json) entries have `id`, `diff`, `kill_hint`, and test files.
- After three lines were inserted above it, [`--run-mutant-id`](evidence/09-run-mutant-id.txt) still found the survivor. It printed only that result and exited 0.
- An [unknown ID](evidence/10-run-mutant-id-unknown.txt) gave a clear message and exit 3.
- Annotations: `branch/if` was skipped on the next line ([run](evidence/12-annotation-run.txt)), but `increment` was not (#270).

### 3. Config file and custom `--exec`

**Goal:** drive a run from YAML and from the shipped exec script. Expected: the config fields take effect, and the script's totals match the built-in runner's.

- [Config](evidence/20-config.yml) settings:
  - `exclude_dirs: [gen]` excluded `gen/`.
  - `skip_without_test` skipped `notest.go`.
  - `json_output` wrote [`report.json`](evidence/20-report.json).
  - `silent_mode` printed only the summary.
  - `min_msi: 40` passed at 52.38%.
  - `ignore_source_lines` failed for statement mutators (#269).
- [`scripts/exec/test-mutated-package.sh`](evidence/18-exec-script.txt) gave the same totals as the built-in runner (20 total, 5 killed, 15 escaped, 25%). The fixture's source hash was unchanged afterwards.
- [`//go:embed`](evidence/19-embed-run.txt) survived mutation. The embed test was never broken by unrelated mutants.

## Rejected and unresolved candidates

- **Rejected — comment loss breaks directives.** The `statement/return` mutant in [the exec run](evidence/18-exec-script.txt) drops a comment line inside the function. A `//go:embed` directive at file scope was kept ([run](evidence/19-embed-run.txt)), and the dropped comment is in the mutant copy only.
- **Unresolved — one KILLED/ESCAPED flip.** In a single run, `ann.go:14 (expression/comparison)` (`x > 0` → `x >= 0`, tested only with `-1`) reported KILLED. Six repeat runs, and three replays of the edit-then-run sequence, all reported ESCAPED. The likeliest cause is a timeout under host load, but the summary line for that run was not kept.

## Usability observations

- With `--baseline`, the terminal lists every escaped mutant and says only "N new mutant(s) escaped". It does not say which ones are new. *Suggestion:* mark new escapes, or list them in the gate message.
- `--update-baseline` into a directory that does not exist fails only after the whole run. *Suggestion:* check the path is writable before running mutants.
- `--no-diffs` does not hide diffs that the shipped exec script prints itself ([output](evidence/18-exec-script.txt)).
- The `branch/if` and `statement/remove` diffs show the replacement body without its indentation (`+\t_ = pct` / `+}`). See [run-mutant-id output](evidence/09-run-mutant-id.txt). This is cosmetic.

## Unexplored areas and limits

Not covered:

- `--logger-github` and `--logger-gitlab`
- `--timeout-coefficient`
- `--test-recursive` with baselines
- live TTY progress
- signal interruption
- `mutator-disable-func` and `mutator-disable-regexp`

The fixtures are small, single-package modules. No product source was changed.

## Cleanup

The disposable modules, binary, and `GOCACHE` under `/tmp/mutago-et` were removed after the evidence was copied here.
