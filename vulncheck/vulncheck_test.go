package vulncheck

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// stubScript builds a POSIX sh script that answers both invocation shapes
// Gate uses: `<tool> -version` (emits a valid version line) and
// `<tool> ./...` (the scan itself, exiting with exit).
func stubScript(exit int) string {
	return "#!/bin/sh\n" +
		"if [ \"$1\" = \"-version\" ]; then\n" +
		"  echo \"Scanner: govulncheck@v1.6.0\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"echo \"scan output\"\n" +
		"exit " + strconv.Itoa(exit) + "\n"
}

func writeNamedStub(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func writeStub(t *testing.T, dir string, exit int) string {
	t.Helper()
	return writeNamedStub(t, dir, "govulncheck-stub", stubScript(exit))
}

func TestGateCleanAndFinding(t *testing.T) {
	mdir := t.TempDir()
	reports := t.TempDir()
	mods := []Module{{Name: "cli", Dir: mdir}}

	// clean (exit 0) → nil
	if err := Gate(context.Background(), mods, GateOpts{GovulncheckPath: writeStub(t, t.TempDir(), 0), ReportDir: reports}); err != nil {
		t.Fatalf("clean gate returned error: %v", err)
	}
	// finding (exit 3) → error + report written
	err := Gate(context.Background(), mods, GateOpts{GovulncheckPath: writeStub(t, t.TempDir(), 3), ReportDir: reports})
	if err == nil {
		t.Fatal("finding gate returned nil (should fail closed)")
	}
	if b, e := os.ReadFile(filepath.Join(reports, "cli.txt")); e != nil || len(b) == 0 {
		t.Fatalf("report cli.txt missing/empty: %v", e)
	}
}

func TestGateEmptyModulesFailsClosed(t *testing.T) {
	reports := t.TempDir()
	err := Gate(context.Background(), nil, GateOpts{GovulncheckPath: writeStub(t, t.TempDir(), 0), ReportDir: reports})
	if err == nil {
		t.Fatal("empty modules gate returned nil (should fail closed)")
	}
}

func TestGateRejectsAncientGovulncheck(t *testing.T) {
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = \"-version\" ]; then\n" +
		"  echo \"Scanner: govulncheck@v0.0.9\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"echo \"scan output\"\n" +
		"exit 0\n"
	gv := writeNamedStub(t, t.TempDir(), "govulncheck-old", body)

	mdir := t.TempDir()
	reports := t.TempDir()
	mods := []Module{{Name: "cli", Dir: mdir}}
	err := Gate(context.Background(), mods, GateOpts{GovulncheckPath: gv, ReportDir: reports})
	if err == nil {
		t.Fatal("Gate: want error for ancient (<v1.0.0) govulncheck, got nil")
	}
}

func TestGateProceedsWhenVersionProbeFails(t *testing.T) {
	// -version itself errors/unparseable: belt-and-suspenders only, must not
	// block an otherwise-clean scan.
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = \"-version\" ]; then\n" +
		"  echo \"not a version string\"\n" +
		"  exit 1\n" +
		"fi\n" +
		"echo \"scan output\"\n" +
		"exit 0\n"
	gv := writeNamedStub(t, t.TempDir(), "govulncheck-noversion", body)

	mdir := t.TempDir()
	reports := t.TempDir()
	mods := []Module{{Name: "cli", Dir: mdir}}
	if err := Gate(context.Background(), mods, GateOpts{GovulncheckPath: gv, ReportDir: reports}); err != nil {
		t.Fatalf("Gate: version-probe failure should not block a clean scan: %v", err)
	}
}

func TestGateSurfacesReportWriteError(t *testing.T) {
	mdir := t.TempDir()
	reports := t.TempDir()
	// Pre-create a directory at the exact report path so os.WriteFile fails.
	if err := os.Mkdir(filepath.Join(reports, "cli.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	mods := []Module{{Name: "cli", Dir: mdir}}
	err := Gate(context.Background(), mods, GateOpts{GovulncheckPath: writeStub(t, t.TempDir(), 0), ReportDir: reports})
	if err == nil {
		t.Fatal("Gate: want error when the report can't be written (fail closed)")
	}
}

func TestResolveGovulncheckFromPath(t *testing.T) {
	dir := t.TempDir()
	writeNamedStub(t, dir, "govulncheck", stubScript(0))
	t.Setenv("PATH", dir)

	mdir := t.TempDir()
	reports := t.TempDir()
	mods := []Module{{Name: "cli", Dir: mdir}}
	if err := Gate(context.Background(), mods, GateOpts{ReportDir: reports}); err != nil {
		t.Fatalf("Gate with PATH-resolved govulncheck: %v", err)
	}
}

func TestResolveGovulncheckNotFound(t *testing.T) {
	t.Setenv("PATH", t.TempDir()) // empty: no govulncheck on PATH

	// GoBin stub: `<goBin> env GOPATH` echoes a GOPATH with no bin/govulncheck.
	goBinDir := t.TempDir()
	body := "#!/bin/sh\necho \"" + filepath.Join(goBinDir, "nonexistent-gopath") + "\"\n"
	goBin := writeNamedStub(t, goBinDir, "go-stub", body)

	mdir := t.TempDir()
	reports := t.TempDir()
	mods := []Module{{Name: "cli", Dir: mdir}}
	err := Gate(context.Background(), mods, GateOpts{ReportDir: reports, GoBin: goBin})
	if err == nil {
		t.Fatal("Gate: want error when govulncheck cannot be resolved anywhere")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Gate error = %q, want it to mention %q", err.Error(), "not found")
	}
}
