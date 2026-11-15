#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
skill="$root/.agents/skills/verify-cache-clear"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
jar="${MUTAGO_TLC_JAR:-${XDG_CACHE_HOME:-$HOME/.cache}/mutago-formal-verification/tla2tools-1.7.4.jar}"
if [ ! -f "$jar" ]; then
 mkdir -p "$(dirname "$jar")"
 curl -fsSL https://github.com/tlaplus/tlaplus/releases/download/v1.7.4/tla2tools.jar -o "$tmp/tla2tools.jar"
 mv "$tmp/tla2tools.jar" "$jar"
fi
[ "$(shasum -a 256 "$jar" | cut -d ' ' -f 1)" = 936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88 ] || { echo 'TLC jar checksum mismatch' >&2; exit 1; }
cp "$skill/Cache.tla" "$skill/Cache.cfg" "$tmp/"
(cd "$tmp" && java -XX:+UseParallelGC -Xmx256m -cp "$jar" tlc2.TLC -workers 1 Cache.tla)
python3 - "$root" "$skill" "$tmp" <<'PY'
import json, pathlib, sys
root, skill, tmp = map(pathlib.Path, sys.argv[1:])
p = root / 'internal/parser/parse.go'
s = p.read_text()
needle = 'if version != c.version {'
if s.count(needle) != 2: raise SystemExit('Cannot construct stale-publication mutation')
(tmp / 'broken.go').write_text(s.replace(needle, 'if false {'))
overlay = {str(root / 'internal/parser/verification_overlay_test.go'): str(skill / 'probe_test.go')}
(tmp / 'normal.json').write_text(json.dumps({'Replace': overlay}))
overlay[str(p)] = str(tmp / 'broken.go')
(tmp / 'broken.json').write_text(json.dumps({'Replace': overlay}))
PY
cd "$root"
GOMAXPROCS=1 go test -count=1 -race -overlay "$tmp/normal.json" -run '^TestVerification' ./internal/parser
if GOMAXPROCS=1 go test -count=1 -overlay "$tmp/broken.json" -run '^TestVerificationClearDiscardsInFlightLoads$' ./internal/parser >"$tmp/broken.log" 2>&1; then
 echo 'ERROR: stale publication passed' >&2; exit 1
fi
grep -q 'stale load published after clear' "$tmp/broken.log" || { cat "$tmp/broken.log" >&2; echo 'ERROR: unrelated failure' >&2; exit 1; }
echo 'Detected deliberately stale publication'
GOMAXPROCS=1 go test -count=1 -race -overlay "$tmp/normal.json" -run '^TestVerification' ./internal/parser
