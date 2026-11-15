#!/bin/bash

# Test the current directory without changing the original source file.

if [ -z ${MUTATE_CHANGED+x} ]; then echo "MUTATE_CHANGED is not set"; exit 1; fi
if [ -z ${MUTATE_ORIGINAL+x} ]; then echo "MUTATE_ORIGINAL is not set"; exit 1; fi
if [ -z ${MUTATE_PACKAGE+x} ]; then echo "MUTATE_PACKAGE is not set"; exit 1; fi

script_path=${BASH_SOURCE[0]}
script_dir=${script_path%/*}
if [ "$script_dir" = "$script_path" ]; then script_dir=.; fi
if [ -z "$script_dir" ]; then script_dir=/; fi
source "$script_dir/overlay.sh" || exit 3
source "$script_dir/outcome.sh" || exit 3

GOMUTESTING_DIFF=$(diff -u -- "$MUTATE_ORIGINAL" "$MUTATE_CHANGED")
DIFF_RESULT=$?
export GOMUTESTING_DIFF
if [ "$DIFF_RESULT" -gt 1 ]; then exit 3; fi
prepare_overlay || exit 3

export MUTATE_TIMEOUT=${MUTATE_TIMEOUT:-10}

if [ -n "$TEST_RECURSIVE" ]; then
	TEST_RECURSIVE="/..."
fi

GOMUTESTING_TEST=$(go test -timeout "${MUTATE_TIMEOUT}s" -overlay <(write_overlay) ".$TEST_RECURSIVE" 2>&1)
export GOMUTESTING_RESULT=$?

if [ "$MUTATE_DEBUG" = true ] ; then
	echo "$GOMUTESTING_TEST"
fi

finish_test_run
