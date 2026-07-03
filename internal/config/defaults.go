package config

import (
	"os"
	"strings"
)

// DefaultBridgeServerURL 内置商家云桥接地址；可用环境变量 MIAOBAN_BRIDGE_SERVER_URL 覆盖（私有化部署）。
const DefaultBridgeServerURL = "ws://192.168.50.52:8084/bridge/connector"

// DefaultServerURL 返回 Connector 默认 server_url。
func DefaultServerURL() string {
	if v := strings.TrimSpace(os.Getenv("MIAOBAN_BRIDGE_SERVER_URL")); v != "" {
		return v
	}
	return DefaultBridgeServerURL
}

// ApplyBundledDefaults 首次运行/空配置时填入内置默认值，并尝试从本机 openclaw.json 读取 Gateway。
func ApplyBundledDefaults(c *Config) {
	if c == nil {
		return
	}
	if strings.TrimSpace(c.ServerURL) == "" {
		c.ServerURL = DefaultServerURL()
	}
	ApplyOpenClawAutodetect(c)
	if c.ChannelOpenClaw() {
		mode := strings.ToLower(strings.TrimSpace(c.OpenClaw.Mode))
		if mode == "" || mode == "echo" {
			if strings.TrimSpace(c.OpenClaw.GatewayToken) != "" || strings.TrimSpace(c.OpenClaw.BearerToken) != "" {
				c.OpenClaw.Mode = "gateway_ws"
			}
		}
		if strings.TrimSpace(c.OpenClaw.BaseURL) == "" {
			c.OpenClaw.BaseURL = "http://127.0.0.1:18789"
		}
	}
	PrepareForUse(c)
}

// HasBundledServerURL 是否已具备可直连的商家云地址（用于简化向导）。
func HasBundledServerURL(c *Config) bool {
	if c == nil {
		return false
	}
	return strings.TrimSpace(c.ServerURL) != ""
}
