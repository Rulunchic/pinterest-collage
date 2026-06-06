// Package main implements pinterest-collage: a desktop wallpaper generator
// that arranges 12-15 photos in a Pinterest-style masonry layout, then
// applies the result to the Windows desktop background.
package main

import (
	"flag"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"
)

// Windows constants
const (
	smCXVirtualScreen = 78
	smCYVirtualScreen = 79
	smXVirtualScreen  = 76
	smYVirtualScreen  = 77

	spiSetDesktopWallpaper = 0x0014
	spiUpdateIniFile       = 0x01
	spiSendChange          = 0x02
)

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	procGetSystemMetrics     = user32.NewProc("GetSystemMetrics")
	procSystemParametersInfo = user32.NewProc("SystemParametersInfoW")
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// All flags are defined in one FlagSet with hardcoded defaults
	// so flag.Parse() can find every flag the user might pass. After
	// parsing, -init is handled and exits; otherwise we load the
	// config and overlay it on the flags the user did NOT set on the
	// command line. The precedence ends up:
	//
	//     CLI flag  >  config.json  >  built-in default
	//
	initFlag := flag.Bool("init", false, "write a default config.json next to the exe (or to -config-out) and exit")
	configOut := flag.String("config-out", "", "destination for -init (default: <exe-dir>/config.json)")
	configPath := flag.String("config", "", "path to config.json (default: search ./ then <exe-dir> then %APPDATA%/pinterest-collage/)")
	out := flag.String("out", "", "output PNG path (default: %TEMP%\\pinterest-collage-wallpaper.png)")
	interval := flag.Duration("interval", 3*time.Minute, "regenerate and reapply every interval; 0 = one-shot")
	noApply := flag.Bool("no-apply", false, "generate image but don't set as wallpaper")
	minTiles := flag.Int("min", 12, "minimum number of tiles")
	maxTiles := flag.Int("max", 15, "maximum number of tiles")
	imagesDir := flag.String("images", "images", "directory with photos")
	marginTop := flag.Int("margin-top", 40, "top margin in pixels (default: Zebar 30 + GlazeWM 10)")
	marginBottom := flag.Int("margin-bottom", 10, "bottom margin in pixels (GlazeWM outer gap)")
	marginLeft := flag.Int("margin-left", 10, "left margin in pixels (GlazeWM outer gap)")
	marginRight := flag.Int("margin-right", 10, "right margin in pixels (GlazeWM outer gap)")
	gap := flag.Int("gap", 12, "inter-tile gap in pixels")
	install := flag.Bool("install", false, "register for autostart at user logon and exit")
	uninstall := flag.Bool("uninstall", false, "remove the autostart entry and exit")
	flag.Parse()

	if *initFlag {
		out := *configOut
		if out == "" {
			if exe, err := os.Executable(); err == nil {
				out = filepath.Join(filepath.Dir(exe), "config.json")
			} else {
				out = "config.json"
			}
		}
		if err := writeDefaultConfig(out); err != nil {
			logger.Error("init failed", "err", err, "path", out)
			os.Exit(1)
		}
		logger.Info("wrote default config", "path", out)
		return
	}

	cfg := loadConfig(*configPath, logger)
	bg, err := cfg.bg()
	if err != nil {
		logger.Error("bad background_color in config", "value", cfg.BackgroundColor, "err", err)
		os.Exit(2)
	}

	// Overlay config on top of flag values for any flag the user did
	// NOT pass. flag.Visit visits only flags that were set, so this
	// is the canonical way to distinguish "user typed it" from
	// "default value".
	userSet := map[string]bool{}
	flag.Visit(func(f *flag.Flag) { userSet[f.Name] = true })
	if !userSet["min"] {
		*minTiles = cfg.MinTiles
	}
	if !userSet["max"] {
		*maxTiles = cfg.MaxTiles
	}
	if !userSet["interval"] {
		*interval = cfg.interval()
	}
	if !userSet["images"] {
		*imagesDir = cfg.ImagesDir
	}
	if !userSet["margin-top"] {
		*marginTop = cfg.MarginTop
	}
	if !userSet["margin-bottom"] {
		*marginBottom = cfg.MarginBottom
	}
	if !userSet["margin-left"] {
		*marginLeft = cfg.MarginLeft
	}
	if !userSet["margin-right"] {
		*marginRight = cfg.MarginRight
	}
	if !userSet["gap"] {
		*gap = cfg.Gap
	}

	if *minTiles < 1 || *maxTiles < *minTiles {
		logger.Error("invalid tile range", "min", *minTiles, "max", *maxTiles)
		os.Exit(2)
	}

	if *install && *uninstall {
		logger.Error("install and uninstall are mutually exclusive")
		os.Exit(2)
	}
	if *install {
		if err := setAutoStart(logger); err != nil {
			logger.Error("install failed", "err", err)
			os.Exit(1)
		}
		return
	}
	if *uninstall {
		if err := removeAutoStart(logger); err != nil {
			logger.Error("uninstall failed", "err", err)
			os.Exit(1)
		}
		return
	}

	x, y, w, h := screenBounds()
	if w == 0 || h == 0 {
		logger.Error("could not detect screen size")
		os.Exit(1)
	}
	logger.Info("screen detected", "width", w, "height", h, "origin", fmt.Sprintf("(%d,%d)", x, y))

	resolved := resolveImagesDir(*imagesDir)
	images := loadImages(resolved, logger)
	if len(images) == 0 {
		logger.Info("no photos loaded", "dir", resolved)
	} else {
		logger.Info("loaded photos", "count", len(images), "dir", resolved)
	}

	if *out == "" {
		*out = filepath.Join(os.TempDir(), "pinterest-collage-wallpaper.jpg")
	}

	run := func() {
		count := *minTiles + rand.Intn(*maxTiles-*minTiles+1)
		tiles := generate(w, h, count, images, bg, *marginTop, *marginBottom, *marginLeft, *marginRight, *gap)
		img := render(tiles, w, h, bg)
		if err := saveJPEG(img, *out); err != nil {
			logger.Error("save failed", "err", err, "path", *out)
			os.Exit(1)
		}
		logger.Info("wrote png", "path", *out, "tiles", len(tiles),
			"margins", fmt.Sprintf("t=%d,b=%d,l=%d,r=%d", *marginTop, *marginBottom, *marginLeft, *marginRight),
			"gap", *gap)
		if !*noApply {
			if err := setWallpaper(*out); err != nil {
				logger.Error("set wallpaper failed", "err", err)
				os.Exit(1)
			}
			logger.Info("wallpaper applied")
		}
	}

	if *interval > 0 {
		logger.Info("periodic mode", "interval", interval.String())
		for {
			run()
			time.Sleep(*interval)
		}
	}
	run()
}

// resolveImagesDir makes a relative -images path work the same way whether
// the program is launched from the project directory (interactive use) or
// from a logon shortcut whose CWD is elsewhere. Absolute paths are passed
// through untouched.
func resolveImagesDir(given string) string {
	if given == "" || filepath.IsAbs(given) {
		return given
	}
	if _, err := os.Stat(given); err == nil {
		return given
	}
	if exe, err := os.Executable(); err == nil {
		alt := filepath.Join(filepath.Dir(exe), given)
		if _, err := os.Stat(alt); err == nil {
			return alt
		}
	}
	return given
}

// screenBounds returns the virtual screen rectangle covering all monitors.
func screenBounds() (x, y, w, h int) {
	vx, _, _ := procGetSystemMetrics.Call(smXVirtualScreen)
	vy, _, _ := procGetSystemMetrics.Call(smYVirtualScreen)
	vw, _, _ := procGetSystemMetrics.Call(smCXVirtualScreen)
	vh, _, _ := procGetSystemMetrics.Call(smCYVirtualScreen)
	return int(vx), int(vy), int(vw), int(vh)
}

// setWallpaper sets the desktop wallpaper via SystemParametersInfoW.
// The image at path becomes the desktop background; the OS applies the
// user's current wallpaper style (Fill, Stretch, Fit, Tile).
func setWallpaper(path string) error {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	ret, _, _ := procSystemParametersInfo.Call(
		uintptr(spiSetDesktopWallpaper),
		0,
		uintptr(unsafe.Pointer(p)),
		uintptr(spiUpdateIniFile|spiSendChange),
	)
	if ret == 0 {
		return fmt.Errorf("SystemParametersInfoW returned 0")
	}
	return nil
}
