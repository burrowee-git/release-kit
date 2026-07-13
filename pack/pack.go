// Package pack assembles a flat zip archive (like `zip -j`) from caller-supplied
// files, preserving each file's permission bits.
package pack

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
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
// source file's mode preserved.
func Zip(spec Spec) error {
	zf, err := os.Create(spec.Out)
	if err != nil {
		return err
	}
	defer zf.Close()
	zw := zip.NewWriter(zf)
	for _, c := range spec.Contents {
		name := c.Name
		if name == "" {
			name = filepath.Base(c.Src)
		}
		info, err := os.Stat(c.Src)
		if err != nil {
			return err
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		hdr.Name = name
		hdr.Method = zip.Deflate
		w, err := zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		src, err := os.Open(c.Src)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, src)
		src.Close()
		if err != nil {
			return err
		}
	}
	return zw.Close()
}
