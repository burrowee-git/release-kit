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
	{
		for action in pass fail skip; do
			printf '%s tests %s cases %s\n' "$action" \
				"$(awk -v a="$action" '$1 == a && $3 !~ /\// {n++} END {print n+0}' "$work/results.txt")" \
				"$(awk -v a="$action" '$1 == a && $3 ~ /\// {n++} END {print n+0}' "$work/results.txt")"
		done
		grep -E '^(fail|skip) ' "$work/results.txt" || true
	} >"$work/counts.txt"
	jq -r 'select(.Output != null) | .Output' "$work/test.json" | grep -- '-test.shuffle' | LC_ALL=C sort | uniq -c >"$work/seeds.txt" || true
}

run_suite() {
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
	(cd "$work/src" && go test "${args[@]}" "$@" "${packages[@]}") >"$sink" 2>"$work/test.err" || status=$?
	echo "$status" >"$work/exit"
	if [ "$want_cover" = 1 ] && [ -s "$work/cover.out" ]; then
		record_covered_set
	fi
	if [ "$want_json" = 1 ]; then
		record_counts
	fi
}

unset GOTMPDIR
export GOTOOLCHAIN=$toolchain GOWORK=off GOFLAGS='' GOPROXY=off TMPDIR=/tmp
checkout
(cd "$work/src" && go version) >"$work/go-version.txt"
if ! grep -q "^go version $toolchain " "$work/go-version.txt"; then
	echo "target-suite: wanted $toolchain, got: $(cat "$work/go-version.txt")" >&2
	exit 3
fi
run_suite "$@"
