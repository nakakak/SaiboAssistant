package config

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DeriveCloudAPIBase returns http(s)://host:port from a WebSocket bridge URL (path is ignored).
func DeriveCloudAPIBase(serverWS string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(serverWS))
	if err != nil {
		return "", err
	}
	if u.Host == "" {
		return "", fmt.Errorf("invalid server_url host")
	}
	scheme := "https"
	switch strings.ToLower(u.Scheme) {
	case "ws":
		scheme = "http"
	case "wss":
		scheme = "https"
	case "http", "https":
		scheme = u.Scheme
	default:
		return "", fmt.Errorf("unsupported server_url scheme %q", u.Scheme)
	}
	return scheme + "://" + u.Host, nil
}

func digitsOnlyPairCode(raw string) string {
	var b strings.Builder
	for _, r := range strings.TrimSpace(raw) {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	return b.String()
}

type connectorCodeResolveResponse struct {
	DeviceMAC string `json:"device_mac"`
}

// ResolveDeviceMAC fills cfg.DeviceMAC by calling the merchant cloud when MAC is empty but pair_code is set.
// If cfgPath is non-empty and MAC was resolved, writes back config so the user need not look up MAC again.
func ResolveDeviceMAC(ctx context.Context, cfg *Config, httpClient *http.Client, cfgPath string) error {
	if cfg == nil {
		return fmt.Errorf("nil config")
	}
	if strings.TrimSpace(cfg.DeviceMAC) != "" {
		return nil
	}
	code := digitsOnlyPairCode(cfg.PairCode)
	if code == "" {
		return nil
	}
	if len(code) != 6 {
		return fmt.Errorf("pair_code must be 6 digits")
	}
	base, err := DeriveCloudAPIBase(cfg.ServerURL)
	if err != nil {
		return fmt.Errorf("resolve mac: %w", err)
	}
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	endpoint := strings.TrimRight(base, "/") + "/api/public/connector-code-resolve"
	body, _ := json.Marshal(map[string]string{"code": code})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("resolve mac http: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("resolve mac: http %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var wrap struct {
		Status string                       `json:"status"`
		Data   connectorCodeResolveResponse `json:"data"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return fmt.Errorf("resolve mac decode: %w", err)
	}
	mac := strings.TrimSpace(wrap.Data.DeviceMAC)
	if mac == "" {
		return fmt.Errorf("resolve mac: empty device_mac in response")
	}
	cfg.DeviceMAC = mac

	if cfgPath != "" {
		if err := Save(cfgPath, cfg); err != nil {
			// 仍可用内存中的 MAC 连接；仅无法持久化
			log.Printf("config: resolved device_mac=%q but save failed: %v", mac, err)
		}
	}
	return nil
}

// NewResolveHTTPClient returns a client suitable for one-off resolve calls (timeout bounded).
func NewResolveHTTPClient() *http.Client {
	return &http.Client{Timeout: 15 * time.Second}
}
