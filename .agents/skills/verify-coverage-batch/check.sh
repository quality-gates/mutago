#!/usr/bin/env bash
set -euo pipefail
skill=$(cd "$(dirname "$0")" && pwd)
root=$(cd "$skill/../../.." && pwd)
tmp=$(mktemp -d "${TMPDIR:-/tmp}/mutago-coverage-verify.XXXXXX")
trap 'rm -rf "$tmp"' EXIT
jar=${MUTAGO_TLC_JAR:-${XDG_CACHE_HOME:-$HOME/.cache}/mutago-formal-verification/tla2tools-1.7.4.jar}
if [[ ! -f $jar ]]; then
  mkdir -p "$(dirname "$jar")"
  curl -fLsS --retry 2 -o "$tmp/tla2tools.jar" https://github.com/tlaplus/tlaplus/releases/download/v1.7.4/tla2tools.jar
  mv "$tmp/tla2tools.jar" "$jar"
fi
python3 - "$jar" <<'PY'
import hashlib, pathlib, sys
expected = '936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88'
if hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest() != expected:
    sys.exit('TLC jar checksum mismatch')
PY
cp "$skill/CoverageBatch.tla" "$skill/CoverageBatch.cfg" "$tmp/"
(cd "$tmp" && java -XX:+UseParallelGC -Xmx256m -cp "$jar" tlc2.TLC -workers 1 -config CoverageBatch.cfg CoverageBatch.tla)
python3 - "$tmp/CoverageBatch.tla" <<'PY'
import pathlib, sys
path = pathlib.Path(sys.argv[1])
model = path.read_text()
old = 'IF failures = {} THEN "complete" ELSE "error"'
if model.count(old) != 1:
    sys.exit('Cannot construct the model sensitivity mutation')
path.write_text(model.replace(old, '"complete"'))
PY
if (cd "$tmp" && java -XX:+UseParallelGC -Xmx256m -cp "$jar" tlc2.TLC -workers 1 -config CoverageBatch.cfg CoverageBatch.tla) >"$tmp/broken-model.log" 2>&1; then
  echo 'Sensitivity check failed: broken model passed' >&2
  exit 1
fi
if ! grep -q 'Invariant CompleteOnlyWithAllProfiles is violated' "$tmp/broken-model.log"; then
  cat "$tmp/broken-model.log" >&2
  echo 'Model sensitivity check failed for an unrelated reason' >&2
  exit 1
fi
echo 'PASS: incomplete-profile model mutation was detected'
python3 - "$root" "$skill" "$tmp" <<'PY'
import json, pathlib, sys
root, skill, tmp = map(pathlib.Path, sys.argv[1:])
source = root / 'internal/coverage/coverage.go'
original = source.read_text()
old = 'if len(errs) > 0 {'
if original.count(old) != 1:
    sys.exit('Cannot construct the sensitivity mutation; inspect the coverage merger')
(tmp / 'broken.go').write_text(original.replace(old, 'if false && len(errs) > 0 {'))
test = root / 'internal/coverage/fuzz_test.go'
base = {str(test): str(skill / 'probe_test.go')}
(tmp / 'normal.json').write_text(json.dumps({'Replace': base}))
(tmp / 'broken.json').write_text(json.dumps({'Replace': {**base, str(source): str(tmp / 'broken.go')}}))
PY
cd "$root"
GOMAXPROCS=1 go test -count=1 -overlay "$tmp/normal.json" -run '^TestVerifyCoverageBatch$' ./internal/coverage
if GOMAXPROCS=1 go test -count=1 -overlay "$tmp/broken.json" -run '^TestVerifyCoverageBatch$' ./internal/coverage >"$tmp/broken.log" 2>&1; then
  echo 'Sensitivity check failed: broken merger passed' >&2
  exit 1
fi
if ! grep -q 'accepted partial profile' "$tmp/broken.log"; then
  cat "$tmp/broken.log" >&2
  echo 'Sensitivity check failed for an unrelated reason' >&2
  exit 1
fi
echo 'PASS: incomplete-profile mutation was detected'
