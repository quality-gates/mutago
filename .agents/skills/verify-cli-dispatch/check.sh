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
