// Package vulncheck is a fail-closed CVE gate: it runs source-mode govulncheck
// over a set of modules and returns an error if any has a reachable known
// vulnerability. Generalized from a release-time CVE gate; brand-agnostic —
// the caller supplies the module list and tool paths.
package vulncheck

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// Module is one Go module directory to scan.
type Module struct {
	Name string
	Dir  string
}

// GateOpts configures the scan. GovulncheckPath/GoBin default to PATH lookups;
// ReportDir receives one <Name>.txt per module.
type GateOpts struct {
	GovulncheckPath string
	ReportDir       string
	GoBin           string
}

// Gate scans every module with `GOWORK=off govulncheck ./...`, writes each
// module's output to ReportDir/<Name>.txt, and returns a non-nil error if ANY
// module has a finding, its report can't be written, or the scan itself errors
// (fail-closed). nil = all clean.
func Gate(ctx context.Context, modules []Module, opts GateOpts) error {
	if len(modules) == 0 {
		return fmt.Errorf("vulncheck: no modules to scan")
	}
	gv := opts.GovulncheckPath
	if gv == "" {
		gv = resolveGovulncheck(ctx, opts.GoBin)
	}
	if gv == "" {
		return fmt.Errorf("govulncheck not found (install: go install golang.org/x/vuln/cmd/govulncheck@latest)")
	}
	if err := checkMinVersion(ctx, gv); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.ReportDir, 0o755); err != nil {
		return err
	}
	var failed []string
	for _, m := range modules {
		// exit-code-based fail-closed: govulncheck exits non-zero on findings.
		// Do NOT add -json/-format json — JSON mode always exits 0 and would
		// silently pass every scan.
		cmd := exec.CommandContext(ctx, gv, "./...")
		cmd.Dir = m.Dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		out, err := cmd.CombinedOutput()
		writeErr := os.WriteFile(filepath.Join(opts.ReportDir, m.Name+".txt"), out, 0o644)
		// The report IS the audit evidence: a clean scan whose report can't be
		// written must not pass as "clean".
		if err != nil || writeErr != nil {
			failed = append(failed, m.Name)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("CVE gate failed for %v (reports in %s)", failed, opts.ReportDir)
	}
	return nil
}

var govulncheckVersionRe = regexp.MustCompile(`govulncheck@v(\d+)\.\d+\.\d+`)

// checkMinVersion rejects a govulncheck older than v1.0.0: pre-v1 releases can
// exit 0 even with findings, which would silently defeat the fail-closed
// contract. This is belt-and-suspenders on top of the exit-code check above —
// if the version can't be determined (probe errors, or a future version-string
// format we don't recognize), it does NOT block the gate; only a confidently
// parsed sub-v1.0.0 version does.
func checkMinVersion(ctx context.Context, gv string) error {
	out, err := exec.CommandContext(ctx, gv, "-version").CombinedOutput()
	if err != nil {
		return nil
	}
	m := govulncheckVersionRe.FindSubmatch(out)
	if m == nil {
		return nil
	}
	major, err := strconv.Atoi(string(m[1]))
	if err != nil {
		return nil
	}
	if major < 1 {
		return fmt.Errorf("govulncheck version too old (major=%d, need >=v1.0.0): older versions can exit 0 even with findings", major)
	}
	return nil
}

func resolveGovulncheck(ctx context.Context, goBin string) string {
	if p, err := exec.LookPath("govulncheck"); err == nil {
		return p
	}
	if goBin == "" {
		goBin = "go"
	}
	out, err := exec.CommandContext(ctx, goBin, "env", "GOPATH").Output()
	if err != nil {
		return ""
	}
	cand := filepath.Join(strings.TrimSpace(string(out)), "bin", "govulncheck")
	if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
		return cand
	}
	return ""
}
