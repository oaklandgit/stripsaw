package main

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/png"
	"io"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

const kittyChunk = 4096

// KittyCapable guesses whether the terminal speaks the Kitty graphics protocol.
func KittyCapable() bool {
	term := strings.ToLower(os.Getenv("TERM"))
	prog := strings.ToLower(os.Getenv("TERM_PROGRAM"))
	return strings.Contains(term, "kitty") || strings.Contains(term, "ghostty") ||
		prog == "ghostty" || prog == "wezterm" || os.Getenv("KITTY_WINDOW_ID") != ""
}

// TermSize reports the terminal size in pixels and character cells.
// Pixel values are 0 when the terminal doesn't report them.
func TermSize() (xpx, ypx, cols, rows int) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, 0, 0
	}
	return int(ws.Xpixel), int(ws.Ypixel), int(ws.Col), int(ws.Row)
}

// KittyShow transmits and displays img at the cursor, at 1:1 pixel size.
// q=2 suppresses the terminal's replies so they don't end up on stdin.
func KittyShow(w io.Writer, img image.Image) error {
	var pngBuf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(&pngBuf, img); err != nil {
		return err
	}
	data := base64.StdEncoding.EncodeToString(pngBuf.Bytes())

	var out bytes.Buffer
	for i := 0; i < len(data); i += kittyChunk {
		end := min(i+kittyChunk, len(data))
		more := 1
		if end == len(data) {
			more = 0
		}
		if i == 0 {
			fmt.Fprintf(&out, "\x1b_Ga=T,f=100,q=2,m=%d;", more)
		} else {
			fmt.Fprintf(&out, "\x1b_Gm=%d;", more)
		}
		out.WriteString(data[i:end])
		out.WriteString("\x1b\\")
	}
	_, err := w.Write(out.Bytes())
	return err
}

// ClearScreen deletes all Kitty images, clears the screen and scrollback,
// and homes the cursor.
func ClearScreen(w io.Writer) {
	io.WriteString(w, "\x1b_Ga=d,d=A,q=2\x1b\\\x1b[2J\x1b[3J\x1b[H")
}
