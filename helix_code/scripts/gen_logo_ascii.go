//go:build ignore

// gen_logo_ascii.go regenerates assets/images/logo-ascii.txt from the brand
// Logo.png as COLORED tview ASCII art (each cell carries a [#rrggbb] tag sampled
// from the image, so the dashboard logo matches assets/Logo.png in shape AND
// colour). It is the §11.4.77 regeneration mechanism for the (derivative)
// logo-ascii.txt. Default size 40x13 fits the dashboard's bordered 15-row header.
//
// Usage (from helix_code/):  go run scripts/gen_logo_ascii.go <src.png> <out.txt> [W] [H]
//   go run scripts/gen_logo_ascii.go ../assets/Logo.png assets/images/logo-ascii.txt
package main

import (
	"fmt"
	"image"
	_ "image/png"
	"os"
	"strconv"
	"strings"
)

func main() {
	src := "../assets/Logo.png"
	out := "assets/images/logo-ascii.txt"
	W, H := 40, 13
	if len(os.Args) > 1 { src = os.Args[1] }
	if len(os.Args) > 2 { out = os.Args[2] }
	if len(os.Args) > 3 { W, _ = strconv.Atoi(os.Args[3]) }
	if len(os.Args) > 4 { H, _ = strconv.Atoi(os.Args[4]) }
	f, err := os.Open(src)
	if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil { fmt.Fprintln(os.Stderr, err); os.Exit(1) }
	b := img.Bounds(); iw, ih := b.Dx(), b.Dy()
	var sb strings.Builder
	for cy := 0; cy < H; cy++ {
		line := strings.Builder{}; last := ""
		for cx := 0; cx < W; cx++ {
			x0, x1 := b.Min.X+cx*iw/W, b.Min.X+(cx+1)*iw/W
			y0, y1 := b.Min.Y+cy*ih/H, b.Min.Y+(cy+1)*ih/H
			var rs, gs, bs, as, n uint64
			for y := y0; y < y1; y += 3 {
				for x := x0; x < x1; x += 3 {
					r, g, bl, a := img.At(x, y).RGBA()
					rs += uint64(r >> 8); gs += uint64(g >> 8); bs += uint64(bl >> 8); as += uint64(a >> 8); n++
				}
			}
			if n == 0 { line.WriteByte(' '); continue }
			r, g, bl, a := rs/n, gs/n, bs/n, as/n
			if a < 70 { // transparent background
				if last != "" { line.WriteString("[-]"); last = "" }
				line.WriteByte(' '); continue
			}
			lum := (299*r + 587*g + 114*bl) / 1000
			var ch byte
			switch {
			case lum > 222: ch = '@'
			case lum > 195: ch = '#'
			case lum > 165: ch = '*'
			default: ch = '+'
			}
			tag := fmt.Sprintf("[#%02x%02x%02x]", r, g, bl)
			if tag != last { line.WriteString(tag); last = tag }
			line.WriteByte(ch)
		}
		sb.WriteString(line.String()); sb.WriteString("[-]\n")
	}
	data := []byte(sb.String())
	if err := os.WriteFile(out, data, 0644); err != nil {
		fmt.Fprintln(os.Stderr, err); os.Exit(1)
	}
	// Mirror to the terminal_ui package so the //go:embed copy stays in sync
	// (the brand asset at assets/images is the runtime-override source; the
	// package copy is the build-time embedded fallback).
	mirror := "applications/terminal_ui/logo-ascii.txt"
	if out == "assets/images/logo-ascii.txt" {
		_ = os.WriteFile(mirror, data, 0644)
		fmt.Printf("wrote %s + %s (%dx%d) from %s\n", out, mirror, W, H, src)
	} else {
		fmt.Printf("wrote %s (%dx%d) from %s\n", out, W, H, src)
	}
}
