package driver

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"openclaw-connector/internal/config"

	"github.com/gorilla/websocket"
)

var remoteControlUIHintOnce sync.Once

// GatewayWS 通过 OpenClaw Gateway 的 WebSocket JSON-RPC 协议调用本机 Agent（与 npm openclaw 包内 gateway 一致）。
type GatewayWS struct {
	WSURL       string
	Token       string
	SessionKey  string
	Model       string // 可选：显式 provider/model（如 minimax-portal/MiniMax-M2.5）避免落到网关默认不可用模型
	WaitTimeout time.Duration
	Dialer      websocket.Dialer
	// SameHost：client 使用 openclaw-control-ui + ui，避免 token 握手清空 scopes（与 DialLoopback 无关）
	SameHost bool
	// DialLoopback：为 true 时将配置中的非 loopback 主机改为 127.0.0.1 拨号（仅 Gateway 与 Connector 同机且配置写 LAN IP 时）
	DialLoopback bool
}

func (g GatewayWS) Run(ctx context.Context, taskID, text string) (string, string, error) {
	if strings.TrimSpace(g.WSURL) == "" || strings.TrimSpace(g.Token) == "" {
		return "", "", fmt.Errorf("gateway_ws: gateway_ws_url and gateway_token are required")
	}
	sk := strings.TrimSpace(g.SessionKey)
	if sk == "" {
		sk = "main"
	}
	dialURL := g.WSURL
	if g.DialLoopback {
		if u, err := loopbackGatewayWSURL(g.WSURL); err == nil && u != "" {
			if u != g.WSURL {
				fmt.Printf("connector: gateway_ws dial_loopback %s (config %s)\n", u, g.WSURL)
			}
			dialURL = u
		}
	}
	if g.SameHost && !isWSHostLoopback(dialURL) {
		remoteControlUIHintOnce.Do(func() {
			fmt.Printf("connector: hint: gateway_same_host 会使用 Control UI；若 connect 报 device identity，须在「运行 gateway 的机器」上执行:\n  openclaw config set gateway.controlUi.dangerouslyDisableDeviceAuth true --strict-json\n  并重启 gateway；或改用 SSH -L 把本机端口转到网关 127.0.0.1，再把 gateway_ws_url 设为 ws://127.0.0.1:<端口>。\n")
		})
	}
	clientHint := "gateway-client"
	if g.SameHost {
		clientHint = "openclaw-control-ui"
	}
	fmt.Printf("connector: openclaw gateway_ws start task_id=%s url=%s session=%q same_host=%v client=%s\n",
		taskID, dialURL, sk, g.SameHost, clientHint)
	wait := g.WaitTimeout
	if wait <= 0 {
		wait = 3 * time.Minute
	}
	d := g.Dialer
	if d.HandshakeTimeout == 0 {
		d.HandshakeTimeout = 15 * time.Second
	}

	header := http.Header{}
	if o := originForGatewayWSURL(dialURL); o != "" {
		header.Set("Origin", o)
	} else if os.Getenv("OPENCLAW_ALLOW_INSECURE_PRIVATE_WS") == "1" {
		header.Set("Origin", "http://127.0.0.1")
	}

	conn, _, err := d.DialContext(ctx, dialURL, header)
	if err != nil {
		return "", "", fmt.Errorf("gateway_ws dial: %w", err)
	}
	defer conn.Close()

	readCtx, cancelRead := context.WithCancel(ctx)
	defer cancelRead()

	// agent 先发 accepted 再异步结束：中间可能长时间无 JSON；默认 120s 读超时易误判。至少覆盖 wait + 余量。
	readIdle := wait + 5*time.Minute
	if readIdle < 10*time.Minute {
		readIdle = 10 * time.Minute
	}

	c := &gatewayWSConn{
		conn:     conn,
		pend:     make(map[string]*gatewayWSPending),
		chal:     make(chan string, 1),
		closed:   make(chan struct{}),
		readIdle: readIdle,
	}
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(readIdle))
	})
	go c.readLoop(readCtx)

	if _, err := c.waitChallenge(ctx, 15*time.Second); err != nil {
		return "", "", err
	}

	clientID, clientMode := "gateway-client", "backend"
	if g.SameHost {
		// 同机见 loopbackDial：token 且无 device 时 gateway-client 会被清空 scopes；Control UI + 本机可保留
		clientID, clientMode = "openclaw-control-ui", "ui"
	}
	connectParams := map[string]interface{}{
		"minProtocol": 4,
		"maxProtocol": 4,
		"client": map[string]interface{}{
			"id":          clientID,
			"displayName": "miaoban-bridge-connector",
			"version":     "0.1",
			"platform":    runtime.GOOS,
			"mode":        clientMode,
		},
		"caps": []interface{}{},
		"auth": map[string]interface{}{
			"token": g.Token,
		},
		"role": "operator",
		// agent / chat.history 等分别要求 operator.write、operator.read（仅 admin 不足以通过校验）
		"scopes": []interface{}{
			"operator.admin",
			"operator.read",
			"operator.write",
		},
	}

	if _, err := c.rpc(ctx, "connect", connectParams, false); err != nil {
		return "", "", wrapGatewayConnectErr(err)
	}

	agentParams := map[string]interface{}{
		"message":          text,
		"idempotencyKey":   taskID,
		"sessionKey":       sk,
		"deliver":          false,
		"bestEffortDeliver": false,
	}
	if provider, model := parseGatewayModelRef(g.Model); model != "" {
		agentParams["model"] = model
		if provider != "" {
			agentParams["provider"] = provider
		}
	}
	// Gateway 对 agent 先发 status=accepted，再在 run 结束时再 respond 一次；须等同 npm GatewayClient expectFinal。
	agentPayload, err := c.rpc(ctx, "agent", agentParams, true)
	if err != nil {
		return "", "", fmt.Errorf("gateway_ws agent: %w", err)
	}
	var accepted struct {
		RunID  string `json:"runId"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal(agentPayload, &accepted); err != nil {
		return "", "", fmt.Errorf("gateway_ws agent response: %w", err)
	}
	runID := strings.TrimSpace(accepted.RunID)
	if runID == "" {
		runID = taskID
	}

	waitParams := map[string]interface{}{
		"runId":     runID,
		"timeoutMs": int(wait / time.Millisecond),
	}
	waitPayload, err := c.rpc(ctx, "agent.wait", waitParams, false)
	if err != nil {
		return "", "", fmt.Errorf("gateway_ws agent.wait: %w", err)
	}
	var w struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	_ = json.Unmarshal(waitPayload, &w)
	switch w.Status {
	case "ok":
	case "timeout":
		return "", "", fmt.Errorf("gateway_ws: agent timed out")
	case "error":
		if strings.TrimSpace(w.Error) == "" {
			return "", "", fmt.Errorf("gateway_ws: agent error")
		}
		return "", "", fmt.Errorf("gateway_ws: %s", w.Error)
	default:
		return "", "", fmt.Errorf("gateway_ws: unexpected agent.wait status %q", w.Status)
	}

	histParams := map[string]interface{}{
		"sessionKey": sk,
		"limit":      50,
	}
	histPayload, err := c.rpc(ctx, "chat.history", histParams, false)
	if err != nil {
		return "", "", fmt.Errorf("gateway_ws chat.history: %w", err)
	}
	var hist struct {
		Messages []json.RawMessage `json:"messages"`
	}
	if err := json.Unmarshal(histPayload, &hist); err != nil {
		return "", "", fmt.Errorf("gateway_ws chat.history decode: %w", err)
	}
	out := lastAssistantText(hist.Messages)
	if strings.TrimSpace(out) == "" {
		return "", "", fmt.Errorf("gateway_ws: empty assistant reply (check session %q)", sk)
	}
	fmt.Printf("connector: openclaw gateway_ws ok task_id=%s chars=%d\n", taskID, len(out))
	return out, out, nil
}

func wrapGatewayConnectErr(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	if strings.Contains(strings.ToLower(msg), "control ui requires device identity") {
		return fmt.Errorf(
			"gateway_ws connect: %w\n"+
				"connector: 任选其一（在「运行 openclaw gateway 的那台机器」上操作，不是本机 Mac）：\n"+
				"  A) openclaw config set gateway.controlUi.dangerouslyDisableDeviceAuth true --strict-json\n"+
				"     然后重启该机器上的 gateway（仅建议可信内网，用完可关掉）。\n"+
				"  B) SSH 隧道：ssh -N -L 18789:127.0.0.1:18789 user@<网关IP>，再把 gateway_ws_url 改为 ws://127.0.0.1:18789；\n"+
				"     网关需 gateway.controlUi.allowInsecureAuth=true（内网 quickstart 常已开启）。\n"+
				"  C) 把本 connector 安装到网关同机，并对 127.0.0.1 使用 gateway_dial_loopback: true。\n",
			err)
	}
	return fmt.Errorf("gateway_ws connect: %w", err)
}

func isWSHostLoopback(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	h := u.Hostname()
	return h == "127.0.0.1" || h == "localhost" || h == "::1"
}

func parseGatewayModelRef(raw string) (provider string, model string) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", ""
	}
	if i := strings.Index(v, "/"); i > 0 && i < len(v)-1 {
		return strings.TrimSpace(v[:i]), strings.TrimSpace(v[i+1:])
	}
	return "", v
}

type gatewayWSConn struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	pend   map[string]*gatewayWSPending
	chal   chan string
	closed chan struct{}
	once   sync.Once

	readIdle time.Duration // 每次 ReadMessage 前 SetReadDeadline；Pong 时顺延

	closeMu  sync.Mutex
	closeErr error // readLoop 退出前设置，供 rpc 包装错误
}

// gatewayWSPending 与 OpenClaw GatewayClient：expectFinal 时忽略首条 status=accepted 的 res，等待同 id 的最终 res。
type gatewayWSPending struct {
	ch          chan gatewayWSResponse
	expectFinal bool
}

type gatewayWSResponse struct {
	ok      bool
	payload json.RawMessage
	errMsg  string
	errCode string
}

func (c *gatewayWSConn) shutdown() {
	c.once.Do(func() { close(c.closed) })
}

func (c *gatewayWSConn) readLoop(ctx context.Context) {
	defer c.shutdown()
	idle := c.readIdle
	if idle <= 0 {
		idle = 10 * time.Minute
	}
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}
		_ = c.conn.SetReadDeadline(time.Now().Add(idle))
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			c.setCloseErr(err)
			// Normal during shutdown: caller closes conn after RPC completes.
			if errors.Is(err, net.ErrClosed) || strings.Contains(err.Error(), "use of closed network connection") {
				return
			}
			var ce *websocket.CloseError
			if errors.As(err, &ce) && ce != nil {
				fmt.Printf("connector: gateway_ws read closed code=%d text=%q\n", ce.Code, ce.Text)
			} else {
				fmt.Printf("connector: gateway_ws read err: %v\n", err)
			}
			return
		}
		var probe struct {
			Type string `json:"type"`
		}
		if json.Unmarshal(data, &probe) != nil {
			continue
		}
		switch probe.Type {
		case "event":
			var ev struct {
				Event   string          `json:"event"`
				Payload json.RawMessage `json:"payload"`
			}
			if json.Unmarshal(data, &ev) != nil {
				continue
			}
			if ev.Event == "connect.challenge" {
				var p struct {
					Nonce string `json:"nonce"`
				}
				_ = json.Unmarshal(ev.Payload, &p)
				if strings.TrimSpace(p.Nonce) != "" {
					select {
					case c.chal <- strings.TrimSpace(p.Nonce):
					default:
					}
				}
			}
		case "res":
			var res struct {
				ID      string          `json:"id"`
				Ok      bool            `json:"ok"`
				Payload json.RawMessage `json:"payload"`
				Error   *struct {
					Code    string `json:"code"`
					Message string `json:"message"`
				} `json:"error"`
			}
			if json.Unmarshal(data, &res) != nil || res.ID == "" {
				continue
			}
			if !c.deliverRes(res.ID, res.Ok, res.Payload, res.Error) {
				continue
			}
		}
	}
}

// deliverRes 将 res 交给对应 pending；expectFinal 且 payload.status==accepted 时不投递、不移除（对齐 method-scopes GatewayClient）。
func (c *gatewayWSConn) deliverRes(id string, ok bool, payload json.RawMessage, errObj *struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}) bool {
	var wrap struct {
		Status string `json:"status"`
	}
	_ = json.Unmarshal(payload, &wrap)
	status := wrap.Status

	c.mu.Lock()
	p, found := c.pend[id]
	if !found || p == nil {
		c.mu.Unlock()
		return false
	}
	if p.expectFinal && status == "accepted" {
		c.mu.Unlock()
		return true
	}
	delete(c.pend, id)
	ch := p.ch
	c.mu.Unlock()

	gr := gatewayWSResponse{ok: ok, payload: payload}
	if errObj != nil {
		gr.errCode = errObj.Code
		gr.errMsg = errObj.Message
	}
	select {
	case ch <- gr:
	default:
	}
	return true
}

func (c *gatewayWSConn) removePending(id string) {
	c.mu.Lock()
	delete(c.pend, id)
	c.mu.Unlock()
}

func (c *gatewayWSConn) setCloseErr(err error) {
	c.closeMu.Lock()
	c.closeErr = err
	c.closeMu.Unlock()
}

func (c *gatewayWSConn) formatCloseErr() string {
	c.closeMu.Lock()
	err := c.closeErr
	c.closeMu.Unlock()
	if err == nil {
		return ""
	}
	var ce *websocket.CloseError
	if errors.As(err, &ce) && ce != nil {
		return fmt.Sprintf(" (ws close %d: %q)", ce.Code, ce.Text)
	}
	return ": " + err.Error()
}

func (c *gatewayWSConn) waitChallenge(ctx context.Context, d time.Duration) (string, error) {
	select {
	case <-ctx.Done():
		return "", ctx.Err()
	case n := <-c.chal:
		return n, nil
	case <-time.After(d):
		return "", fmt.Errorf("gateway_ws: connect.challenge timeout")
	case <-c.closed:
		return "", fmt.Errorf("gateway_ws: connection closed before challenge")
	}
}

func (c *gatewayWSConn) rpc(ctx context.Context, method string, params interface{}, expectFinal bool) (json.RawMessage, error) {
	id, err := randomRequestID()
	if err != nil {
		return nil, err
	}
	ch := make(chan gatewayWSResponse, 1)
	c.mu.Lock()
	c.pend[id] = &gatewayWSPending{ch: ch, expectFinal: expectFinal}
	c.mu.Unlock()

	frame := map[string]interface{}{
		"type":   "req",
		"id":     id,
		"method": method,
		"params": params,
	}
	body, err := json.Marshal(frame)
	if err != nil {
		c.removePending(id)
		return nil, err
	}
	if err := c.conn.WriteMessage(websocket.TextMessage, body); err != nil {
		c.removePending(id)
		return nil, err
	}

	select {
	case <-ctx.Done():
		c.removePending(id)
		return nil, ctx.Err()
	case <-c.closed:
		c.removePending(id)
		return nil, fmt.Errorf("gateway_ws: connection closed%s", c.formatCloseErr())
	case r := <-ch:
		if !r.ok {
			if strings.TrimSpace(r.errMsg) != "" {
				return nil, fmt.Errorf("%s (%s)", r.errMsg, r.errCode)
			}
			return nil, fmt.Errorf("gateway error %s", r.errCode)
		}
		return r.payload, nil
	}
}

func randomRequestID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func lastAssistantText(messages []json.RawMessage) string {
	for i := len(messages) - 1; i >= 0; i-- {
		var m struct {
			Role    string          `json:"role"`
			Content json.RawMessage `json:"content"`
		}
		if json.Unmarshal(messages[i], &m) != nil {
			continue
		}
		if strings.ToLower(strings.TrimSpace(m.Role)) != "assistant" {
			continue
		}
		s := normalizeChatContent(m.Content)
		if strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
	}
	return ""
}

func normalizeChatContent(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return s
	}
	var parts []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if json.Unmarshal(raw, &parts) == nil {
		var b strings.Builder
		for _, p := range parts {
			if strings.EqualFold(p.Type, "text") && p.Text != "" {
				b.WriteString(p.Text)
			}
		}
		return b.String()
	}
	return string(raw)
}

// resolveGatewayWSURL 优先使用专用字段；否则把 http(s) base_url 转为 ws(s)。
func resolveGatewayWSURL(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	u := strings.TrimSpace(cfg.OpenClaw.GatewayWSURL)
	if u != "" {
		return u
	}
	base := strings.TrimSpace(cfg.OpenClaw.BaseURL)
	if base == "" {
		return ""
	}
	parsed, err := url.Parse(base)
	if err != nil {
		return ""
	}
	switch parsed.Scheme {
	case "http":
		parsed.Scheme = "ws"
	case "https":
		parsed.Scheme = "wss"
	default:
		return ""
	}
	parsed.Path = ""
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return strings.TrimRight(parsed.String(), "/")
}

func resolveGatewayToken(cfg *config.Config) string {
	if cfg == nil {
		return ""
	}
	if t := strings.TrimSpace(cfg.OpenClaw.GatewayToken); t != "" {
		return t
	}
	return strings.TrimSpace(cfg.OpenClaw.BearerToken)
}

// loopbackGatewayWSURL 将 ws(s)://非本机地址 转为 127.0.0.1，保留端口（同机部署 Gateway 时使用）。
func loopbackGatewayWSURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return "", err
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return raw, nil
	}
	host := u.Hostname()
	if host == "127.0.0.1" || host == "localhost" || host == "::1" {
		return raw, nil
	}
	port := u.Port()
	if port == "" {
		if u.Scheme == "wss" {
			port = "443"
		} else {
			port = "80"
		}
	}
	u2 := *u
	u2.Host = net.JoinHostPort("127.0.0.1", port)
	return u2.String(), nil
}

func originForGatewayWSURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	if u.Scheme != "ws" && u.Scheme != "wss" {
		return ""
	}
	port := u.Port()
	if port == "" {
		port = "80"
		if u.Scheme == "wss" {
			port = "443"
		}
	}
	host := "127.0.0.1"
	if h := u.Hostname(); h == "127.0.0.1" || h == "localhost" {
		host = "127.0.0.1"
	}
	if u.Scheme == "wss" {
		return "https://" + net.JoinHostPort(host, port)
	}
	return "http://" + net.JoinHostPort(host, port)
}
