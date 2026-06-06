package main

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"golang.org/x/sys/windows/registry"
)

const autoStartKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const autoStartName = "pinterest-collage"

// setAutoStart registers the program under HKCU\...\Run so Windows
// launches it at user logon. The value is the quoted absolute path of
// the running executable; no extra args are written (the program uses
// the config-driven interval — 3 minutes by default — out of the box).
func setAutoStart(logger *slog.Logger) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate exe: %w", err)
	}
	key, _, err := registry.CreateKey(registry.CURRENT_USER, autoStartKey, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("open run key: %w", err)
	}
	defer key.Close()

	if err := key.SetStringValue(autoStartName, strconv.Quote(exe)); err != nil {
		return fmt.Errorf("set value: %w", err)
	}
	logger.Info("autostart registered", "key", `HKCU\`+autoStartKey, "name", autoStartName, "exe", exe)
	return nil
}

// removeAutoStart deletes the autostart value. Missing entry is not an
// error: a no-op uninstall should still report success.
func removeAutoStart(logger *slog.Logger) error {
	key, err := registry.OpenKey(registry.CURRENT_USER, autoStartKey, registry.SET_VALUE)
	if err != nil {
		if err == registry.ErrNotExist {
			logger.Info("autostart not present, nothing to remove")
			return nil
		}
		return fmt.Errorf("open run key: %w", err)
	}
	defer key.Close()

	if err := key.DeleteValue(autoStartName); err != nil {
		if err == registry.ErrNotExist {
			logger.Info("autostart not present, nothing to remove")
			return nil
		}
		return fmt.Errorf("delete value: %w", err)
	}
	logger.Info("autostart removed", "name", autoStartName)
	return nil
}
