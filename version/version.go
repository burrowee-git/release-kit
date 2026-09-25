package version

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type BumpKind int

const (
	BumpPatch BumpKind = iota
	BumpMinor
	BumpMajor
)

type Scheme func(semver, sha, dateUTC string) string

func DateVersionScheme(semver, sha, dateUTC string) string {
	return "v" + semver + "." + dateUTC + "." + sha
}

var semverRe = regexp.MustCompile(`^\d+\.\d+\.\d+$`)

func Bump(cur string, kind BumpKind) (string, error) {
	if !semverRe.MatchString(cur) {
		return "", fmt.Errorf("version: not MAJOR.MINOR.PATCH: %q", cur)
	}
	p := strings.SplitN(cur, ".", 3)
	maj, err := strconv.Atoi(p[0])
	if err != nil {
		return "", fmt.Errorf("version: parse major %q: %w", p[0], err)
	}
	min, err := strconv.Atoi(p[1])
	if err != nil {
		return "", fmt.Errorf("version: parse minor %q: %w", p[1], err)
	}
	pat, err := strconv.Atoi(p[2])
	if err != nil {
		return "", fmt.Errorf("version: parse patch %q: %w", p[2], err)
	}
	switch kind {
	case BumpPatch:
		pat++
	case BumpMinor:
		min, pat = min+1, 0
	case BumpMajor:
		maj, min, pat = maj+1, 0, 0
	default:
		return "", fmt.Errorf("version: unknown bump kind %d", kind)
	}
	return fmt.Sprintf("%d.%d.%d", maj, min, pat), nil
}

func Stamp(ctx context.Context, semverFile, srcDir string, scheme Scheme) (string, error) {
	raw, err := os.ReadFile(semverFile)
	if err != nil {
		return "", fmt.Errorf("version: read %s: %w", semverFile, err)
	}
	semver := strings.TrimSpace(string(raw))
	if !semverRe.MatchString(semver) {
		return "", fmt.Errorf("version: %s: not MAJOR.MINOR.PATCH: %q", semverFile, semver)
	}
	out, err := exec.CommandContext(ctx, "git", "-C", srcDir, "rev-parse", "--short=8", "HEAD").Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("version: git sha of %s: %w\n%s", srcDir, err, ee.Stderr)
		}
		return "", fmt.Errorf("version: git sha of %s: %w", srcDir, err)
	}
	sha := strings.TrimSpace(string(out))
	return scheme(semver, sha, time.Now().UTC().Format("2006.01.02")), nil
}
