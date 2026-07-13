// Package pack assembles a flat zip archive (like `zip -j`) from caller-supplied
// files, preserving each file's permission bits.
package pack

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Content is one file to add. Name is its in-archive name (basename of Src if empty).
type Content struct {
	Src  string
	Name string
}

// Spec describes an archive to build at Out from Contents.
type Spec struct {
	Contents []Content
	Out      string
}

// Zip writes the archive. Entries are added flat (no directories), with each
// source file's mode preserved. In-archive names must be non-absolute, free of
// ".." path elements, and unique across Contents.
func Zip(spec Spec) (err error) {
	zf, ferr := os.Create(spec.Out)
	if ferr != nil {
		return fmt.Errorf("pack: create %s: %w", spec.Out, ferr)
	}
	defer func() {
		if cerr := zf.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("pack: close %s: %w", spec.Out, cerr)
		}
	}()

	zw := zip.NewWriter(zf)
	seen := make(map[string]bool, len(spec.Contents))
	for _, c := range spec.Contents {
		name := c.Name
		if name == "" {
			name = filepath.Base(c.Src)
		}
		if sanitizeErr := validateName(name); sanitizeErr != nil {
			return fmt.Errorf("pack: %s: %w", c.Src, sanitizeErr)
		}
		if seen[name] {
			return fmt.Errorf("pack: %s: duplicate in-archive name %q", c.Src, name)
		}
		seen[name] = true

		info, statErr := os.Stat(c.Src)
		if statErr != nil {
			return fmt.Errorf("pack: %s: %w", c.Src, statErr)
		}
		hdr, hdrErr := zip.FileInfoHeader(info)
		if hdrErr != nil {
			return fmt.Errorf("pack: %s: %w", c.Src, hdrErr)
		}
		hdr.Name = name
		hdr.Method = zip.Deflate
		w, createErr := zw.CreateHeader(hdr)
		if createErr != nil {
			return fmt.Errorf("pack: %s: %w", c.Src, createErr)
		}
		src, openErr := os.Open(c.Src)
		if openErr != nil {
			return fmt.Errorf("pack: %s: %w", c.Src, openErr)
		}
		_, copyErr := io.Copy(w, src)
		_ = src.Close()
		if copyErr != nil {
			return fmt.Errorf("pack: %s: %w", c.Src, copyErr)
		}
	}
	if closeErr := zw.Close(); closeErr != nil {
		return fmt.Errorf("pack: %s: %w", spec.Out, closeErr)
	}
	return nil
}

// validateName rejects in-archive names that are absolute, empty, or contain
// ".." path elements (zip-slip-on-write hardening).
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("empty in-archive name")
	}
	if filepath.IsAbs(name) || strings.HasPrefix(name, "/") {
		return fmt.Errorf("absolute in-archive name %q", name)
	}
	for _, part := range strings.Split(filepath.ToSlash(name), "/") {
		if part == ".." {
			return fmt.Errorf("in-archive name %q contains \"..\"", name)
		}
	}
	return nil
}
