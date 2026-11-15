#!/usr/bin/env bash
set -euo pipefail
skill=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$skill/../../.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/mutago-diff-verify.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
python3 - "$root" "$skill" "$tmp" <<'PY'
import json, pathlib, sys
root, skill, tmp = map(pathlib.Path, sys.argv[1:])
cases = [
    ('unified', 'internal/parser/diff.go', 'internal/parser/diff_test.go',
     'if s.firstChange == 0 && action.changedLine > 0 {',
     'if action.changedLine > 0 {'),
    ('git', 'internal/gitdiff/gitdiff.go', 'internal/gitdiff/gitdiff_test.go',
     'if count == 0 {', 'if count < 0 {'),
]
for name, source, test, old, new in cases:
    source_path = root / source
    source_text = source_path.read_text()
    if source_text.count(old) != 1:
        sys.exit(f'{name}: cannot apply sensitivity mutation; inspect the source')
    broken = tmp / (name + '.go')
    broken.write_text(source_text.replace(old, new))
    overlay = {str(root / test): str(skill / (name + '_probe_test.go'))}
    (tmp / (name + '-normal.json')).write_text(json.dumps({'Replace': overlay}))
    (tmp / (name + '-broken.json')).write_text(json.dumps({'Replace': {**overlay, str(source_path): str(broken)}}))
PY
cd "$root"
for spec in 'unified internal/parser TestVerifyUnifiedDiff' 'git internal/gitdiff TestVerifyChangedLines'; do
  read -r name package test <<< "$spec"
  echo "Checking $name parser"
  GOFLAGS=-buildvcs=false GOMAXPROCS=1 go test -count=1 -overlay "$tmp/$name-normal.json" -run "^$test\$" "./$package"
  if GOFLAGS=-buildvcs=false GOMAXPROCS=1 go test -count=1 -overlay "$tmp/$name-broken.json" -run "^$test\$" "./$package" >"$tmp/$name-broken.log" 2>&1; then
    echo "FAIL: $name probe did not detect the broken parser" >&2
    exit 1
  fi
  if ! grep -q -- "--- FAIL: $test" "$tmp/$name-broken.log"; then
    cat "$tmp/$name-broken.log" >&2
    echo "FAIL: $name sensitivity check failed outside the probe" >&2
    exit 1
  fi
  echo "PASS: $name probe detected the broken parser"
  GOFLAGS=-buildvcs=false GOMAXPROCS=1 go test -count=1 -overlay "$tmp/$name-normal.json" -run "^$test\$" "./$package"
done
