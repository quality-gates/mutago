#!/usr/bin/env bash
set -euo pipefail
root=$(git rev-parse --show-toplevel)
skill="$root/.agents/skills/verify-cache-publication"
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
for name, source, probe, needle, replacement in [
 ('parser', 'parse.go', 'cache_probe_test.go', 'if existing, ok := c.packages[dir]; ok {', 'if existing, ok := c.packages[dir]; ok && false {'),
 ('annotation', 'block.go', 'annotation_probe_test.go', 'for _, infos := range blockExclusions.statements[node] {', 'for _, infos := range brokenStatementsAtPosition(node) {'),
]:
 src = root / 'internal' / name / source
 content = src.read_text()
 if content.count(needle) != 1: raise SystemExit('Cannot construct sensitivity mutation for ' + name)
 broken = tmp / (name + '.go')
 broken_content = content.replace(needle,replacement)
 if name == 'annotation':
  broken_content += '\nfunc brokenStatementsAtPosition(node ast.Stmt) map[*Processor][]mutatorInfo {\n for stmt, owners := range blockExclusions.statements { if stmt.Pos() == node.Pos() { return owners } }; return nil\n}\n'
  owner_broken = tmp / 'annotation-owner.go'
  owner_broken.write_text(content.replace('delete(owners, p)', 'clear(owners)'))
 broken.write_text(broken_content)
 overlay = {str(root / 'internal' / name / 'verification_overlay_test.go'): str(skill / probe)}
 (tmp / (name + '.json')).write_text(json.dumps({'Replace': overlay}))
 overlay[str(src)] = str(broken)
 (tmp / (name + '-broken.json')).write_text(json.dumps({'Replace': overlay}))
 if name == 'annotation':
  overlay[str(src)] = str(owner_broken)
  (tmp / 'annotation-owner-broken.json').write_text(json.dumps({'Replace': overlay}))
PY
cd "$root"
for package in parser annotation; do
 GOMAXPROCS=1 go test -count=1 -overlay "$tmp/$package.json" -run '^TestVerification' "./internal/$package"
 if GOMAXPROCS=1 go test -count=1 -overlay "$tmp/$package-broken.json" -run '^TestVerification' "./internal/$package" >"$tmp/$package.log" 2>&1; then
  echo "ERROR: broken $package passed" >&2; exit 1
 fi
 if ! grep -Eq 'first publisher replaced|cross-file annotation interfered' "$tmp/$package.log"; then
  cat "$tmp/$package.log" >&2; echo "ERROR: unrelated failure in $package" >&2; exit 1
 fi
 echo "Detected deliberately broken $package publication"
 GOMAXPROCS=1 go test -count=1 -overlay "$tmp/$package.json" -run '^TestVerification' "./internal/$package"
done

if GOMAXPROCS=1 go test -count=1 -overlay "$tmp/annotation-owner-broken.json" -run '^TestVerificationAnnotationOwners$' ./internal/annotation >"$tmp/owner.log" 2>&1; then
 echo 'ERROR: broken owner cleanup passed' >&2; exit 1
fi
if ! grep -q "closing one owner removed another owner's exclusion" "$tmp/owner.log"; then
 cat "$tmp/owner.log" >&2; echo 'ERROR: unrelated owner cleanup failure' >&2; exit 1
fi
echo 'Detected deliberately broken owner cleanup'
if GOMAXPROCS=1 go test -count=1 -overlay "$tmp/annotation-broken.json" -run '^TestStatementMutationUnaffectedByOtherFileAnnotation$' ./mutator/statement >"$tmp/statement.log" 2>&1; then
 echo 'ERROR: broken statement isolation passed' >&2; exit 1
fi
if ! grep -q 'unannotated increment produced 0 mutations' "$tmp/statement.log"; then
 cat "$tmp/statement.log" >&2; echo 'ERROR: unrelated statement isolation failure' >&2; exit 1
fi
echo 'Detected cross-file interference in the real statement mutator'
