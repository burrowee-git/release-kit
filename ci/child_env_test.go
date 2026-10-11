package ci

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func childEnv(t *testing.T, inherited, extra []string, dirs ...string) []string {
	t.Helper()
	env, err := buildChildEnv(inherited, extra, dirs)
	if err != nil {
		t.Fatal(err)
	}
	return env
}

func buildChildEnv(inherited, extra, dirs []string) ([]string, error) {
	if n := countPathEntries(extra); n != 0 {
		return nil, errors.New("caller-supplied env carries PATH=; pass the shim dirs as arguments")
	}
	if n := countPathEntries(inherited); n != 1 {
		return nil, errors.New("inherited env must carry exactly one PATH= entry")
	}
	if slices.Contains(dirs, "") {
		return nil, errors.New("an empty shim dir would put the current directory on PATH")
	}
	var env []string
	path := ""
	for _, kv := range inherited {
		if v, ok := strings.CutPrefix(kv, "PATH="); ok {
			path = v
			continue
		}
		env = append(env, kv)
	}
	if path == "" {
		return nil, errors.New("inherited PATH is empty and would put the current directory on PATH")
	}
	env = append(env, extra...)
	return append(env, "PATH="+strings.Join(append(slices.Clone(dirs), path), ":")), nil
}

func countPathEntries(env []string) int {
	n := 0
	for _, kv := range env {
		if strings.HasPrefix(kv, "PATH=") {
			n++
		}
	}
	return n
}

func TestChildEnvBuildsOnePathFromDirsThenInherited(t *testing.T) {
	env, err := buildChildEnv([]string{"A=1", "PATH=/usr/bin"}, []string{"B=2"}, []string{"/shim", "/stub"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"A=1", "B=2", "PATH=/shim:/stub:/usr/bin"}
	if !slices.Equal(env, want) {
		t.Fatalf("env %v; want %v", env, want)
	}
}

func TestChildEnvRefusesAPathFromTheCaller(t *testing.T) {
	if _, err := buildChildEnv([]string{"PATH=/usr/bin"}, []string{"PATH=/elsewhere"}, []string{"/shim"}); err == nil {
		t.Fatal("a PATH= in the caller-supplied env was accepted")
	}
}

func TestChildEnvRefusesAnInheritedEnvWithoutExactlyOnePath(t *testing.T) {
	for name, inherited := range map[string][]string{
		"none":      {"A=1"},
		"duplicate": {"PATH=/usr/bin", "PATH=/bin"},
	} {
		if _, err := buildChildEnv(inherited, nil, []string{"/shim"}); err == nil {
			t.Fatalf("%s: an inherited env with %d PATH= entries was accepted", name, countPathEntries(inherited))
		}
	}
}

func TestChildEnvRefusesAnEmptyInheritedPathValue(t *testing.T) {
	if _, err := buildChildEnv([]string{"PATH="}, nil, []string{"/shim"}); err == nil {
		t.Fatal("an empty inherited PATH value was accepted")
	}
}

func TestChildEnvRefusesAnEmptyDir(t *testing.T) {
	if _, err := buildChildEnv([]string{"PATH=/usr/bin"}, nil, []string{"/shim", ""}); err == nil {
		t.Fatal("an empty dir argument was accepted")
	}
}
