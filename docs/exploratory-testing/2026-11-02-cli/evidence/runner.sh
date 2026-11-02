#!/bin/sh
printf '%s\n' "$MUTATE_TIMEOUT" >> runner-calls.txt
exec "$RUNNER_SCRIPT"
