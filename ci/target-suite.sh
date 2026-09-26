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

stop_group() {
	if [ -n "$group" ]; then
		kill -TERM -- "-$group" 2>/dev/null || kill -TERM "$group" 2>/dev/null || true
	fi
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
	jq -r 'select(.Output != null) | .Output' "$work/test.json" | grep -- '-test.shuffle' | LC_ALL=C sort | uniq -c >"$work/seeds.txt" || true
}

in_own_group() {
	local sink=$1 errs=$2
	shift 2
	(cd "$work/src" && exec setsid "$@") >"$sink" 2>"$errs" </dev/null &
	group=$!
	local status=0
	wait "$group" || status=$?
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
go_status=0
run_go_test "$@" || go_status=$?
echo "$go_status" >"$work/exit"
