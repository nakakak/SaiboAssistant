package config

import (
	"net/url"
	"strings"
)

const wizardPendingToken = "__WIZARD_PENDING__"

// PrepareForUse 在加载/保存配置时应用图形界面未暴露的合理默认值（如 gateway_same_host、清理占位 token）。
func PrepareForUse(c *Config) {
	if c == nil {
		return
	}
	if strings.TrimSpace(c.Token) == wizardPendingToken {
		c.Token = ""
	}
	if !c.ChannelOpenClaw() {
		return
	}
	if strings.ToLower(strings.TrimSpace(c.OpenClaw.Mode)) != "gateway_ws" {
		return
	}
	// OpenClaw：token 握手须用 Control UI 身份，否则网关会清空 scopes（missing scope: operator.write）
	c.OpenClaw.GatewaySameHost = true
	host := openClawGatewayHost(c)
	if host != "" && isLoopbackHost(host) {
		c.OpenClaw.GatewayDialLoopback = true
	} else {
		c.OpenClaw.GatewayDialLoopback = false
	}
}

func openClawGatewayHost(c *Config) string {
	if c == nil {
		return ""
	}
	if h := hostFromURL(strings.TrimSpace(c.OpenClaw.GatewayWSURL)); h != "" {
		return h
	}
	return hostFromURL(strings.TrimSpace(c.OpenClaw.BaseURL))
}

func hostFromURL(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(u.Hostname())
}

func isLoopbackHost(host string) bool {
	switch strings.ToLower(host) {
	case "127.0.0.1", "localhost", "::1":
		return true
	default:
		return false
	}
}
