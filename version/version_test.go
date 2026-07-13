package version

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBump(t *testing.T) {
	cases := []struct {
		cur  string
		kind BumpKind
		want string
	}{
		{"0.1.9", BumpPatch, "0.1.10"},
		{"0.1.9", BumpMinor, "0.2.0"},
		{"1.4.2", BumpMajor, "2.0.0"},
	}
	for _, c := range cases {
		got, err := Bump(c.cur, c.kind)
		if err != nil || got != c.want {
			t.Errorf("Bump(%q,%v)=%q,%v want %q", c.cur, c.kind, got, err, c.want)
		}
	}
	if _, err := Bump("notsemver", BumpPatch); err == nil {
		t.Error("Bump accepted a non-semver")
	}
}

func TestDateVersionScheme(t *testing.T) {
	if got := DateVersionScheme("0.1.0", "abcd1234", "2026.07.12"); got != "v0.1.0.2026.07.12.abcd1234" {
		t.Errorf("got %q", got)
	}
}

func TestStamp(t *testing.T) {
	dir := t.TempDir()
	git := func(args ...string) {
		c := exec.Command("git", append([]string{"-C", dir}, args...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	git("init", "-q")
	os.WriteFile(filepath.Join(dir, "f"), []byte("x"), 0o644)
	git("add", "-A")
	git("commit", "-q", "-m", "c")
	semFile := filepath.Join(dir, "ver")
	os.WriteFile(semFile, []byte("0.1.0\n"), 0o644)

	got, err := Stamp(semFile, dir, DateVersionScheme)
	if err != nil {
		t.Fatal(err)
	}
	today := time.Now().UTC().Format("2006.01.02")
	if !strings.HasPrefix(got, "v0.1.0."+today+".") {
		t.Errorf("stamp %q missing v0.1.0.%s. prefix", got, today)
	}
	if len(strings.TrimPrefix(got, "v0.1.0."+today+".")) != 8 {
		t.Errorf("stamp %q sha8 suffix wrong length", got)
	}
}
