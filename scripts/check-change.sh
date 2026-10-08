#!/usr/bin/env bash
set -euo pipefail

# Local and PR checks use changed lines. Main CI passes --full.
base="${1:-origin/main}"
set --
if [[ "$base" != --full ]]; then
	git rev-parse --verify "$base^{commit}" >/dev/null
	set -- --git-diff-lines --git-diff-base "$base" --ignore-msi-with-no-mutations
fi

temporary_directory="$(mktemp -d)"
trap 'rm -rf "$temporary_directory"' EXIT
export GOCACHE="${GOCACHE:-$temporary_directory/cache}"
binary="${MUTAGO_BIN:-$temporary_directory/mutago}"
if [[ -z "${MUTAGO_BIN:-}" ]]; then
	go build -o "$binary" ./cmd/mutago
fi
# Keep existing reports intact.
printf 'json_output: false\n' > "$temporary_directory/config.yml"

# Targets have isolated unit tests. CLI tests recurse; importing and parser
# depend on package-loading test infrastructure.
"$binary" \
	--config "$temporary_directory/config.yml" \
	--workers "${MUTAGO_WORKERS:-1}" \
	--exec-timeout 30 \
	--coverage \
	"$@" \
	--min-msi 75 \
	--min-covered-msi 80 \
	github.com/quality-gates/mutago/v2/mutator/arithmetic \
	github.com/quality-gates/mutago/v2/mutator/branch \
	github.com/quality-gates/mutago/v2/mutator/composite \
	github.com/quality-gates/mutago/v2/mutator/concurrency \
	github.com/quality-gates/mutago/v2/mutator/conditional \
	github.com/quality-gates/mutago/v2/mutator/expression \
	github.com/quality-gates/mutago/v2/mutator/loop \
	github.com/quality-gates/mutago/v2/mutator/numbers \
	github.com/quality-gates/mutago/v2/mutator/select \
	github.com/quality-gates/mutago/v2/mutator/statement \
	github.com/quality-gates/mutago/v2/internal/filter \
	github.com/quality-gates/mutago/v2/internal/coverage \
	github.com/quality-gates/mutago/v2/internal/gitdiff \
	github.com/quality-gates/mutago/v2/internal/models
