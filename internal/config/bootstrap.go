package config

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// bootstrapYAML is a minimal valid config so Load/Validate passes before the user
// completes the in-app wizard. Do not embed a real server_url — each deployment differs.
const bootstrapYAML = `# 喵伴桥接 — 首次运行自动生成；请在向导中输入设备屏幕上的 6 位配对码。
first_run: true
server_url: ""
channels:
  openclaw: true
  telnet: false
openclaw:
  mode: gateway_ws
  base_url: "http://127.0.0.1:18789"
dictation:
  enabled: false
  inject: false
  subtitle: false
`

// EnsureFirstRunConfig returns a path to an existing or newly created config file.
// It prefers `preferred` (usually beside the executable); if that directory is not
// writable, it falls back to os.UserConfigDir()/SaiboAssistant/config.yaml.
func EnsureFirstRunConfig(preferred string) (string, error) {
	_, statErr := os.Stat(preferred)
	if statErr == nil {
		return preferred, nil
	}
	if !os.IsNotExist(statErr) {
		return "", statErr
	}
	writeErr := os.WriteFile(preferred, []byte(bootstrapYAML), 0600)
	if writeErr == nil {
		return preferred, nil
	}
	cfgRoot, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("write config at %s: %w", preferred, writeErr)
	}
	fallback := filepath.Join(cfgRoot, "SaiboAssistant", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(fallback), 0700); err != nil {
		return "", err
	}
	if _, err := os.Stat(fallback); err == nil {
		log.Printf("config: using %s (could not create beside program: %v)", fallback, writeErr)
		return fallback, nil
	}
	if err := os.WriteFile(fallback, []byte(bootstrapYAML), 0600); err != nil {
		return "", fmt.Errorf("write fallback config %s: %w", fallback, err)
	}
	log.Printf("config: created %s (could not write beside program: %v)", fallback, writeErr)
	return fallback, nil
}
