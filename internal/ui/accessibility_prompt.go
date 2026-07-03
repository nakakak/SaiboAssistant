package ui

import (
	"log"
	"runtime"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/widget"
	"openclaw-connector/internal/config"
	"openclaw-connector/internal/platform"
)

// EnsureMacAccessibilityForInject prompts for Accessibility when dictation inject is enabled.
// On macOS this triggers the system dialog (AXIsProcessTrustedWithOptions) and offers to open Settings.
func EnsureMacAccessibilityForInject(parent fyne.Window, cfg *config.Config) {
	if runtime.GOOS != "darwin" || cfg == nil || !cfg.DictationInject() {
		return
	}
	if platform.AccessibilityTrusted() {
		log.Println("accessibility: already trusted for inject")
		return
	}

	log.Println("accessibility: requesting macOS Accessibility trust for dictation inject")
	platform.RequestAccessibilityPrompt()

	showAccessibilityGuide(parent)
}

func showAccessibilityGuide(parent fyne.Window) {
	if platform.AccessibilityTrusted() {
		dialog.ShowInformation("辅助功能已开启",
			"听写注入已可使用：识别内容将写入当前焦点输入框。", parent)
		return
	}

	body := "听写「注入」需要允许本应用控制电脑（辅助功能），用于模拟 ⌘A / ⌘V 写入前台输入框。\n\n" +
		"1. 若刚弹出系统提示，请点击「打开系统设置」\n" +
		"2. 在「隐私与安全性 → 辅助功能」中打开「赛搏小小助手」「openclaw-connector」或「Terminal」（若用 go run）\n" +
		"3. 返回后点击下方「重新检测」"

	d := dialog.NewCustomConfirm(
		"需要辅助功能权限",
		"打开系统设置",
		"稍后",
		widget.NewLabel(body),
		func(openSettings bool) {
			if openSettings {
				_ = openMacAccessibilityPrefs()
				platform.RequestAccessibilityPrompt()
			}
			promptRecheckAccessibility(parent)
		},
		parent,
	)
	d.Show()
}

func promptRecheckAccessibility(parent fyne.Window) {
	recheck := func() {
		if platform.AccessibilityTrusted() {
			dialog.ShowInformation("已授权",
				"辅助功能已开启，可以开始使用听写注入。", parent)
			return
		}
		platform.RequestAccessibilityPrompt()
		dialog.ShowConfirm("尚未检测到授权",
			"请确认已在「辅助功能」列表中勾选本应用，然后重试。\n\n是否再次打开系统设置？",
			func(again bool) {
				if again {
					_ = openMacAccessibilityPrefs()
					platform.RequestAccessibilityPrompt()
					promptRecheckAccessibility(parent)
				}
			}, parent)
	}

	dialog.ShowConfirm("授权辅助功能",
		"完成系统设置中的勾选后，点击「重新检测」。",
		func(ok bool) {
			if ok {
				recheck()
			}
		}, parent)
}
