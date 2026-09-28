package ci

import (
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

const (
	sunPathUsable     = 107
	promisedHeadroom  = 13
	tempDirNameMax    = 64
	tempDirSubdir     = "/001/"
	headroomSockName  = "headroom.sock"
	longestTempDirTag = "4294967295"
)

func scratchTemplate(t *testing.T) string {
	t.Helper()
	src, err := os.ReadFile("target-suite.sh")
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`\nscratch=\$\(mktemp -d (\S+)\)\n`).FindSubmatch(src)
	if m == nil {
		t.Fatal("target-suite.sh does not create its scratch with mktemp -d <template>")
	}
	return string(m[1])
}

func TestScratchLeavesSocketHeadroomForTheLongestTempDir(t *testing.T) {
	template := scratchTemplate(t)
	if len(headroomSockName) != promisedHeadroom {
		t.Fatalf("the bound name %q is %d bytes; it must use all %d promised", headroomSockName, len(headroomSockName), promisedHeadroom)
	}
	name := strings.Repeat("N", tempDirNameMax) + longestTempDirTag
	prefix := template + "/" + name + tempDirSubdir
	if left := sunPathUsable - len(prefix); left < promisedHeadroom {
		t.Fatalf("the longest t.TempDir() under %s is %d bytes, leaving %d of the %d sun_path allows; the manual promises %d",
			template, len(prefix), left, sunPathUsable, promisedHeadroom)
	}
	out, err := exec.Command("mktemp", "-d", template).Output()
	if err != nil {
		t.Fatal(err)
	}
	dir := strings.TrimSpace(string(out))
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	sub := filepath.Join(dir, name, strings.Trim(tempDirSubdir, "/"))
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	l, err := net.Listen("unix", filepath.Join(sub, headroomSockName))
	if err != nil {
		t.Fatalf("binding a socket in the longest t.TempDir() under %s: %v", template, err)
	}
	_ = l.Close()
}
