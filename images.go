package main

import (
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
		imgs = append(imgs, img)
	}
	if len(skipped) > 0 {
		logger.Warn("skipped files", "files", strings.Join(skipped, ", "))
	}
	return imgs
}
