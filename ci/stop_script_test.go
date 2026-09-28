package ci

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

func stopScript(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("run-tests.sh")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)\nstop_script='([^']*)'\n`).FindSubmatch(src)
	if m == nil {
		t.Fatal("run-tests.sh has no stop_script='…' block")
	}
	return string(m[1])
}

func traceStop(t *testing.T, pgid, prelude string) string {
	t.Helper()
	file := filepath.Join(t.TempDir(), "pgid")
	if err := os.WriteFile(file, []byte(pgid+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("bash", "-x", "-s", "--", file)
	cmd.Stdin = strings.NewReader(prelude + stopScript(t))
	var trace bytes.Buffer
	cmd.Stderr = &trace
	_ = cmd.Run()
	return trace.String()
}

func tracedKills(trace string) []string {
	var kills []string
	for _, line := range strings.Split(trace, "\n") {
		if strings.HasPrefix(line, "+") && strings.Contains(line, "kill") {
			kills = append(kills, line)
		}
	}
	return kills
}

func TestStopScriptSignalsNothingForAnUnsafeProcessGroup(t *testing.T) {
	for _, pgid := range []string{"", "0", "1", "00", "01", "001", "-5", "1x"} {
		t.Run(strconv.Quote(pgid), func(t *testing.T) {
			if kills := tracedKills(traceStop(t, pgid, "kill() { :; }\n")); len(kills) != 0 {
				t.Fatalf("pgid %q reached %q; only a plain id above 1 may be signalled, since -0 and -1 mean every process", pgid, kills)
			}
		})
	}
}

func TestStopScriptTerminatesAValidProcessGroup(t *testing.T) {
	victim := exec.Command("sh", "-c", `while kill -0 "$1" 2>/dev/null; do sleep 0.2; done`, "victim", strconv.Itoa(os.Getpid()))
	victim.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := victim.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- victim.Wait() }()
	t.Cleanup(func() { _ = syscall.Kill(-victim.Process.Pid, syscall.SIGKILL) })
	pgid := strconv.Itoa(victim.Process.Pid)

	kills := tracedKills(traceStop(t, pgid, ""))
	if len(kills) == 0 || !strings.Contains(kills[0], "kill -s TERM -- -"+pgid) {
		t.Fatalf("traced kills %q; want the first to be kill -s TERM -- -%s", kills, pgid)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatalf("process group %s still alive 5s after the stop script", pgid)
	}
}
