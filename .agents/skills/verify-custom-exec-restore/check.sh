#!/usr/bin/env bash
set -euo pipefail

skill_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(cd "$skill_dir/../../.." && pwd)
scratch=$(mktemp -d "${TMPDIR:-/tmp}/mutago-custom-exec-verify.XXXXXX")
trap 'rm -rf "$scratch"' EXIT

expected_sha=936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88
if [[ -n ${MUTAGO_TLC_JAR:-} ]]; then
  jar=$MUTAGO_TLC_JAR
else
  cache_dir=${XDG_CACHE_HOME:-"$HOME/.cache"}/mutago-formal-verification
  jar=$cache_dir/tla2tools-1.7.4.jar
  mkdir -p "$cache_dir"
  if [[ ! -f $jar ]]; then
    curl -fLsS --retry 2 -o "$scratch/tla2tools.jar" \
      https://github.com/tlaplus/tlaplus/releases/download/v1.7.4/tla2tools.jar
    mv "$scratch/tla2tools.jar" "$jar"
  fi
fi
actual_sha=$(python3 - "$jar" <<'PY'
import hashlib, pathlib, sys
print(hashlib.sha256(pathlib.Path(sys.argv[1]).read_bytes()).hexdigest())
PY
)
if [[ $actual_sha != "$expected_sha" ]]; then
  echo "TLC jar checksum mismatch: $jar" >&2
  exit 2
fi

cp "$skill_dir/CustomExec.tla" "$skill_dir/CustomExec.cfg" "$skill_dir/normal.cfg" "$scratch/"
python3 - "$repo_root" "$skill_dir/probe_test.go" "$scratch/overlay.json" <<'PY'
import json, pathlib, sys
root, probe, output = map(pathlib.Path, sys.argv[1:])
target = root / "internal/engine/scope_test.go"
if not target.is_file():
    sys.exit(f"Overlay target is missing: {target}; choose another engine test file")
output.write_text(json.dumps({"Replace": {str(target): str(probe)}}))
PY

failed=0
echo "Checking ordinary completion in the model"
if (cd "$scratch" && java -XX:+UseParallelGC -Xmx256m -cp "$jar" tlc2.TLC \
    -workers 1 -metadir "$scratch/normal-states" -config normal.cfg CustomExec.tla) >"$scratch/normal.log" 2>&1; then
  echo "PASS: ordinary completion restores the source in the model"
else
  cat "$scratch/normal.log"
  failed=1
fi

echo "Checking cancellation and timeout in the model"
if (cd "$scratch" && java -XX:+UseParallelGC -Xmx256m -cp "$jar" tlc2.TLC \
    -workers 1 -metadir "$scratch/cancel-states" -config CustomExec.cfg CustomExec.tla) >"$scratch/cancel.log" 2>&1; then
  echo "PASS: the model restores the source on every explored path"
else
  cat "$scratch/cancel.log"
  failed=1
fi

echo "Checking the real custom executor and both bundled scripts"
if (cd "$repo_root" && MUTAGO_VERIFY_ROOT="$repo_root" GOMAXPROCS=1 \
    go test -count=1 -overlay "$scratch/overlay.json" \
      -run '^TestCustomExecRestoresSource$' -v ./internal/engine); then
  echo "PASS: the real scripts restored the source in every probe"
else
  failed=1
fi
exit "$failed"
