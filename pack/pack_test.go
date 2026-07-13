package pack

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestZipFlatWithExecBit(t *testing.T) {
	dir := t.TempDir()
	binp := filepath.Join(dir, "tool")
	os.WriteFile(binp, []byte("#!/bin/sh\n"), 0o755)
	txt := filepath.Join(dir, "notes.txt")
	os.WriteFile(txt, []byte("hi"), 0o644)
	out := filepath.Join(dir, "out.zip")

	err := Zip(Spec{Out: out, Contents: []Content{
		{Src: binp}, // basename → "tool"
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
	// content check on README.txt
	rc, _ := r.Open("README.txt")
	b, _ := io.ReadAll(rc)
	rc.Close()
	if string(b) != "hi" {
		t.Errorf("README.txt content=%q", b)
	}
}
