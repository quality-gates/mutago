#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
skill="$root/.agents/skills/verify-preparation-fallback"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
python3 - "$root" "$skill" "$tmp" <<'PY'
import json, pathlib, sys
root, skill, tmp = map(pathlib.Path, sys.argv[1:])
p = root / 'internal/engine/engine.go'
s = p.read_text()
changes = {
 'cancel': ('if ctxErr := ctx.Err(); ctxErr != nil {\n\t\treturn nil, ctxErr\n\t}\n\tconsole.Verbose(opts, "Per-test coverage unavailable', 'if false {\n\t\treturn nil, ctx.Err()\n\t}\n\tconsole.Verbose(opts, "Per-test coverage unavailable'),
 'stable': ('exec "$src" "$@"\n`', 'exit 77\n`'),
}
base = {str(root / 'internal/engine/verification_fallback_test.go'): str(skill / 'probe_test.go')}
(tmp / 'normal.json').write_text(json.dumps({'Replace': base}))
for name, (old, new) in changes.items():
 if s.count(old) != 1: raise SystemExit('Cannot construct ' + name + ' mutation')
 broken = tmp / (name + '.go'); broken.write_text(s.replace(old, new))
 (tmp / (name + '.json')).write_text(json.dumps({'Replace': {**base, str(p): str(broken)}}))
PY
cd "$root"
GOMAXPROCS=1 go test -count=1 -timeout 60s -overlay "$tmp/normal.json" -run '^TestVerify(CoverageFallback|StableExecFallback)$' ./internal/engine
for variant in cancel stable; do
 if GOMAXPROCS=1 go test -count=1 -timeout 60s -overlay "$tmp/$variant.json" -run '^TestVerify(CoverageFallback|StableExecFallback)$' ./internal/engine >"$tmp/$variant.log" 2>&1; then
  echo "ERROR: broken $variant passed" >&2; exit 1
 fi
 case "$variant" in cancel) pattern='cancellation lost';; stable) pattern='fallback failed';; esac
 grep -q "$pattern" "$tmp/$variant.log" || { cat "$tmp/$variant.log" >&2; echo 'ERROR: unrelated failure' >&2; exit 1; }
 echo "Detected broken $variant fallback"
done
GOMAXPROCS=1 go test -count=1 -timeout 60s -overlay "$tmp/normal.json" -run '^TestVerify(CoverageFallback|StableExecFallback)$' ./internal/engine
