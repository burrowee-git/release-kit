package ci

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

func TestTargetSuiteRemovesItsScratchAfterTheRun(t *testing.T) {
	log := filepath.Join(t.TempDir(), "shim.log")
	cmd, work := targetSuiteCmd(t, log, "0", "FAKE_SCRATCH=1")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("target-suite.sh: %v %s", err, out)
	}
	b, err := os.ReadFile(log)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`tmpdir (\S+) gotmpdir (\S*)\n`).FindSubmatch(b)
	if m != nil {
		t.Cleanup(func() { removeRunScratch(string(m[1]), work) })
	}
	if m == nil || string(m[1]) != string(m[2]) || !runScratch.Match(m[1]) {
		t.Fatalf("go test saw %q; want TMPDIR = GOTMPDIR = a fresh /tmp/ci.XXXXXX", b)
	}
	if _, err := os.Stat(string(m[1])); !os.IsNotExist(err) {
		t.Fatalf("the run's scratch %s and what go test wrote there survived the run (stat: %v)", m[1], err)
	}
	if _, err := os.Stat(filepath.Join(work, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("the scratch record outlived the run (stat: %v)", err)
	}
}

var unsafeScratch = []string{"", "/", "/tmp", "/tmp/ci.", "/tmp/ci.abc", "/tmp/ci.a/../x",
	"/tmp/ci.abcdefg", "/tmp/ci.abc-ef", "/tmp/other-run.abcdef", "/home/x/ci.abcdef", "/tmp/release-kit-run.abcdef"}

func shellFunc(t *testing.T, file, name string) string {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?s)\n` + name + `\(\) \{\n.*?\n\}\n`).Find(src)
	if m == nil {
		t.Fatalf("%s has no %s function", file, name)
	}
	return string(m)
}

func tracedRemovals(t *testing.T, script string) string {
	t.Helper()
	out, _ := exec.Command("bash", "-c", "rm() { echo \"RM $*\"; }\nchmod() { echo \"CHMOD $*\"; }\nkill() { return 1; }\n"+script).CombinedOutput()
	return string(out)
}

func TestScratchRemovalTouchesOnlyItsOwnTemplate(t *testing.T) {
	fn := shellFunc(t, "target-suite.sh", "remove_scratch")
	stop := stopScript(t)
	record := filepath.Join(t.TempDir(), "scratch")
	for _, dir := range append(unsafeScratch, "/tmp/ci.Ab3dE9") {
		want := ""
		if dir == "/tmp/ci.Ab3dE9" {
			want = "CHMOD -R u+w -- /tmp/ci.Ab3dE9\nRM -rf -- /tmp/ci.Ab3dE9\n"
		}
		if got := tracedRemovals(t, fn+"remove_scratch '"+dir+"'\n"); got != want {
			t.Errorf("remove_scratch %q ran %q, want %q", dir, got, want)
		}
		if err := os.WriteFile(record, []byte(dir+"\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		script := "set -- /nonexistent/pgid " + record + "\n" + stop + "\n"
		if got := tracedRemovals(t, script); strings.TrimSpace(got) != strings.TrimSpace(want) {
			t.Errorf("the stop script with scratch %q ran %q, want %q", dir, got, want)
		}
	}
}

const stageProbe = `printf 'stage %s tmpdir %s gotmpdir %s exists %s seen %s\n' "$1" "$TMPDIR" "$GOTMPDIR" ` +
	`"$([ -d "$TMPDIR" ] && echo yes || echo no)" "$(ls "$TMPDIR" 2>/dev/null | tr '\n' ,)" >>"$STAGE_LOG"; ` +
	`: >"$TMPDIR/from-stage-$1"`

type stageSeen struct {
	tmpdir, gotmpdir, exists, seen string
}

func runTwoStages(t *testing.T, work, log string) []stageSeen {
	t.Helper()
	var funcs strings.Builder
	for _, name := range []string{"remove_scratch", "group_gone", "reap_group", "in_own_group"} {
		funcs.WriteString(shellFunc(t, "target-suite.sh", name))
	}
	script := "work=" + work + "\ngroup=\n" + funcs.String() +
		"scratch=$(mktemp -d /tmp/ci.XXXXXX)\necho \"$scratch\" >\"$work/scratch\"\n" +
		"in_own_group \"$work/s1.out\" \"$work/s1.err\" bash -c \"$STAGE\" stage 1\n" +
		"in_own_group \"$work/s2.out\" \"$work/s2.err\" bash -c \"$STAGE\" stage 2\n"
	cmd := exec.Command("bash", "-c", script)
	cmd.Env = append(os.Environ(), "STAGE_LOG="+log, "STAGE="+stageProbe)
	out, _ := cmd.CombinedOutput()
	b, _ := os.ReadFile(log)
	lines := regexp.MustCompile(`stage (\d) tmpdir (\S*) gotmpdir (\S*) exists (\S+) seen (\S*)\n`).FindAllSubmatch(b, -1)
	var stages []stageSeen
	for _, m := range lines {
		dir := string(m[2])
		t.Cleanup(func() { removeRunScratch(dir, work) })
		stages = append(stages, stageSeen{string(m[2]), string(m[3]), string(m[4]), string(m[5])})
	}
	if len(stages) != 2 {
		t.Fatalf("want two stage lines, got %q (harness output %s)", b, out)
	}
	return stages
}

func TestScratchLivesForTheWholeRunAcrossStages(t *testing.T) {
	work := t.TempDir()
	t.Cleanup(func() { removeRecordedScratch(work) })
	if err := os.Mkdir(filepath.Join(work, "src"), 0o755); err != nil {
		t.Fatal(err)
	}
	stages := runTwoStages(t, work, filepath.Join(t.TempDir(), "stages.log"))
	first, second := stages[0], stages[1]
	if !runScratch.MatchString(first.tmpdir) || first.tmpdir != first.gotmpdir {
		t.Fatalf("stage 1 saw TMPDIR %q GOTMPDIR %q; want both a fresh /tmp/ci.XXXXXX", first.tmpdir, first.gotmpdir)
	}
	if second.tmpdir != first.tmpdir || second.gotmpdir != first.tmpdir {
		t.Fatalf("stage 2 saw TMPDIR %q GOTMPDIR %q; want stage 1's %q, one scratch per run", second.tmpdir, second.gotmpdir, first.tmpdir)
	}
	if first.exists != "yes" || second.exists != "yes" {
		t.Fatalf("the scratch existed in stage 1: %s, in stage 2: %s; want it for the whole run", first.exists, second.exists)
	}
	if !strings.Contains(second.seen, "from-stage-1") {
		t.Fatalf("stage 2 found %q in the scratch; want stage 1's from-stage-1", second.seen)
	}
	end := "work=" + work + "\nscratch=" + first.tmpdir + "\n" +
		shellFunc(t, "target-suite.sh", "remove_scratch") + shellFunc(t, "target-suite.sh", "end_scratch") + "end_scratch\n"
	if out, err := exec.Command("bash", "-c", end).CombinedOutput(); err != nil {
		t.Fatalf("end_scratch: %v %s", err, out)
	}
	if _, err := os.Stat(first.tmpdir); !os.IsNotExist(err) {
		t.Fatalf("the run's scratch %s survived end_scratch (stat: %v)", first.tmpdir, err)
	}
	if _, err := os.Stat(filepath.Join(work, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("the scratch record survived end_scratch (stat: %v)", err)
	}
}

func runScratchStage(t *testing.T, env ...string) (scratch, work, out string) {
	t.Helper()
	log := filepath.Join(t.TempDir(), "shim.log")
	cmd, work := targetSuiteCmd(t, log, "0", env...)
	b, runErr := cmd.CombinedOutput()
	logged, _ := os.ReadFile(log)
	m := regexp.MustCompile(`tmpdir (\S+) gotmpdir`).FindSubmatch(logged)
	if m != nil {
		t.Cleanup(func() { removeRunScratch(string(m[1]), work) })
	}
	if m == nil || !runScratch.Match(m[1]) {
		t.Fatalf("the stage logged no run scratch: %q (run: %v %s)", logged, runErr, b)
	}
	scratch = string(m[1])
	exit, err := os.ReadFile(filepath.Join(work, "exit"))
	if err != nil || strings.TrimSpace(string(exit)) != "0" {
		t.Fatalf("the run lost its result: exit file %q (%v); run: %v %s", exit, err, runErr, b)
	}
	return scratch, work, string(b)
}

func TestTargetSuiteRemovesAReadOnlyDirectoryItsStageLeft(t *testing.T) {
	scratch, _, out := runScratchStage(t, "FAKE_READONLY_SCRATCH=1")
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Fatalf("a 0500 directory the stage left kept the scratch %s in RAM /tmp (stat: %v); output %s", scratch, err, out)
	}
	if strings.Contains(out, "could not fully remove") {
		t.Fatalf("a user-owned read-only directory must not defeat the removal; output %s", out)
	}
}

func TestTargetSuiteRecordsItsExitWhenTheScratchResistsRemoval(t *testing.T) {
	scratch, _, out := runScratchStage(t, "FAKE_SCRATCH=1", "FAKE_RM_FAILS=1")
	if want := "target-suite: could not fully remove " + scratch; !strings.Contains(out, want) {
		t.Fatalf("the run did not warn %q; output %s", want, out)
	}
}

func TestTargetSuiteHangupRemovesTheRunsScratch(t *testing.T) {
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
	b, err := os.ReadFile(filepath.Join(work, "scratch"))
	scratch := strings.TrimSpace(string(b))
	t.Cleanup(func() { removeRunScratch(scratch, work) })
	if err != nil || !runScratch.MatchString(scratch) {
		t.Fatalf("no run scratch recorded while the stage ran: %q (%v)", b, err)
	}
	if err := cmd.Process.Signal(syscall.SIGHUP); err != nil {
		t.Fatal(err)
	}
	if code := exitCode(t, cmd); code != 130 {
		t.Fatalf("exit %d; want 130 after a hangup", code)
	}
	if _, err := os.Stat(scratch); !os.IsNotExist(err) {
		t.Fatalf("a hangup left the run's scratch %s in RAM /tmp (stat: %v)", scratch, err)
	}
	if _, err := os.Stat(filepath.Join(work, "scratch")); !os.IsNotExist(err) {
		t.Fatalf("a hangup left the scratch record behind (stat: %v)", err)
	}
	if _, err := os.Stat(filepath.Join(work, "exit")); !os.IsNotExist(err) {
		t.Fatalf("a hangup recorded an exit (stat: %v); an interrupted run has no result", err)
	}
}

func TestRunScratchCleanupRemovesOnlyARunShapedDirectoryOwnedLikeTheTest(t *testing.T) {
	ref := t.TempDir()
	made, err := exec.Command("mktemp", "-d", "/tmp/agent-run.XXXXXX").Output()
	if err != nil {
		t.Fatal(err)
	}
	dir := strings.TrimSpace(string(made))
	t.Cleanup(func() {
		_ = os.Chmod(filepath.Join(dir, "ro"), 0o700)
		_ = os.RemoveAll(dir)
	})
	if err := os.MkdirAll(filepath.Join(dir, "ro", "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(filepath.Join(dir, "ro"), 0o500); err != nil {
		t.Fatal(err)
	}
	target := t.TempDir()
	link := "/tmp/lnk" + strings.TrimPrefix(filepath.Base(dir), "agent-run")
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(link) })
	for _, kept := range []string{"/tmp", target, link} {
		removeRunScratch(kept, ref)
		if _, err := os.Lstat(kept); err != nil {
			t.Fatalf("removeRunScratch removed %s, which is not a run-shaped directory (lstat: %v)", kept, err)
		}
	}
	removeRunScratch(dir, filepath.Join(ref, "absent"))
	if _, err := os.Lstat(dir); err != nil {
		t.Fatalf("removeRunScratch removed %s against an unreadable ownership reference (lstat: %v)", dir, err)
	}
	if !sameOwner(t, "/", ref) {
		removeRunScratch(dir, "/")
		if _, err := os.Lstat(dir); err != nil {
			t.Fatalf("removeRunScratch removed %s although its owner differs from the reference / (lstat: %v)", dir, err)
		}
	}
	removeRunScratch(dir, ref)
	if _, err := os.Lstat(dir); !os.IsNotExist(err) {
		t.Fatalf("removeRunScratch left the run-shaped scratch %s, read-only child and all (lstat: %v)", dir, err)
	}
}

func sameOwner(t *testing.T, a, b string) bool {
	t.Helper()
	ai, err := os.Stat(a)
	if err != nil {
		t.Fatal(err)
	}
	bi, err := os.Stat(b)
	if err != nil {
		t.Fatal(err)
	}
	as, aOK := ai.Sys().(*syscall.Stat_t)
	bs, bOK := bi.Sys().(*syscall.Stat_t)
	return aOK && bOK && as.Uid == bs.Uid
}
