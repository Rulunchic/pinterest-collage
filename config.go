package main

import (
	"encoding/json"
	"fmt"
	"image/color"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config is the on-disk shape of config.json. Field tags drive JSON
// decoding; defaults are baked in below so a missing file is a no-op.
type Config struct {
	MinTiles        int    `json:"min_tiles"`
	MaxTiles        int    `json:"max_tiles"`
	IntervalSeconds int    `json:"interval_seconds"`
	MarginTop       int    `json:"margin_top"`
	MarginBottom    int    `json:"margin_bottom"`
	MarginLeft      int    `json:"margin_left"`
	MarginRight     int    `json:"margin_right"`
	Gap             int    `json:"gap"`
	ImagesDir       string `json:"images_dir"`
	BackgroundColor string `json:"background_color"`
}

func defaultConfig() Config {
	return Config{
		MinTiles:        12,
		MaxTiles:        15,
		IntervalSeconds: 180,
		MarginTop:       40,
		MarginBottom:    10,
		MarginLeft:      10,
		MarginRight:     10,
		Gap:             12,
		ImagesDir:       "images",
		BackgroundColor: "#f0edec",
	}
}

func (c Config) interval() time.Duration {
	if c.IntervalSeconds < 0 {
		return 0
	}
	return time.Duration(c.IntervalSeconds) * time.Second
}

func (c Config) bg() (color.RGBA, error) {
	return parseHexColor(c.BackgroundColor)
}

// loadConfig reads the first config.json it finds. Lookup order:
//  1. the explicit -config path, if any
//  2. ./config.json (CWD)
//  3. <exe-dir>/config.json
//  4. %APPDATA%\pinterest-collage\config.json
//
// A missing file is fine — defaults are returned. A present-but-broken
// file is logged and ignored.
func loadConfig(explicitPath string, logger *slog.Logger) Config {
	cfg := defaultConfig()

	paths := configSearchPaths(explicitPath)
	for _, p := range paths {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		var got Config
		if err := json.Unmarshal(data, &got); err != nil {
			logger.Warn("config is invalid JSON, using defaults", "path", p, "err", err)
			return cfg
		}
		logger.Info("loaded config", "path", p)
		return mergeConfig(cfg, got)
	}
	return cfg
}

func configSearchPaths(explicit string) []string {
	paths := []string{}
	if explicit != "" {
		paths = append(paths, explicit)
	}
	paths = append(paths, "config.json")
	if exe, err := os.Executable(); err == nil {
		paths = append(paths, filepath.Join(filepath.Dir(exe), "config.json"))
	}
	if appData, err := os.UserConfigDir(); err == nil {
		paths = append(paths, filepath.Join(appData, "pinterest-collage", "config.json"))
	}
	return paths
}

// mergeConfig layers got on top of base. Zero values in got (which
// JSON treats as missing) leave base untouched.
func mergeConfig(base, got Config) Config {
	if got.MinTiles != 0 {
		base.MinTiles = got.MinTiles
	}
	if got.MaxTiles != 0 {
		base.MaxTiles = got.MaxTiles
	}
	if got.IntervalSeconds != 0 {
		base.IntervalSeconds = got.IntervalSeconds
	}
	if got.MarginTop != 0 {
		base.MarginTop = got.MarginTop
	}
	if got.MarginBottom != 0 {
		base.MarginBottom = got.MarginBottom
	}
	if got.MarginLeft != 0 {
		base.MarginLeft = got.MarginLeft
	}
	if got.MarginRight != 0 {
		base.MarginRight = got.MarginRight
	}
	if got.Gap != 0 {
		base.Gap = got.Gap
	}
	if got.ImagesDir != "" {
		base.ImagesDir = got.ImagesDir
	}
	if got.BackgroundColor != "" {
		base.BackgroundColor = got.BackgroundColor
	}
	return base
}

// writeDefaultConfig writes the default config as nicely-indented JSON
// to path. Used by -init to seed a fresh config file.
func writeDefaultConfig(path string) error {
	data, err := json.MarshalIndent(defaultConfig(), "", "    ")
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, data, 0o644)
}

// parseHexColor parses "#rrggbb" or "rrggbb" into a color.RGBA.
func parseHexColor(s string) (color.RGBA, error) {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "#")
	if len(s) != 6 {
		return color.RGBA{}, fmt.Errorf("color must be 6 hex digits, got %q", s)
	}
	hex := func(off int) (uint8, error) {
		v, err := strconv.ParseUint(s[off:off+2], 16, 8)
		if err != nil {
			return 0, err
		}
		return uint8(v), nil
	}
	r, err := hex(0)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("bad red channel: %w", err)
	}
	g, err := hex(2)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("bad green channel: %w", err)
	}
	b, err := hex(4)
	if err != nil {
		return color.RGBA{}, fmt.Errorf("bad blue channel: %w", err)
	}
	return color.RGBA{R: r, G: g, B: b, A: 0xff}, nil
}
