// Package vulncheck is a fail-closed CVE gate: it runs source-mode govulncheck
// over a set of modules and returns an error if any has a reachable known
// vulnerability. Generalized from burrowee's release-time gate; brand-agnostic —
// the caller supplies the module list and tool paths.
package vulncheck

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
// module has a finding or the scan itself errors (fail-closed). nil = all clean.
func Gate(modules []Module, opts GateOpts) error {
	gv := opts.GovulncheckPath
	if gv == "" {
		gv = resolveGovulncheck(opts.GoBin)
	}
	if gv == "" {
		return fmt.Errorf("govulncheck not found (install: go install golang.org/x/vuln/cmd/govulncheck@latest)")
	}
	if err := os.MkdirAll(opts.ReportDir, 0o755); err != nil {
		return err
	}
	var failed []string
	for _, m := range modules {
		cmd := exec.Command(gv, "./...")
		cmd.Dir = m.Dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		out, err := cmd.CombinedOutput()
		_ = os.WriteFile(filepath.Join(opts.ReportDir, m.Name+".txt"), out, 0o644)
		if err != nil {
			failed = append(failed, m.Name)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("CVE gate failed for %v (reports in %s)", failed, opts.ReportDir)
	}
	return nil
}

func resolveGovulncheck(goBin string) string {
	if p, err := exec.LookPath("govulncheck"); err == nil {
		return p
	}
	if goBin == "" {
		goBin = "go"
	}
	out, err := exec.Command(goBin, "env", "GOPATH").Output()
	if err != nil {
		return ""
	}
	cand := filepath.Join(string(trim(out)), "bin", "govulncheck")
	if fi, err := os.Stat(cand); err == nil && !fi.IsDir() {
		return cand
	}
	return ""
}

func trim(b []byte) []byte {
	for len(b) > 0 && (b[len(b)-1] == '\n' || b[len(b)-1] == '\r' || b[len(b)-1] == ' ') {
		b = b[:len(b)-1]
	}
	return b
}
