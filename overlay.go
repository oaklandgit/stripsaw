package main

import (
	"image"
	"image/color"
	"math"
	"strconv"

	"golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	rulerTop  = 30 // height of the x ruler margin
	rulerLeft = 44 // width of the y ruler margin
	edgePad   = 14 // padding on sides without a ruler, so end labels aren't clipped
	splitLine = 3  // thickness of split lines in the preview
)

var (
	bgColor    = color.RGBA{0x1e, 0x1e, 0x1e, 0xff}
	tickColor  = color.RGBA{0xdd, 0xdd, 0xdd, 0xff}
	guide10    = color.NRGBA{0x00, 0xa0, 0xff, 0x90}
	guide5     = color.NRGBA{0x00, 0xa0, 0xff, 0x40}
	splitColor = color.NRGBA{0xff, 0x20, 0x50, 0xff}
)

// RenderOpts controls what Render draws on top of the image.
type RenderOpts struct {
	Layout    Layout
	RowSplits []float64
	ColSplits [][]float64 // one list per row; may be shorter than the row count
	Numbers   bool        // label each panel with its number
	Plain     bool        // no rulers, guides or margins (thumbnails)
}

// Render scales src to fit maxW×maxH and adds rulers, guides and any splits.
func Render(src image.Image, maxW, maxH int, o RenderOpts) *image.RGBA {
	xRuler := !o.Plain && o.Layout != LayoutRows
	yRuler := !o.Plain && o.Layout != LayoutCols

	left, top, right, bottom := edgePad, 8, edgePad, 8
	if o.Plain {
		left, top, right, bottom = 0, 0, 0, 0
	}
	if xRuler {
		top = rulerTop
	}
	if yRuler {
		left = rulerLeft
	}

	sb := src.Bounds()
	scale := math.Min(float64(maxW-left-right)/float64(sb.Dx()), float64(maxH-top-bottom)/float64(sb.Dy()))
	dw := max(1, int(math.Round(float64(sb.Dx())*scale)))
	dh := max(1, int(math.Round(float64(sb.Dy())*scale)))

	canvas := image.NewRGBA(image.Rect(0, 0, left+dw+right, top+dh+bottom))
	draw.Draw(canvas, canvas.Bounds(), image.NewUniform(bgColor), image.Point{}, draw.Src)
	dr := image.Rect(left, top, left+dw, top+dh)
	draw.Draw(canvas, dr, image.White, image.Point{}, draw.Src) // backdrop for transparent PNGs
	draw.CatmullRom.Scale(canvas, dr, src, sb, draw.Over, nil)

	xAt := func(p float64) int { return left + int(math.Round(p/100*float64(dw))) }
	yAt := func(p float64) int { return top + int(math.Round(p/100*float64(dh))) }

	if xRuler {
		for p := 0; p <= 100; p++ {
			x := xAt(float64(p))
			if p > 0 && p < 100 && p%5 == 0 {
				fill(canvas, image.Rect(x, top, x+1, top+dh), guideFor(p))
			}
			n := tickLen(p)
			fill(canvas, image.Rect(x, top-1-n, x+1, top-1), tickColor)
			if p%10 == 0 {
				s := strconv.Itoa(p)
				drawText(canvas, s, x-textWidth(s)/2, top-15, tickColor)
			}
		}
	}
	if yRuler {
		for p := 0; p <= 100; p++ {
			y := yAt(float64(p))
			if p > 0 && p < 100 && p%5 == 0 {
				fill(canvas, image.Rect(left, y, left+dw, y+1), guideFor(p))
			}
			n := tickLen(p)
			fill(canvas, image.Rect(left-1-n, y, left-1, y+1), tickColor)
			if p%10 == 0 {
				s := strconv.Itoa(p)
				drawText(canvas, s, left-15-textWidth(s), y+4, tickColor)
			}
		}
	}

	// Split lines: full-width row lines, then column lines within each row.
	ye := append(append([]float64{0}, o.RowSplits...), 100)
	for _, p := range o.RowSplits {
		y := yAt(p)
		fill(canvas, image.Rect(left, y-splitLine/2, left+dw, y-splitLine/2+splitLine), splitColor)
	}
	for r, cols := range o.ColSplits {
		if r+1 >= len(ye) {
			break
		}
		y0, y1 := yAt(ye[r]), yAt(ye[r+1])
		for _, p := range cols {
			x := xAt(p)
			fill(canvas, image.Rect(x-splitLine/2, y0, x-splitLine/2+splitLine, y1), splitColor)
		}
	}

	if o.Numbers {
		cols := make([][]float64, len(o.RowSplits)+1)
		copy(cols, o.ColSplits)
		if panels, err := BuildPanels(dw, dh, o.RowSplits, cols); err == nil {
			for _, p := range panels {
				drawBigLabel(canvas, strconv.Itoa(p.Index+1), left+p.X+p.W/2, top+p.Y+p.H/2, 3)
			}
		}
	}
	return canvas
}

func tickLen(p int) int {
	switch {
	case p%10 == 0:
		return 11
	case p%5 == 0:
		return 7
	default:
		return 3
	}
}

func guideFor(p int) color.Color {
	if p%10 == 0 {
		return guide10
	}
	return guide5
}

func fill(dst draw.Image, r image.Rectangle, c color.Color) {
	draw.Draw(dst, r, image.NewUniform(c), image.Point{}, draw.Over)
}

func textWidth(s string) int {
	return font.MeasureString(basicfont.Face7x13, s).Ceil()
}

// drawText draws s with its baseline at y.
func drawText(dst draw.Image, s string, x, y int, c color.Color) {
	d := font.Drawer{Dst: dst, Src: image.NewUniform(c), Face: basicfont.Face7x13, Dot: fixed.P(x, y)}
	d.DrawString(s)
}

// drawBigLabel draws s in a white box centered on (cx, cy), enlarged by scale.
func drawBigLabel(dst draw.Image, s string, cx, cy, scale int) {
	w, h := textWidth(s)+6, 16
	tmp := image.NewRGBA(image.Rect(0, 0, w, h))
	draw.Draw(tmp, tmp.Bounds(), image.NewUniform(color.NRGBA{0xff, 0xff, 0xff, 0xe0}), image.Point{}, draw.Src)
	drawText(tmp, s, 3, 12, splitColor)
	sw, sh := w*scale, h*scale
	r := image.Rect(cx-sw/2, cy-sh/2, cx-sw/2+sw, cy-sh/2+sh)
	draw.NearestNeighbor.Scale(dst, r, tmp, tmp.Bounds(), draw.Over, nil)
}
