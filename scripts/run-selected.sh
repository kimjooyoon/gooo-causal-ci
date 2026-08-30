#!/usr/bin/env bash
set -euo pipefail

if test "$#" -ne 2; then
	printf 'usage: run-selected.sh PLAN OUTPUT\n' >&2
	exit 64
fi

plan=$1
output=$2
observations='[]'
while IFS=$'\t' read -r id action command; do
	case "$action" in
	EXECUTE)
		eval "$command"
		status=EXECUTED
		;;
	REUSE)
		status=REUSED
		;;
	SKIP)
		status=SKIPPED
		;;
	*)
		printf 'unexpected plan action %s for %s\n' "$action" "$id" >&2
		exit 65
		;;
	esac
	observations=$(jq -c --arg id "$id" --arg status "$status" '. + [{id:$id,status:$status}]' <<<"$observations")
done < <(jq -r '.tests[] | [.id,.action,(.command|@sh // "")] | @tsv' "$plan")

printf '%s\n' "$observations" > "$output"
