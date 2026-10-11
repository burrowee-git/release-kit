package ci

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const staleGoShim = `#!/bin/sh
case "$1" in
version) echo "go version go1.26.6 linux/arm64" ;;
esac
`

func TestRunTestsPinsGo1269(t *testing.T) {
	b, err := os.ReadFile("run-tests.sh")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^toolchain=(\S+)$`).FindSubmatch(b)
	if m == nil {
		t.Fatal("run-tests.sh has no toolchain= line")
	}
	if got := string(m[1]); got != "go1.26.9" {
		t.Fatalf("run-tests.sh pins toolchain=%s; want go1.26.9", got)
	}
}

func TestTargetSuiteRefusesGo1266(t *testing.T) {
	log := filepath.Join(t.TempDir(), "shim.log")
	cmd, _ := targetSuiteCmd(t, log, "0")
	stale := writeShims(t, map[string]string{"go": staleGoShim})
	cmd.Env = childEnv(t, cmd.Env, nil, stale)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if !errors.As(err, &exit) {
		t.Fatalf("target-suite accepted a go1.26.6 toolchain: %v %s", err, stderr.String())
	}
	if code := exit.ExitCode(); code != 3 || !strings.Contains(stderr.String(), "target-suite: wanted go1.26.9") {
		t.Fatalf("exit %d, stderr %q; want exit 3 and 'target-suite: wanted go1.26.9'", code, stderr.String())
	}
}
