#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
skill="$root/.agents/skills/verify-run-shutdown"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
jar="${XDG_CACHE_HOME:-$HOME/.cache}/mutago-formal-verification/tla2tools-1.7.4.jar"
mkdir -p "$(dirname "$jar")"
if [ ! -f "$jar" ]; then
  curl -fsSL https://github.com/tlaplus/tlaplus/releases/download/v1.7.4/tla2tools.jar -o "$tmp/tla2tools.jar"
  mv "$tmp/tla2tools.jar" "$jar"
fi
[ "$(shasum -a 256 "$jar" | cut -d ' ' -f 1)" = 936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88 ] || { echo 'TLC jar checksum mismatch' >&2; exit 1; }
cp "$skill/Shutdown.tla" "$skill/Shutdown.cfg" "$tmp/"
(cd "$tmp" && java -XX:+UseParallelGC -Xmx256m -cp "$jar" tlc2.TLC -workers 1 Shutdown.tla)
python3 - "$root" "$skill" "$tmp" <<'PY'
import json, pathlib, sys
root, skill, tmp = map(pathlib.Path, sys.argv[1:])
p = root / 'internal/engine/engine.go'
text = p.read_text()
needle = 'setup.closed.Do(func() {'
if text.count(needle) != 1: raise SystemExit('Cannot construct shutdown sensitivity mutation')
text = text.replace(needle, 'runShutdownEveryTime(func() {')
text += '\nfunc runShutdownEveryTime(f func()) { f() }\n'
(tmp / 'broken.go').write_text(text)
overlay = {str(root / 'internal/engine/verification_overlay_test.go'): str(skill / 'probe_test.go')}
(tmp / 'normal.json').write_text(json.dumps({'Replace': overlay}))
overlay[str(p)] = str(tmp / 'broken.go')
(tmp / 'broken.json').write_text(json.dumps({'Replace': overlay}))
PY
cd "$root"
GOMAXPROCS=1 go test -count=1 -timeout 30s -race -overlay "$tmp/normal.json" -run '^TestShutdownVerification$' ./internal/engine
if GOMAXPROCS=1 go test -count=1 -timeout 30s -overlay "$tmp/broken.json" -run '^TestShutdownVerification$' ./internal/engine >"$tmp/broken.log" 2>&1; then
  echo 'ERROR: repeated shutdown passed' >&2; exit 1
fi
if ! grep -Eq 'close of closed channel|summary count' "$tmp/broken.log"; then
  cat "$tmp/broken.log" >&2; echo 'ERROR: unrelated failure' >&2; exit 1
fi
echo 'Detected deliberately non-idempotent shutdown'
GOMAXPROCS=1 go test -count=1 -timeout 30s -race -overlay "$tmp/normal.json" -run '^TestShutdownVerification$' ./internal/engine
