#!/usr/bin/env bash
set -euo pipefail

toolchain=go1.26.6
brand=burrowee
host=${BURROWEE_CI_HOST:-burrowee-ci}
ssh_opts=(-o BatchMode=yes -o ConnectTimeout=15 -o ServerAliveInterval=15 -o ServerAliveCountMax=8)
artifacts=(exit go-version.txt test.out test.json test.err cover.out covered.txt results.txt counts.txt failed-packages.txt seeds.txt)

usage() {
	cat <<USAGE
usage: ci/run-tests.sh [--ref <branch|tag>] [--cover] [--json] [--out <dir>] [<package>…] [-- <go test flags>…]

Runs this repo's suite on the Linux target ($host), never on this machine:
a clone of <ref> (default the current branch) is sent to \$HOME/ci-runs there
and 'go test -count=1 <package>…' (default ./...) runs with $toolchain from the
module cache (GOPROXY=off, TMPDIR=/tmp) under 'ci-lock run $brand'. Artifacts
come back to <out>.

  --ref <branch|tag>  the committed ref to test (default the current branch)
  --cover             add -covermode=set -coverpkg=./... and return cover.out and covered.txt
  --json              add -json and return test.json, results.txt, counts.txt and seeds.txt
  --out <dir>         where artifacts land (default a new .ci-out/<short sha>.XXXXXX per run)
  -h, --help          this page
  <package>…          go test package patterns, e.g. ./vulncheck/
  -- <flags>          passed to go test, e.g. -- -shuffle=on -count=5 -run TestX

Environment: BURROWEE_CI_HOST (default burrowee-ci); BURROWEE_CI_PROJECT
(default the ref) and BURROWEE_CI_SESSION, recorded by the lock.
Exit: the suite's own status; 2 usage; 3 the target could not run it.
USAGE
}

refuse() {
	echo "run-tests: $1" >&2
	usage >&2
	exit 2
}

unavailable() {
	echo "run-tests: $1" >&2
	exit 3
}

ref=
want_cover=0
want_json=0
out=
packages=()
go_flags=()

parse_args() {
	while [ $# -gt 0 ]; do
		case $1 in
		-h | --help) usage; exit 0 ;;
		--ref) [ $# -ge 2 ] || refuse "--ref needs a value"; ref=$2; shift 2 ;;
		--out) [ $# -ge 2 ] || refuse "--out needs a value"; out=$2; shift 2 ;;
		--cover) want_cover=1; shift ;;
		--json) want_json=1; shift ;;
		--) shift; go_flags=("$@"); return ;;
		./*) packages+=("$1"); shift ;;
		*) refuse "unknown argument '$1'" ;;
		esac
	done
}

resolve_ref() {
	if [ -z "$ref" ]; then
		ref=$(git -C "$root" symbolic-ref --quiet --short HEAD) || refuse "HEAD is detached; pass --ref <branch|tag>"
	elif ! git -C "$root" show-ref --quiet --verify "refs/heads/$ref" &&
		! git -C "$root" show-ref --quiet --verify "refs/tags/$ref"; then
		refuse "'$ref' is not a local branch or tag"
	fi
	sha=$(git -C "$root" rev-parse --verify "$ref^{commit}")
}

is_run_dir() {
	[[ $1 =~ ^/[A-Za-z0-9._/-]+/ci-runs/release-kit\.[A-Za-z0-9]{6}$ ]] && [[ $1 != *..* ]]
}

remote_command() {
	local lock_args=(run "$brand" --project "${BURROWEE_CI_PROJECT:-$ref}")
	[ -n "${BURROWEE_CI_SESSION:-}" ] && lock_args+=(--session "$BURROWEE_CI_SESSION")
	local command=(ci-lock "${lock_args[@]}" -- bash "$remote_work/target-suite.sh" "$remote_work" "$sha" "$toolchain" "$want_cover" "$want_json")
	command+=("${#packages[@]}" "${packages[@]+"${packages[@]}"}" "${go_flags[@]+"${go_flags[@]}"}")
	printf '%q ' "${command[@]}"
}

remove_remote_work() {
	if ! is_run_dir "$remote_work"; then
		echo "run-tests: not removing '$remote_work' on $host: not a run directory" >&2
		return
	fi
	ssh "${ssh_opts[@]}" "$host" "rm -rf -- $(printf '%q' "$remote_work")" ||
		echo "run-tests: could not remove $host:$remote_work" >&2
}

stop_script='f=$1
[ -f "$f" ] || exit 0
g=$(cat "$f")
case $g in "" | *[!0-9]* | 0* | 1) exit 0 ;; esac
kill -s TERM -- "-$g" 2>/dev/null || kill -s TERM "$g" 2>/dev/null || exit 0
i=0
while [ "$i" -lt 100 ] && { kill -0 -- "-$g" 2>/dev/null || kill -0 "$g" 2>/dev/null; }; do
	sleep 0.1
	i=$((i + 1))
done
[ "$i" -lt 100 ] || echo "run-tests: process group $g still alive after 10s" >&2'

stop_remote_suite() {
	is_run_dir "$remote_work" || return 0
	printf '%s\n' "$stop_script" | ssh "${ssh_opts[@]}" "$host" "bash -s -- $(printf '%q' "$remote_work/pgid")" ||
		echo "run-tests: could not stop the suite on $host" >&2
}

cleanup() {
	if [ -n "$run_pid" ]; then
		kill "$run_pid" 2>/dev/null || true
		wait "$run_pid" 2>/dev/null || true
		if [ -n "$remote_work" ]; then
			stop_remote_suite
		fi
	fi
	rm -rf -- "$local_tmp"
	if [ -n "$remote_work" ]; then
		remove_remote_work
	fi
}

send_clone() {
	git -C "$root" bundle create "$local_tmp/repo.bundle" "$ref" >/dev/null 2>&1 || refuse "could not bundle '$ref'"
	remote_work=$(ssh "${ssh_opts[@]}" "$host" 'mkdir -p "$HOME/ci-runs" && mktemp -d "$HOME/ci-runs/release-kit.XXXXXX"') ||
		{ remote_work=; unavailable "$host did not answer; nothing ran"; }
	if ! is_run_dir "$remote_work"; then
		echo "run-tests: $host returned an unusable workdir '$remote_work'; nothing ran, nothing removed" >&2
		remote_work=
		exit 3
	fi
	scp -q "${ssh_opts[@]}" "$local_tmp/repo.bundle" "$root/ci/target-suite.sh" "$host:$remote_work/" ||
		unavailable "could not copy the clone to $host"
}

fetch_artifacts() {
	ssh "${ssh_opts[@]}" "$host" "cd $(printf '%q' "$remote_work") && tar cf - \$(ls ${artifacts[*]} 2>/dev/null)" |
		tar xf - -C "$out" || true
	[ -f "$out/exit" ] || unavailable "no suite result came back from $host"
}

run_on_target() {
	ssh -tt "${ssh_opts[@]}" "$host" "$(remote_command)" </dev/null &
	run_pid=$!
	local status=0
	wait "$run_pid" || status=$?
	run_pid=
	return "$status"
}

report() {
	cat "$out/go-version.txt"
	if [ "$want_json" = 1 ]; then
		cat "$out/counts.txt" "$out/seeds.txt"
	else
		cat "$out/test.out"
	fi
	if [ -s "$out/test.err" ]; then
		cat "$out/test.err" >&2
	fi
	if [ "$want_cover" = 1 ] && [ -f "$out/covered.txt" ]; then
		echo "covered blocks: $(wc -l <"$out/covered.txt" | tr -d ' ')"
	fi
}

parse_args "$@"
root=$(git rev-parse --show-toplevel)
resolve_ref
if [ -z "$out" ]; then
	mkdir -p "$root/.ci-out"
	out=$(mktemp -d "$root/.ci-out/${sha:0:12}.XXXXXX")
fi
mkdir -p "$out"
for artifact in "${artifacts[@]}"; do
	rm -f "${out:?}/$artifact"
done
local_tmp=$(mktemp -d "${TMPDIR:-/tmp}/release-kit-run-tests.XXXXXX")
remote_work=
run_pid=
trap cleanup EXIT
trap 'exit 130' INT
trap 'exit 143' TERM

send_clone
echo "run-tests: $ref $sha on $host ($toolchain), artifacts -> $out"
run_on_target || unavailable "the run on $host did not finish (exit $?); no suite result"
fetch_artifacts
report
exit "$(cat "$out/exit")"
