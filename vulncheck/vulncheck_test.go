package vulncheck

import (
	"os"
	"path/filepath"
	"testing"
)

func writeStub(t *testing.T, dir string, exit int) string {
	t.Helper()
	p := filepath.Join(dir, "govulncheck-stub")
	body := "#!/bin/sh\necho \"scan output\"\nexit " + itoa(exit) + "\n"
	if err := os.WriteFile(p, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	return "3"
}

func TestGateCleanAndFinding(t *testing.T) {
	mdir := t.TempDir()
	reports := t.TempDir()
	mods := []Module{{Name: "cli", Dir: mdir}}

	// clean (exit 0) → nil
	if err := Gate(mods, GateOpts{GovulncheckPath: writeStub(t, t.TempDir(), 0), ReportDir: reports}); err != nil {
		t.Fatalf("clean gate returned error: %v", err)
	}
	// finding (exit 3) → error + report written
	err := Gate(mods, GateOpts{GovulncheckPath: writeStub(t, t.TempDir(), 3), ReportDir: reports})
	if err == nil {
		t.Fatal("finding gate returned nil (should fail closed)")
	}
	if b, e := os.ReadFile(filepath.Join(reports, "cli.txt")); e != nil || len(b) == 0 {
		t.Fatalf("report cli.txt missing/empty: %v", e)
	}
}

func TestGateEmptyModulesFailsClosed(t *testing.T) {
	reports := t.TempDir()
	err := Gate(nil, GateOpts{GovulncheckPath: writeStub(t, t.TempDir(), 0), ReportDir: reports})
	if err == nil {
		t.Fatal("empty modules gate returned nil (should fail closed)")
	}
}
