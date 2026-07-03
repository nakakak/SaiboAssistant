package dictation

import (
	"log"
	"strings"
	"sync"
	"time"

	"github.com/atotto/clipboard"
)

// dictationBuildTag 便于对照日志确认已运行最新 Connector 二进制。
const dictationBuildTag = "20260703-stream-replace-no-delta"

// Config controls desktop dictation handling from Connector WS.
type Config struct {
	Enabled  bool
	Inject   bool
	Subtitle bool // 内置 Fyne 置顶字幕窗（全桌面平台，无 Python 依赖）
}

// Handler displays streaming partial STT within one utterance; accumulates finals until clear.
type Handler struct {
	cfg Config
	sub *subtitleProc

	modeMu sync.RWMutex
	// allowInject / allowSub 来自 YAML：设备仅能在已允许的能力内切换。
	allowInject bool
	allowSub    bool
	pcMode      string // "subtitle" | "inject" | "both"

	mu              sync.Mutex
	committed       []string // 已确认句子（final=true），直到 clear 或新一轮开始传输
	lastPartial     [2]string
	lastClear           time.Time
	lastTransmitStart   time.Time
	lastInjectedShow    string // 上次已写入输入框的合并文，避免重复 partial 触发 ⌘A
	injectTargetApp     string // 开始传输时前台 App；注入时切回该 App
	listenActive        bool   // 设备「开始传输」后为 true；点波纹暂停(segment_end)后为 false
	telnetPageActive    bool   // 在听写页内（subtitle_session active）
	// postEnterASRPrefix：设备点「发送」后豆包仍可能继续下发「整段累积」partial；从后续包中剥掉已提交前缀，避免下一句仍带着上一句。
	postEnterASRPrefix string
}

func New(cfg Config) *Handler {
	h := &Handler{
		cfg:         cfg,
		allowInject: cfg.Inject,
		allowSub:    cfg.Subtitle,
	}
	if cfg.Subtitle {
		h.sub = newSubtitleProc()
	}
	if h.allowSub && h.allowInject {
		h.pcMode = "both"
	} else if h.allowSub {
		h.pcMode = "subtitle"
	} else if h.allowInject {
		h.pcMode = "inject"
	} else {
		h.pcMode = "subtitle"
	}
	log.Printf("dictation: init build=%s enabled=%v inject=%v subtitle=%v pc_mode=%s",
		dictationBuildTag, cfg.Enabled, cfg.Inject, cfg.Subtitle, h.pcMode)
	LogInjectCapability(cfg.Inject)
	return h
}

func joinCommitted(parts []string) string {
	return strings.TrimSpace(strings.Join(parts, ""))
}

func (h *Handler) mergedShowLocked(partialStream string) string {
	base := joinCommitted(h.committed)
	p := strings.TrimSpace(partialStream)
	if base == "" {
		return p
	}
	if p == "" {
		return base
	}
	if strings.HasPrefix(p, base) {
		return p
	}
	if strings.Contains(base, p) {
		return base
	}
	return strings.TrimSpace(base + " " + p)
}

func (h *Handler) ensureCombinedMode() {
	if h.allowSub && h.allowInject {
		h.applyPcMode("both")
	} else if h.allowInject {
		h.applyPcMode("inject")
	}
}

func controlTransmitStart(m map[string]interface{}) bool {
	v, ok := m["transmit"]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return t != 0
	default:
		return false
	}
}

func subtitleSessionActive(m map[string]interface{}) bool {
	v, ok := m["active"]
	if !ok {
		return false
	}
	switch t := v.(type) {
	case bool:
		return t
	case string:
		s := strings.ToLower(strings.TrimSpace(t))
		return s == "true" || s == "1" || s == "yes"
	case float64:
		return t != 0
	default:
		return false
	}
}

func shouldSkipToolStatus(t string) bool {
	prefixes := []string{"正在调用工具", "Calling tool"}
	s := strings.TrimSpace(t)
	for _, p := range prefixes {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	return false
}

func (h *Handler) getPcMode() string {
	h.modeMu.RLock()
	defer h.modeMu.RUnlock()
	return h.pcMode
}

func (h *Handler) useSubtitle() bool {
	mode := h.getPcMode()
	return h.allowSub && (mode == "subtitle" || mode == "both")
}

func (h *Handler) useInject() bool {
	mode := h.getPcMode()
	return h.allowInject && (mode == "inject" || mode == "both")
}

func (h *Handler) applyPcMode(mode string) {
	m := strings.ToLower(strings.TrimSpace(mode))
	h.modeMu.Lock()
	defer h.modeMu.Unlock()
	switch m {
	case "both":
		if h.allowSub && h.allowInject {
			h.pcMode = "both"
		} else if h.allowSub {
			h.pcMode = "subtitle"
		} else if h.allowInject {
			h.pcMode = "inject"
		} else {
			h.pcMode = "subtitle"
		}
	case "inject":
		if h.allowInject {
			h.pcMode = "inject"
		} else if h.allowSub {
			h.pcMode = "subtitle"
		}
	default:
		if h.allowSub {
			h.pcMode = "subtitle"
		} else if h.allowInject {
			h.pcMode = "inject"
		}
	}
	log.Printf("dictation: pc_mode=%s", h.pcMode)
}

func (h *Handler) resetStreamBuffer() {
	h.mu.Lock()
	h.lastPartial = [2]string{}
	h.mu.Unlock()
}

// beginNewTransmitRound 设备点「开始传输」：清空上一轮合并文本，避免显示在新一轮聆听里。
func (h *Handler) beginNewTransmitRound() {
	h.mu.Lock()
	h.committed = nil
	h.lastPartial = [2]string{}
	h.postEnterASRPrefix = ""
	h.lastInjectedShow = ""
	h.mu.Unlock()
	log.Println("dictation: new transmit round (cleared text buffer)")
	resetInjectFocus()
	if h.sub != nil && h.useSubtitle() {
		subtitleMu.Lock()
		dismissed := subtitleUserDismissed
		pinned := subtitleSessionPinned
		subtitleMu.Unlock()
		if pinned && !dismissed {
			h.sub.push("\u00a0")
		}
	}
}

func (h *Handler) shouldClearInjectField(target string) bool {
	if !h.allowInject {
		return false
	}
	if h.useInject() {
		return true
	}
	return strings.TrimSpace(target) != "" || hasSessionInjectTarget()
}

func (h *Handler) clearStateAndUI() {
	now := time.Now()
	h.mu.Lock()
	if !h.lastClear.IsZero() && now.Sub(h.lastClear) < 350*time.Millisecond {
		h.mu.Unlock()
		log.Println("dictation: [cleared] debounced (duplicate tap)")
		return
	}
	h.lastClear = now
	h.committed = nil
	h.lastPartial = [2]string{}
	h.postEnterASRPrefix = ""
	h.lastInjectedShow = ""
	target := normalizeAppName(h.injectTargetApp)
	h.mu.Unlock()
	resetStreamInjectCache()
	mouseOK := hasSessionInjectTarget()
	log.Printf("dictation: [cleared] build=%s allowInject=%v useInject=%v pc_mode=%s target=%q mouse=%v",
		dictationBuildTag, h.allowInject, h.useInject(), h.getPcMode(), target, mouseOK)
	// 必须先清注入框再动悬浮窗，否则 Fyne 字幕窗抢焦点会导致 Mac 输入框清不掉
	if h.shouldClearInjectField(target) {
		if err := clearInjectField(target); err != nil {
			log.Printf("dictation: clear inject field: %v", err)
		} else {
			log.Println("dictation: inject field cleared")
		}
	} else if h.allowInject {
		log.Println("dictation: clear inject skipped (no inject mode / target)")
	}
	if h.sub != nil {
		h.sub.clear()
		if h.useSubtitle() {
			subtitleMu.Lock()
			pinned := subtitleSessionPinned
			subtitleMu.Unlock()
			if pinned {
				fyneApplySubtitleText("\u00a0")
			}
		}
	}
	// 保留 injectTargetApp 与鼠标坐标，便于清空后继续向同一输入框注入
}

func (h *Handler) currentShowLocked() string {
	return h.mergedShowLocked(h.lastPartial[0])
}

// flushInject pushes merged text to the inject target (subtitle unchanged).
func (h *Handler) flushInject(reason string) {
	if !h.useInject() {
		return
	}
	h.mu.Lock()
	if !h.listenActive {
		h.mu.Unlock()
		log.Printf("dictation: inject flush skipped (%s, session inactive)", reason)
		return
	}
	show := h.currentShowLocked()
	target := normalizeAppName(h.injectTargetApp)
	h.mu.Unlock()
	if show == "" {
		return
	}
	log.Printf("dictation: inject flush (%s) %q target=%q", reason, show, target)
	if err := replaceFocusedField(show, target, true); err != nil {
		log.Printf("dictation: inject FAILED: %v", err)
	} else {
		h.mu.Lock()
		h.lastInjectedShow = show
		h.mu.Unlock()
		log.Printf("dictation: inject OK (%d chars)", len(show))
	}
}

// HandleControlMsg handles dictation.control: clear, settings (pc_mode), enter, subtitle_session, segment_end.
func (h *Handler) HandleControlMsg(m map[string]interface{}) {
	if !h.cfg.Enabled {
		return
	}
	ev, _ := m["event"].(string)
	ev = strings.ToLower(strings.TrimSpace(ev))
	switch ev {
	case "clear":
		h.clearStateAndUI()
	case "segment_end":
		// 设备点聆听波纹结束本段拾音：若 final 尚未注入则补一次，避免与 STT 重复写两遍
		h.mu.Lock()
		show := h.currentShowLocked()
		last := h.lastInjectedShow
		h.listenActive = false
		h.mu.Unlock()
		if show != "" && show != last {
			h.flushInject("segment_end")
		} else if show != "" {
			log.Println("dictation: segment_end flush skipped (already injected)")
		}
		h.mu.Lock()
		h.lastInjectedShow = ""
		h.mu.Unlock()
		resetInjectFocus()
		log.Println("dictation: listen paused (segment_end); tap 开始传输 to resume inject")
		if h.sub != nil && h.useSubtitle() {
			h.mu.Lock()
			show := h.currentShowLocked()
			h.mu.Unlock()
			if show != "" {
				h.sub.push(show)
			} else {
				subtitleMu.Lock()
				pinned := subtitleSessionPinned
				subtitleMu.Unlock()
				if pinned {
					h.sub.setSessionPinned(true)
				}
			}
		}
	case "enter":
		if h.allowSub && h.allowInject {
			h.ensureCombinedMode()
		}
		h.mu.Lock()
		prefix := strings.TrimSpace(h.currentShowLocked())
		h.committed = nil
		h.lastPartial = [2]string{}
		if prefix != "" {
			h.postEnterASRPrefix = prefix
			log.Printf("dictation: enter — strip cumulative ASR after this prefix (%d runes)", len([]rune(prefix)))
		} else {
			h.postEnterASRPrefix = ""
		}
		h.mu.Unlock()
		if h.useInject() {
			h.mu.Lock()
			target := normalizeAppName(h.injectTargetApp)
			h.mu.Unlock()
			if err := injectSendEnterKey(target); err != nil {
				log.Printf("dictation: inject enter failed: %v", err)
			}
			resetInjectFocus()
			h.mu.Lock()
			h.lastInjectedShow = ""
			h.mu.Unlock()
		} else {
			log.Printf("dictation: inject enter skipped pc_mode=%s allowInject=%v", h.getPcMode(), h.allowInject)
		}
	case "settings":
		mode, _ := m["pc_mode"].(string)
		h.applyPcMode(mode)
		log.Printf("dictation: pc_mode=%s (subtitle waits for transmit)", h.getPcMode())
	case "subtitle_session":
		active := subtitleSessionActive(m)
		if active {
			h.mu.Lock()
			h.telnetPageActive = true
			h.mu.Unlock()
			if h.allowSub && h.allowInject {
				h.applyPcMode("both")
			} else if h.allowInject {
				h.applyPcMode("inject")
			}
			if controlTransmitStart(m) {
				h.mu.Lock()
				dupTransmit := !h.lastTransmitStart.IsZero() && time.Since(h.lastTransmitStart) < 500*time.Millisecond
				if !dupTransmit {
					h.lastTransmitStart = time.Now()
				}
				h.mu.Unlock()
				if dupTransmit {
					log.Println("dictation: transmit debounced (duplicate start)")
				} else {
					if h.sub != nil && h.useSubtitle() {
						h.sub.setSessionPinned(true)
						log.Println("dictation: subtitle pinned (transmit started)")
					} else if h.useInject() {
						log.Println("dictation: inject-only (no subtitle window)")
					}
					resetInjectFocus()
					h.beginNewTransmitRound()
					h.mu.Lock()
					h.listenActive = true
					h.mu.Unlock()
					if h.useInject() {
						h.mu.Lock()
						existing := h.injectTargetApp
						h.mu.Unlock()
						app := normalizeAppName(captureOrPreserveInjectTarget(existing, "transmit"))
						h.mu.Lock()
						if app != "" {
							h.injectTargetApp = app
						}
						h.mu.Unlock()
						logSavedInjectPoint("transmit")
						// 暂停后再次「开始传输」不强制清空输入框，避免误删已注入内容；仅用户点「清空」时清
					}
				}
			} else {
				log.Println("dictation: telnet page open (no subtitle until 开始传输)")
				if h.useInject() {
					h.mu.Lock()
					existing := h.injectTargetApp
					h.mu.Unlock()
					if app := normalizeAppName(refreshInjectTargetOnPageEnter(existing)); app != "" {
						h.mu.Lock()
						h.injectTargetApp = app
						h.mu.Unlock()
					}
				}
			}
		} else {
			h.mu.Lock()
			h.listenActive = false
			h.telnetPageActive = false
			h.committed = nil
			h.lastPartial = [2]string{}
			h.postEnterASRPrefix = ""
			h.lastInjectedShow = ""
			h.mu.Unlock()
			h.resetStreamBuffer()
			resetInjectFocus()
			if h.sub != nil && h.useSubtitle() {
				h.sub.setSessionPinned(false)
				log.Println("dictation: subtitle unpinned (left telnet page)")
			}
			log.Println("dictation: session ended (buffer cleared; inject target kept)")
		}
	default:
		if ev == "" {
			h.clearStateAndUI()
		}
	}
}

// HandleSTT processes one dictation.stt payload from the server.
func (h *Handler) HandleSTT(text string, isFinal bool) {
	if !h.cfg.Enabled {
		return
	}
	h.ensureCombinedMode()
	text = strings.TrimSpace(text)
	if text == "" || shouldSkipToolStatus(text) {
		return
	}

	h.mu.Lock()

	p := strings.TrimSpace(h.postEnterASRPrefix)
	if p != "" {
		if strings.HasPrefix(text, p) {
			text = strings.TrimSpace(text[len(p):])
			text = strings.TrimLeft(text, " ，。、；：")
			if text == "" {
				h.mu.Unlock()
				return
			}
		} else {
			h.postEnterASRPrefix = ""
		}
	}

	var show string
	if isFinal {
		finalText := strings.TrimSpace(text)
		if len(h.committed) >= 2 {
			lastLine := strings.TrimSpace(h.committed[len(h.committed)-2])
			if lastLine == finalText && strings.TrimSpace(h.lastPartial[0]) == "" {
				h.mu.Unlock()
				log.Printf("dictation: skip duplicate final %q", finalText)
				return
			}
		}
		stream := strings.TrimSpace(h.lastPartial[0])
		if stream != "" {
			if strings.HasPrefix(text, stream) {
				stream += text[len(stream):]
			} else if !strings.HasSuffix(stream, text) {
				stream = strings.TrimSpace(stream + "\n" + text)
			}
			h.committed = append(h.committed, strings.TrimSpace(stream), "\n")
		} else {
			h.committed = append(h.committed, text, "\n")
		}
		h.lastPartial = [2]string{"", ""}
		show = joinCommitted(h.committed)
	} else {
		stream := h.lastPartial[0]
		prevRaw := h.lastPartial[1]
		if text == stream || strings.TrimSpace(text) == strings.TrimSpace(stream) {
			// 服务端重复下发同一 partial
		} else if stream == "" || strings.HasPrefix(text, stream) {
			// 豆包等 ASR 常发累积式 partial：直接采用较长整段，避免「明天」+「明天星期几」拼接
			stream = text
		} else if strings.HasPrefix(stream, text) {
			// partial 变短，保留已有流
		} else if prevRaw != "" && strings.HasPrefix(text, prevRaw) {
			stream += text[len(prevRaw):]
		} else if prevRaw != "" && strings.HasPrefix(prevRaw, text) {
			// partial got shorter; keep monotonic stream
		} else {
			if stream != "" && !strings.HasSuffix(stream, " ") && !strings.HasSuffix(stream, "\n") {
				stream += " "
			}
			stream += text
		}
		h.lastPartial = [2]string{stream, text}
		show = h.mergedShowLocked(stream)
	}

	listenOK := h.listenActive

	var doInject bool
	skipDup := false
	if h.useInject() && listenOK && show != "" {
		if show == h.lastInjectedShow {
			skipDup = true
		} else {
			doInject = true
		}
	}
	targetApp := normalizeAppName(h.injectTargetApp)
	h.mu.Unlock()
	if skipDup {
		return
	}
	if targetApp == "" {
		targetApp = refreshInjectTargetFromSavedMouse("")
		if targetApp != "" {
			h.mu.Lock()
			h.injectTargetApp = targetApp
			h.mu.Unlock()
		}
	}
	log.Printf("dictation: %s (final=%v inject=%v listen=%v target=%q mouse=%v)", show, isFinal, h.useInject(), listenOK, targetApp, hasSessionInjectTarget())
	if h.useInject() && show != "" && !listenOK {
		log.Println("dictation: inject skipped (paused; tap 开始传输 on device)")
	}
	if doInject {
		if strings.TrimSpace(targetApp) == "" && !hasSessionInjectTarget() {
			log.Println("dictation: inject skipped (no target; move mouse to Mac input, then 开始传输)")
		} else if err := replaceFocusedField(show, targetApp, isFinal); err != nil {
			log.Printf("dictation: inject FAILED: %v", err)
		} else {
			h.mu.Lock()
			h.lastInjectedShow = show
			h.mu.Unlock()
			if isFinal {
				log.Printf("dictation: inject OK (%d chars)", len(show))
			} else {
				log.Printf("dictation: inject OK partial (%d chars)", len(show))
			}
		}
	}
	if h.sub != nil && h.useSubtitle() {
		h.sub.push(show)
	}
	if isFinal && h.useSubtitle() && !h.useInject() {
		if t := strings.TrimSpace(show); t != "" {
			_ = clipboard.WriteAll(t)
		}
	}
}
