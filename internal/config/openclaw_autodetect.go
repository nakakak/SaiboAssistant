package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type openclawJSON struct {
	Gateway struct {
		Port int `json:"port"`
		Auth struct {
			Token string `json:"token"`
			Mode  string `json:"mode"`
		} `json:"auth"`
	} `json:"gateway"`
}

// OpenClawConfigPath 返回本机 openclaw.json 路径（~/.openclaw/openclaw.json）。
func OpenClawConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".openclaw", "openclaw.json"), nil
}

// ApplyOpenClawAutodetect 从 openclaw.json 填充 gateway_token / base_url（不覆盖用户已填项）。
func ApplyOpenClawAutodetect(c *Config) {
	if c == nil {
		return
	}
	path, err := OpenClawConfigPath()
	if err != nil {
		return
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return
	}
	var doc openclawJSON
	if err := json.Unmarshal(raw, &doc); err != nil {
		return
	}
	port := doc.Gateway.Port
	if port <= 0 {
		port = 18789
	}
	if strings.TrimSpace(c.OpenClaw.GatewayToken) == "" && strings.TrimSpace(c.OpenClaw.BearerToken) == "" {
		if tok := strings.TrimSpace(doc.Gateway.Auth.Token); tok != "" {
			c.OpenClaw.GatewayToken = tok
		}
	}
	if strings.TrimSpace(c.OpenClaw.BaseURL) == "" {
		c.OpenClaw.BaseURL = fmt.Sprintf("http://127.0.0.1:%d", port)
	}
	if strings.TrimSpace(c.OpenClaw.GatewayWSURL) == "" {
		c.OpenClaw.GatewayWSURL = fmt.Sprintf("ws://127.0.0.1:%d", port)
	}
}
