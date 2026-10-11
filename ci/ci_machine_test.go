package ci

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const orderShim = `#!/bin/sh
case "$*" in
*mktemp*) echo "$FAKE_WORK" ;;
esac
echo "ssh $*" >>"$SHIM_LOG"
`

const ensureShim = `#!/bin/sh
echo "ci-watch $*" >>"$SHIM_LOG"
`

const failingEnsureShim = `#!/bin/sh
echo "ci-watch $*" >>"$SHIM_LOG"
exit 1
`

func runRunnerWithMachineEnv(t *testing.T, ensure string, extra ...string) (log, stderr string, code int) {
	t.Helper()
	repo := gitRepoWithRunner(t)
	shims := writeShims(t, map[string]string{"ssh": orderShim, "scp": "#!/bin/sh\n", "ci-watch": ensure})
	logPath := filepath.Join(t.TempDir(), "shim.log")
	cmd := exec.Command("bash", "ci/run-tests.sh")
	cmd.Dir = repo
	env := []string{"HOME=" + os.Getenv("HOME"), "SHIM_LOG=" + logPath, "FAKE_WORK=/home/ci/ci-runs/release-kit.abc123"}
	cmd.Env = childEnv(t, []string{"PATH=" + os.Getenv("PATH")}, append(env, extra...), shims)
	var errBuf strings.Builder
	cmd.Stderr = &errBuf
	err := cmd.Run()
	if ee, ok := err.(*exec.ExitError); ok {
		code = ee.ExitCode()
	}
	b, _ := os.ReadFile(logPath)
	return string(b), errBuf.String(), code
}

func TestRunTestsEnsuresTheMachineBeforeItsFirstSsh(t *testing.T) {
	log, _, _ := runRunnerWithMachineEnv(t, ensureShim)
	lines := strings.Split(strings.TrimSpace(log), "\n")
	if len(lines) < 2 || lines[0] != "ci-watch ensure masdetta-ci" || !strings.HasPrefix(lines[1], "ssh ") || !strings.Contains(lines[1], " masdetta-ci ") {
		t.Fatalf("want 'ci-watch ensure masdetta-ci' first, then ssh to masdetta-ci; shim log:\n%s", log)
	}
}

func TestRunTestsHonoursCIMachine(t *testing.T) {
	log, _, _ := runRunnerWithMachineEnv(t, ensureShim, "CI_MACHINE=other-ci")
	if !strings.HasPrefix(log, "ci-watch ensure other-ci\n") || !strings.Contains(log, " other-ci ") {
		t.Fatalf("CI_MACHINE must name both the ensure and the ssh target; shim log:\n%s", log)
	}
}

func TestRunTestsBurroweeCIHostWinsOverCIMachine(t *testing.T) {
	log, _, _ := runRunnerWithMachineEnv(t, ensureShim, "CI_MACHINE=other-ci", "BURROWEE_CI_HOST=host-ci")
	if !strings.HasPrefix(log, "ci-watch ensure host-ci\n") || !strings.Contains(log, " host-ci ") || strings.Contains(log, "other-ci") {
		t.Fatalf("BURROWEE_CI_HOST must override CI_MACHINE; shim log:\n%s", log)
	}
}

func TestRunTestsSkipsEnsureWithCINoAutostart(t *testing.T) {
	log, _, _ := runRunnerWithMachineEnv(t, ensureShim, "CI_NO_AUTOSTART=1")
	if strings.Contains(log, "ci-watch") || !strings.HasPrefix(log, "ssh ") {
		t.Fatalf("CI_NO_AUTOSTART must skip ci-watch ensure; shim log:\n%s", log)
	}
}

func TestRunTestsStopsBeforeSshWhenEnsureFails(t *testing.T) {
	log, stderr, code := runRunnerWithMachineEnv(t, failingEnsureShim)
	if code == 0 || strings.Contains(log, "ssh ") || !strings.Contains(stderr, "masdetta-ci") {
		t.Fatalf("a failed ensure must stop before ssh, say so on stderr and exit non-zero; code %d, stderr %q, log:\n%s", code, stderr, log)
	}
}
