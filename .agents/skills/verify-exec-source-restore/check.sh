#!/usr/bin/env bash
# Every stop of the shipped custom exec scripts, at every file step.
# Exit 0: MUTATE_ORIGINAL matched the bytes from before the script.
# Exit 1: a stop left different bytes. The FAIL lines are the counterexamples.
# Exit 2: a schedule no longer matches the code. That is a harness gap.
set -u
set -o pipefail

root=$(cd "$(dirname "$0")/../../.." && pwd)
cd "$root"

real_diff=$(command -v diff)
real_mv=$(command -v mv)
real_cp=$(command -v cp)
real_sleep=$(command -v sleep)

scripts=(
	scripts/exec/test-mutated-package.sh
	scripts/exec/test-current-directory.sh
)
points=(after_diff after_save after_copy during_test after_restore)
signals=(none TERM KILL)

anchors=(
	'diff -u $MUTATE_ORIGINAL $MUTATE_CHANGED'
	'mv $MUTATE_ORIGINAL $MUTATE_ORIGINAL.tmp'
	'cp $MUTATE_CHANGED $MUTATE_ORIGINAL'
	'go test'
	'mv $MUTATE_ORIGINAL.tmp $MUTATE_ORIGINAL'
	'trap sig_handler SIGHUP SIGINT SIGTERM'
)

fails=0
models=0
oks=0

say() { printf '%s\n' "$*"; }

require_anchors() {
	local script=$1 anchor
	for anchor in "${anchors[@]}"; do
		if ! grep -F -q -- "$anchor" "$script"; then
			say "MODEL $script is missing the step: $anchor"
			say "The schedules pause on diff, mv, cp, and go test. Update the harness from the script, then rerun."
			models=$((models + 1))
		fi
	done
}

require_engine() {
	local body
	body=$(awk '
		/^func runCustomExec\(/ { inside = 1 }
		inside { print }
		inside && /^func / && !/^func runCustomExec\(/ { exit }
	' internal/engine/engine.go)
	if [[ -z "$body" ]]; then
		say "MODEL internal/engine/engine.go has no runCustomExec"
		say "The cancel path moved. Point the harness at the function that starts the custom exec, then rerun."
		models=$((models + 1))
		return
	fi
	if ! grep -q 'exec.CommandContext' <<<"$body"; then
		say "MODEL runCustomExec no longer starts the script with exec.CommandContext"
		say "The cancel path moved. Point the harness at the new start, then rerun."
		models=$((models + 1))
	fi
	if grep -E -q '\.Cancel[[:space:]]*=' <<<"$body"; then
		say "MODEL runCustomExec sets Command.Cancel"
		say "The KILL schedules are CommandContext's default os.Process.Kill. A custom Cancel is a different stop."
		models=$((models + 1))
	fi
	local runner
	runner=$(awk '
		/^func resolveRunner\(/ { inside = 1 }
		inside { print }
		inside && /^func / && !/^func resolveRunner\(/ { exit }
	' internal/engine/runner.go)
	if ! grep -q 'r.workers = 1' <<<"$runner"; then
		say "MODEL resolveRunner no longer sets workers to 1 for a custom command"
		say "These schedules run one script. Overlapping mv, cp, and clean_up are outside this check."
		models=$((models + 1))
	fi
}

# Prints the Wait error from exec.CommandContext's default cancel.
probe_command_context() {
	local dir
	dir=$(mktemp -d)
	cat >"$dir/probe.go" <<'EOF'
package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"
)

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(ctx, "sleep", "30")
	if err := cmd.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "start: %v\n", err)
		os.Exit(2)
	}
	time.Sleep(150 * time.Millisecond)
	cancel()
	err := cmd.Wait()
	if err == nil {
		fmt.Println("exit 0")
		return
	}
	fmt.Println(err.Error())
}
EOF
	(
		cd "$dir"
		go mod init probe >/dev/null 2>&1
		go run .
	)
	local status=$?
	rm -rf "$dir"
	return "$status"
}

file_state() {
	local work=$1
	if [[ ! -f "$work/foo.go" ]]; then
		printf 'missing'
	elif cmp -s "$work/foo.go" "$work/original.bytes"; then
		printf 'original'
	elif cmp -s "$work/foo.go" "$work/mutant.go"; then
		printf 'mutant'
	else
		printf 'other'
	fi
}

state_words() {
	case "$1" in
	missing) printf 'MUTATE_ORIGINAL is gone and MUTATE_ORIGINAL.tmp holds the bytes from before the script' ;;
	mutant) printf 'MUTATE_ORIGINAL holds the mutant' ;;
	other) printf 'MUTATE_ORIGINAL holds unexpected bytes' ;;
	original) printf 'MUTATE_ORIGINAL holds the bytes from before the script' ;;
	*) printf 'MUTATE_ORIGINAL is in an unknown state' ;;
	esac
}

steps_until() {
	local point=$1
	local order=(after_diff after_save after_copy during_test after_restore)
	local text=(
		'diff read MUTATE_ORIGINAL and MUTATE_CHANGED.'
		'mv renamed MUTATE_ORIGINAL to MUTATE_ORIGINAL.tmp.'
		'cp copied MUTATE_CHANGED onto MUTATE_ORIGINAL.'
		'go test was running.'
		'clean_up moved MUTATE_ORIGINAL.tmp back onto MUTATE_ORIGINAL.'
	)
	local i
	for i in "${!order[@]}"; do
		printf '  %s\n' "${text[$i]}"
		if [[ "${order[$i]}" == "$point" ]]; then
			return
		fi
	done
}

stop_words() {
	local signal=$1
	case "$signal" in
	none) printf 'The script ran on to its exit.\n' ;;
	TERM) printf 'SIGTERM reached the script. sig_handler calls clean_up for SIGHUP, SIGINT, and SIGTERM.\n' ;;
	KILL)
		printf '%s\n' \
			'SIGKILL reached the script process only, the same stop as runCustomExec: exec.CommandContext calls os.Process.Kill, and Kill sends SIGKILL to that process.' \
			'The trap names SIGHUP, SIGINT, and SIGTERM, so clean_up did not run.'
		;;
	esac
}

write_wrappers() {
	local bin=$1
	mkdir -p "$bin"
	cat >"$bin/diff" <<'EOF'
#!/bin/bash
"$MUTAGO_VFY_REAL_DIFF" "$@"
status=$?
if [[ "${MUTAGO_VFY_PAUSE:-}" == after_diff ]]; then
	printf '%s\n' "$PPID" >"$MUTAGO_VFY_DIR/script.pid"
	touch "$MUTAGO_VFY_DIR/ready"
	trap 'exit 0' TERM INT HUP
	while [[ ! -f "$MUTAGO_VFY_DIR/continue" ]]; do
		"$MUTAGO_VFY_REAL_SLEEP" 0.05
	done
fi
exit "$status"
EOF
	cat >"$bin/mv" <<'EOF'
#!/bin/bash
src=${1:-}
dst=${2:-}
"$MUTAGO_VFY_REAL_MV" "$@"
status=$?
point=${MUTAGO_VFY_PAUSE:-}
hit=0
if [[ "$point" == after_save && "$dst" == *.tmp && "$src" != *.tmp ]]; then
	hit=1
fi
if [[ "$point" == after_restore && "$src" == *.tmp ]]; then
	hit=1
fi
if [[ "$hit" -eq 1 ]]; then
	printf '%s\n' "$PPID" >"$MUTAGO_VFY_DIR/script.pid"
	touch "$MUTAGO_VFY_DIR/ready"
	trap 'exit 0' TERM INT HUP
	while [[ ! -f "$MUTAGO_VFY_DIR/continue" ]]; do
		"$MUTAGO_VFY_REAL_SLEEP" 0.05
	done
fi
exit "$status"
EOF
	cat >"$bin/cp" <<'EOF'
#!/bin/bash
"$MUTAGO_VFY_REAL_CP" "$@"
status=$?
if [[ "${MUTAGO_VFY_PAUSE:-}" == after_copy ]]; then
	printf '%s\n' "$PPID" >"$MUTAGO_VFY_DIR/script.pid"
	touch "$MUTAGO_VFY_DIR/ready"
	trap 'exit 0' TERM INT HUP
	while [[ ! -f "$MUTAGO_VFY_DIR/continue" ]]; do
		"$MUTAGO_VFY_REAL_SLEEP" 0.05
	done
fi
exit "$status"
EOF
	cat >"$bin/go" <<'EOF'
#!/bin/bash
if [[ "${1:-}" != test ]]; then
	printf 'go wrapper saw unexpected args: %s\n' "$*" >>"$MUTAGO_VFY_DIR/model"
	exit 99
fi
if [[ "${MUTAGO_VFY_PAUSE:-}" == during_test ]]; then
	printf '%s\n' "$PPID" >"$MUTAGO_VFY_DIR/script.pid"
	touch "$MUTAGO_VFY_DIR/ready"
	trap 'exit 0' TERM INT HUP
	while [[ ! -f "$MUTAGO_VFY_DIR/continue" ]]; do
		"$MUTAGO_VFY_REAL_SLEEP" 0.05
	done
fi
exit 0
EOF
	chmod +x "$bin/diff" "$bin/mv" "$bin/cp" "$bin/go"
}

prepare_work() {
	local work=$1
	cat >"$work/foo.go" <<'EOF'
package fixture

func Foo() int { return 1 }
EOF
	cp "$work/foo.go" "$work/original.bytes"
	cat >"$work/mutant.go" <<'EOF'
package fixture

func Foo() int { return 2 }
EOF
	write_wrappers "$work/bin"
}

run_schedule() {
	local script=$1 point=$2 signal=$3
	local work
	work=$(mktemp -d)
	prepare_work "$work"
	# The driver shell owns the script job. diff and go test run inside
	# command substitutions, so a wrapper's parent is not the script that
	# installed the trap. Monitor mode makes the script its own process
	# group: SIGKILL goes to that process only, as os.Process.Kill does,
	# and SIGTERM goes to the group, as a terminal stop does.
	(
		set -m
		cd "$work" || exit 99
		export PATH="$work/bin:${PATH}"
		export MUTAGO_VFY_DIR="$work"
		export MUTAGO_VFY_PAUSE="$point"
		export MUTAGO_VFY_REAL_DIFF="$real_diff"
		export MUTAGO_VFY_REAL_MV="$real_mv"
		export MUTAGO_VFY_REAL_CP="$real_cp"
		export MUTAGO_VFY_REAL_SLEEP="$real_sleep"
		export MUTATE_ORIGINAL="foo.go"
		export MUTATE_CHANGED="$work/mutant.go"
		export MUTATE_PACKAGE="example.com/fixture"
		export MUTATE_TIMEOUT=1
		export MUTATE_DEBUG=false
		export MUTATE_VERBOSE=false
		unset TEST_RECURSIVE
		bash "$root/$script" >"$work/script.out" 2>&1 &
		spid=$!
		ready=0
		alive=0
		i=0
		for i in $(seq 1 50); do
			if [[ -f "$work/ready" ]]; then
				ready=1
				break
			fi
			if ! kill -0 "$spid" 2>/dev/null; then
				break
			fi
			sleep 0.05
		done
		if [[ "$ready" -eq 1 ]]; then
			case "$signal" in
			none) touch "$work/continue" ;;
			TERM) kill -TERM -"$spid" 2>/dev/null || true ;;
			KILL) kill -KILL "$spid" 2>/dev/null || true ;;
			esac
		fi
		for i in $(seq 1 50); do
			if ! kill -0 "$spid" 2>/dev/null; then
				break
			fi
			sleep 0.05
		done
		if kill -0 "$spid" 2>/dev/null; then
			alive=1
		fi
		# Observe before reaping. Process.Kill leaves the script's children running.
		state=$(file_state "$work")
		kill -KILL -"$spid" 2>/dev/null || true
		wait "$spid" 2>/dev/null || true
		printf '%s %s %s\n' "$ready" "$alive" "$state" >"$work/result"
	) 2>"$work/driver.err"
	if [[ ! -f "$work/result" ]]; then
		say "MODEL $script $point $signal produced no result"
		say "  The driver failed before it could observe MUTATE_ORIGINAL. Read the harness before treating this as a bad file."
		models=$((models + 1))
		rm -rf "$work"
		return
	fi
	local ready alive state
	read -r ready alive state <"$work/result"
	rm -rf "$work"
	classify "$script" "$point" "$signal" "$ready" "$alive" "$state"
}

classify() {
	local script=$1 point=$2 signal=$3 ready=$4 alive=$5 state=$6
	if [[ "$alive" -eq 1 && "$signal" == none ]]; then
		say "MODEL $script $point $signal was still running when the wait ended"
		say "  The schedule did not reach a stop. The harness no longer matches the script."
		models=$((models + 1))
		return
	fi
	if [[ "$ready" -eq 0 && "$state" == original ]]; then
		say "MODEL $script $point $signal never reached $point, and the file was already the original"
		say "  This pause no longer matches a command in the script."
		models=$((models + 1))
		return
	fi
	if [[ "$state" == original && "$alive" -eq 0 ]]; then
		say "ok $script $point $signal"
		oks=$((oks + 1))
		return
	fi
	fails=$((fails + 1))
	say "FAIL $script $point $signal"
	if [[ "$ready" -eq 1 ]]; then
		steps_until "$point"
		stop_words "$signal" | sed 's/^/  /'
	else
		say "  The script stopped before $point."
	fi
	say "  $(state_words "$state")."
}

say "verify-exec-source-restore"
say "repo $root"

for script in "${scripts[@]}"; do
	require_anchors "$script"
done
require_engine

if [[ "$models" -gt 0 ]]; then
	say "$models harness gap(s). Schedules not run."
	exit 2
fi

cancel_msg=$(probe_command_context)
cancel_status=$?
if [[ "$cancel_status" -ne 0 || "$cancel_msg" != *"killed"* ]]; then
	say "FAIL exec.CommandContext cancel: ${cancel_msg:-no output}"
	say "  runCustomExec relies on CommandContext's default Cancel, which calls os.Process.Kill."
	say "  The KILL schedules are that stop. A different Wait error means the engine's stop changed."
	fails=$((fails + 1))
else
	say "ok exec.CommandContext cancel: $cancel_msg"
	oks=$((oks + 1))
fi

for script in "${scripts[@]}"; do
	for point in "${points[@]}"; do
		for signal in "${signals[@]}"; do
			run_schedule "$script" "$point" "$signal"
		done
	done
done

say "$oks restored, $fails broke the property, $models harness gap(s)"
if [[ "$fails" -gt 0 ]]; then
	exit 1
fi
if [[ "$models" -gt 0 ]]; then
	exit 2
fi
exit 0
