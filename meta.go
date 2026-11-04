package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
)

// DefaultManifest is the file name of the per-directory manifest.
const DefaultManifest = "panels.json"

const manifestVersion = 1

// Manifest holds the panel data for every marked image in a directory.
type Manifest struct {
	Version int     `json:"version"`
	Comics  []*Meta `json:"comics"` // sorted by Image
}

// Meta is the panel data for one image.
type Meta struct {
	Image     string      `json:"image"`
	Width     int         `json:"width"`
	Height    int         `json:"height"`
	Layout    Layout      `json:"layout"`
	RowSplits []float64   `json:"rowSplits"`
	ColSplits [][]float64 `json:"colSplits"`
	Panels    []Panel     `json:"panels"`
}

func NewMeta(image string, w, h int, layout Layout, rowSplits []float64, colSplits [][]float64) (*Meta, error) {
	panels, err := BuildPanels(w, h, rowSplits, colSplits)
	if err != nil {
		return nil, err
	}
	// Normalize nils so the JSON always has arrays, never null.
	if rowSplits == nil {
		rowSplits = []float64{}
	}
	cols := make([][]float64, len(colSplits))
	for i, c := range colSplits {
		if c == nil {
			c = []float64{}
		}
		cols[i] = c
	}
	return &Meta{
		Image: image, Width: w, Height: h, Layout: layout,
		RowSplits: rowSplits, ColSplits: cols, Panels: panels,
	}, nil
}

// LoadManifest reads a manifest; a missing file yields an empty one.
func LoadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return &Manifest{Version: manifestVersion, Comics: []*Meta{}}, nil
	}
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if m.Comics == nil {
		m.Comics = []*Meta{}
	}
	return &m, nil
}

// Get returns the record for an image file name, or nil.
func (m *Manifest) Get(image string) *Meta {
	i := sort.Search(len(m.Comics), func(i int) bool { return m.Comics[i].Image >= image })
	if i < len(m.Comics) && m.Comics[i].Image == image {
		return m.Comics[i]
	}
	return nil
}

// Put inserts or replaces a record, keeping the list sorted by image name.
func (m *Manifest) Put(meta *Meta) {
	i := sort.Search(len(m.Comics), func(i int) bool { return m.Comics[i].Image >= meta.Image })
	if i < len(m.Comics) && m.Comics[i].Image == meta.Image {
		m.Comics[i] = meta
		return
	}
	m.Comics = append(m.Comics, nil)
	copy(m.Comics[i+1:], m.Comics[i:])
	m.Comics[i] = meta
}

// Save writes via a temp file and rename so an interrupted run never
// leaves a half-written manifest behind.
func (m *Manifest) Save(path string) error {
	m.Version = manifestVersion
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".stripsaw-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(append(data, '\n')); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

// Describe summarizes panel data in the same syntax the prompts accept.
func (m *Meta) Describe() string {
	switch m.Layout {
	case LayoutCols:
		if len(m.ColSplits) == 1 {
			return fmt.Sprintf("cols: %s", FormatSplits(m.ColSplits[0]))
		}
	case LayoutRows:
		return fmt.Sprintf("rows: %s", FormatSplits(m.RowSplits))
	}
	s := fmt.Sprintf("grid: rows %s", FormatSplits(m.RowSplits))
	for i, c := range m.ColSplits {
		s += fmt.Sprintf(" | row %d: %s", i+1, FormatSplits(c))
	}
	return s
}
