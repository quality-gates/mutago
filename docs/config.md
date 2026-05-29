# Configuration File

mutago can be configured via a YAML file. Pass it with `--config <path>`. No default config file is loaded automatically.

A JSON Schema for editor validation and auto-completion is at [`schema/config-schema.json`](../schema/config-schema.json). Enable it in VS Code and other YAML-aware editors with a header comment:

```yaml
# yaml-language-server: $schema=https://raw.githubusercontent.com/quality-gates/mutago/master/schema/config-schema.json
skip_without_test: true
min_msi: 70
```

## Schema

```yaml
skip_without_test: true
skip_with_build_tags: true
json_output: false
html_output: false
silent_mode: false
min_msi: 0
min_covered_msi: 0
exclude_dirs: []
disable_mutators: []
enable_mutators: []
ignore_source_lines: []
```

## Fields

| Field | Type | Default | Description |
| :---- | :--- | :------ | :---------- |
| `skip_without_test` | bool | `true` | Skip source files that have no corresponding `_test.go` file |
| `skip_with_build_tags` | bool | `true` | Skip test files that have build tags |
| `json_output` | bool | `false` | Write a full mutation report to `report.json` |
| `html_output` | bool | `false` | Write an HTML report to `mutago-report.html` |
| `silent_mode` | bool | `false` | Suppress per-mutation output (summary only) |
| `min_msi` | float | `0` | Minimum overall MSI (0–100); `0` disables the gate |
| `min_covered_msi` | float | `0` | Minimum covered-code MSI (0–100); `0` disables the gate |
| `exclude_dirs` | []string | `[]` | Directory path prefixes to exclude from mutation |
| `disable_mutators` | []string | `[]` | Mutator names to disable. Merged with `--disable` CLI flags (union). Supports trailing-`*` wildcard, e.g. `arithmetic/*`. Run `mutago --list-mutators` for all names. |
| `enable_mutators` | []string | `[]` | Allowlist: if non-empty, only matching mutators run. Supports trailing-`*` wildcard. `--disable` can still exclude entries from this list. |
| `ignore_source_lines` | []string | `[]` | List of regexes. Any source line matching one of these patterns is skipped entirely. Useful for suppressing mutations on generated code or boilerplate. |

## Notes

- CLI flags override config file values.
- Unknown keys in the config file cause an error (strict parsing).
- `exclude_dirs` values are matched as path *prefixes*, not globs. For example, `vendor` excludes any path starting with `vendor`.

## Mutator control

`enable_mutators` is an allowlist: when set, only the listed mutators run. `disable_mutators` is then applied on top as a denylist, so you can narrow an allowlist further. CLI `--disable` is always merged with `disable_mutators`.

Disable an entire category using a trailing `*`:

```yaml
disable_mutators:
  - arithmetic/*
  - numbers/float-negate
```

Run only a specific category:

```yaml
enable_mutators:
  - branch/*
```

Combine — enable a category but suppress one noisy member:

```yaml
enable_mutators:
  - expression/*
disable_mutators:
  - expression/context-nil
```

Run `mutago --list-mutators` to see all available names.

> **Shell quoting**: when passing wildcard patterns via `--disable` on the command line, always quote them to prevent your shell from expanding `*` as a filename glob: `--disable 'arithmetic/*'`. In config files, no quoting is needed.

## Example

```yaml
skip_without_test: true
min_msi: 70
min_covered_msi: 80
exclude_dirs:
  - vendor
  - testdata
```
