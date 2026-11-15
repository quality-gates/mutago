#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
skill="$root/.agents/skills/verify-script-outcomes"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
cp -R "$root/scripts/exec" "$tmp/scripts"
"$skill/probe.sh" "$tmp/scripts"
python3 - "$tmp/scripts/overlay.sh" <<'PY'
import pathlib, sys
p = pathlib.Path(sys.argv[1])
s = p.read_text()
needle = '\t\toutcome=2\n'
if s.count(needle) != 1: raise SystemExit('Cannot construct build-failure sensitivity mutation')
p.write_text(s.replace(needle, '\t\toutcome=1\n'))
PY
if "$skill/probe.sh" "$tmp/scripts" >"$tmp/broken.log" 2>&1; then
  echo 'ERROR: broken build outcome passed' >&2; exit 1
fi
if ! grep -q 'build_failure: expected exit 2, got 0' "$tmp/broken.log"; then
  cat "$tmp/broken.log" >&2; echo 'ERROR: unrelated sensitivity failure' >&2; exit 1
fi
echo 'Detected broken build-failure classification'
"$skill/probe.sh" "$root/scripts/exec"
