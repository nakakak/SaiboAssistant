package config

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultConfigPath returns the path to config.yaml beside the running executable.
// When running under `go run` or from a temp test binary, it falls back to ./config.yaml in the current directory.
func DefaultConfigPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join(".", "config.yaml"), nil
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		exe, _ = os.Executable()
	}
	exe = filepath.Clean(exe)
	// go test / go run: avoid writing config next to ephemeral exe
	if strings.Contains(exe, "go-build") {
		return filepath.Join(".", "config.yaml"), nil
	}
	dir := filepath.Dir(exe)
	return filepath.Join(dir, "config.yaml"), nil
}
