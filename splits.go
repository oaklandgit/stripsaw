package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// Layout describes how an image is divided into panels.
type Layout string

const (
	LayoutCols Layout = "cols" // a single row of panels, split along x
	LayoutRows Layout = "rows" // a single column of panels, split along y
	LayoutGrid Layout = "grid" // rows split along y, then each row split along x
)

func (l Layout) Next() Layout {
	switch l {
	case LayoutCols:
		return LayoutRows
	case LayoutRows:
		return LayoutGrid
	default:
		return LayoutCols
	}
}

func ParseLayout(s string) (Layout, error) {
	switch l := Layout(strings.ToLower(s)); l {
	case LayoutCols, LayoutRows, LayoutGrid:
		return l, nil
	}
	return "", fmt.Errorf("unknown mode %q (want cols, rows or grid)", s)
}

// Panel is one panel's rectangle, in pixels and as 0–1 fractions of the image.
type Panel struct {
	Index int     `json:"index"`
	Row   int     `json:"row"`
	Col   int     `json:"col"`
	X     int     `json:"x"`
	Y     int     `json:"y"`
	W     int     `json:"w"`
	H     int     `json:"h"`
	PX    float64 `json:"px"`
	PY    float64 `json:"py"`
	PW    float64 `json:"pw"`
	PH    float64 `json:"ph"`
}

// ParseSplits parses a list like "30, 66,92" into strictly increasing
// percentages between 0 and 100 (exclusive). Empty input yields an empty list.
func ParseSplits(s string) ([]float64, error) {
	fields := strings.FieldsFunc(s, func(r rune) bool { return r == ',' || unicode.IsSpace(r) })
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		num := strings.TrimSuffix(f, "%")
		v, err := strconv.ParseFloat(num, 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			return nil, fmt.Errorf("%q is not a number", f)
		}
		if v <= 0 || v >= 100 {
			return nil, fmt.Errorf("%s is out of range (splits must be between 0 and 100)", num)
		}
		if n := len(out); n > 0 && v <= out[n-1] {
			return nil, fmt.Errorf("splits must increase: %s comes after %s", num, FormatSplits(out[n-1:]))
		}
		out = append(out, v)
	}
	return out, nil
}

func FormatSplits(v []float64) string {
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = strconv.FormatFloat(f, 'f', -1, 64)
	}
	return strings.Join(parts, ",")
}

// edges converts percentage splits into pixel boundaries, including 0 and total.
func edges(splits []float64, total int) ([]int, error) {
	e := make([]int, 0, len(splits)+2)
	e = append(e, 0)
	for _, s := range splits {
		e = append(e, int(math.Round(s/100*float64(total))))
	}
	e = append(e, total)
	for i := 1; i < len(e); i++ {
		if e[i] <= e[i-1] {
			return nil, fmt.Errorf("splits are too close together for a %dpx dimension", total)
		}
	}
	return e, nil
}

// BuildPanels turns row splits (y %) and per-row column splits (x %) into
// panels in reading order. colSplits must have one entry per row.
func BuildPanels(w, h int, rowSplits []float64, colSplits [][]float64) ([]Panel, error) {
	if len(colSplits) != len(rowSplits)+1 {
		return nil, fmt.Errorf("have %d rows but %d column lists", len(rowSplits)+1, len(colSplits))
	}
	ye, err := edges(rowSplits, h)
	if err != nil {
		return nil, err
	}
	var panels []Panel
	for r := 0; r+1 < len(ye); r++ {
		xe, err := edges(colSplits[r], w)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", r+1, err)
		}
		for c := 0; c+1 < len(xe); c++ {
			p := Panel{
				Index: len(panels), Row: r, Col: c,
				X: xe[c], Y: ye[r], W: xe[c+1] - xe[c], H: ye[r+1] - ye[r],
			}
			p.PX, p.PY = frac(p.X, w), frac(p.Y, h)
			p.PW, p.PH = frac(p.W, w), frac(p.H, h)
			panels = append(panels, p)
		}
	}
	return panels, nil
}

func frac(v, total int) float64 {
	return math.Round(float64(v)/float64(total)*1e6) / 1e6
}
