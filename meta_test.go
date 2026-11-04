package main

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func mustMeta(t *testing.T, name string, w, h int, cols []float64) *Meta {
	t.Helper()
	m, err := NewMeta(name, w, h, LayoutCols, nil, [][]float64{cols})
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func TestManifestPutGet(t *testing.T) {
	m := &Manifest{}
	m.Put(mustMeta(t, "b.png", 100, 100, []float64{50}))
	m.Put(mustMeta(t, "a.png", 100, 100, nil))
	m.Put(mustMeta(t, "c.png", 100, 100, nil))
	m.Put(mustMeta(t, "b.png", 100, 100, []float64{25, 75})) // replace

	var names []string
	for _, c := range m.Comics {
		names = append(names, c.Image)
	}
	if !reflect.DeepEqual(names, []string{"a.png", "b.png", "c.png"}) {
		t.Errorf("order = %v", names)
	}
	if got := m.Get("b.png"); got == nil || len(got.Panels) != 3 {
		t.Errorf("Get(b.png) = %+v", got)
	}
	if m.Get("zzz.png") != nil || m.Get("0.png") != nil {
		t.Error("Get of absent image should be nil")
	}
}

func TestManifestSaveLoad(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, DefaultManifest)

	empty, err := LoadManifest(path)
	if err != nil || len(empty.Comics) != 0 {
		t.Fatalf("missing manifest: %+v, %v", empty, err)
	}

	m := &Manifest{}
	m.Put(mustMeta(t, "comic.png", 3000, 1000, []float64{30, 66}))
	if err := m.Save(path); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	if !strings.Contains(string(data), `"rowSplits": []`) || !strings.Contains(string(data), `"version": 1`) {
		t.Errorf("unexpected JSON:\n%s", data)
	}
	got, err := LoadManifest(path)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, m) {
		t.Errorf("round trip mismatch:\n got %+v\nwant %+v", got, m)
	}
	entries, _ := os.ReadDir(dir)
	if len(entries) != 1 {
		t.Errorf("temp file left behind: %v", entries)
	}
	if got.Comics[0].Describe() != "cols: 30,66" {
		t.Errorf("Describe = %q", got.Comics[0].Describe())
	}
}

func TestNewMetaRowsHasEmptyColArrays(t *testing.T) {
	m, err := NewMeta("x.png", 100, 300, LayoutRows, []float64{50}, [][]float64{nil, nil})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range m.ColSplits {
		if c == nil {
			t.Error("nil column list would marshal as null")
		}
	}
	if m.Describe() != "rows: 50" {
		t.Errorf("Describe = %q", m.Describe())
	}
}

func TestScan(t *testing.T) {
	dir := t.TempDir()
	writePNG := func(name string, w, h int) {
		f, err := os.Create(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		png.Encode(f, image.NewGray(image.Rect(0, 0, w, h)))
		f.Close()
	}
	writePNG("marked.png", 100, 50)
	writePNG("stale.png", 200, 50)
	writePNG("todo.PNG", 10, 10)
	os.WriteFile(filepath.Join(dir, "broken.png"), []byte("nope"), 0o644)
	os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("x"), 0o644)

	m := &Manifest{}
	m.Put(mustMeta(t, "marked.png", 100, 50, nil))
	m.Put(mustMeta(t, "stale.png", 100, 50, nil))
	m.Put(mustMeta(t, "gone.png", 100, 50, nil))

	entries, err := Scan(dir, "png,jpg", m)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Status{
		"broken.png": StatusBroken, "gone.png": StatusMissing, "marked.png": StatusMarked,
		"stale.png": StatusStale, "todo.PNG": StatusUnmarked,
	}
	if len(entries) != len(want) {
		t.Fatalf("got %d entries: %+v", len(entries), entries)
	}
	for _, e := range entries {
		if want[e.Name] != e.Status {
			t.Errorf("%s: status %d, want %d", e.Name, e.Status, want[e.Name])
		}
	}
}
