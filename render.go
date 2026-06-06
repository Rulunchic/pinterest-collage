package main

import (
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"math"
	"os"
	"sync"

	xdraw "golang.org/x/image/draw"
)

// render paints the canvas in `bg` first, then pastes each tile's
// photo (with rounded corners) on top. The inter-tile gap and the
// outer margins are just the canvas showing through.
func render(tiles []Tile, width, height int, bg color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(img, img.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)

	for _, t := range tiles {
		if t.Img == nil {
			continue
		}
		scaled := scaleFit(t.Img, t.W, t.H)
		r := image.Rect(t.X, t.Y, t.X+t.W, t.Y+t.H)
		m := roundedMask(t.W, t.H, cornerRadius(t.W, t.H))
		draw.DrawMask(img, r, scaled, image.Point{}, m, image.Point{}, draw.Over)
	}
	return img
}

// scaleFit scales src to exactly w×h using BiLinear (fast, good quality).
func scaleFit(src image.Image, w, h int) *image.RGBA {
	sb := src.Bounds()
	sw, sh := sb.Dx(), sb.Dy()
	if sw == 0 || sh == 0 || w == 0 || h == 0 {
		return image.NewRGBA(image.Rect(0, 0, w, h))
	}

	scale := math.Min(float64(w)/float64(sw), float64(h)/float64(sh))
	newW := int(math.Round(float64(sw) * scale))
	newH := int(math.Round(float64(sh) * scale))
	if newW < 1 {
		newW = 1
	}
	if newH < 1 {
		newH = 1
	}

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), src, sb, draw.Over, nil)
	return dst
}

// cornerRadius picks a per-tile corner radius. Pinterest-style: small
// and uniform, scaling with the smaller tile dimension, clamped to
// 8-16 px.
func cornerRadius(w, h int) int {
	r := w
	if h < w {
		r = h
	}
	r /= 20
	if r < 8 {
		r = 8
	}
	if r > 16 {
		r = 16
	}
	return r
}

// maskCache avoids recomputing identical masks every frame.
var maskCache sync.Map // key: [2]int{w,h} → *image.Alpha

// roundedMask returns a cached alpha mask for the given tile size.
func roundedMask(w, h, r int) *image.Alpha {
	key := [2]int{w, h}
	if v, ok := maskCache.Load(key); ok {
		return v.(*image.Alpha)
	}
	m := image.NewAlpha(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			dx, dy := 0, 0
			if x < r {
				dx = r - x
			}
			if x >= w-r {
				dx = x - (w - r - 1)
			}
			if y < r {
				dy = r - y
			}
			if y >= h-r {
				dy = y - (h - r - 1)
			}
			a := uint8(255)
			if dx > 0 && dy > 0 {
				d := math.Sqrt(float64(dx*dx + dy*dy))
				if d > float64(r) {
					a = 0
				} else if d > float64(r)-1 {
					a = uint8((float64(r) - d) * 255)
				}
			}
			m.SetAlpha(x, y, color.Alpha{A: a})
		}
	}
	maskCache.Store(key, m)
	return m
}

func saveJPEG(img *image.RGBA, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return jpeg.Encode(f, img, &jpeg.Options{Quality: 95})
}

