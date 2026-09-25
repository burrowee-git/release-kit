package ci

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func runScript(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{"run-tests.sh"}, args...)...)
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
		t.Fatalf("run-tests.sh did not run: %v", err)
		return "", "", -1
	}
}

func TestRunTestsHelpPrintsUsageToStdout(t *testing.T) {
	for _, flag := range []string{"-h", "--help"} {
		stdout, stderr, code := runScript(t, flag)
		if code != 0 || !strings.HasPrefix(stdout, "usage: ci/run-tests.sh") || stderr != "" {
			t.Fatalf("%s: exit %d, stdout %q, stderr %q; want exit 0 and the usage on stdout only", flag, code, stdout, stderr)
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
			stdout, stderr, code := runScript(t, tc.args...)
			if code != 2 || stdout != "" || !strings.Contains(stderr, tc.says) || !strings.Contains(stderr, "usage: ci/run-tests.sh") {
				t.Fatalf("exit %d, stdout %q, stderr %q; want exit 2, %q and the usage on stderr", code, stdout, stderr, tc.says)
			}
		})
	}
}
