package ci

import (
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const sshShim = `#!/bin/sh
watch_test() {
	i=0
	while [ "$i" -lt 150 ] && kill -0 "$TEST_PID" 2>/dev/null; do
		sleep 0.2
		i=$((i + 1))
	done
}
case "$*" in
*mktemp*) echo "$FAKE_WORK" ;;
*ci-lock*)
	echo run >>"$SHIM_LOG"
	watch_test &
	trap 'kill $!; sleep 0.5; echo run-killed >>"$SHIM_LOG"; exit 143' TERM
	wait
	;;
*pgid*)
	for last; do :; done
	echo stop >>"$SHIM_LOG"
	sh -c "$last"
	;;
*"rm -rf"*) echo rm >>"$SHIM_LOG" ;;
*) echo "other $*" >>"$SHIM_LOG" ;;
esac
`

const rmShim = `#!/bin/sh
if [ -n "$FAKE_RM_FAILS" ]; then
	for a; do
		case $a in /tmp/ci.*) echo "rm: cannot remove '$a': Operation not permitted" >&2; exit 1 ;; esac
	done
fi
exec REAL_RM "$@"
`

const goSuiteShim = `#!/bin/sh
watch_test() {
	i=0
	while [ "$i" -lt 150 ] && kill -0 "$TEST_PID" 2>/dev/null; do
		sleep 0.2
		i=$((i + 1))
	done
}
case "$1" in
version) echo "go version go1.26.6 linux/arm64" ;;
test)
	[ -z "$SCRATCH_SEEN" ] || echo "$TMPDIR" >>"$SCRATCH_SEEN"
	if [ -n "$FAKE_SCRATCH" ]; then
		mkdir -p "$TMPDIR/go-build4242/b001" && echo x >"$TMPDIR/go-build4242/b001/pkg.test"
		echo "tmpdir $TMPDIR gotmpdir $GOTMPDIR" >>"$SHIM_LOG"
		echo '{"Action":"pass","Package":"example.invalid/scratch","Test":"TestScratch"}'
		exit 0
	fi
	if [ -n "$FAKE_READONLY_SCRATCH" ]; then
		mkdir "$TMPDIR/ro" && echo x >"$TMPDIR/ro/f" && chmod 0500 "$TMPDIR/ro"
		echo "tmpdir $TMPDIR gotmpdir $GOTMPDIR" >>"$SHIM_LOG"
		echo '{"Action":"pass","Package":"example.invalid/readonly","Test":"TestLeavesReadOnly"}'
		exit 0
	fi
	if [ -n "$FAKE_LEAK_EXIT" ]; then
		(
			if [ -n "$FAKE_LEAK_IGNORES_TERM" ]; then
				trap '' TERM
			else
				trap 'echo leak-term >>"$SHIM_LOG"; exit 143' TERM
			fi
			echo "leak-pid $(sh -c 'echo $PPID')" >>"$SHIM_LOG"
			watch_test
		) &
		echo '{"Action":"pass","Package":"example.invalid/leaky","Test":"TestLeaks"}'
		exit "$FAKE_LEAK_EXIT"
	fi
	if [ -n "$FAKE_TEST_JSON" ]; then
		for a; do
			case $a in -coverprofile=*) [ -z "$FAKE_COVER" ] || cp "$FAKE_COVER" "${a#-coverprofile=}" ;; esac
		done
		cat "$FAKE_TEST_JSON"
		exit 1
	fi
	(
		trap 'echo child-term >>"$SHIM_LOG"; exit 143' TERM
		watch_test
	) &
	watch_test &
	trap 'kill $!; echo go-term >>"$SHIM_LOG"; exit 143' TERM
	echo "pgid $(ps -o pgid= -p $$ | tr -d ' ')" >>"$SHIM_LOG"
	echo started >>"$SHIM_LOG"
	wait
	;;
esac
`

const gitCloneShim = `#!/bin/sh
for last; do :; done
case "$*" in
*clone*) mkdir -p "$last" ;;
esac
`

func writeShims(t *testing.T, shims map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	if _, ok := shims["ci-watch"]; !ok {
		if err := os.WriteFile(filepath.Join(dir, "ci-watch"), []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for name, body := range shims {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func waitForLine(t *testing.T, log, line string) {
	t.Helper()
	deadline := time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		if b, _ := os.ReadFile(log); strings.Contains(string(b), line+"\n") {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	b, _ := os.ReadFile(log)
	t.Fatalf("no %q in the shim log within 20s; log: %q", line, b)
}

func exitCode(t *testing.T, cmd *exec.Cmd) int {
	t.Helper()
	err := cmd.Wait()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	if err != nil {
		t.Fatal(err)
	}
	return 0
}

func gitRepoWithRunner(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "work"},
		{"-c", "user.name=t", "-c", "user.email=t@t.invalid", "commit", "-q", "--allow-empty", "-m", "c"},
	} {
		if out, err := exec.Command("git", append([]string{"-C", repo}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v %s", args, err, out)
		}
	}
	if err := os.MkdirAll(filepath.Join(repo, "ci"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"run-tests.sh", "target-suite.sh"} {
		b, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, "ci", name), b, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	return repo
}

const fakeRemoteSuite = `#!/bin/sh
parent=$1
if [ -z "${FAKE_SUITE_CHILD:-}" ]; then
	FAKE_SUITE_CHILD=1 sh "$0" "$parent" &
	trap 'echo suite-term >>"$SHIM_LOG"; exit 143' TERM
else
	trap 'echo suite-child-term >>"$SHIM_LOG"; exit 143' TERM
fi
while kill -0 "$parent" 2>/dev/null; do sleep 0.2; done
`

func startRemoteSuite(t *testing.T, work, log string) {
	t.Helper()
	script := filepath.Join(t.TempDir(), "suite.sh")
	if err := os.WriteFile(script, []byte(fakeRemoteSuite), 0o755); err != nil {
		t.Fatal(err)
	}
	suite := exec.Command("sh", script, strconv.Itoa(os.Getpid()))
	suite.Env = append(os.Environ(), "SHIM_LOG="+log)
	suite.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := suite.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = suite.Wait(); close(done) }()
	t.Cleanup(func() {
		_ = syscall.Kill(-suite.Process.Pid, syscall.SIGKILL)
		<-done
	})
	pgid := strconv.Itoa(suite.Process.Pid)
	if err := os.WriteFile(filepath.Join(work, "pgid"), []byte(pgid+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRunTestsInterruptStopsTheRemoteSuiteBeforeRemovingItsWorkdir(t *testing.T) {
	repo := gitRepoWithRunner(t)
	shims := writeShims(t, map[string]string{"ssh": sshShim, "scp": "#!/bin/sh\n"})
	log := filepath.Join(t.TempDir(), "shim.log")
	work := filepath.Join(t.TempDir(), "ci-runs", "release-kit.abc123")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	startRemoteSuite(t, work, log)
	scratch := makeRemoteScratch(t, work)
	cmd := exec.Command("bash", "ci/run-tests.sh")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH="+shims+":"+os.Getenv("PATH"), "SHIM_LOG="+log, "FAKE_WORK="+work,
		"TEST_PID="+strconv.Itoa(os.Getpid()))
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitForLine(t, log, "run")
	if err := cmd.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	code := exitCode(t, cmd)

	b, _ := os.ReadFile(log)
	got := strings.Split(strings.TrimSuffix(string(b), "\n"), "\n")
	if len(got) == 6 {
		slices.Sort(got[3:5])
	}
	want := []string{"run", "run-killed", "stop", "suite-child-term", "suite-term", "rm"}
	if !slices.Equal(got, want) || code != 130 {
		t.Fatalf("exit %d, remote actions %q; want exit 130, the local run killed, then every process in "+
			"the recorded group stopped, then its workdir removed", code, b)
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Fatalf("the interrupted run's scratch %s survived (stat: %v); go test's leftovers would sit in RAM /tmp", scratch, err)
	}
}

func removeRecordedScratch(work string) {
	if b, err := os.ReadFile(filepath.Join(work, "scratch")); err == nil {
		removeRunScratch(strings.TrimSpace(string(b)), work)
	}
}

func removeSeenScratch(seen, work string) {
	b, _ := os.ReadFile(seen)
	for _, dir := range strings.Fields(string(b)) {
		removeRunScratch(dir, work)
	}
}

var runScratchShape = regexp.MustCompile(`^/tmp/[A-Za-z0-9][A-Za-z0-9_-]*\.[A-Za-z0-9]{6}$`)

func removeRunScratch(dir, ownedLike string) {
	if !runScratchShape.MatchString(dir) {
		return
	}
	info, err := os.Lstat(dir)
	if err != nil || !info.IsDir() {
		return
	}
	ref, err := os.Stat(ownedLike)
	if err != nil {
		return
	}
	got, isStat := info.Sys().(*syscall.Stat_t)
	want, isRefStat := ref.Sys().(*syscall.Stat_t)
	if !isStat || !isRefStat || got.Uid != want.Uid {
		return
	}
	_ = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(p, 0o700)
		}
		return nil
	})
	_ = os.RemoveAll(dir)
}

var runScratch = regexp.MustCompile(`^/tmp/ci\.[A-Za-z0-9]{6}$`)

func makeRemoteScratch(t *testing.T, work string) string {
	t.Helper()
	out, err := exec.Command("mktemp", "-d", "/tmp/ci.XXXXXX").Output()
	if err != nil {
		t.Fatal(err)
	}
	dir := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.MkdirAll(filepath.Join(dir, "go-build4242"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "scratch"), []byte(dir+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func targetSuiteCmd(t *testing.T, log, cover string, env ...string) (*exec.Cmd, string) {
	t.Helper()
	home := t.TempDir()
	work := filepath.Join(home, "ci-runs", "release-kit.abc123")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	realRm, err := exec.LookPath("rm")
	if err != nil {
		t.Fatal(err)
	}
	shims := writeShims(t, map[string]string{"go": goSuiteShim, "git": gitCloneShim, "rm": strings.ReplaceAll(rmShim, "REAL_RM", realRm)})
	seen := filepath.Join(home, "scratch-seen")
	t.Cleanup(func() {
		removeRecordedScratch(work)
		removeSeenScratch(seen, work)
	})
	cmd := exec.Command("bash", "target-suite.sh", work, "0000000", "go1.26.6", cover, "1", "0")
	cmd.Env = append(os.Environ(), append([]string{"HOME=" + home, "SHIM_LOG=" + log, "SCRATCH_SEEN=" + seen, "TEST_PID=" + strconv.Itoa(os.Getpid()),
		"PATH=" + shims + ":" + os.Getenv("PATH")}, env...)...)
	return cmd, work
}

func TestTargetSuiteRecordsAndHangupKillsTheSuitesWholeProcessGroup(t *testing.T) {
	log := filepath.Join(t.TempDir(), "shim.log")
	cmd, work := targetSuiteCmd(t, log, "0")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if pgid, err := os.ReadFile(filepath.Join(work, "pgid")); err == nil {
			if g, err := strconv.Atoi(strings.TrimSpace(string(pgid))); err == nil && g > 1 {
				_ = syscall.Kill(-g, syscall.SIGKILL)
			}
		}
		if cmd.ProcessState == nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
			_ = cmd.Wait()
		}
	})
	waitForLine(t, log, "started")
	recorded, err := os.ReadFile(filepath.Join(work, "pgid"))
	if err != nil {
		t.Fatal(err)
	}
	waitForLine(t, log, "pgid "+strings.TrimSpace(string(recorded)))
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	code := exitCode(t, cmd)
	waitForLine(t, log, "go-term")
	waitForLine(t, log, "child-term")
	if code != 130 {
		t.Fatalf("exit %d; want 130 after a hangup", code)
	}
}

func TestRecordCountsNamesAFailedPackage(t *testing.T) {
	fixture := filepath.Join(t.TempDir(), "test.json")
	events := `{"Action":"run","Package":"example.invalid/good","Test":"TestA"}
{"Action":"pass","Package":"example.invalid/good","Test":"TestA"}
{"Action":"skip","Package":"example.invalid/good","Test":"TestB"}
{"Action":"pass","Package":"example.invalid/good","Test":"TestA/ok"}
{"Action":"skip","Package":"example.invalid/good","Test":"TestA/case"}
{"Action":"pass","Package":"example.invalid/good"}
{"Action":"output","Package":"example.invalid/broken","Output":"FAIL\texample.invalid/broken [build failed]\n"}
{"Action":"fail","Package":"example.invalid/broken"}
`
	if err := os.WriteFile(fixture, []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd, work := targetSuiteCmd(t, filepath.Join(t.TempDir(), "shim.log"), "0", "FAKE_TEST_JSON="+fixture)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("target-suite.sh: %v %s", err, out)
	}
	if _, err := os.Stat(filepath.Join(work, "pgid")); !os.IsNotExist(err) {
		t.Fatalf("a finished suite left its process-group record behind (stat: %v)", err)
	}
	counts, err := os.ReadFile(filepath.Join(work, "counts.txt"))
	if err != nil {
		t.Fatal(err)
	}
	want := "pass tests 1 cases 1\nfail tests 0 cases 0\nskip tests 1 cases 1\n" +
		"skip example.invalid/good TestA/case\nskip example.invalid/good TestB\n" +
		"fail packages 1\nfail package example.invalid/broken\n"
	if string(counts) != want {
		t.Fatalf("counts.txt = %q, want %q", counts, want)
	}
}

func TestRecordsTheCoveredSetAndSeeds(t *testing.T) {
	dir := t.TempDir()
	cover := filepath.Join(dir, "cover.out")
	profile := "mode: set\n" +
		"example.invalid/b/b.go:1.1,2.2 1 1\n" +
		"example.invalid/a/a.go:1.1,2.2 1 1\n" +
		"example.invalid/a/a.go:3.1,4.2 1 0\n" +
		"example.invalid/a_test/x.go:1.1,2.2 1 1\n" +
		"example.invalid/a/a.go:1.1,2.2 1 1\n"
	events := `{"Action":"output","Package":"example.invalid/b","Output":"-test.shuffle 7\n"}` + "\n" +
		`{"Action":"output","Package":"example.invalid/a","Output":"-test.shuffle 42\n"}` + "\n" +
		`{"Action":"output","Package":"example.invalid/a","Output":"=== RUN   TestA\n"}` + "\n" +
		`{"Action":"pass","Package":"example.invalid/a","Test":"TestA"}` + "\n" +
		`{"Action":"pass","Package":"example.invalid/b","Test":"TestB"}` + "\n"
	fixture := filepath.Join(dir, "test.json")
	for path, body := range map[string]string{cover: profile, fixture: events} {
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cmd, work := targetSuiteCmd(t, filepath.Join(dir, "shim.log"), "1", "FAKE_TEST_JSON="+fixture, "FAKE_COVER="+cover)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("target-suite.sh: %v %s", err, out)
	}
	covered, err := os.ReadFile(filepath.Join(work, "covered.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "example.invalid/a/a.go:1.1,2.2\nexample.invalid/b/b.go:1.1,2.2\n"; string(covered) != want {
		t.Fatalf("covered.txt = %q, want %q (union, count > 0, _test packages dropped, sorted)", covered, want)
	}
	seeds, err := os.ReadFile(filepath.Join(work, "seeds.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if want := "example.invalid/a 42\nexample.invalid/b 7\n"; string(seeds) != want {
		t.Fatalf("seeds.txt = %q, want %q (one <package> <seed> line per shuffled package, sorted)", seeds, want)
	}
}
