package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"
	"time"

	"openclaw-connector/internal/config"
	"openclaw-connector/internal/dictation"
	"openclaw-connector/internal/driver"

	"github.com/gorilla/websocket"
)

const schemaVersion = 1

// Client 与商家云 WebSocket 交互
type Client struct {
	cfg    *config.Config
	drv    driver.Driver
	dialer websocket.Dialer
	dict   *dictation.Handler
	// OnConnected 在收到 bridge.connector_ready 时调用一次（可选，用于 UI 状态）
	OnConnected func()
	// OnDisconnected 在连接断开时调用（可选）
	OnDisconnected func(err error)
}

func New(cfg *config.Config, drv driver.Driver) *Client {
	return &Client{
		cfg:    cfg,
		drv:    drv,
		dialer: websocket.Dialer{HandshakeTimeout: 15 * time.Second},
		dict: dictation.New(dictation.Config{
			Enabled:  cfg.DictationEnabled(),
			Inject:   cfg.DictationInject(),
			Subtitle: cfg.DictationSubtitle(),
		}),
	}
}

func (c *Client) Run(ctx context.Context) error {
	u, err := url.Parse(c.cfg.ServerURL)
	if err != nil {
		return err
	}
	q := u.Query()
	token := strings.TrimSpace(c.cfg.Token)
	if token == "__WIZARD_PENDING__" {
		token = ""
	}
	switch {
	case token != "":
		q.Set("token", token)
	case strings.TrimSpace(c.cfg.DeviceMAC) != "":
		q.Set("device_mac", strings.TrimSpace(c.cfg.DeviceMAC))
		if pc := strings.TrimSpace(c.cfg.PairCode); pc != "" {
			q.Set("pair_code", pc)
		}
	default:
		q.Set("agent_id", strconv.FormatUint(uint64(c.cfg.AgentID), 10))
		q.Set("bridge_token", strings.TrimSpace(c.cfg.ServerToken))
	}
	u.RawQuery = q.Encode()
	wsURL := u.String()
	if !strings.HasPrefix(wsURL, "ws") {
		return fmt.Errorf("server_url must be ws:// or wss://")
	}

	conn, resp, err := c.dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		dialErr := err
		if resp != nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			msg := strings.TrimSpace(string(body))
			if msg != "" {
				dialErr = fmt.Errorf("dial: %w (http %d: %s)", err, resp.StatusCode, msg)
			} else {
				dialErr = fmt.Errorf("dial: %w (http %d)", err, resp.StatusCode)
			}
		} else {
			dialErr = fmt.Errorf("dial: %w", err)
		}
		if c.OnDisconnected != nil {
			c.OnDisconnected(dialErr)
		}
		return dialErr
	}
	fmt.Printf("connector: connected ws=%s\n", redactConnectorURL(wsURL))
	defer conn.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			if c.OnDisconnected != nil {
				c.OnDisconnected(err)
			}
			return err
		}
		var msg map[string]interface{}
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		t, _ := msg["type"].(string)
		if t != "" && t != "bridge.pong" && t != "dictation.stt" {
			// Avoid spamming pongs; keep other message types visible for debugging.
			fmt.Printf("connector: recv type=%s\n", t)
		}
		switch t {
		case "bridge.connector_ready":
			if c.OnConnected != nil {
				c.OnConnected()
				c.OnConnected = nil
			}
			continue
		case "bridge.pong":
			continue
		case "dictation.stt":
			if !c.cfg.ChannelTelnet() {
				continue
			}
			txt, _ := msg["text"].(string)
			final := jsonFinal(msg["final"])
			if strings.TrimSpace(txt) != "" {
				fmt.Printf("connector: recv dictation.stt final=%v len=%d\n", final, len(txt))
			}
			c.dict.HandleSTT(txt, final)
		case "dictation.control":
			if !c.cfg.ChannelTelnet() {
				continue
			}
			c.dict.HandleControlMsg(msg)
		case "task.dispatch":
			c.handleDispatch(ctx, conn, msg)
		default:
			// ignore
		}
	}
}

func (c *Client) handleDispatch(ctx context.Context, conn *websocket.Conn, msg map[string]interface{}) {
	if !c.cfg.ChannelOpenClaw() {
		taskID, _ := msg["task_id"].(string)
		if taskID != "" {
			sendTaskFailed(conn, taskID, "CHANNEL_DISABLED", "本机未开启 OpenClaw 通道（channels.openclaw）")
		}
		return
	}
	taskID, _ := msg["task_id"].(string)
	input, _ := msg["input"].(map[string]interface{})
	text, _ := input["text"].(string)
	if taskID == "" {
		return
	}
	fmt.Printf("connector: task.dispatch task_id=%s text=%q\n", taskID, text)
	// 每任务按当前配置重建驱动，避免启动后改过 gateway_same_host 仍用旧握手方式。
	c.drv = driver.NewFromConfig(c.cfg)
	display, speak, err := c.drv.Run(ctx, taskID, text)
	if err != nil {
		sendTaskFailed(conn, taskID, "TOOL_ERROR", err.Error())
		return
	}
	if detail, ok := bridgeResultLooksLikeErrorJSON(display, speak); ok {
		fmt.Printf("connector: task.result rejected task_id=%s detail=%s\n", taskID, detail)
		sendTaskFailed(conn, taskID, "INVALID_RESULT", "任务执行失败，请稍后再试")
		return
	}
	out, _ := json.Marshal(map[string]interface{}{
		"type":           "task.result",
		"schema_version": schemaVersion,
		"task_id":        taskID,
		"result": map[string]interface{}{
			"display_text": display,
			"speak_text":   speak,
		},
		"ts_ms": time.Now().UnixMilli(),
	})
	if werr := conn.WriteMessage(websocket.TextMessage, out); werr != nil {
		fmt.Printf("connector: write task.result task_id=%s err=%v\n", taskID, werr)
		return
	}
	fmt.Printf("connector: task.result task_id=%s\n", taskID)
}

func sendTaskFailed(conn *websocket.Conn, taskID, code, message string) {
	if strings.TrimSpace(code) == "" {
		code = "TOOL_ERROR"
	}
	if strings.TrimSpace(message) == "" {
		message = "任务执行失败，请稍后再试"
	}
	fmt.Printf("connector: task.failed task_id=%s code=%s message=%s\n", taskID, code, message)
	out, _ := json.Marshal(map[string]interface{}{
		"type":           "task.failed",
		"schema_version": schemaVersion,
		"task_id":        taskID,
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
		"ts_ms": time.Now().UnixMilli(),
	})
	if werr := conn.WriteMessage(websocket.TextMessage, out); werr != nil {
		fmt.Printf("connector: write task.failed task_id=%s err=%v\n", taskID, werr)
	}
}

func bridgeResultLooksLikeErrorJSON(display, speak string) (string, bool) {
	for _, raw := range []string{speak, display} {
		s := strings.TrimSpace(raw)
		if s == "" {
			continue
		}
		var obj map[string]interface{}
		if err := json.Unmarshal([]byte(s), &obj); err == nil {
			if detail, ok := mapLooksLikeFailure(obj); ok {
				return detail, true
			}
		}
		lower := strings.ToLower(s)
		if strings.HasPrefix(lower, "{") &&
			(strings.Contains(lower, "\"ok\": false") ||
				strings.Contains(lower, "\"returncode\":") ||
				strings.Contains(lower, "\"error\":") ||
				strings.Contains(lower, "python exited with") ||
				strings.Contains(lower, "runpipeline failed")) {
			return "json-like failure payload", true
		}
	}
	return "", false
}

func mapLooksLikeFailure(obj map[string]interface{}) (string, bool) {
	if v, ok := obj["ok"].(bool); ok && !v {
		return "ok=false", true
	}
	if msg := strings.TrimSpace(asString(obj["error"])); msg != "" {
		return msg, true
	}
	if rc := asInt(obj["returncode"]); rc != 0 {
		return fmt.Sprintf("returncode=%d", rc), true
	}
	if status := strings.ToLower(strings.TrimSpace(asString(obj["status"]))); status == "error" || status == "failed" {
		return "status=" + status, true
	}
	return "", false
}

func asString(v interface{}) string {
	switch x := v.(type) {
	case string:
		return x
	case map[string]interface{}:
		if msg, ok := x["message"].(string); ok {
			return msg
		}
	}
	return ""
}

func asInt(v interface{}) int64 {
	switch x := v.(type) {
	case int:
		return int64(x)
	case int32:
		return int64(x)
	case int64:
		return x
	case float64:
		return int64(x)
	case json.Number:
		n, _ := x.Int64()
		return n
	}
	return 0
}

func redactConnectorURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	q := u.Query()
	if q.Get("bridge_token") != "" {
		q.Set("bridge_token", "***")
	}
	if q.Get("token") != "" {
		q.Set("token", "***")
	}
	if q.Get("pair_code") != "" {
		q.Set("pair_code", "***")
	}
	u.RawQuery = q.Encode()
	return u.String()
}

func jsonFinal(v interface{}) bool {
	if b, ok := v.(bool); ok {
		return b
	}
	if f, ok := v.(float64); ok {
		return f != 0
	}
	return false
}
