package ui

import (
	"log"
	"runtime"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"openclaw-connector/internal/config"
)

// showOnboarding 在已有 Fyne 应用上显示首次配置向导；保存后关闭窗口并回调 onDone，不退出应用。
func showOnboarding(a fyne.App, cfgPath string, onDone func(saved bool)) {
	cfg, err := config.UnmarshalFromFile(cfgPath)
	if err != nil {
		log.Printf("onboarding: load config: %v", err)
		onDone(false)
		return
	}
	config.ApplyBundledDefaults(cfg)

	var saved bool
	step := 0

	chOpen := widget.NewCheck("OpenClaw：云端任务发到本电脑", nil)
	chOpen.SetChecked(cfg.ChannelOpenClaw())
	chTel := widget.NewCheck("听写：语音转文字传到本电脑", nil)
	chTel.SetChecked(cfg.ChannelTelnet())

	srv := widget.NewEntry()
	srv.SetPlaceHolder("向商家索取，例如 wss://域名/bridge/connector 或 ws://IP:端口/bridge/connector")
	if u := strings.TrimSpace(cfg.ServerURL); u != "" {
		srv.SetText(u)
	}
	srv.Disable() // 内置默认商家云地址，用户无需手填

	pairHint := widget.NewLabel("在喵伴上打开「OpenClaw 对话」或「听写转发」页，将屏幕显示的 6 位配对码填入下方（Connector 离线时才显示；若助手已连接请先退出助手再看屏）。")
	pairHint.Wrapping = fyne.TextWrapWord

	deviceID := NewDeviceIDFields(cfg.DeviceMAC, cfg.PairCode)

	useRealOpenClaw := widget.NewCheck("连接 OpenClaw 处理任务（否则只做线路自测 echo）", nil)
	modeLower := strings.ToLower(strings.TrimSpace(cfg.OpenClaw.Mode))
	gwTokStr := strings.TrimSpace(cfg.OpenClaw.GatewayToken)
	if gwTokStr == "" {
		gwTokStr = strings.TrimSpace(cfg.OpenClaw.BearerToken)
	}
	useRealOpenClaw.SetChecked(modeLower == "gateway_ws" && gwTokStr != "")

	baseURL := widget.NewEntry()
	bu := strings.TrimSpace(cfg.OpenClaw.BaseURL)
	if bu == "" {
		bu = "http://127.0.0.1:18789"
	}
	baseURL.SetText(bu)
	baseURL.SetPlaceHolder("本机示例 http://127.0.0.1:18789；云端填你的 OpenClaw 服务地址")

	gwTok := widget.NewPasswordEntry()
	gwTok.SetText(gwTokStr)
	gwTok.SetPlaceHolder("openclaw.json → gateway.auth.token")

	gwURL := widget.NewEntry()
	gwURL.SetText(strings.TrimSpace(cfg.OpenClaw.GatewayWSURL))
	gwURL.SetPlaceHolder("可填 ws://主机:18789；留空则由 OpenClaw 地址推导")

	model := widget.NewEntry()
	model.SetText(strings.TrimSpace(cfg.OpenClaw.Model))
	model.SetPlaceHolder("一般留空")

	advOC := container.NewVBox(
		widget.NewForm(
			widget.NewFormItem("gateway_ws_url（可选）", gwURL),
			widget.NewFormItem("model（可选）", model),
		),
	)
	advOCOpen := false
	advOCBtn := widget.NewButton("▼ 更多 OpenClaw 选项", nil)

	gwBlock := container.NewVBox(
		widget.NewLabel("填写 OpenClaw Gateway 的 Token 与访问地址（可部署在本机或云端，与商家云无关）。\n本机地址（如 127.0.0.1:18789）保存后会自动配置网关连接方式，无需在 config.yaml 里手改 gateway_same_host。"),
		widget.NewForm(
			widget.NewFormItem("Gateway Token", gwTok),
			widget.NewFormItem("OpenClaw 地址", baseURL),
		),
		advOCBtn,
		advOC,
	)

	agreePerm := widget.NewCheck("我已阅读并同意为所选功能授予所需系统权限与网络访问（见上方权限说明）", nil)
	agreeVoluntary := widget.NewCheck("我自愿使用「"+AppDisplayNameZh+"」，并已阅读、理解并同意下方自愿使用说明", nil)

	perm := widget.NewLabel(onboardingPermissionText())
	perm.Wrapping = fyne.TextWrapWord
	permScroll := container.NewScroll(perm)
	permScroll.SetMinSize(fyne.NewSize(540, 130))

	voluntary := widget.NewLabel(onboardingVoluntaryUseText())
	voluntary.Wrapping = fyne.TextWrapWord
	voluntaryScroll := container.NewScroll(voluntary)
	voluntaryScroll.SetMinSize(fyne.NewSize(540, 110))

	cloudHint := widget.NewLabel(onboardingCloudHelpShort())
	cloudHint.Wrapping = fyne.TextWrapWord

	advOCBtn.OnTapped = func() {
		advOCOpen = !advOCOpen
		if advOCOpen {
			advOC.Show()
			advOCBtn.SetText("▲ 收起更多选项")
		} else {
			advOC.Hide()
			advOCBtn.SetText("▼ 更多 OpenClaw 选项")
		}
	}
	advOC.Hide()

	openClawForm := container.NewVBox(
		widget.NewSeparator(),
		widget.NewLabel("OpenClaw"),
		useRealOpenClaw,
		gwBlock,
	)
	cloudForm := container.NewVBox(
		cloudHint,
		pairHint,
		widget.NewForm(
			widget.NewFormItem("商家云地址（已内置）", srv),
		),
		deviceID.Widget,
	)
	pairingBody := container.NewVBox(cloudForm, openClawForm)
	pairingScroll := container.NewScroll(pairingBody)
	pairingScroll.SetMinSize(fyne.NewSize(580, 300))

	center := container.NewStack()
	w := a.NewWindow(AppDisplayNameZh + " — 首次配置")
	w.Resize(fyne.NewSize(620, 620))
	w.SetFixedSize(true)
	if res := AppIconResource(); res != nil {
		w.SetIcon(res)
	}

	backBtn := widget.NewButton("上一步", nil)
	nextBtn := widget.NewButton("下一步", nil)
	openPriv := widget.NewButton("打开系统隐私设置（macOS 辅助功能）", func() {
		if runtime.GOOS == "darwin" {
			if err := openMacAccessibilityPrefs(); err != nil {
				dialog.ShowError(err, w)
			}
		} else {
			dialog.ShowInformation("提示", "Windows / Linux 请在系统设置或桌面环境中授予剪贴板、辅助输入等相关权限（见上文说明）。", w)
		}
	})

	updatePairingVisibility := func() {
		if chOpen.Checked {
			openClawForm.Show()
			if useRealOpenClaw.Checked {
				gwBlock.Show()
			} else {
				gwBlock.Hide()
			}
		} else {
			openClawForm.Hide()
		}
	}
	chOpen.OnChanged = func(_ bool) { updatePairingVisibility() }
	chTel.OnChanged = func(_ bool) { updatePairingVisibility() }
	useRealOpenClaw.OnChanged = func(_ bool) { updatePairingVisibility() }

	save := func() {
		if !chOpen.Checked && !chTel.Checked {
			dialog.ShowInformation("请选择", "请至少选择一项功能（OpenClaw 或 听写转发）。", w)
			return
		}
		if !agreePerm.Checked || !agreeVoluntary.Checked {
			dialog.ShowInformation("确认", "请勾选权限同意与自愿使用同意两项后再完成配置。", w)
			return
		}
		nc, err := config.UnmarshalFromFile(cfgPath)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		t, f := true, false
		if chOpen.Checked {
			nc.Channels.OpenClaw = &t
		} else {
			nc.Channels.OpenClaw = &f
		}
		if chTel.Checked {
			nc.Channels.Telnet = &t
		} else {
			nc.Channels.Telnet = &f
		}
		if strings.TrimSpace(srv.Text) == "" {
			dialog.ShowInformation("填写商家云", "请填写商家云地址。", w)
			return
		}
		if err := deviceID.Validate(); err != nil {
			dialog.ShowInformation("填写设备信息", err.Error(), w)
			return
		}
		nc.ServerURL = strings.TrimSpace(srv.Text)
		if nc.ServerURL == "" {
			nc.ServerURL = config.DefaultServerURL()
		}
		nc.Token = ""
		deviceID.Apply(nc)
		if chOpen.Checked {
			if useRealOpenClaw.Checked {
				nc.OpenClaw.Mode = "gateway_ws"
				bu := strings.TrimSpace(baseURL.Text)
				if bu == "" {
					bu = "http://127.0.0.1:18789"
				}
				nc.OpenClaw.BaseURL = bu
				nc.OpenClaw.GatewayWSURL = strings.TrimSpace(gwURL.Text)
				nc.OpenClaw.GatewayToken = strings.TrimSpace(gwTok.Text)
				nc.OpenClaw.BearerToken = ""
				nc.OpenClaw.Model = strings.TrimSpace(model.Text)
			} else {
				nc.OpenClaw.Mode = "echo"
				nc.OpenClaw.BaseURL = ""
				nc.OpenClaw.GatewayWSURL = ""
				nc.OpenClaw.GatewayToken = ""
				nc.OpenClaw.BearerToken = ""
				nc.OpenClaw.Model = ""
			}
		} else {
			nc.OpenClaw.Mode = "echo"
			nc.OpenClaw.BaseURL = ""
			nc.OpenClaw.GatewayWSURL = ""
			nc.OpenClaw.GatewayToken = ""
			nc.OpenClaw.BearerToken = ""
			nc.OpenClaw.Model = ""
		}
		if chTel.Checked {
			t, f := true, false
			nc.Dictation.Enabled = &t
			nc.Dictation.Inject = &t
			nc.Dictation.Subtitle = &f // 默认仅注入，不启用悬浮字幕
		} else {
			f := false
			nc.Dictation.Enabled = &f
			nc.Dictation.Inject = &f
			nc.Dictation.Subtitle = &f
		}
		nc.FirstRun = false
		if err := config.Save(cfgPath, nc); err != nil {
			dialog.ShowError(err, w)
			return
		}
		if chTel.Checked && nc.DictationInject() {
			EnsureMacAccessibilityForInject(w, nc)
		}
		saved = true
		onDone(true) // 先进入主界面（运行状态窗），再关向导
		w.Hide()
	}

	centerRow := func(obj fyne.CanvasObject) fyne.CanvasObject {
		return container.NewHBox(layout.NewSpacer(), obj, layout.NewSpacer())
	}

	welcomeAndFeatures := func() fyne.CanvasObject {
		title := canvas.NewText(AppDisplayNameZh, theme.ForegroundColor())
		title.TextSize = 28
		title.TextStyle = fyne.TextStyle{Bold: true}
		title.Alignment = fyne.TextAlignCenter

		featHint := widget.NewLabel("勾选你需要的功能（可多选）。")
		featHint.Alignment = fyne.TextAlignCenter
		featHint.Wrapping = fyne.TextWrapOff

		var hero fyne.CanvasObject
		if res := AppIconResource(); res != nil {
			img := canvas.NewImageFromResource(res)
			img.FillMode = canvas.ImageFillContain
			img.SetMinSize(fyne.NewSize(96, 96))
			hero = container.NewCenter(img)
		} else {
			hero = layout.NewSpacer()
		}
		checks := container.NewVBox(chOpen, chTel)
		return container.NewPadded(container.NewVBox(
			hero,
			centerRow(title),
			widget.NewSeparator(),
			featHint,
			centerRow(checks),
		))
	}

	perms := func() fyne.CanvasObject {
		intro := widget.NewLabel("请阅读以下内容，并勾选下方两项同意声明。")
		intro.Wrapping = fyne.TextWrapWord
		return container.NewVBox(
			intro,
			widget.NewLabel("一、权限与网络"),
			permScroll,
			widget.NewLabel("二、自愿使用"),
			voluntaryScroll,
			agreePerm,
			agreeVoluntary,
			openPriv,
		)
	}

	var redraw func()
	backBtn.OnTapped = func() {
		if step > 0 {
			step--
			redraw()
		}
	}
	redraw = func() {
		center.RemoveAll()
		switch step {
		case 0:
			center.Add(welcomeAndFeatures())
			backBtn.Disable()
			nextBtn.SetText("下一步")
			nextBtn.OnTapped = func() {
				if !chOpen.Checked && !chTel.Checked {
					dialog.ShowInformation("请选择", "请至少选择一项功能。", w)
					return
				}
				updatePairingVisibility()
				step = 1
				redraw()
			}
		case 1:
			updatePairingVisibility()
			center.Add(container.NewBorder(
				widget.NewLabel("必填：商家云地址；连接码与设备 MAC 二选一。其余保持默认即可。"),
				nil, nil, nil,
				pairingScroll,
			))
			backBtn.Enable()
			nextBtn.SetText("下一步")
			nextBtn.OnTapped = func() {
				if strings.TrimSpace(srv.Text) == "" {
					dialog.ShowInformation("填写商家云", "请填写商家云地址。", w)
					return
				}
				if err := deviceID.Validate(); err != nil {
					dialog.ShowInformation("填写设备信息", err.Error(), w)
					return
				}
				step = 2
				redraw()
			}
		case 2:
			center.Add(perms())
			backBtn.Enable()
			nextBtn.SetText("完成并保存")
			nextBtn.OnTapped = save
		default:
			step = 0
			redraw()
		}
	}

	nav := container.NewBorder(nil, nil,
		backBtn,
		nextBtn,
		layout.NewSpacer(),
	)
	w.SetContent(container.NewBorder(nil, nav, nil, nil, center))
	w.SetOnClosed(func() {
		if !saved {
			log.Println("onboarding: closed without saving; first_run unchanged")
			onDone(false)
		}
	})
	redraw()
	w.Show()
	if cfg.DictationInject() && cfg.ChannelTelnet() {
		EnsureMacAccessibilityForInject(w, cfg)
	}
}

func onboardingCloudHelpShort() string {
	return "商家云地址须含路径 /bridge/connector。连接码与设备 MAC 二选一：填连接码时程序会在首次连接时向云端解析 MAC；若已知 MAC 可直接选 MAC 填写。"
}

func onboardingPermissionText() string {
	return `需要访问网络连接商家云；若开启听写注入，macOS 需在「辅助功能」中允许本应用控制输入等能力。

详细选项可随时在托盘「连接设置」中修改。`
}

func onboardingVoluntaryUseText() string {
	return `本软件由您自愿下载、安装并使用。您应确保使用行为符合当地法律法规及所在组织的规定。

软件将按您的配置连接商家云、OpenClaw 等服务，并在您授权后使用网络与系统能力（如麦克风、辅助功能等）。因网络环境、第三方服务、配置不当或误操作等导致的数据丢失、业务中断或其他后果，须由您自行评估并承担相应风险。

若您不同意上述内容，请关闭本向导，不要勾选并完成配置。`
}

// OpenChannelSettingsDialog edits channels (non-first-run).
func OpenChannelSettingsDialog(parent fyne.Window, cfgPath string, onSaved func()) {
	cfg, err := config.Load(cfgPath)
	if err != nil {
		dialog.ShowError(err, parent)
		return
	}
	chOpen := widget.NewCheck("开启 OpenClaw 通道", nil)
	chOpen.SetChecked(cfg.ChannelOpenClaw())
	chTel := widget.NewCheck("开启 Telnet / 听写转发", nil)
	chTel.SetChecked(cfg.ChannelTelnet())

	content := container.NewVBox(
		chOpen,
		chTel,
		widget.NewLabel("至少开启一项。修改后需重连云端。"),
	)

	d := dialog.NewCustomConfirm("通道设置", "保存", "取消", content, func(ok bool) {
		if !ok {
			return
		}
		if !chOpen.Checked && !chTel.Checked {
			dialog.ShowInformation("提示", "请至少开启一个通道。", parent)
			return
		}
		nc, err := config.UnmarshalFromFile(cfgPath)
		if err != nil {
			dialog.ShowError(err, parent)
			return
		}
		t, f := true, false
		if chOpen.Checked {
			nc.Channels.OpenClaw = &t
		} else {
			nc.Channels.OpenClaw = &f
		}
		if chTel.Checked {
			nc.Channels.Telnet = &t
		} else {
			nc.Channels.Telnet = &f
		}
		if err := config.Save(cfgPath, nc); err != nil {
			dialog.ShowError(err, parent)
			return
		}
		if onSaved != nil {
			onSaved()
		}
	}, parent)
	d.Show()
}
