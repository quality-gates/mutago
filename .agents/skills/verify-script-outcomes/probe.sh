#!/usr/bin/env bash
set -euo pipefail
scripts=$1
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkdir "$tmp/bin" "$tmp/work"
printf 'original\n' > "$tmp/work/original.go"
printf 'changed\n' > "$tmp/mutant.go"
cat > "$tmp/bin/go" <<'GO'
#!/usr/bin/env bash
printf '%s' "$FAKE_OUTPUT"
exit "$FAKE_STATUS"
GO
chmod +x "$tmp/bin/go"
for script in test-mutated-package.sh test-current-directory.sh; do
  for case_name in pass test_failure build_failure setup_failure driver_failure test_diagnostic unknown; do
    case "$case_name" in
      pass) status=0; output='ok'; expected=1 ;;
      test_failure) status=1; output='FAIL test'; expected=0 ;;
      build_failure) status=1; output='FAIL pkg [build failed]'; expected=2 ;;
      setup_failure) status=1; output='FAIL pkg [setup failed]'; expected=2 ;;
      driver_failure) status=1; output='go: cannot read overlay'; expected=3 ;;
      test_diagnostic) status=1; output=$'go: test diagnostic\nFAIL test'; expected=0 ;;
      unknown) status=7; output='unknown error'; expected=7 ;;
    esac
    set +e
    (cd "$tmp/work" && PATH="$tmp/bin:$PATH" FAKE_STATUS="$status" FAKE_OUTPUT="$output" MUTATE_ORIGINAL=original.go MUTATE_CHANGED="$tmp/mutant.go" MUTATE_PACKAGE=example.test bash "$scripts/$script" >"$tmp/out" 2>"$tmp/err")
    got=$?
    set -e
    if [ "$got" != "$expected" ]; then
      echo "$script $case_name: expected exit $expected, got $got" >&2
      exit 1
    fi
  done
  set +e
  (cd "$tmp/work" && PATH="$tmp/bin:$PATH" MUTATE_ORIGINAL=missing.go MUTATE_CHANGED="$tmp/mutant.go" MUTATE_PACKAGE=example.test bash "$scripts/$script" >"$tmp/out" 2>"$tmp/err")
  got=$?
  set -e
  if [ "$got" != 3 ]; then echo "$script missing diff input: expected exit 3, got $got" >&2; exit 1; fi
done
echo 'PASS: both script outcome protocols'
