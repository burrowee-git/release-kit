package vulncheck

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

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

	if err := Gate(context.Background(), mods, GateOpts{GovulncheckPath: writeStub(t, t.TempDir(), 0), ReportDir: reports}); err != nil {
		t.Fatalf("clean gate returned error: %v", err)
	}
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

func TestGateSurfacesReportWriteError(t *testing.T) {
	mdir := t.TempDir()
	reports := t.TempDir()
	if err := os.Mkdir(filepath.Join(reports, "cli.txt"), 0o755); err != nil {
		t.Fatal(err)
	}
	mods := []Module{{Name: "cli", Dir: mdir}}
	err := Gate(context.Background(), mods, GateOpts{GovulncheckPath: writeStub(t, t.TempDir(), 0), ReportDir: reports})
	if err == nil {
		t.Fatal("Gate: want error when the report can't be written (fail closed)")
	}
}

func TestGateScansInExitCodeModeWithWorkspaceOff(t *testing.T) {
	t.Setenv("GOWORK", "auto")
	record := filepath.Join(t.TempDir(), "scan")
	body := "#!/bin/sh\n" +
		"if [ \"$1\" = \"-version\" ]; then\n" +
		"  echo \"Scanner: govulncheck@v1.6.0\"\n" +
		"  exit 0\n" +
		"fi\n" +
		"printf 'GOWORK=%s\\n' \"$GOWORK\" > \"" + record + "\"\n" +
		"printf '%s\\n' \"$@\" >> \"" + record + "\"\n" +
		"exit 0\n"
	gv := writeNamedStub(t, t.TempDir(), "govulncheck-recorder", body)

	mods := []Module{{Name: "cli", Dir: t.TempDir()}}
	if err := Gate(context.Background(), mods, GateOpts{GovulncheckPath: gv, ReportDir: t.TempDir()}); err != nil {
		t.Fatalf("Gate: %v", err)
	}
	data, err := os.ReadFile(record)
	if err != nil {
		t.Fatal(err)
	}
	got := strings.Split(strings.TrimSpace(string(data)), "\n")
	want := []string{"GOWORK=off", "./..."}
	if !slices.Equal(got, want) {
		t.Errorf("scan invocation = %q, want %q", got, want)
	}
}

func versionStub(versionLine string, versionExit int) string {
	return "#!/bin/sh\n" +
		"if [ \"$1\" = \"-version\" ]; then\n" +
		"  echo \"" + versionLine + "\"\n" +
		"  exit " + strconv.Itoa(versionExit) + "\n" +
		"fi\n" +
		"echo \"scan output\"\n" +
		"exit 0\n"
}

func TestGateVersionFloor(t *testing.T) {
	cases := []struct {
		name        string
		versionLine string
		versionExit int
		wantErr     bool
	}{
		{name: "ancient", versionLine: "Scanner: govulncheck@v0.0.9", versionExit: 0, wantErr: true},
		{name: "probe_fails", versionLine: "not a version string", versionExit: 1, wantErr: false},
		{name: "unparseable", versionLine: "not a version string", versionExit: 0, wantErr: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gv := writeNamedStub(t, t.TempDir(), "govulncheck-stub", versionStub(tc.versionLine, tc.versionExit))
			mods := []Module{{Name: "cli", Dir: t.TempDir()}}
			err := Gate(context.Background(), mods, GateOpts{GovulncheckPath: gv, ReportDir: t.TempDir()})
			if (err != nil) != tc.wantErr {
				t.Errorf("Gate error = %v, want error: %v", err, tc.wantErr)
			}
		})
	}
}

func govulncheckOnPath(t *testing.T) GateOpts {
	dir := t.TempDir()
	writeNamedStub(t, dir, "govulncheck", stubScript(0))
	t.Setenv("PATH", dir)
	return GateOpts{}
}

func govulncheckInGopathBin(t *testing.T) GateOpts {
	gopath := t.TempDir()
	if err := os.Mkdir(filepath.Join(gopath, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeNamedStub(t, filepath.Join(gopath, "bin"), "govulncheck", stubScript(0))
	goBin := writeNamedStub(t, t.TempDir(), "go-stub", "#!/bin/sh\necho \""+gopath+"\"\n")
	return GateOpts{GoBin: goBin}
}

func govulncheckNowhere(t *testing.T) GateOpts {
	goBinDir := t.TempDir()
	body := "#!/bin/sh\necho \"" + filepath.Join(goBinDir, "nonexistent-gopath") + "\"\n"
	return GateOpts{GoBin: writeNamedStub(t, goBinDir, "go-stub", body)}
}

func TestResolveGovulncheck(t *testing.T) {
	cases := []struct {
		name    string
		setup   func(t *testing.T) GateOpts
		wantErr string
	}{
		{name: "path", setup: govulncheckOnPath},
		{name: "gopath_bin", setup: govulncheckInGopathBin},
		{name: "none", setup: govulncheckNowhere, wantErr: "not found"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			opts := tc.setup(t)
			opts.ReportDir = t.TempDir()
			err := Gate(context.Background(), []Module{{Name: "cli", Dir: t.TempDir()}}, opts)
			if tc.wantErr == "" && err != nil {
				t.Errorf("Gate: %v", err)
			}
			if tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Errorf("Gate error = %v, want it to mention %q", err, tc.wantErr)
			}
		})
	}
}
