#!/usr/bin/env bash
set -euo pipefail

work=$1
sha=$2
toolchain=$3
want_cover=$4
want_json=$5
package_count=$6
shift 6
packages=("${@:1:$package_count}")
shift "$package_count"
[ "${#packages[@]}" -gt 0 ] || packages=(./...)

case $work in
"$HOME"/ci-runs/release-kit.??????) ;;
*)
	echo "target-suite: refusing workdir '$work': not a run directory under $HOME/ci-runs" >&2
	exit 3
	;;
esac

group=
scratch=

remove_scratch() {
	case $1 in /tmp/ci.??????) ;; *) return 0 ;; esac
	case ${1#/tmp/ci.} in *[!A-Za-z0-9]*) return 0 ;; esac
	chmod -R u+w -- "$1" 2>/dev/null || true
	rm -rf -- "$1"
}

end_scratch() {
	local status=0
	remove_scratch "$scratch" || status=$?
	rm -f "$work/scratch"
	return "$status"
}

stop_group() {
	if [ -n "$group" ]; then
		kill -TERM -- "-$group" 2>/dev/null || kill -TERM "$group" 2>/dev/null || true
		group_gone "$group" || true
	fi
	end_scratch || echo "target-suite: could not fully remove $scratch" >&2
	exit 130
}

trap stop_group HUP INT TERM

checkout() {
	git -c init.defaultBranch=main clone -q --no-checkout "$work/repo.bundle" "$work/src"
	git -C "$work/src" -c advice.detachedHead=false checkout -q "$sha"
}

record_covered_set() {
	awk 'FNR>1 && $NF>0 {print $1}' "$work/cover.out" | awk '$1 !~ /_test\//' | LC_ALL=C sort -u >"$work/covered.txt"
}

record_counts() {
	jq -r 'select(.Test != null and (.Action == "pass" or .Action == "fail" or .Action == "skip")) | "\(.Action) \(.Package) \(.Test)"' \
		"$work/test.json" | LC_ALL=C sort >"$work/results.txt"
	jq -r 'select(.Test == null and .Action == "fail") | .Package' "$work/test.json" | LC_ALL=C sort -u >"$work/failed-packages.txt"
	{
		for action in pass fail skip; do
			printf '%s tests %s cases %s\n' "$action" \
				"$(awk -v a="$action" '$1 == a && $3 !~ /\// {n++} END {print n+0}' "$work/results.txt")" \
				"$(awk -v a="$action" '$1 == a && $3 ~ /\// {n++} END {print n+0}' "$work/results.txt")"
		done
		grep -E '^(fail|skip) ' "$work/results.txt" || true
		printf 'fail packages %s\n' "$(grep -c . "$work/failed-packages.txt" || true)"
		sed 's/^/fail package /' "$work/failed-packages.txt"
	} >"$work/counts.txt"
	jq -r 'select(.Output != null and (.Output | startswith("-test.shuffle "))) | "\(.Package) \(.Output | ltrimstr("-test.shuffle ") | rtrimstr("\n"))"' \
		"$work/test.json" | LC_ALL=C sort -u >"$work/seeds.txt" || true
}

group_gone() {
	local i=0
	while [ "$i" -lt 100 ] && kill -0 -- "-$1" 2>/dev/null; do
		sleep 0.1
		i=$((i + 1))
	done
	! kill -0 -- "-$1" 2>/dev/null
}

reap_group() {
	local g=$1
	case $g in "" | *[!0-9]* | 0* | 1) return 0 ;; esac
	kill -s TERM -- "-$g" 2>/dev/null || return 0
	group_gone "$g" && return 0
	echo "target-suite: process group $g outlived TERM for 10s; sending KILL" >&2
	kill -s KILL -- "-$g" 2>/dev/null || return 0
	group_gone "$g" || echo "target-suite: process group $g still alive 10s after KILL" >&2
}

in_own_group() {
	local sink=$1 errs=$2
	shift 2
	(cd "$work/src" && export TMPDIR="$scratch" GOTMPDIR="$scratch" && exec setsid "$@") >"$sink" 2>"$errs" </dev/null &
	group=$!
	echo "$group" >"$work/pgid"
	local status=0
	wait "$group" || status=$?
	reap_group "$(cat "$work/pgid" 2>/dev/null)"
	rm -f "$work/pgid"
	group=
	return "$status"
}

run_go_test() {
	local args=(-count=1)
	if [ "$want_cover" = 1 ]; then
		args+=(-covermode=set -coverpkg=./... "-coverprofile=$work/cover.out")
	fi
	local sink=$work/test.out
	if [ "$want_json" = 1 ]; then
		args+=(-json)
		sink=$work/test.json
	fi
	local status=0
	in_own_group "$sink" "$work/test.err" go test "${args[@]}" "$@" "${packages[@]}" || status=$?
	if [ "$want_cover" = 1 ] && [ -s "$work/cover.out" ]; then
		record_covered_set
	fi
	if [ "$want_json" = 1 ]; then
		record_counts
	fi
	return "$status"
}

unset GOTMPDIR
export GOTOOLCHAIN=$toolchain GOWORK=off GOFLAGS='' GOPROXY=off TMPDIR=/tmp
checkout
(cd "$work/src" && go version) >"$work/go-version.txt"
if ! grep -q "^go version $toolchain " "$work/go-version.txt"; then
	echo "target-suite: wanted $toolchain, got: $(cat "$work/go-version.txt")" >&2
	exit 3
fi
scratch=$(mktemp -d /tmp/ci.XXXXXX)
echo "$scratch" >"$work/scratch"
go_status=0
run_go_test "$@" || go_status=$?
echo "$go_status" >"$work/exit"
end_scratch || echo "target-suite: could not fully remove $scratch" >&2
