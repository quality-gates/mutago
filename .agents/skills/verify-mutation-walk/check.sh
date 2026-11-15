#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
skill="$root/.agents/skills/verify-mutation-walk"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
jar="${XDG_CACHE_HOME:-$HOME/.cache}/mutago-formal-verification/tla2tools-1.7.4.jar"
sha=936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88
mkdir -p "$(dirname "$jar")"
if [ ! -f "$jar" ]; then
  curl -fsSL https://github.com/tlaplus/tlaplus/releases/download/v1.7.4/tla2tools.jar -o "$tmp/tla2tools.jar"
  mv "$tmp/tla2tools.jar" "$jar"
fi
actual=$(shasum -a 256 "$jar" | cut -d ' ' -f 1)
[ "$actual" = "$sha" ] || { echo "TLC jar checksum mismatch" >&2; exit 1; }
cp "$skill/Walk.tla" "$skill/Walk.cfg" "$tmp/"
(cd "$tmp" && java -cp "$jar" tlc2.TLC -workers 1 -cleanup Walk.tla)
python3 - "$root" "$skill" "$tmp" <<'PY'
import json, pathlib, sys
root, skill, tmp = map(pathlib.Path, sys.argv[1:])
base = {str(root / 'walk_test.go'): str(skill / 'probe_test.go')}
(tmp / 'overlay.json').write_text(json.dumps({'Replace': base}))
source = (root / 'walk.go').read_text()
needle = 'defer m.Reset()'
if source.count(needle) != 1:
    raise SystemExit('expected exactly one deferred reset; update the sensitivity check')
(tmp / 'broken.go').write_text(source.replace(needle, 'defer func() {}()', 1))
base[str(root / 'walk.go')] = str(tmp / 'broken.go')
(tmp / 'broken-overlay.json').write_text(json.dumps({'Replace': base}))
needle = 'h.phase = waitingForReset'
if source.count(needle) != 1:
    raise SystemExit('expected exactly one applied-to-reset transition')
(tmp / 'broken-phase.go').write_text(source.replace(needle, 'h.phase = waitingForApplied', 1))
base[str(root / 'walk.go')] = str(tmp / 'broken-phase.go')
(tmp / 'broken-phase-overlay.json').write_text(json.dumps({'Replace': base}))
PY
cd "$root"
go test -count=1 -timeout 30s -overlay "$tmp/overlay.json" -run '^TestWalk(Restoration|Handshake)Probe$' .
if go test -count=1 -timeout 30s -overlay "$tmp/broken-overlay.json" -run '^TestWalkRestorationProbe$' . >"$tmp/broken.log" 2>&1; then
  echo 'ERROR: missing-reset mutation passed' >&2; exit 1
fi
if ! grep -q 'source not restored' "$tmp/broken.log"; then
  cat "$tmp/broken.log" >&2
  echo 'ERROR: broken implementation failed for an unrelated reason' >&2; exit 1
fi
echo 'Missing-reset implementation failed the restoration probe as expected.'
if go test -count=1 -timeout 30s -overlay "$tmp/broken-phase-overlay.json" -run '^TestWalkHandshakeProbe$' . >"$tmp/broken-phase.log" 2>&1; then
  echo 'ERROR: wrong handshake phase passed' >&2; exit 1
fi
if ! grep -q 'mutation handshake: reset out of order' "$tmp/broken-phase.log"; then
  cat "$tmp/broken-phase.log" >&2
  echo 'ERROR: wrong-phase implementation failed for an unrelated reason' >&2; exit 1
fi
echo 'Wrong-phase implementation failed the handshake probe as expected.'
go test -count=1 -timeout 30s -overlay "$tmp/overlay.json" -run '^TestWalk(Restoration|Handshake)Probe$' .
