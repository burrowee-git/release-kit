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

type Module struct {
	Name string
	Dir  string
}

type GateOpts struct {
	GovulncheckPath string
	ReportDir       string
	GoBin           string
}

func Gate(ctx context.Context, modules []Module, opts GateOpts) error {
	if len(modules) == 0 {
		return fmt.Errorf("vulncheck: no modules to scan")
	}
	gv := opts.GovulncheckPath
	if gv == "" {
		gv = resolveGovulncheck(ctx, opts.GoBin)
	}
	if gv == "" {
		return fmt.Errorf("vulncheck: govulncheck not found (install: go install golang.org/x/vuln/cmd/govulncheck@latest)")
	}
	if err := checkMinVersion(ctx, gv); err != nil {
		return err
	}
	if err := os.MkdirAll(opts.ReportDir, 0o755); err != nil {
		return fmt.Errorf("vulncheck: create report dir: %w", err)
	}
	var failed []string
	for _, m := range modules {
		cmd := exec.CommandContext(ctx, gv, "./...")
		cmd.Dir = m.Dir
		cmd.Env = append(os.Environ(), "GOWORK=off")
		out, err := cmd.CombinedOutput()
		writeErr := os.WriteFile(filepath.Join(opts.ReportDir, m.Name+".txt"), out, 0o644)
		if err != nil || writeErr != nil {
			failed = append(failed, m.Name)
		}
	}
	if len(failed) > 0 {
		return fmt.Errorf("vulncheck: CVE gate failed for %v (reports in %s)", failed, opts.ReportDir)
	}
	return nil
}

var govulncheckVersionRe = regexp.MustCompile(`govulncheck@v(\d+)\.\d+\.\d+`)

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
