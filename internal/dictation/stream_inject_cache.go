package dictation

import "sync"

var (
	streamInjectMu         sync.Mutex
	lastStreamInjectedText string // 流式听写已写入内容（Mac append-at-end）；其它平台为 no-op 占位
)

func resetStreamInjectCache() {
	streamInjectMu.Lock()
	lastStreamInjectedText = ""
	streamInjectMu.Unlock()
}

func noteStreamInjected(text string) {
	streamInjectMu.Lock()
	lastStreamInjectedText = text
	streamInjectMu.Unlock()
}
