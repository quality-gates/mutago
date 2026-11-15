#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
skill="$root/.agents/skills/verify-cli-dispatch"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
python3 - "$root" "$skill" "$tmp" <<'PY'
import json, pathlib, sys
root, skill, tmp = map(pathlib.Path, sys.argv[1:])
source = root / 'cmd/mutago/main.go'
code = source.read_text()
needle = 'if opts.General.Version {'
if code.count(needle) != 1: raise SystemExit('Cannot create version precedence mutation; review source')
(tmp / 'broken.go').write_text(code.replace(needle, 'if false && opts.General.Version {'))
probe = {str(root / 'cmd/mutago/verification_overlay_test.go'): str(skill / 'probe_test.go')}
(tmp / 'normal.json').write_text(json.dumps({'Replace': probe}))
probe[str(source)] = str(tmp / 'broken.go')
(tmp / 'broken.json').write_text(json.dumps({'Replace': probe}))
PY
cd "$root"
GOMAXPROCS=1 go test -count=1 -overlay "$tmp/normal.json" -run '^TestVerifyCLI(EarlyExitPrecedence|HandledBeforeErrors)$' ./cmd/mutago
if GOMAXPROCS=1 go test -count=1 -overlay "$tmp/broken.json" -run '^TestVerifyCLI(EarlyExitPrecedence|HandledBeforeErrors)$' ./cmd/mutago >"$tmp/broken.log" 2>&1; then
 echo 'ERROR: broken version precedence passed' >&2; exit 1
fi
if ! grep -q 'precedence bits=' "$tmp/broken.log"; then cat "$tmp/broken.log" >&2; echo 'ERROR: unrelated failure' >&2; exit 1; fi
echo 'PASS: disabled version branch was detected'
GOMAXPROCS=1 go test -count=1 -overlay "$tmp/normal.json" -run '^TestVerifyCLI(EarlyExitPrecedence|HandledBeforeErrors)$' ./cmd/mutago
jar="${XDG_CACHE_HOME:-$HOME/.cache}/mutago-formal-verification/tla2tools-1.7.4.jar"
mkdir -p "$(dirname "$jar")"
if [ ! -f "$jar" ]; then
 curl -fsSL https://github.com/tlaplus/tlaplus/releases/download/v1.7.4/tla2tools.jar -o "$tmp/tla2tools.jar"
 mv "$tmp/tla2tools.jar" "$jar"
fi
[ "$(shasum -a 256 "$jar" | cut -d ' ' -f 1)" = 936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88 ] || { echo 'TLC jar checksum mismatch' >&2; exit 1; }
cp "$skill/Interrupts.tla" "$skill/Interrupts.cfg" "$tmp/"
(cd "$tmp" && java -XX:+UseParallelGC -Xmx256m -cp "$jar" tlc2.TLC -workers 1 Interrupts.tla)
python3 - "$root" "$tmp" <<'PY'
import json, pathlib, sys
root, tmp = map(pathlib.Path, sys.argv[1:])
source = root / 'cmd/mutago/main.go'
code = source.read_text()
needle = 'case <-signals:\n\t\t\tsignal.Stop(signals)'
if code.count(needle) != 1: raise SystemExit('Cannot create signal sensitivity mutation; review source')
(tmp / 'broken-signal.go').write_text(code.replace(needle, 'case <-signals:\n\t\t\t_ = signals'))
(tmp / 'broken-signal.json').write_text(json.dumps({'Replace': {str(source): str(tmp / 'broken-signal.go')}}))
PY
GOMAXPROCS=1 go test -count=1 -run '^TestInterruptLifecycle$' ./cmd/mutago
if GOMAXPROCS=1 go test -count=1 -overlay "$tmp/broken-signal.json" -run '^TestInterruptLifecycle/interrupt$' ./cmd/mutago >"$tmp/broken-signal.log" 2>&1; then
 echo 'ERROR: broken signal interception passed' >&2; exit 1
fi
if ! grep -q 'second signal did not terminate process' "$tmp/broken-signal.log"; then cat "$tmp/broken-signal.log" >&2; echo 'ERROR: unrelated signal failure' >&2; exit 1; fi
echo 'PASS: missing signal stop was detected'
GOMAXPROCS=1 go test -count=1 -run '^TestInterruptLifecycle$' ./cmd/mutago
