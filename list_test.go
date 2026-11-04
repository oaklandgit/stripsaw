package main

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

func listEntries(n int) []Entry {
	var out []Entry
	for i := 0; i < n; i++ {
		e := Entry{Name: fmt.Sprintf("strip-%03d.png", i), W: 3000, H: 1000}
		if i%3 == 0 {
			e.Status = StatusMarked
			e.Meta = &Meta{Image: e.Name, Width: 3000, Height: 1000, Layout: LayoutCols,
				ColSplits: [][]float64{{50}}, Panels: make([]Panel, 2)}
		}
		out = append(out, e)
	}
	out[4].Status, out[4].Meta = StatusStale, &Meta{Width: 10, Height: 10}
	return out
}

func TestPrintListCompactWhenTall(t *testing.T) {
	var buf bytes.Buffer
	printList(&buf, "panels.json", listEntries(60), listOpts{termCols: 100, termRows: 30, current: "strip-007.png"})
	out := buf.String()
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) > 30 {
		t.Errorf("compact list is %d lines, terminal has 30:\n%s", len(lines), out)
	}
	if !strings.Contains(lines[0], "strip-000.png") || !strings.Contains(lines[0], "strip-") ||
		strings.Count(lines[0], ".png") < 2 {
		t.Errorf("first row should hold several columns: %q", lines[0])
	}
	if !strings.Contains(out, "▶✓ strip-") && !strings.Contains(out, "▶· strip-007.png") {
		t.Errorf("current image not marked:\n%s", out)
	}
	if !strings.Contains(out, "image is now 3000×1000") {
		t.Errorf("stale entry should get a detail line:\n%s", out)
	}
	if !strings.Contains(lines[len(lines)-2], "%") || !strings.Contains(lines[len(lines)-3], "20 of 60 images marked") {
		t.Errorf("summary and progress should be at the bottom:\n%s", strings.Join(lines[len(lines)-4:], "\n"))
	}
}

func TestPrintListLongWhenItFits(t *testing.T) {
	var buf bytes.Buffer
	printList(&buf, "panels.json", listEntries(6), listOpts{termCols: 100, termRows: 30})
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != 6+3 || !strings.Contains(lines[0], "cols: 50") {
		t.Errorf("expected one detailed line per image then summary:\n%s", buf.String())
	}
	if !strings.HasSuffix(lines[len(lines)-1], "%") {
		t.Errorf("progress bar should be last: %q", lines[len(lines)-1])
	}
}
