package main

import (
	"fmt"
	"image"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Status is an image's state relative to the manifest.
type Status int

const (
	StatusUnmarked Status = iota // image has no record
	StatusMarked                 // record matches the image
	StatusStale                  // image size differs from the record; re-mark it
	StatusMissing                // record exists but the image file is gone
	StatusBroken                 // image file can't be decoded
)

// Entry is one image (or orphaned record) in a directory.
type Entry struct {
	Name   string // file name, as stored in the manifest
	Path   string
	Meta   *Meta // nil if unmarked
	Status Status
	W, H   int // actual image size, when readable
	Err    error
}

// Scan matches the images in dir against the manifest, sorted by name.
func Scan(dir, extList string, m *Manifest) ([]Entry, error) {
	exts := map[string]bool{}
	for _, e := range strings.Split(extList, ",") {
		if e = strings.TrimPrefix(strings.ToLower(strings.TrimSpace(e)), "."); e != "" {
			exts["."+e] = true
		}
	}
	files, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var out []Entry
	seen := map[string]bool{}
	for _, f := range files {
		if f.IsDir() || !exts[strings.ToLower(filepath.Ext(f.Name()))] {
			continue
		}
		e := Entry{Name: f.Name(), Path: filepath.Join(dir, f.Name()), Meta: m.Get(f.Name())}
		seen[e.Name] = true
		e.W, e.H, e.Err = imageSize(e.Path)
		switch {
		case e.Err != nil:
			e.Status = StatusBroken
		case e.Meta == nil:
			e.Status = StatusUnmarked
		case e.Meta.Width != e.W || e.Meta.Height != e.H:
			e.Status = StatusStale
		default:
			e.Status = StatusMarked
		}
		out = append(out, e)
	}
	for _, meta := range m.Comics {
		if !seen[meta.Image] {
			p := filepath.Join(dir, meta.Image)
			if _, err := os.Stat(p); err != nil {
				out = append(out, Entry{Name: meta.Image, Path: p, Meta: meta, Status: StatusMissing})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// imageSize reads just the image header.
func imageSize(path string) (int, int, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0, err
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0, fmt.Errorf("can't read image: %w", err)
	}
	return cfg.Width, cfg.Height, nil
}
