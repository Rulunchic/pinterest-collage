package main

import (
	"image"
	"image/draw"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"math"
	"os"
	"path/filepath"
	"strings"

	xdraw "golang.org/x/image/draw"
)

// loadImages decodes every JPEG/PNG/GIF file in dir (non-recursive).
// Returns (nil, nil) when dir is empty, missing, or has no recognized
// images — callers should treat that as "fall back to random colors".
func loadImages(dir string, logger *slog.Logger) []image.Image {
	if dir == "" {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		logger.Warn("read dir failed", "dir", dir, "err", err)
		return nil
	}
	var imgs []image.Image
	var skipped []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		switch ext {
		case ".jpg", ".jpeg", ".png", ".gif":
		default:
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, err := os.Open(path)
		if err != nil {
			skipped = append(skipped, e.Name()+": open")
			continue
		}
		img, _, err := image.Decode(f)
		f.Close()
		if err != nil {
			skipped = append(skipped, e.Name()+": decode")
			continue
		}
		imgs = append(imgs, preResize(img))
	}
	if len(skipped) > 0 {
		logger.Warn("skipped files", "files", strings.Join(skipped, ", "))
	}
	return imgs
}

// maxPreloadDim is the max width or height of a pre-resized source image.
// Smaller sources mean faster per-frame scaling.
const maxPreloadDim = 1200

// preResize scales img down so neither dimension exceeds maxPreloadDim.
// Images already within bounds are returned as-is.
func preResize(img image.Image) image.Image {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= maxPreloadDim && h <= maxPreloadDim {
		return img
	}
	scale := math.Min(float64(maxPreloadDim)/float64(w), float64(maxPreloadDim)/float64(h))
	nw := int(math.Round(float64(w) * scale))
	nh := int(math.Round(float64(h) * scale))
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	xdraw.BiLinear.Scale(dst, dst.Bounds(), img, b, draw.Over, nil)
	return dst
}
