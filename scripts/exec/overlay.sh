prepare_overlay() {
	local directory name logical_directory physical_directory physical_pwd
	case "$MUTATE_ORIGINAL" in
	*/*)
		directory=${MUTATE_ORIGINAL%/*}
		name=${MUTATE_ORIGINAL##*/}
		[ -n "$directory" ] || directory=/
		;;
	*)
		directory=.
		name=$MUTATE_ORIGINAL
		;;
	esac
	logical_directory=$(cd -L -- "$directory" && printf '%s.' "$PWD") || return 1
	logical_directory=${logical_directory%.}
	physical_directory=$(cd -P -- "$directory" && printf '%s.' "$PWD") || return 1
	physical_directory=${physical_directory%.}
	physical_pwd=$(cd -P . && printf '%s.' "$PWD") || return 1
	physical_pwd=${physical_pwd%.}

	OVERLAY_ORIGINAL="${physical_directory%/}/$name"
	OVERLAY_LOGICAL_ORIGINAL="${logical_directory%/}/$name"
	OVERLAY_CWD_ORIGINAL=$OVERLAY_ORIGINAL
	if [ "$physical_pwd" = / ]; then
		OVERLAY_CWD_ORIGINAL="${PWD%/}$OVERLAY_ORIGINAL"
	elif [[ "$OVERLAY_ORIGINAL" == "$physical_pwd/"* ]]; then
		OVERLAY_CWD_ORIGINAL="${PWD%/}/${OVERLAY_ORIGINAL#"$physical_pwd/"}"
	fi
}

json_string() {
	local LC_ALL=C value=$1 character code i
	printf '"'
	for ((i = 0; i < ${#value}; i++)); do
		character=${value:i:1}
		case "$character" in
		'"') printf '\\"' ;;
		\\) printf '\\\\' ;;
		*)
			printf -v code '%d' "'$character"
			if ((code < 32)); then
				printf '\\u%04x' "$code"
			else
				printf '%s' "$character"
			fi
			;;
		esac
	done
	printf '"'
}

write_overlay() {
	printf '{"Replace":{'
	json_string "$OVERLAY_ORIGINAL"
	printf ':'
	json_string "$MUTATE_CHANGED"
	if [ "$OVERLAY_LOGICAL_ORIGINAL" != "$OVERLAY_ORIGINAL" ]; then
		printf ','
		json_string "$OVERLAY_LOGICAL_ORIGINAL"
		printf ':'
		json_string "$MUTATE_CHANGED"
	fi
	if [ "$OVERLAY_CWD_ORIGINAL" != "$OVERLAY_ORIGINAL" ] && [ "$OVERLAY_CWD_ORIGINAL" != "$OVERLAY_LOGICAL_ORIGINAL" ]; then
		printf ','
		json_string "$OVERLAY_CWD_ORIGINAL"
		printf ':'
		json_string "$MUTATE_CHANGED"
	fi
	printf '}}'
}
