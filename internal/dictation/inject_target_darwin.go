//go:build darwin

package dictation

import (
	"log"
	"strings"
)

// captureOrPreserveInjectTarget 记录鼠标坐标；前台为 Connector 时用鼠标下的 App 作为注入目标。
func captureOrPreserveInjectTarget(existing, reason string) string {
	captureSessionMousePoint()

	front, err := getFrontmostAppName()
	if err != nil {
		log.Printf("dictation: capture frontmost failed (%s): %v", reason, err)
	} else if !isConnectorProcess(front) {
		front = normalizeAppName(front)
		if pt := appNameAtSavedMouse(); pt != "" && !strings.EqualFold(pt, front) {
			log.Printf("dictation: session app=%q at point (frontmost %q, %s)", pt, front, reason)
			return pt
		}
		log.Printf("dictation: session app=%q mouse target saved (%s)", front, reason)
		return front
	}

	if at, err := getAppNameAtMousePoint(); err == nil {
		at = normalizeAppName(at)
		if isConnectorProcess(at) {
			log.Printf("dictation: mouse over connector UI (%s); move cursor to Mac input box", reason)
		} else {
			log.Printf("dictation: session app=%q at mouse saved (%s, connector frontmost)", at, reason)
			return at
		}
	}

	existing = normalizeAppName(existing)
	if existing != "" && !isConnectorProcess(existing) {
		log.Printf("dictation: keep inject target app=%q (%s)", existing, reason)
		return existing
	}

	if hasSessionInjectTarget() {
		log.Printf("dictation: mouse saved (%s); move cursor over Mac input, then speak", reason)
		return existing
	}

	if err == nil {
		log.Printf("dictation: 请把鼠标移到 Mac 输入框上再点「开始传输」（当前前台=%q）", front)
	} else {
		log.Printf("dictation: 请把鼠标移到 Mac 输入框上再点「开始传输」")
	}
	return ""
}

// 开始传输时记录前台 App；若注入时焦点在 Connector，会保留已有注入点。
func captureInjectTarget() string {
	return captureOrPreserveInjectTarget("", "transmit")
}

// refreshInjectTargetOnPageEnter 进入听写页时刷新或保留注入点（无需开始传输）。
func refreshInjectTargetOnPageEnter(existing string) string {
	return captureOrPreserveInjectTarget(existing, "page_enter")
}
