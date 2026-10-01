package pack

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestZipFlatWithExecBit(t *testing.T) {
	dir := t.TempDir()
	binp := filepath.Join(dir, "tool")
	if err := os.WriteFile(binp, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	txt := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txt, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.zip")

	err := Zip(Spec{Out: out, Contents: []Content{
		{Src: binp},
		{Src: txt, Name: "README.txt"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	r, err := zip.OpenReader(out)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	found := map[string]os.FileMode{}
	for _, f := range r.File {
		found[f.Name] = f.Mode()
		if f.Method != zip.Deflate {
			t.Errorf("%s method = %d, want Deflate", f.Name, f.Method)
		}
	}
	if _, ok := found["tool"]; !ok {
		t.Fatal("missing tool entry")
	}
	if found["tool"]&0o100 == 0 {
		t.Error("tool lost its exec bit")
	}
	if _, ok := found["README.txt"]; !ok {
		t.Fatal("missing renamed README.txt entry")
	}
	rc, err := r.Open("README.txt")
	if err != nil {
		t.Fatal(err)
	}
	b, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "hi" {
		t.Errorf("README.txt content=%q", b)
	}
}

func TestZipRejectsUnsafeNames(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name     string
		contents []Content
		says     string
	}{
		{name: "traversal", contents: []Content{{Src: a, Name: "../evil"}}, says: `contains ".."`},
		{name: "absolute", contents: []Content{{Src: a, Name: "/etc/evil"}}, says: "absolute in-archive name"},
		{name: "duplicate", contents: []Content{{Src: a, Name: "same.txt"}, {Src: b, Name: "same.txt"}}, says: `duplicate in-archive name "same.txt"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := Zip(Spec{Out: filepath.Join(t.TempDir(), "out.zip"), Contents: tc.contents})
			if err == nil {
				t.Fatalf("Zip accepted an unsafe name, want an error")
			}
			if !strings.Contains(err.Error(), tc.says) {
				t.Errorf("error %q does not mention %q", err, tc.says)
			}
		})
	}
}

func TestZipReportsIOErrors(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txt, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		spec Spec
		says string
	}{
		{name: "missing_src", spec: Spec{Out: filepath.Join(dir, "a.zip"), Contents: []Content{{Src: filepath.Join(dir, "absent")}}}, says: "no such file or directory"},
		{name: "out_not_creatable", spec: Spec{Out: filepath.Join(dir, "no-such-dir", "b.zip"), Contents: []Content{{Src: txt}}}, says: "pack: create"},
		{name: "name_too_long", spec: Spec{Out: filepath.Join(dir, "c.zip"), Contents: []Content{{Src: txt, Name: strings.Repeat("n", 65536)}}}, says: "pack: " + txt + ": zip: FileHeader.Name too long"},
		{name: "src_is_directory", spec: Spec{Out: filepath.Join(dir, "d.zip"), Contents: []Content{{Src: dir, Name: "dir"}}}, says: "pack: " + dir + ": read " + dir + ": is a directory"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := Zip(tc.spec); err == nil || !strings.Contains(err.Error(), tc.says) {
				t.Errorf("Zip error = %v, want one containing %q", err, tc.says)
			}
		})
	}
}

func TestZipReportsAnUnreadableSource(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root opens a mode-000 file; owner: release-kit maintainers; re-enable: run the suite as a non-root user")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "locked")
	if err := os.WriteFile(src, []byte("x"), 0o000); err != nil {
		t.Fatal(err)
	}
	err := Zip(Spec{Out: filepath.Join(dir, "e.zip"), Contents: []Content{{Src: src}}})
	if want := "pack: " + src + ": open " + src + ": permission denied"; err == nil || err.Error() != want {
		t.Fatalf("Zip error = %v, want %q", err, want)
	}
}
