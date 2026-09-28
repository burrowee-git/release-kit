package ci

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const lockShim = `#!/bin/sh
case "$*" in
*mktemp*) echo "$FAKE_WORK" ;;
*ci-lock*) echo "lock $*" >>"$SHIM_LOG"; exit "${FAKE_LOCK_EXIT:-0}" ;;
*"tar cf"*) echo fetch >>"$SHIM_LOG" ;;
*) echo other >>"$SHIM_LOG" ;;
esac
`

func runUnderLockExit(t *testing.T, lockExit string) (log string, stderr string, code int) {
	t.Helper()
	repo := gitRepoWithRunner(t)
	shims := writeShims(t, map[string]string{"ssh": lockShim, "scp": "#!/bin/sh\n"})
	log = filepath.Join(t.TempDir(), "shim.log")
	cmd := exec.Command("bash", "ci/run-tests.sh")
	cmd.Dir = repo
	cmd.Env = append(os.Environ(), "PATH="+shims+":"+os.Getenv("PATH"), "SHIM_LOG="+log,
		"FAKE_WORK=/home/ci/ci-runs/release-kit.abc123", "FAKE_LOCK_EXIT="+lockExit)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	code = exitCode(t, cmd)
	b, _ := os.ReadFile(log)
	return string(b), errOut.String(), code
}

func TestRunTestsTakesTheReleaseKitProductLock(t *testing.T) {
	log, _, _ := runUnderLockExit(t, "0")
	if !regexp.MustCompile(`(?m)^lock .*ci-lock run burrowee-release-kit --project work `).MatchString(log) {
		t.Fatalf("the command reaching ssh must be 'ci-lock run burrowee-release-kit --project <ref>'; shim log:\n%s", log)
	}
}

func TestRunTestsReportsALockNotAcquired(t *testing.T) {
	log, stderr, code := runUnderLockExit(t, "75")
	if code != 75 || !strings.Contains(stderr, "burrowee-release-kit") || !strings.Contains(stderr, "not acquired") {
		t.Fatalf("exit %d, stderr %q; want exit 75 naming the burrowee-release-kit lock as not acquired", code, stderr)
	}
	if strings.Contains(log, "fetch") {
		t.Fatalf("a run that never got the lock fetched artifacts; shim log:\n%s", log)
	}
}

func TestRunTestsNamesTheInstallStepForAnUnprovisionedLock(t *testing.T) {
	log, stderr, code := runUnderLockExit(t, "2")
	if code != 3 || !strings.Contains(stderr, "not provisioned") || !strings.Contains(stderr, "ci-lock install") {
		t.Fatalf("exit %d, stderr %q; want exit 3 saying the lock is not provisioned and naming 'ci-lock install'", code, stderr)
	}
	if strings.Contains(log, "fetch") {
		t.Fatalf("a run refused by ci-lock fetched artifacts; shim log:\n%s", log)
	}
}

func TestRunTestsProductEqualsTheRegistryProduct(t *testing.T) {
	if _, err := exec.LookPath("ci-lock"); err != nil || os.Getenv("CODING_ROOT") == "" {
		t.Skip("owner: the Burrowee CI runner maintainer; needs ci-lock on PATH and CODING_ROOT set (the workstation registry); " +
			"re-enable condition: run where both hold")
	}
	b, err := os.ReadFile("run-tests.sh")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^product=(\S+)$`).FindSubmatch(b)
	if m == nil {
		t.Fatal("run-tests.sh sets no 'product=<name>' constant")
	}
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("ci-lock", "products", "--path", root).Output()
	if err != nil {
		t.Fatalf("ci-lock products --path %s: %v", root, err)
	}
	if got, want := string(m[1]), strings.TrimSpace(string(out)); got != want {
		t.Fatalf("run-tests.sh product %q, ci-lock products --path %q", got, want)
	}
}
