#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
skill="$root/.agents/skills/verify-finalization-indexes"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
jar="${XDG_CACHE_HOME:-$HOME/.cache}/mutago-formal-verification/tla2tools-1.7.4.jar"
mkdir -p "$(dirname "$jar")"
if [ ! -f "$jar" ]; then curl -fsSL https://github.com/tlaplus/tlaplus/releases/download/v1.7.4/tla2tools.jar -o "$tmp/jar"; mv "$tmp/jar" "$jar"; fi
[ "$(shasum -a 256 "$jar" | cut -d ' ' -f 1)" = 936a262061c914694dfd669a543be24573c45d5aa0ff20a8b96b23d01e050e88 ] || { echo 'TLC jar checksum mismatch' >&2; exit 1; }
cp "$skill/Finalize.tla" "$skill/Finalize.cfg" "$tmp/"
(cd "$tmp" && java -XX:+UseParallelGC -Xmx256m -cp "$jar" tlc2.TLC -workers 1 Finalize.tla)
python3 - "$root" "$tmp" <<'PY'
import json, pathlib, sys
root, tmp = map(pathlib.Path, sys.argv[1:])
for name, source, old, new in [
 ('finalize', 'internal/engine/engine.go', 'opts.Baseline.Update && report.Stats.ErrorCount == 0', 'opts.Baseline.Update'),
 ('generate', 'internal/engine/mode.go', 's.writeErr = err', 's.writeErr = nil'),
 ('index', 'astutil/query.go', 'idx.ensure(info, stmts, start, end)', ''),
 ('path', 'internal/coverage/coverage.go', 'if cached, ok := r.resolved.Load(absFile); ok {', 'if cached, ok := r.resolved.Load(absFile); false && ok {'),
]:
 p=root/source; text=p.read_text()
 if text.count(old)!=1: raise SystemExit('cannot create '+name+' mutation')
 broken=tmp/(name+'.go'); broken.write_text(text.replace(old,new))
 (tmp/(name+'.json')).write_text(json.dumps({'Replace':{str(p):str(broken)}}))
PY
cd "$root"
for item in 'finalize internal/engine TestFinalizationPrecedence' 'generate internal/engine TestGenerateRemembersFailureAfterSuccessfulWrite' 'index astutil TestIncrementalIdentifierIndexMatchesFreshAfterClear' 'path internal/coverage TestPathResolutionRepeatedHitsAndMisses'; do
  read -r name package testname <<< "$item"
  GOMAXPROCS=1 GOFLAGS=-buildvcs=false go test -count=1 -run "^${testname}$" "./$package"
  if GOMAXPROCS=1 GOFLAGS=-buildvcs=false go test -count=1 -overlay "$tmp/$name.json" -run "^${testname}$" "./$package" >"$tmp/$name.log" 2>&1; then echo "broken $name passed" >&2; exit 1; fi
  case "$name" in
    finalize) expected='exit =';;
    generate) expected='failure was forgotten';;
    index) expected='expected identifier uses';;
    path) expected='cached miss changed';;
  esac
  if ! grep -q "$expected" "$tmp/$name.log"; then cat "$tmp/$name.log" >&2; echo "unrelated $name failure" >&2; exit 1; fi
  echo "Detected broken $name"
  GOMAXPROCS=1 GOFLAGS=-buildvcs=false go test -count=1 -run "^${testname}$" "./$package"
done
