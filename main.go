// Command stripsaw steps through a directory of comic images, shows each one
// in the terminal with a ruler overlay, and records the panel splits you type
// into a per-directory manifest (panels.json).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	_ "golang.org/x/image/webp"
)

const usageText = `Usage:
  stripsaw [mark] [flags] <dir>   mark panels on images that aren't in the manifest yet
  stripsaw list [flags] <dir>     show which images are marked

Run "stripsaw list -h" for list flags.`

const helpText = `Enter splits as comma-separated percentages read off the ruler, e.g. 30,66,92.
Leave the line empty for a single panel.

Modes (switch at any prompt; the mode stays on for later images):
  c   cols  one row of panels, split along x
  r   rows  one column of stacked panels, split along y
  g   grid  rows split along y first, then each row split along x
  m   cycle cols -> rows -> grid

Other commands:
  s   skip this image (nothing is saved)
  l   list all images and progress, then return here
  q   quit
  ?   show this help`

// step is the result of a prompt: carry on, or a command that interrupts it.
type step int

const (
	stepDone step = iota
	stepAgain
	stepRedraw
	stepSkip
	stepQuit
)

type session struct {
	mode     Layout
	in       *bufio.Reader
	out      io.Writer
	manifest *Manifest
	mpath    string
	dir      string
	ext      string
	current  string // image being marked
	saved    int
	skipped  int
	flash    string // message shown once under the next redraw
	title    string // header line for the current image
}

func main() {
	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "list":
			runList(args[1:])
			return
		case "mark":
			args = args[1:]
		}
	}
	runMark(args)
}

func runMark(args []string) {
	fl := flag.NewFlagSet("mark", flag.ExitOnError)
	modeFlag := fl.String("mode", "cols", "starting mode: cols, rows or grid (change any time with c/r/g/m)")
	redo := fl.Bool("redo", false, "also show images that are already in the manifest")
	extFlag := fl.String("ext", "png,jpg,jpeg", "comma-separated image extensions to include")
	manifestFlag := fl.String("manifest", "", "manifest path (default <dir>/"+DefaultManifest+")")
	fl.Usage = func() {
		fmt.Fprintf(os.Stderr, "%s\n\nMark flags:\n", usageText)
		fl.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\n%s\n", helpText)
	}
	fl.Parse(args)
	if fl.NArg() != 1 {
		fl.Usage()
		os.Exit(2)
	}
	mode, err := ParseLayout(*modeFlag)
	if err != nil {
		fatal(err)
	}

	dir := fl.Arg(0)
	mpath := manifestPath(dir, *manifestFlag)
	manifest, err := LoadManifest(mpath)
	if err != nil {
		fatal(err)
	}
	entries, err := Scan(dir, *extFlag, manifest)
	if err != nil {
		fatal(err)
	}
	var pending []Entry
	images := 0
	for _, e := range entries {
		if e.Status == StatusMissing {
			continue
		}
		images++
		if *redo || e.Status == StatusUnmarked || e.Status == StatusStale || e.Status == StatusBroken {
			pending = append(pending, e)
		}
	}
	if images == 0 {
		fatal(fmt.Errorf("no images with extensions %s in %s", *extFlag, dir))
	}
	if len(pending) == 0 {
		fmt.Printf("All %d images in %s are in %s. Use --redo to revisit them.\n", images, dir, filepath.Base(mpath))
		return
	}
	if !KittyCapable() {
		fmt.Fprintln(os.Stderr, "warning: this terminal may not support the Kitty graphics protocol (works in Ghostty, kitty, WezTerm).")
	}

	s := &session{mode: mode, in: bufio.NewReader(os.Stdin), out: os.Stdout, manifest: manifest, mpath: mpath, dir: dir, ext: *extFlag}
	quit := false
	for i, e := range pending {
		st, err := s.markImage(i, len(pending), e)
		switch {
		case err != nil:
			s.flash = fmt.Sprintf("✗ %s: %v (skipped)", e.Name, err)
			s.skipped++
		case st == stepQuit:
			quit = true
		case st == stepSkip:
			s.skipped++
		default:
			s.saved++
			s.flash = fmt.Sprintf("✓ saved %s to %s", e.Name, filepath.Base(mpath))
		}
		if quit {
			break
		}
	}

	ClearScreen(s.out)
	if s.flash != "" && !quit {
		fmt.Println(s.flash)
	}
	marked := 0
	if after, err := Scan(dir, *extFlag, manifest); err == nil {
		for _, e := range after {
			if e.Status == StatusMarked {
				marked++
			}
		}
	}
	fmt.Printf("Saved %d, skipped %d. %d of %d images in %s are marked.\n", s.saved, s.skipped, marked, images, dir)
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "stripsaw:", err)
	os.Exit(1)
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// markImage runs the prompt/preview loop for one image until it is saved,
// skipped, or the user quits.
func (s *session) markImage(i, n int, e Entry) (step, error) {
	if e.Err != nil {
		return stepSkip, e.Err
	}
	src, err := loadImage(e.Path)
	if err != nil {
		return stepSkip, err
	}
	s.current = e.Name
	w, h := src.Bounds().Dx(), src.Bounds().Dy()
	var previous string
	if e.Meta != nil {
		previous = e.Meta.Describe()
		if e.Status == StatusStale {
			previous += fmt.Sprintf("  (marked when the image was %d×%d)", e.Meta.Width, e.Meta.Height)
		}
	}

outer:
	for {
		s.title = fmt.Sprintf("[%d/%d] %s  %d×%d  mode: %s   (c/r/g/m mode · s skip · l list · q quit · ? help)",
			i+1, n, e.Name, w, h, s.mode)
		s.draw(src, RenderOpts{Layout: s.mode})
		if previous != "" {
			fmt.Fprintf(s.out, "previous: %s\n", previous)
		}

		rows, cols, st := s.collect(src)
		switch st {
		case stepRedraw:
			continue
		case stepSkip, stepQuit:
			return st, nil
		}

		meta, err := NewMeta(e.Name, w, h, s.mode, rows, cols)
		if err != nil {
			s.flash = "✗ " + err.Error()
			continue
		}

		s.draw(src, RenderOpts{Layout: s.mode, RowSplits: rows, ColSplits: cols, Numbers: true})
		for {
			line, ok := s.ask(fmt.Sprintf("%d panels. Enter = save · e = edit · s = skip · q = quit > ", len(meta.Panels)))
			if !ok {
				return stepQuit, nil
			}
			switch strings.ToLower(line) {
			case "":
				s.manifest.Put(meta)
				if err := s.manifest.Save(s.mpath); err != nil {
					return stepSkip, err
				}
				return stepDone, nil
			case "e":
				continue outer
			}
			switch st, _ := s.command(line); st {
			case stepRedraw:
				continue outer
			case stepSkip, stepQuit:
				return st, nil
			case stepAgain:
				continue
			}
			fmt.Fprintln(s.out, "  press Enter to save, or e to edit")
		}
	}
}

// collect prompts for the splits of the current mode. Rows mode and cols mode
// use the same shape as grid: rowSplits plus one column list per row.
func (s *session) collect(src image.Image) ([]float64, [][]float64, step) {
	switch s.mode {
	case LayoutCols:
		v, st := s.askSplits("[cols] x splits %> ")
		return []float64{}, [][]float64{v}, st
	case LayoutRows:
		v, st := s.askSplits("[rows] y splits %> ")
		cols := make([][]float64, len(v)+1)
		for i := range cols {
			cols[i] = []float64{}
		}
		return v, cols, st
	}

	rows, st := s.askSplits("[grid] row splits (y%)> ")
	if st != stepDone {
		return nil, nil, st
	}
	if len(rows) > 0 {
		// Show the rows so the per-row prompts can refer to their numbers.
		s.draw(src, RenderOpts{Layout: s.mode, RowSplits: rows, ColSplits: make([][]float64, len(rows)+1), Numbers: true})
	}
	cols := make([][]float64, len(rows)+1)
	for r := range cols {
		prompt := "[grid] x splits %> "
		if len(rows) > 0 {
			prompt = fmt.Sprintf("  row %d x splits %%> ", r+1)
		}
		v, st := s.askSplits(prompt)
		if st != stepDone {
			return nil, nil, st
		}
		cols[r] = v
	}
	return rows, cols, stepDone
}

// askSplits prompts until it gets a valid split list or a command.
func (s *session) askSplits(prompt string) ([]float64, step) {
	for {
		line, ok := s.ask(prompt)
		if !ok {
			return nil, stepQuit
		}
		if st, isCmd := s.command(line); isCmd {
			if st == stepAgain {
				continue
			}
			return nil, st
		}
		v, err := ParseSplits(line)
		if err != nil {
			fmt.Fprintf(s.out, "  ✗ %v\n", err)
			continue
		}
		return v, stepDone
	}
}

// command handles single-letter commands. Mode changes take effect
// immediately and persist for the following images.
func (s *session) command(line string) (step, bool) {
	switch strings.ToLower(line) {
	case "c":
		s.mode = LayoutCols
	case "r":
		s.mode = LayoutRows
	case "g":
		s.mode = LayoutGrid
	case "m":
		s.mode = s.mode.Next()
	case "s":
		return stepSkip, true
	case "l":
		if !s.showList() {
			return stepQuit, true
		}
	case "q":
		return stepQuit, true
	case "?", "h", "help":
		fmt.Fprintln(s.out, helpText)
		return stepAgain, true
	default:
		return stepDone, false
	}
	return stepRedraw, true
}

// ask prints prompt and reads a trimmed line; ok is false at end of input.
func (s *session) ask(prompt string) (string, bool) {
	fmt.Fprint(s.out, prompt)
	line, err := s.in.ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(s.out)
		return "", false
	}
	return strings.TrimSpace(line), true
}

// draw clears the screen and shows the header, any flash message, and the
// rendered image sized to leave room for the prompts below it.
func (s *session) draw(src image.Image, o RenderOpts) {
	const reservedRows = 9
	maxW, maxH := 1600, 900
	if xpx, ypx, cols, rows := TermSize(); xpx > 0 && ypx > 0 && cols > 0 && rows > reservedRows {
		cellW, cellH := xpx/cols, ypx/rows
		maxW, maxH = xpx-cellW, ypx-reservedRows*cellH
	}
	ClearScreen(s.out)
	fmt.Fprintln(s.out, s.title)
	if s.flash != "" {
		fmt.Fprintln(s.out, s.flash)
		s.flash = ""
	}
	if err := KittyShow(s.out, Render(src, maxW, maxH, o)); err != nil {
		fmt.Fprintf(s.out, "✗ display failed: %v\n", err)
	}
	fmt.Fprintln(s.out)
}

// showList clears the screen, prints every image's status with the current
// one highlighted, and waits for Enter. It returns false at end of input.
func (s *session) showList() bool {
	ClearScreen(s.out)
	entries, err := Scan(s.dir, s.ext, s.manifest)
	if err != nil {
		fmt.Fprintf(s.out, "✗ %v\n", err)
	} else {
		printList(s.out, s.mpath, entries, listOpts{current: s.current, reserve: 3})
	}
	fmt.Fprintf(s.out, "This session: %d saved, %d skipped.\n\n", s.saved, s.skipped)
	_, ok := s.ask("Press Enter to go back > ")
	return ok
}
