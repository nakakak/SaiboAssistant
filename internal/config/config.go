package config

import (
	"fmt"
	"net/http"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config Connector 配置（与 OPENCLAW_MIAOBAN 文档一致）
type Config struct {
	ServerURL string `yaml:"server_url"` // wss://host:port/bridge/connector 基址，不含 query
	// DeviceMAC 喵伴设备 MAC；商家云按 devices.device_id 查绑定智能体并注册 Connector
	DeviceMAC string `yaml:"device_mac"`
	// PairCode：6 位验证码。可不填 device_mac：启动时用验证码请求云端解析 MAC 并写回配置文件（须与 server_url 同主机 HTTP API）。
	PairCode string `yaml:"pair_code"`
	// AgentID / ServerToken / Token 仅兼容旧版部署
	AgentID     uint   `yaml:"agent_id"`
	ServerToken string `yaml:"server_token"`
	Token       string `yaml:"token"`
	// OpenclawInstanceID 与云端 agent id 一致，通常由云端解析，无需手填
	OpenclawInstanceID string `yaml:"openclaw_instance_id"`

	// FirstRun 为 true 时：图形模式下启动前先走「通道与权限」向导（完成后写回 false）。旧配置无此字段时为 false，不弹向导。
	FirstRun bool `yaml:"first_run"`

	// Channels 功能通道：可单独开启 OpenClaw 任务桥接与 Telnet/听写转发。未配置时默认两者均开启（兼容旧版）。
	Channels struct {
		OpenClaw *bool `yaml:"openclaw,omitempty"`
		Telnet   *bool `yaml:"telnet,omitempty"`
	} `yaml:"channels,omitempty"`

	OpenClaw struct {
		Mode        string `yaml:"mode"` // echo | http | gateway_ws
		BaseURL     string `yaml:"base_url"`
		HTTPPath    string `yaml:"http_path"`    // 默认 /v1/chat/completions（OpenAI 兼容）
		HTTPMethod  string `yaml:"http_method"`  // 默认 POST
		BearerToken string `yaml:"bearer_token"` // http：Bearer；gateway_ws：可与 gateway_token 二选一
		Model       string `yaml:"model"`        // 请求体 model 字段
		// gateway_ws：本机 OpenClaw Gateway（默认端口 18789），协议见 npm 包 openclaw 的 gateway 实现
		GatewayWSURL         string `yaml:"gateway_ws_url"`          // 如 ws://127.0.0.1:18789；可留空，由 base_url 推导
		GatewayToken         string `yaml:"gateway_token"`           // openclaw.json → gateway.auth.token（与商家云 token 无关）
		GatewaySessionKey    string `yaml:"gateway_session_key"`     // 默认 main
		GatewayWaitTimeoutMS int    `yaml:"gateway_wait_timeout_ms"` // agent.wait 超时，毫秒，0 表示用内置默认
		// GatewaySameHost：使用 openclaw-control-ui + ui 模式握手，便于 token 连接保留 operator scopes（与拨号地址无关，见 README）
		GatewaySameHost bool `yaml:"gateway_same_host"`
		// GatewayDialLoopback：为 true 时将 gateway_ws_url 中的非 loopback 主机名改为 127.0.0.1 再拨号。
		// 仅当 Connector 进程与 OpenClaw Gateway 在同一台机器、且配置里写的是局域网 IP 时需要。
		// Connector 在笔记本等设备上、Gateway 在另一台服务器（如 192.168.x.x）时必须为 false，否则会连到本机错误端口并出现 1006/EOF。
		GatewayDialLoopback bool `yaml:"gateway_dial_loopback"`
	} `yaml:"openclaw"`

	// Dictation: 听写经商家云 dictation.stt 下行（与 OpenClaw 同一条 WS）。enabled 缺省为 true。
	Dictation struct {
		Enabled  *bool `yaml:"enabled"`
		Inject   *bool `yaml:"inject"`   // nil：默认 true（听写页为悬浮+注入）
		Subtitle *bool `yaml:"subtitle"` // nil：默认 false（仅注入，不弹悬浮字幕窗）
	} `yaml:"dictation"`
}

func UnmarshalFromFile(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := yaml.Unmarshal(b, &c); err != nil {
		return nil, err
	}
	ApplyBundledDefaults(&c)
	return &c, nil
}

func Load(path string) (*Config, error) {
	c, err := UnmarshalFromFile(path)
	if err != nil {
		return nil, err
	}
	PrepareForUse(c)
	if err := Validate(c); err != nil {
		return nil, err
	}
	return c, nil
}

// Validate checks required fields after unmarshaling (also used before saving from UI).
func Validate(c *Config) error {
	if c == nil {
		return fmt.Errorf("nil config")
	}
	if c.FirstRun {
		return validateFirstRunBootstrap(c)
	}
	return validateRuntime(c)
}

func validateFirstRunBootstrap(c *Config) error {
	if !c.ChannelOpenClaw() && !c.ChannelTelnet() {
		return fmt.Errorf("至少开启 channels.openclaw 或 channels.telnet 之一")
	}
	return nil
}

func validateRuntime(c *Config) error {
	if strings.TrimSpace(c.ServerURL) == "" {
		return fmt.Errorf("server_url is required")
	}
	hasMAC := strings.TrimSpace(c.DeviceMAC) != ""
	hasPair := strings.TrimSpace(c.PairCode) != ""
	hasLegacyToken := strings.TrimSpace(c.Token) != ""
	hasAgent := c.AgentID > 0 && strings.TrimSpace(c.ServerToken) != ""
	if hasPair && !hasMAC && len(digitsOnlyPairCode(c.PairCode)) != 6 {
		return fmt.Errorf("pair_code must be exactly 6 digits when device_mac is omitted")
	}
	if !hasMAC && !hasPair && !hasLegacyToken && !hasAgent {
		return fmt.Errorf("set device_mac, or pair_code (验证码) to resolve MAC, or legacy agent_id + server_token, or legacy token")
	}
	if !c.ChannelOpenClaw() && !c.ChannelTelnet() {
		return fmt.Errorf("至少开启 channels.openclaw 或 channels.telnet 之一")
	}
	if c.OpenClaw.Mode == "" {
		c.OpenClaw.Mode = "echo"
	}
	if c.OpenClaw.HTTPPath == "" {
		c.OpenClaw.HTTPPath = "/v1/chat/completions"
	}
	if strings.TrimSpace(c.OpenClaw.HTTPMethod) == "" {
		c.OpenClaw.HTTPMethod = http.MethodPost
	}
	mode := strings.ToLower(strings.TrimSpace(c.OpenClaw.Mode))
	if mode == "gateway_ws" && c.ChannelOpenClaw() {
		hasWS := strings.TrimSpace(c.OpenClaw.GatewayWSURL) != ""
		hasBase := strings.TrimSpace(c.OpenClaw.BaseURL) != ""
		if !hasWS && !hasBase {
			return fmt.Errorf("openclaw.gateway_ws: set gateway_ws_url or base_url (http/https) to derive ws://")
		}
		tok := strings.TrimSpace(c.OpenClaw.GatewayToken)
		if tok == "" {
			tok = strings.TrimSpace(c.OpenClaw.BearerToken)
		}
		if tok == "" {
			return fmt.Errorf("openclaw.gateway_ws: gateway_token or bearer_token is required (same as OpenClaw gateway.auth.token)")
		}
	}
	return nil
}

// Save writes c to path as YAML (0600). Used by图形配置界面。
func Save(path string, c *Config) error {
	if c == nil {
		return fmt.Errorf("nil config")
	}
	PrepareForUse(c)
	if err := Validate(c); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ChannelOpenClaw 是否处理云端 task.dispatch（OpenClaw 桥接）。未配置 channels.openclaw 时默认 true。
func (c *Config) ChannelOpenClaw() bool {
	if c == nil || c.Channels.OpenClaw == nil {
		return true
	}
	return *c.Channels.OpenClaw
}

// ChannelTelnet 是否处理 dictation（听写/Telnet 转发）。未配置 channels.telnet 时默认 true。
func (c *Config) ChannelTelnet() bool {
	if c == nil || c.Channels.Telnet == nil {
		return true
	}
	return *c.Channels.Telnet
}

// DictationEnabled 未配置 dictation.enabled 时默认开启；若 Telnet 通道关闭则恒为 false。
func (c *Config) DictationEnabled() bool {
	if c == nil || !c.ChannelTelnet() {
		return false
	}
	if c.Dictation.Enabled == nil {
		return true
	}
	return *c.Dictation.Enabled
}

// DictationInject 为 true 时：将合并听写写入前台焦点框（剪贴板 + 全选 + 粘贴）。macOS 需「辅助功能」；Windows 依赖合成键；Linux 需 PATH 中有 xdotool（X11）或 wtype（Wayland），且需 xclip/xsel 等以支持剪贴板。未配置时默认 true。
func (c *Config) DictationInject() bool {
	if c == nil || !c.ChannelTelnet() {
		return false
	}
	if c.Dictation.Inject == nil {
		return true
	}
	return *c.Dictation.Inject
}

// DictationSubtitle 未配置 dictation.subtitle 时默认 false（仅注入）。Telnet 通道关闭时为 false。
func (c *Config) DictationSubtitle() bool {
	if c == nil || !c.ChannelTelnet() {
		return false
	}
	if c.Dictation.Subtitle != nil {
		return *c.Dictation.Subtitle
	}
	return false
}
