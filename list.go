package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

const (
	ansiReset  = "\x1b[0m"
	ansiGreen  = "\x1b[32m"
	ansiYellow = "\x1b[33m"
	ansiRed    = "\x1b[31m"
	ansiDim    = "\x1b[2m"
	ansiBold   = "\x1b[1m"
)

func runList(args []string) {
	fl := flag.NewFlagSet("list", flag.ExitOnError)
	extFlag := fl.String("ext", "png,jpg,jpeg", "comma-separated image extensions to include")
	manifestFlag := fl.String("manifest", "", "manifest path (default <dir>/"+DefaultManifest+")")
	todo := fl.Bool("todo", false, "only show images that still need marking")
	long := fl.Bool("long", false, "one image per line with details, even if the list is taller than the terminal")
	thumbs := fl.Bool("thumbs", false, "show a thumbnail of each image with its splits (Kitty graphics)")
	fl.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: stripsaw list [flags] <dir>\n\n")
		fl.PrintDefaults()
	}
	fl.Parse(args)
	if fl.NArg() != 1 {
		fl.Usage()
		os.Exit(2)
	}
	dir := fl.Arg(0)
	mpath := manifestPath(dir, *manifestFlag)
	m, err := LoadManifest(mpath)
	if err != nil {
		fatal(err)
	}
	entries, err := Scan(dir, *extFlag, m)
	if err != nil {
		fatal(err)
	}
	showThumbs := *thumbs
	if showThumbs && !KittyCapable() {
		fmt.Fprintln(os.Stderr, "warning: this terminal may not support the Kitty graphics protocol; skipping thumbnails.")
		showThumbs = false
	}
	printList(os.Stdout, mpath, entries, listOpts{todo: *todo, long: *long, thumbs: showThumbs})
}

type listOpts struct {
	todo    bool   // hide marked and missing entries
	long    bool   // never use the compact multi-column layout
	thumbs  bool   // show a Kitty thumbnail under each image (implies long)
	current string // image to highlight (interactive mode)
	reserve int    // extra lines the caller prints after the list

	// Terminal size in cells; 0 means ask the terminal.
	termCols, termRows int
}

// printList prints the status of each entry, then a summary and progress bar
// last so they stay on screen. If the one-per-line list wouldn't fit in the
// terminal, it switches to a compact multi-column layout.
func printList(w io.Writer, mpath string, entries []Entry, o listOpts) {
	color := useColor()
	paint := func(code, s string) string {
		if !color {
			return s
		}
		return code + s + ansiReset
	}

	counts := map[Status]int{}
	for _, e := range entries {
		counts[e.Status]++
	}
	var visible, problems []Entry
	nameW := 0
	for _, e := range entries {
		if o.todo && (e.Status == StatusMarked || e.Status == StatusMissing) {
			continue
		}
		visible = append(visible, e)
		nameW = max(nameW, utf8.RuneCountInString(e.Name))
		if e.Status == StatusStale || e.Status == StatusMissing || e.Status == StatusBroken {
			problems = append(problems, e)
		}
	}
	nameW = min(nameW, 48)
	images := len(entries) - counts[StatusMissing]

	if o.termCols == 0 || o.termRows == 0 {
		_, _, o.termCols, o.termRows = TermSize()
	}
	const summaryLines = 4 // blank, summary, progress bar, legend
	compact := !o.long && !o.thumbs && o.termRows > 0 && o.termCols > 0 &&
		len(visible)+summaryLines+o.reserve > o.termRows

	if compact {
		printColumns(w, visible, nameW, o, paint)
		if len(problems) > 0 {
			fmt.Fprintln(w)
			for _, e := range problems {
				fmt.Fprintln(w, detailLine(e, nameW, paint))
			}
		}
	} else {
		thumbW := 720
		if xpx, _, cols, _ := TermSize(); xpx > 0 && cols > 0 {
			thumbW = min(thumbW, xpx-2*xpx/cols)
		}
		for _, e := range visible {
			line := detailLine(e, nameW, paint)
			if o.current != "" {
				if e.Name == o.current {
					line = paint(ansiBold, "▶ ") + line + paint(ansiBold, "  ← current")
				} else {
					line = "  " + line
				}
			}
			fmt.Fprintln(w, line)
			if o.thumbs && e.Status != StatusMissing && e.Status != StatusBroken {
				showThumb(w, e, thumbW)
			}
		}
	}

	summary := fmt.Sprintf("%s  %d of %d images marked", paint(ansiBold, filepath.Base(mpath)), counts[StatusMarked], images)
	for _, x := range []struct {
		s    Status
		word string
	}{{StatusUnmarked, "to do"}, {StatusStale, "changed since marking"}, {StatusMissing, "missing"}, {StatusBroken, "unreadable"}} {
		if counts[x.s] > 0 {
			summary += fmt.Sprintf(", %d %s", counts[x.s], x.word)
		}
	}
	fmt.Fprintf(w, "\n%s\n", summary)
	if images > 0 {
		fmt.Fprintln(w, progressBar(counts[StatusMarked], images, 40, paint))
	}
	if compact {
		fmt.Fprintln(w, paint(ansiDim, "✓ marked  · to do  ! changed since marking  ✗ missing or unreadable  (--long for details)"))
	}
}

// printColumns lays entries out column by column, like ls.
func printColumns(w io.Writer, entries []Entry, nameW int, o listOpts, paint func(code, s string) string) {
	const gap = 3
	cellW := 3 + nameW // current marker, status mark, space, name
	ncols := max(1, (o.termCols+gap)/(cellW+gap))
	nrows := (len(entries) + ncols - 1) / ncols
	for r := 0; r < nrows; r++ {
		var line strings.Builder
		for c := 0; c < ncols; c++ {
			i := c*nrows + r
			if i >= len(entries) {
				break
			}
			e := entries[i]
			name := truncate(e.Name, nameW)
			cur := " "
			if e.Name == o.current {
				cur, name = paint(ansiBold, "▶"), paint(ansiBold, name)
			}
			if c > 0 {
				line.WriteString(strings.Repeat(" ", gap))
			}
			line.WriteString(cur + statusMark(e.Status, paint) + " " + name)
			if next := (c+1)*nrows + r; next < len(entries) {
				line.WriteString(strings.Repeat(" ", nameW-utf8.RuneCountInString(truncate(e.Name, nameW))))
			}
		}
		fmt.Fprintln(w, line.String())
	}
}

func statusMark(s Status, paint func(code, s string) string) string {
	switch s {
	case StatusMarked:
		return paint(ansiGreen, "✓")
	case StatusStale:
		return paint(ansiYellow, "!")
	case StatusMissing, StatusBroken:
		return paint(ansiRed, "✗")
	default:
		return paint(ansiDim, "·")
	}
}

// detailLine is the one-per-line description of an entry.
func detailLine(e Entry, nameW int, paint func(code, s string) string) string {
	mark, name := statusMark(e.Status, paint), padRight(e.Name, nameW)
	switch e.Status {
	case StatusMarked:
		return fmt.Sprintf("%s  %s  %s  %s", mark, name,
			paint(ansiDim, fmt.Sprintf("%-10s %2d panels", fmt.Sprintf("%d×%d", e.W, e.H), len(e.Meta.Panels))), e.Meta.Describe())
	case StatusStale:
		return fmt.Sprintf("%s  %s  %s", mark, name,
			paint(ansiYellow, fmt.Sprintf("image is now %d×%d, was %d×%d when marked; re-mark it", e.W, e.H, e.Meta.Width, e.Meta.Height)))
	case StatusMissing:
		return fmt.Sprintf("%s  %s  %s", mark, name, paint(ansiRed, "in manifest, but the file is missing"))
	case StatusBroken:
		return fmt.Sprintf("%s  %s  %s", mark, name, paint(ansiRed, e.Err.Error()))
	default:
		return fmt.Sprintf("%s  %s  %s", mark, name, paint(ansiDim, fmt.Sprintf("%d×%d", e.W, e.H)))
	}
}

func truncate(s string, w int) string {
	if r := []rune(s); len(r) > w {
		return string(r[:w-1]) + "…"
	}
	return s
}

func showThumb(w io.Writer, e Entry, maxW int) {
	src, err := loadImage(e.Path)
	if err != nil {
		fmt.Fprintf(w, "   (%v)\n", err)
		return
	}
	o := RenderOpts{Plain: true}
	if e.Status == StatusMarked {
		o.Layout, o.RowSplits, o.ColSplits = e.Meta.Layout, e.Meta.RowSplits, e.Meta.ColSplits
	}
	fmt.Fprint(w, "   ")
	if err := KittyShow(w, Render(src, maxW, 180, o)); err != nil {
		fmt.Fprintf(w, "(%v)", err)
	}
	fmt.Fprint(w, "\n\n")
}

func progressBar(done, total, width int, paint func(code, s string) string) string {
	filled := done * width / total
	return paint(ansiGreen, strings.Repeat("█", filled)) + paint(ansiDim, strings.Repeat("░", width-filled)) +
		fmt.Sprintf(" %d%%", done*100/total)
}

func padRight(s string, w int) string {
	if n := utf8.RuneCountInString(s); n < w {
		return s + strings.Repeat(" ", w-n)
	}
	return s
}

// useColor is true when stdout is a terminal and NO_COLOR isn't set.
func useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	fi, err := os.Stdout.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}

func manifestPath(dir, flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	return filepath.Join(dir, DefaultManifest)
}
