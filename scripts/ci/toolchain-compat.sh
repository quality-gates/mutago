#!/bin/sh
# Runs mutago on a package that imports the standard library, using whatever Go
# toolchain is active (set GOTOOLCHAIN to pick one). Fails when the target does
# not load or when mutators that need types produce no mutants (#332).
set -eu

ROOT_DIR=$(cd "$(dirname "$0")/../.." && pwd)
WORK=$(mktemp -d "${TMPDIR:-/tmp}/toolchain-compat.XXXXXX")
trap 'rm -rf "$WORK"' EXIT

go version
(cd "$ROOT_DIR" && go build -o "$WORK/mutago" ./cmd/mutago)
cp -R "$ROOT_DIR/scripts/ci/testdata/stdlibprobe" "$WORK/probe"

cd "$WORK/probe"
if ! "$WORK/mutago" --workers=1 --exec-timeout=60 --no-diffs ./... >"$WORK/out" 2>&1; then
	cat "$WORK/out"
	echo "FAIL: mutago did not complete on a stdlib-importing package"
	exit 1
fi
cat "$WORK/out"

for mutator in statement/return expression/error-guard; do
	if ! grep -Eq "^ +$mutator +[a-z]+ +[0-9]+ / [1-9]" "$WORK/out"; then
		echo "FAIL: no $mutator mutants; type information is missing"
		exit 1
	fi
done
echo "OK: type-dependent mutators produced mutants"
