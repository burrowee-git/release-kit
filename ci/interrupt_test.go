package ci

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

const sshShim = `#!/bin/sh
case "$*" in
*mktemp*) echo "$FAKE_WORK" ;;
*ci-lock*)
	echo run >>"$SHIM_LOG"
	sleep 30 &
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

const goSuiteShim = `#!/bin/sh
case "$1" in
version) echo "go version go1.26.6 linux/arm64" ;;
test)
	if [ -n "$FAKE_TEST_JSON" ]; then
		cat "$FAKE_TEST_JSON"
		exit 1
	fi
	sh -c 'sleep 30 & trap "kill \$!; echo child-term >>\"$SHIM_LOG\"; exit 143" TERM; wait' &
	sleep 30 &
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
	cmd := exec.Command("bash", "ci/run-tests.sh")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH="+shims+":"+os.Getenv("PATH"), "SHIM_LOG="+log, "FAKE_WORK="+work)
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
}

func targetSuiteCmd(t *testing.T, log string, env ...string) (*exec.Cmd, string) {
	t.Helper()
	home := t.TempDir()
	work := filepath.Join(home, "ci-runs", "release-kit.abc123")
	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	shims := writeShims(t, map[string]string{"go": goSuiteShim, "git": gitCloneShim})
	cmd := exec.Command("bash", "target-suite.sh", work, "0000000", "go1.26.6", "0", "1", "0")
	cmd.Env = append(os.Environ(), append([]string{"HOME=" + home, "SHIM_LOG=" + log,
		"PATH=" + shims + ":" + os.Getenv("PATH")}, env...)...)
	return cmd, work
}

func TestTargetSuiteRecordsAndHangupKillsTheSuitesWholeProcessGroup(t *testing.T) {
	log := filepath.Join(t.TempDir(), "shim.log")
	cmd, work := targetSuiteCmd(t, log)
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
{"Action":"pass","Package":"example.invalid/good"}
{"Action":"output","Package":"example.invalid/broken","Output":"FAIL\texample.invalid/broken [build failed]\n"}
{"Action":"fail","Package":"example.invalid/broken"}
`
	if err := os.WriteFile(fixture, []byte(events), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd, work := targetSuiteCmd(t, filepath.Join(t.TempDir(), "shim.log"), "FAKE_TEST_JSON="+fixture)
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
	want := "pass tests 1 cases 0\nfail tests 0 cases 0\nskip tests 1 cases 0\n" +
		"skip example.invalid/good TestB\nfail packages 1\nfail package example.invalid/broken\n"
	if string(counts) != want {
		t.Fatalf("counts.txt = %q, want %q", counts, want)
	}
}
