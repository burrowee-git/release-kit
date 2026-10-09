package ci

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func runBash(t *testing.T, env []string, script string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{script}, args...)...)
	if env != nil {
		cmd.Env = env
	}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	err := cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return out.String(), errOut.String(), 0
	case errors.As(err, &exit):
		return out.String(), errOut.String(), exit.ExitCode()
	default:
		t.Fatalf("%s did not run: %v", script, err)
		return "", "", -1
	}
}

func TestRunTestsHelpPrintsUsageToStdout(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		stdout, stderr, code := runBash(t, nil, "run-tests.sh", flag)
		if code != 0 || !strings.HasPrefix(stdout, "usage: ci/run-tests.sh") || stderr != "" {
			t.Fatalf("%s: exit %d, stdout %q, stderr %q; want exit 0 and the usage on stdout only", flag, code, stdout, stderr)
		}
		if !strings.Contains(stdout, "/tmp/ci.XXXXXX") || strings.Contains(stdout, "TMPDIR=/tmp") {
			t.Fatalf("%s: the usage must name the run's /tmp/ci.XXXXXX scratch and not claim TMPDIR=/tmp:\n%s", flag, stdout)
		}
	}
}

func TestRunTestsRefusesBadArgumentsWithUsage(t *testing.T) {
	cases := []struct {
		name string
		args []string
		says string
	}{
		{"unknown flag", []string{"--shuffle"}, "unknown argument '--shuffle'"},
		{"bare word", []string{"vulncheck"}, "unknown argument 'vulncheck'"},
		{"ref without value", []string{"--ref"}, "--ref needs a value"},
		{"out without value", []string{"--out"}, "--out needs a value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, code := runBash(t, nil, "run-tests.sh", tc.args...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, tc.says) || !strings.Contains(stderr, "usage: ci/run-tests.sh") {
				t.Fatalf("exit %d, stdout %q, stderr %q; want exit 2, %q and the usage on stderr", code, stdout, stderr, tc.says)
			}
		})
	}
}

func TestTargetSuiteRefusesAWorkdirOutsideItsRunRoot(t *testing.T) {
	home := t.TempDir()
	env := append(os.Environ(), "HOME="+home)
	cases := []struct {
		name string
		work string
	}{
		{"empty", ""},
		{"root", "/"},
		{"the home itself", home},
		{"the run root itself", filepath.Join(home, "ci-runs")},
		{"another repo's run", filepath.Join(home, "ci-runs", "edge.abc123")},
		{"a short suffix", filepath.Join(home, "ci-runs", "release-kit.abc")},
		{"another home", "/elsewhere/ci-runs/release-kit.abc123"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, code := runBash(t, env, "target-suite.sh", tc.work, "0000000", "go1.26.9", "0", "0", "0")
			if code != 3 || !strings.Contains(stderr, "refusing workdir") {
				t.Fatalf("exit %d, stderr %q; want exit 3 and a refusal before anything runs", code, stderr)
			}
			if entries, _ := os.ReadDir(home); len(entries) != 0 {
				t.Fatalf("the refused run wrote into HOME: %v", entries)
			}
		})
	}
}
