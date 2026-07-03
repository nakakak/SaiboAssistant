package driver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"openclaw-connector/internal/config"
)

// Driver 调用本地 OpenClaw（或 echo 自测）
type Driver interface {
	Run(ctx context.Context, taskID, text string) (display, speak string, err error)
}

type Echo struct{}

func (Echo) Run(ctx context.Context, taskID, text string) (string, string, error) {
	reply := fmt.Sprintf("[echo task=%s] %s", taskID, text)
	return reply, reply, nil
}

// HTTP 通过 HTTP 调用 OpenAI 兼容或自定义 JSON API
type HTTP struct {
	BaseURL    string
	Path       string
	Method     string
	Bearer     string
	Model      string
	HTTPClient *http.Client
}

func (h HTTP) Run(ctx context.Context, taskID, text string) (string, string, error) {
	if h.HTTPClient == nil {
		h.HTTPClient = &http.Client{Timeout: 30 * time.Second}
	}
	path := h.Path
	if path == "" {
		path = "/v1/chat/completions"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	url := strings.TrimRight(h.BaseURL, "/") + path
	method := h.Method
	if method == "" {
		method = http.MethodPost
	}
	model := h.Model
	if model == "" {
		model = "default"
	}
	body := fmt.Sprintf(`{"model":%q,"messages":[{"role":"user","content":%q}]}`, model, text)
	start := time.Now()
	fmt.Printf("connector: openclaw http start task_id=%s %s %s\n", taskID, method, url)
	req, err := http.NewRequestWithContext(ctx, method, url, strings.NewReader(body))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if h.Bearer != "" {
		req.Header.Set("Authorization", "Bearer "+h.Bearer)
	}
	resp, err := h.HTTPClient.Do(req)
	if err != nil {
		fmt.Printf("connector: openclaw http error task_id=%s dur=%s err=%v\n", taskID, time.Since(start), err)
		return "", "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		fmt.Printf("connector: openclaw http bad_status task_id=%s dur=%s status=%d body=%s\n", taskID, time.Since(start), resp.StatusCode, string(raw))
		return "", "", fmt.Errorf("openclaw http %d: %s", resp.StatusCode, string(raw))
	}
	fmt.Printf("connector: openclaw http ok task_id=%s dur=%s status=%d bytes=%d\n", taskID, time.Since(start), resp.StatusCode, len(raw))
	textOut := extractOpenAIContent(raw)
	if textOut == "" {
		textOut = strings.TrimSpace(string(raw))
	}
	return textOut, textOut, nil
}

// extractOpenAIContent 解析 OpenAI 兼容 choices[0].message.content
func extractOpenAIContent(raw []byte) string {
	var wrap struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(raw, &wrap); err != nil {
		return ""
	}
	if len(wrap.Choices) == 0 {
		return ""
	}
	return strings.TrimSpace(wrap.Choices[0].Message.Content)
}

// NewFromConfig 根据配置构造驱动
func NewFromConfig(cfg *config.Config) Driver {
	if cfg == nil {
		return Echo{}
	}
	switch strings.ToLower(strings.TrimSpace(cfg.OpenClaw.Mode)) {
	case "http":
		return HTTP{
			BaseURL: cfg.OpenClaw.BaseURL,
			Path:    cfg.OpenClaw.HTTPPath,
			Method:  cfg.OpenClaw.HTTPMethod,
			Bearer:  cfg.OpenClaw.BearerToken,
			Model:   cfg.OpenClaw.Model,
		}
	case "gateway_ws":
		wsURL := resolveGatewayWSURL(cfg)
		tok := resolveGatewayToken(cfg)
		wait := time.Duration(cfg.OpenClaw.GatewayWaitTimeoutMS) * time.Millisecond
		sameHost, dialLoopback := cfg.OpenClaw.GatewaySameHost, cfg.OpenClaw.GatewayDialLoopback
		applyGatewayWSHostDefaults(wsURL, &sameHost, &dialLoopback)
		return GatewayWS{
			WSURL:        wsURL,
			Token:        tok,
			SessionKey:   cfg.OpenClaw.GatewaySessionKey,
			Model:        cfg.OpenClaw.Model,
			WaitTimeout:  wait,
			SameHost:     sameHost,
			DialLoopback: dialLoopback,
		}
	default:
		return Echo{}
	}
}
