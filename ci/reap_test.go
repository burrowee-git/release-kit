package ci

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func leakPID(t *testing.T, log string) int {
	t.Helper()
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`leak-pid (\d+)`).FindSubmatch(b)
	if m == nil {
		t.Fatalf("the fake suite never recorded its leaked child; log %q", b)
	}
	pid, err := strconv.Atoi(string(m[1]))
	if err != nil {
		t.Fatal(err)
	}
	return pid
}

func TestTargetSuiteReapsTheSuitesGroupAfterItFinishes(t *testing.T) {
	for _, code := range []string{"0", "1"} {
		t.Run("suite_exit_"+code, func(t *testing.T) {
			log := filepath.Join(t.TempDir(), "shim.log")
			cmd, work := targetSuiteCmd(t, log, "0", "FAKE_LEAK_EXIT="+code)
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("target-suite.sh: %v %s", err, out)
			}
			pid := leakPID(t, log)
			if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
				t.Fatalf("the suite's leaked child %d outlived the run (kill -0: %v); it would hold the ci-lock fd", pid, err)
			}
			b, _ := os.ReadFile(log)
			if !strings.Contains(string(b), "leak-term\n") {
				t.Fatalf("the leaked child was not sent TERM; log %q", b)
			}
			if got, _ := os.ReadFile(filepath.Join(work, "exit")); string(got) != code+"\n" {
				t.Fatalf("recorded exit %q, want the suite's own %q", got, code)
			}
		})
	}
}

func reapGroupFunc(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("target-suite.sh")
	if err != nil {
		t.Fatal(err)
	}
	var funcs string
	for _, name := range []string{"group_gone", "reap_group"} {
		m := regexp.MustCompile(`(?s)\n` + name + `\(\) \{\n.*?\n\}\n`).Find(src)
		if m == nil {
			t.Fatalf("target-suite.sh has no %s function", name)
		}
		funcs += string(m)
	}
	return funcs
}

func traceReap(t *testing.T, pgid string) []string {
	t.Helper()
	script := "kill() { return 1; }\n" + reapGroupFunc(t) + "reap_group " + strconv.Quote(pgid) + "\n"
	out, _ := exec.Command("bash", "-x", "-c", script).CombinedOutput()
	return tracedKills(string(out))
}

func TestReapGroupSignalsOnlyASafeProcessGroup(t *testing.T) {
	for _, pgid := range []string{"", "0", "1", "00", "01", "001", "-5", "1x"} {
		if kills := traceReap(t, pgid); len(kills) != 0 {
			t.Errorf("pgid %q reached %q; only a plain id above 1 may be signalled", pgid, kills)
		}
	}
	if kills := traceReap(t, "4242"); len(kills) == 0 || !strings.Contains(kills[0], "kill -s TERM -- -4242") {
		t.Fatalf("a valid pgid traced %q, want kill -s TERM -- -4242", kills)
	}
}

func TestTargetSuiteKillsALeakThatIgnoresTerm(t *testing.T) {
	log := filepath.Join(t.TempDir(), "shim.log")
	cmd, work := targetSuiteCmd(t, log, "0", "FAKE_LEAK_EXIT=3", "FAKE_LEAK_IGNORES_TERM=1")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("target-suite.sh: %v %s", err, out)
	}
	pid := leakPID(t, log)
	if err := syscall.Kill(pid, 0); !errors.Is(err, syscall.ESRCH) {
		t.Fatalf("a leaked child that ignores TERM outlived the run (kill -0: %v); only KILL removes it", err)
	}
	if !strings.Contains(string(out), "outlived TERM for 10s; sending KILL") {
		t.Fatalf("the KILL escalation was not logged; output %q", out)
	}
	if got, _ := os.ReadFile(filepath.Join(work, "exit")); string(got) != "3\n" {
		t.Fatalf("recorded exit %q, want the suite's own 3", got)
	}
}
