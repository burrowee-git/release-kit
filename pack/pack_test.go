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

func TestZipRejectsPathTraversalName(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txt, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.zip")

	err := Zip(Spec{Out: out, Contents: []Content{
		{Src: txt, Name: "../evil"},
	}})
	if err == nil {
		t.Fatal("expected error for path-traversal name, got nil")
	}
	if !strings.Contains(err.Error(), "..") {
		t.Errorf("error %q does not mention the offending name", err)
	}
}

func TestZipRejectsAbsoluteName(t *testing.T) {
	dir := t.TempDir()
	txt := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txt, []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.zip")

	err := Zip(Spec{Out: out, Contents: []Content{
		{Src: txt, Name: "/etc/evil"},
	}})
	if err == nil {
		t.Fatal("expected error for absolute name, got nil")
	}
}

func TestZipRejectsDuplicateNames(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.txt")
	b := filepath.Join(dir, "b.txt")
	if err := os.WriteFile(a, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "out.zip")

	err := Zip(Spec{Out: out, Contents: []Content{
		{Src: a, Name: "same.txt"},
		{Src: b, Name: "same.txt"},
	}})
	if err == nil {
		t.Fatal("expected error for duplicate in-archive name, got nil")
	}
	if !strings.Contains(err.Error(), "same.txt") {
		t.Errorf("error %q does not mention the colliding name", err)
	}
}
