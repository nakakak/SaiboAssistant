package dictation

import (
	"strings"
	"sync"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// 悬浮字幕专用主题：更大字号，便于远距离阅读。
type dictationSubtitleTheme struct {
	fyne.Theme
}

func newDictationSubtitleTheme() fyne.Theme {
	return &dictationSubtitleTheme{Theme: theme.DarkTheme()}
}

func (t *dictationSubtitleTheme) Size(name fyne.ThemeSizeName) float32 {
	switch name {
	case theme.SizeNameText, theme.SizeNameCaptionText:
		return 22
	case theme.SizeNameSubHeadingText:
		return 24
	case theme.SizeNameHeadingText:
		return 28
	default:
		return t.Theme.Size(name)
	}
}

var (
	subtitleMu              sync.Mutex
	subtitleLabel           *widget.Label
	subtitleWindow          fyne.Window
	subtitleNSWindowPtr     uintptr // darwin：缓存 NSWindow，供主线程外仅读；修改须走 runOnFyneMain
	subtitleNativeOK        bool
	subtitleVisible         bool
	subtitleSessionPinned   bool // 设备在听写页时保持悬浮窗可见（可无文字）
	subtitleUserDismissed   bool // 用户点「关闭」后暂不弹出（注入仍继续）
)

// runOnFyneMain 在 Fyne 主线程同步执行 fn（可从 WebSocket 等后台 goroutine 调用）。
func runOnFyneMain(fn func()) {
	if fn == nil {
		return
	}
	fyne.DoAndWait(fn)
}

func dismissSubtitleByUser() {
	fyne.Do(func() {
		subtitleMu.Lock()
		win := subtitleWindow
		subtitleMu.Unlock()
		if win != nil {
			win.Hide()
		}
		subtitleMu.Lock()
		subtitleVisible = false
		subtitleUserDismissed = true
		subtitleMu.Unlock()
	})
}

func resetSubtitleUIState() {
	subtitleLabel = nil
	subtitleWindow = nil
	subtitleNSWindowPtr = 0
	subtitleNativeOK = false
	subtitleVisible = false
	subtitleSessionPinned = false
	subtitleUserDismissed = false
}

func initSubtitleUIOnMain(parent fyne.App) {
	parent.Settings().SetTheme(newDictationSubtitleTheme())
	label := widget.NewLabel("")
	label.Wrapping = fyne.TextWrapWord
	subtitleLabel = label
	closeBtn := widget.NewButton("关闭", func() { dismissSubtitleByUser() })
	titleBar := container.NewBorder(nil, nil, nil, closeBtn, layout.NewSpacer())
	body := container.NewBorder(titleBar, nil, nil, nil, container.NewPadded(label))
	w := parent.NewWindow("听写字幕")
	subtitleWindow = w
	w.Resize(fyne.NewSize(680, 200))
	w.SetFixedSize(true)
	w.SetContent(body)
	w.SetCloseIntercept(func() { dismissSubtitleByUser() })
	w.Hide()
}

// InitSubtitleUI attaches the dictation subtitle window to an existing Fyne app.
// Must not use DoAndWait here: installMainUI/onboarding already runs on the Fyne main thread.
func InitSubtitleUI(parent fyne.App, subtitleEnabled bool) {
	subtitleMu.Lock()
	resetSubtitleUIState()
	subtitleMu.Unlock()
	if parent == nil || !subtitleEnabled {
		return
	}
	fyne.Do(func() {
		subtitleMu.Lock()
		defer subtitleMu.Unlock()
		initSubtitleUIOnMain(parent)
	})
}

func fyneApplySubtitleText(txt string) {
	subtitleMu.Lock()
	lbl := subtitleLabel
	win := subtitleWindow
	visible := subtitleVisible
	pinned := subtitleSessionPinned
	dismissed := subtitleUserDismissed
	subtitleMu.Unlock()
	if lbl == nil || win == nil {
		return
	}
	nonEmpty := strings.TrimSpace(txt) != ""
	if !nonEmpty && !pinned {
		fyne.Do(func() {
			if visible {
				win.Hide()
			}
			subtitleMu.Lock()
			subtitleVisible = false
			subtitleMu.Unlock()
		})
		return
	}
	fyne.Do(func() {
		if lbl != nil {
			lbl.SetText(txt)
			lbl.Refresh()
		}
		if dismissed {
			return
		}
		if !visible {
			win.Show()
			subtitleMu.Lock()
			subtitleVisible = true
			subtitleMu.Unlock()
		}
		// 每次显示/更新都重新应用 macOS 鼠标与层级（避免注入后置顶后变回穿透）
		configureSubtitleWindow(win)
		subtitleMu.Lock()
		subtitleNativeOK = true
		subtitleMu.Unlock()
		raiseSubtitleWindow(win)
	})
}

func fyneSetText(s string) {
	fyneApplySubtitleText(s)
}

type subtitleProc struct{}

func newSubtitleProc() *subtitleProc { return &subtitleProc{} }

func (s *subtitleProc) push(text string) {
	if s == nil {
		return
	}
	fyneSetText(text)
}

func (s *subtitleProc) clear() {
	if s == nil {
		return
	}
	subtitleMu.Lock()
	pinned := subtitleSessionPinned
	if pinned {
		subtitleUserDismissed = false
	}
	subtitleMu.Unlock()
	if pinned {
		fyneApplySubtitleText("\u00a0")
		return
	}
	fyneSetText("")
}

// setSessionPinned 设备进入/离开听写页时保持或关闭悬浮窗（无说明文字）。
func (s *subtitleProc) setSessionPinned(on bool) {
	if s == nil {
		return
	}
	subtitleMu.Lock()
	wasPinned := subtitleSessionPinned
	subtitleSessionPinned = on
	if on {
		subtitleUserDismissed = false
	}
	subtitleMu.Unlock()
	if on {
		// 已在听写页时不要清空已有字幕（避免重复 subtitle_session 把文字抹掉）
		if !wasPinned {
			fyneApplySubtitleText("\u00a0")
		}
	} else {
		subtitleMu.Lock()
		subtitleUserDismissed = false
		subtitleMu.Unlock()
		fyne.Do(func() {
			subtitleMu.Lock()
			win := subtitleWindow
			subtitleMu.Unlock()
			if win != nil {
				win.Hide()
			}
			subtitleMu.Lock()
			subtitleVisible = false
			if subtitleLabel != nil {
				subtitleLabel.SetText("")
			}
			subtitleMu.Unlock()
		})
	}
}

func (s *subtitleProc) showTransmitReady() {}

// raiseSubtitleAfterInject 在注入把目标 App 置前后，把字幕窗重新 orderFront（macOS）。
func raiseSubtitleAfterInject() {
	subtitleMu.Lock()
	win := subtitleWindow
	visible := subtitleVisible
	pinned := subtitleSessionPinned
	dismissed := subtitleUserDismissed
	subtitleMu.Unlock()
	if win == nil || dismissed || (!visible && !pinned) {
		return
	}
	fyne.Do(func() {
		raiseSubtitleWindow(win)
	})
}
