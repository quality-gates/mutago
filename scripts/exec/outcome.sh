finish_test_run() {
	local outcome=$GOMUTESTING_RESULT
	if [ "$outcome" -eq 1 ] && [[ "$GOMUTESTING_TEST" == "go: "* && "$GOMUTESTING_TEST" != *$'\nFAIL'* ]]; then
		printf '%s\n' "$GOMUTESTING_TEST" >&2
		exit 3
	fi
	if [ "$outcome" -eq 1 ] && echo "$GOMUTESTING_TEST" | grep -q '\[build failed\]\|\[setup failed\]'; then
		outcome=2
	fi
	case "$outcome" in
	0)
		echo "$GOMUTESTING_DIFF"
		exit 1
		;;
	1)
		if [ "$MUTATE_DEBUG" = true ]; then echo "$GOMUTESTING_DIFF"; fi
		exit 0
		;;
	2)
		if [ "$MUTATE_VERBOSE" = true ]; then echo "Mutation did not compile"; fi
		if [ "$MUTATE_DEBUG" = true ]; then echo "$GOMUTESTING_DIFF"; fi
		exit 2
		;;
	*)
		echo "Unknown exit code"
		echo "$GOMUTESTING_DIFF"
		exit "$outcome"
		;;
	esac
}
