#!/bin/bash

# Test the mutated package without changing the original source file.

if [ -z ${MUTATE_CHANGED+x} ]; then echo "MUTATE_CHANGED is not set"; exit 1; fi
if [ -z ${MUTATE_ORIGINAL+x} ]; then echo "MUTATE_ORIGINAL is not set"; exit 1; fi
if [ -z ${MUTATE_PACKAGE+x} ]; then echo "MUTATE_PACKAGE is not set"; exit 1; fi

script_path=${BASH_SOURCE[0]}
script_dir=${script_path%/*}
if [ "$script_dir" = "$script_path" ]; then script_dir=.; fi
if [ -z "$script_dir" ]; then script_dir=/; fi
source "$script_dir/overlay.sh" || exit 3

GOMUTESTING_DIFF=$(diff -u -- "$MUTATE_ORIGINAL" "$MUTATE_CHANGED")
DIFF_RESULT=$?
export GOMUTESTING_DIFF
if [ "$DIFF_RESULT" -gt 1 ]; then exit 3; fi
prepare_overlay || exit 3

export MUTATE_TIMEOUT=${MUTATE_TIMEOUT:-10}

if [ -n "$TEST_RECURSIVE" ]; then
	TEST_RECURSIVE="/..."
fi

GOMUTESTING_TEST=$(go test -timeout "${MUTATE_TIMEOUT}s" -overlay <(write_overlay) "$MUTATE_PACKAGE$TEST_RECURSIVE" 2>&1)
export GOMUTESTING_RESULT=$?

if [ "$MUTATE_DEBUG" = true ] ; then
	echo "$GOMUTESTING_TEST"
fi

if [ "$GOMUTESTING_RESULT" -eq 1 ] && [[ "$GOMUTESTING_TEST" == "go: "* && "$GOMUTESTING_TEST" != *$'\nFAIL'* ]]; then
	printf '%s\n' "$GOMUTESTING_TEST" >&2
	exit 3
fi

# `go test` exits 1 for a build failure exactly as it does for a failing test,
# so the exit code alone cannot tell a killed mutant from one that never
# compiled. Detect the build failure in the output and report SKIP.
if [ $GOMUTESTING_RESULT -eq 1 ] && echo "$GOMUTESTING_TEST" | grep -q '\[build failed\]\|\[setup failed\]' ; then
	GOMUTESTING_RESULT=2
fi

case $GOMUTESTING_RESULT in
0) # tests passed -> ESCAPED
	echo "$GOMUTESTING_DIFF"

	exit 1
	;;
1) # tests failed -> KILLED
	if [ "$MUTATE_DEBUG" = true ] ; then
		echo "$GOMUTESTING_DIFF"
	fi

	exit 0
	;;
2) # did not compile -> SKIP
	if [ "$MUTATE_VERBOSE" = true ] ; then
		echo "Mutation did not compile"
	fi

	if [ "$MUTATE_DEBUG" = true ] ; then
		echo "$GOMUTESTING_DIFF"
	fi

	exit 2
	;;
*) # Unkown exit code -> SKIP
	echo "Unknown exit code"
	echo "$GOMUTESTING_DIFF"

	exit $GOMUTESTING_RESULT
	;;
esac
