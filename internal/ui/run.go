package ui

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"openclaw-connector/internal/cloud"
	"openclaw-connector/internal/config"
	"openclaw-connector/internal/dictation"
	"openclaw-connector/internal/driver"
)

// Runner holds GUI state and the reconnect channel for the WS client.
type Runner struct {
	app            fyne.App
	cfgPath        string
	parentCtx      context.Context
	mu             sync.Mutex
	cfg            *config.Config
	reconnect      chan struct{}
	statusWin         fyne.Window
	statusLabel       *widget.Label
	statusDetail      *widget.Label
	statusServerURL   *widget.Label
	mainInstalled     bool
	connectorStart    bool
}

func (r *Runner) getCfg() *config.Config {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.cfg
}

func (r *Runner) setCfg(c *config.Config) {
	r.mu.Lock()
	r.cfg = c
	r.mu.Unlock()
}

func (r *Runner) reloadCfgFromDisk() {
	c, err := config.Load(r.cfgPath)
	if err != nil {
		log.Printf("ui: reload config: %v", err)
		return
	}
	if err := config.ResolveDeviceMAC(context.Background(), c, config.NewResolveHTTPClient(), r.cfgPath); err != nil {
		log.Printf("ui: resolve device_mac: %v", err)
	}
	if err := config.Validate(c); err != nil {
		log.Printf("ui: config invalid after reload: %v", err)
	}
	r.setCfg(c)
	r.refreshStatusServerURL()
	dictation.InitSubtitleUI(r.app, c.DictationSubtitle())
}

func (r *Runner) triggerReconnect() {
	select {
	case r.reconnect <- struct{}{}:
	default:
	}
}

func drainReconnect(ch chan struct{}) {
	select {
	case <-ch:
	default:
	}
}

func (r *Runner) connectorLoop(ctx context.Context) {
	for {
		if ctx.Err() != nil {
			return
		}
		r.reloadCfgFromDisk()
		cfg := r.getCfg()
		runCtx, runCancel := context.WithCancel(ctx)
		errCh := make(chan error, 1)
		var drv driver.Driver
		if cfg.ChannelOpenClaw() {
			drv = driver.NewFromConfig(cfg)
		} else {
			drv = driver.Echo{}
		}
		r.setConnStatus("正在连接商家云…", strings.TrimSpace(cfg.ServerURL))
		cli := cloud.New(cfg, drv)
		cli.OnConnected = func() {
			r.setConnStatus("已连接商家云", "运行中，可接收云端任务与听写")
		}
		cli.OnDisconnected = func(err error) {
			if err != nil && !errors.Is(err, context.Canceled) {
				r.setConnStatus("未连接（约 2 秒后自动重试）", friendlyConnErr(err))
			}
		}
		go func() {
			errCh <- cli.Run(runCtx)
		}()

		select {
		case <-ctx.Done():
			runCancel()
			<-errCh
			return
		case <-r.reconnect:
			runCancel()
			<-errCh
			r.reloadCfgFromDisk()
			drainReconnect(r.reconnect)
			continue
		case err := <-errCh:
			runCancel()
			if ctx.Err() != nil {
				return
			}
			if err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("connector: disconnected: %v; reconnecting in 2s", err)
				r.setConnStatus("未连接（约 2 秒后自动重试）", friendlyConnErr(err))
			}
			select {
			case <-ctx.Done():
				return
			case <-r.reconnect:
				r.reloadCfgFromDisk()
				drainReconnect(r.reconnect)
				continue
			case <-time.After(2 * time.Second):
				continue
			}
		}
	}
}

func (r *Runner) dialogParent() fyne.Window {
	ws := r.app.Driver().AllWindows()
	if len(ws) > 0 {
		return ws[0]
	}
	w := r.app.NewWindow("")
	w.Hide()
	return w
}

func friendlyConnErr(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	switch {
	case strings.Contains(low, "connection refused"):
		return "无法连上商家云：地址或端口不可达，请核对向导里填写的 server_url（须含 /bridge/connector）。\n" + msg
	case strings.Contains(low, "dial tcp"):
		return "无法连上商家云，请检查 server_url 与网络。\n" + msg
	default:
		return msg
	}
}

func (r *Runner) refreshStatusServerURL() {
	c := r.getCfg()
	if c == nil {
		return
	}
	url := strings.TrimSpace(c.ServerURL)
	fyne.Do(func() {
		if r.statusServerURL != nil {
			r.statusServerURL.SetText(url)
		}
	})
}

func (r *Runner) setConnStatus(main, detail string) {
	fyne.Do(func() {
		if r.statusLabel != nil {
			r.statusLabel.SetText(main)
		}
		if r.statusDetail != nil {
			r.statusDetail.SetText(detail)
		}
	})
}

// showStatusWindowOnMainThread 须在 Fyne 主线程调用（创建/显示运行状态窗）。
func (r *Runner) showStatusWindowOnMainThread(cfg *config.Config) {
	if cfg == nil {
		return
	}
	if r.statusWin != nil {
		r.setCfg(cfg)
		r.refreshStatusServerURL()
		r.statusWin.Show()
		r.statusWin.RequestFocus()
		return
	}
	srv := widget.NewLabel(strings.TrimSpace(cfg.ServerURL))
	srv.Wrapping = fyne.TextWrapWord
	srv.Alignment = fyne.TextAlignCenter
	r.statusServerURL = srv

	r.statusLabel = widget.NewLabel("正在连接商家云…")
	r.statusLabel.Alignment = fyne.TextAlignCenter
	r.statusDetail = widget.NewLabel("")
	r.statusDetail.Alignment = fyne.TextAlignCenter
	r.statusDetail.Wrapping = fyne.TextWrapWord

	settingsBtn := widget.NewButton("连接设置", func() { r.openSettings() })
	reconnectBtn := widget.NewButton("重连云端", func() {
		r.reloadCfgFromDisk()
		r.refreshStatusServerURL()
		r.setConnStatus("正在连接商家云…", "正在使用当前 config.yaml 中的 server_url 重连…")
		r.triggerReconnect()
	})
	quitBtn := widget.NewButton("退出程序", func() { r.quitApp() })

	hint := widget.NewLabel("关闭窗口仅隐藏界面。要完全退出：点「退出程序」，或菜单栏托盘图标 → 退出。")
	hint.Wrapping = fyne.TextWrapWord
	hint.Alignment = fyne.TextAlignCenter

	body := container.NewVBox(
		container.NewCenter(widget.NewLabel(AppDisplayNameZh)),
		srv,
		widget.NewSeparator(),
		r.statusLabel,
		r.statusDetail,
		container.NewCenter(container.NewHBox(settingsBtn, reconnectBtn, quitBtn)),
		hint,
	)
	w := r.app.NewWindow(AppDisplayNameZh + " — 运行状态")
	w.Resize(fyne.NewSize(520, 300))
	w.SetContent(container.NewPadded(body))
	w.SetCloseIntercept(func() { w.Hide() })
	if res := AppIconResource(); res != nil {
		w.SetIcon(res)
	}
	w.Show()
	w.RequestFocus()
	r.statusWin = w
}

func (r *Runner) showStatusWindow(cfg *config.Config) {
	fyne.Do(func() {
		r.showStatusWindowOnMainThread(cfg)
	})
}

func (r *Runner) quitApp() {
	fyne.Do(func() {
		log.Println("ui: quit requested")
		r.app.Quit()
	})
}

func (r *Runner) installMainUI(cfg *config.Config) {
	if r.mainInstalled {
		return
	}
	r.mainInstalled = true

	dictation.InitSubtitleUI(r.app, cfg.DictationSubtitle())
	r.showStatusWindowOnMainThread(cfg)
	log.Println("ui: 运行状态窗口与托盘已就绪")

	if d, ok := r.app.(desktop.App); ok {
		if res := AppIconResource(); res != nil {
			d.SetSystemTrayIcon(res)
		}
		menu := fyne.NewMenu(AppDisplayNameZh,
			fyne.NewMenuItem("运行状态", func() {
				c := r.getCfg()
				if c != nil {
					r.showStatusWindow(c)
				}
			}),
			fyne.NewMenuItem("连接设置", func() { r.openSettings() }),
			fyne.NewMenuItem("通道设置", func() {
				fyne.Do(func() {
					OpenChannelSettingsDialog(r.dialogParent(), r.cfgPath, func() {
						r.reloadCfgFromDisk()
						r.triggerReconnect()
					})
				})
			}),
			fyne.NewMenuItem("辅助功能权限（听写注入）", func() {
				fyne.Do(func() {
					EnsureMacAccessibilityForInject(r.dialogParent(), r.getCfg())
				})
			}),
			fyne.NewMenuItem("使用说明", func() { r.openGuide() }),
			fyne.NewMenuItemSeparator(),
			fyne.NewMenuItem("重连云端", func() {
				r.reloadCfgFromDisk()
				r.refreshStatusServerURL()
				r.setConnStatus("正在连接商家云…", "正在使用当前 config.yaml 中的 server_url 重连…")
				r.triggerReconnect()
			}),
			fyne.NewMenuItem("退出", func() { r.quitApp() }),
		)
		d.SetSystemTrayMenu(menu)
	} else {
		log.Println("ui: system tray not supported; use -headless for headless mode")
		fallback := widget.NewButton("打开连接设置", func() { r.openSettings() })
		chBtn := widget.NewButton("通道设置", func() {
			OpenChannelSettingsDialog(r.dialogParent(), r.cfgPath, func() {
				r.reloadCfgFromDisk()
				r.triggerReconnect()
			})
		})
		gbtn := widget.NewButton("使用说明", func() { r.openGuide() })
		h := r.app.NewWindow(AppDisplayNameZh)
		h.SetContent(container.NewVBox(
			widget.NewLabel("当前环境无系统托盘图标。\n请用下方按钮配置，或使用 -headless 无界面模式。"),
			fallback,
			chBtn,
			gbtn,
		))
		h.Resize(fyne.NewSize(420, 160))
		h.SetCloseIntercept(func() { h.Hide() })
		h.Show()
	}

	if !r.connectorStart {
		r.connectorStart = true
		go r.connectorLoop(r.parentCtx)
	}

	go func() {
		time.Sleep(900 * time.Millisecond)
		fyne.Do(func() {
			EnsureMacAccessibilityForInject(r.dialogParent(), cfg)
		})
	}()
}

// Run starts the Fyne app with system tray, optional dictation subtitle window, and the cloud client.
func Run(parent context.Context, cfg *config.Config, cfgPath string) error {
	a := app.NewWithID("top.cyber.openclaw.connector")
	a.Settings().SetTheme(theme.DarkTheme())
	if res := AppIconResource(); res != nil {
		a.SetIcon(res)
	}

	r := &Runner{
		app:       a,
		cfgPath:   cfgPath,
		cfg:       cfg,
		parentCtx: parent,
		reconnect: make(chan struct{}, 1),
	}

	if cfg.FirstRun {
		showOnboarding(a, cfgPath, func(saved bool) {
			if !saved {
				fyne.Do(func() { a.Quit() })
				return
			}
			log.Println("onboarding: completed")
			loaded, err := config.Load(cfgPath)
			if err != nil {
				log.Printf("onboarding: reload: %v", err)
				fyne.Do(func() {
					dialog.ShowError(err, a.NewWindow(""))
				})
				return
			}
			if err := config.ResolveDeviceMAC(context.Background(), loaded, config.NewResolveHTTPClient(), cfgPath); err != nil {
				log.Printf("onboarding: resolve device_mac: %v", err)
			}
			r.setCfg(loaded)
			// 须在关闭向导窗口之前同步创建运行状态窗，否则 Fyne 会因无窗口而直接退出。
			r.installMainUI(loaded)
		})
	} else {
		r.installMainUI(cfg)
	}

	go func() {
		<-parent.Done()
		fyne.Do(func() { a.Quit() })
	}()

	a.Run()
	return nil
}

func (r *Runner) openGuide() {
	t := widget.NewLabel(GuideZh)
	t.Wrapping = fyne.TextWrapWord
	scroll := container.NewScroll(t)
	scroll.SetMinSize(fyne.NewSize(560, 420))
	w := r.app.NewWindow("使用说明")
	w.SetContent(container.NewBorder(nil, nil, nil, nil, scroll))
	w.Resize(fyne.NewSize(600, 480))
	w.Show()
}

func (r *Runner) openSettings() {
	c := r.getCfg()
	chOpen := widget.NewCheck("通道：OpenClaw 任务桥接", nil)
	chOpen.SetChecked(c.ChannelOpenClaw())
	chTel := widget.NewCheck("通道：Telnet / 听写转发", nil)
	chTel.SetChecked(c.ChannelTelnet())

	srv := widget.NewEntry()
	srv.SetText(c.ServerURL)
	deviceID := NewDeviceIDFields(c.DeviceMAC, c.PairCode)

	mode := widget.NewSelect([]string{"echo", "http", "gateway_ws"}, func(string) {})
	sel := c.OpenClaw.Mode
	if sel == "" {
		sel = "echo"
	}
	mode.SetSelected(sel)

	baseURL := widget.NewEntry()
	baseURL.SetText(c.OpenClaw.BaseURL)
	gwTok := widget.NewPasswordEntry()
	gwTok.SetText(c.OpenClaw.GatewayToken)
	if gwTok.Text == "" {
		gwTok.SetText(c.OpenClaw.BearerToken)
	}
	gwURL := widget.NewEntry()
	gwURL.SetText(c.OpenClaw.GatewayWSURL)
	gwURL.SetPlaceHolder("可留空，由 base_url 推导")
	model := widget.NewEntry()
	model.SetText(c.OpenClaw.Model)

	chEn := widget.NewCheck("接收听写 dictation.stt", nil)
	if c.Dictation.Enabled != nil {
		chEn.SetChecked(*c.Dictation.Enabled)
	} else {
		chEn.SetChecked(true)
	}
	if !c.ChannelTelnet() {
		chEn.SetChecked(false)
	}
	chInj := widget.NewCheck("听写注入到输入框（inject）", nil)
	if c.Dictation.Inject != nil {
		chInj.SetChecked(*c.Dictation.Inject)
	} else {
		chInj.SetChecked(true)
	}

	ocHint := widget.NewLabel("OpenClaw 为 gateway_ws 且地址为本机（127.0.0.1）时，保存后会自动写入 gateway_same_host，无需手改 config.yaml。")
	ocHint.Wrapping = fyne.TextWrapWord

	form := widget.NewForm(
		widget.NewFormItem("功能通道", container.NewVBox(chOpen, chTel)),
		widget.NewFormItem("商家云 WebSocket (server_url)", srv),
		widget.NewFormItem("设备标识（二选一）", deviceID.Widget),
		widget.NewFormItem("OpenClaw 说明", ocHint),
		widget.NewFormItem("openclaw.mode", container.NewVBox(mode)),
		widget.NewFormItem("openclaw.base_url", baseURL),
		widget.NewFormItem("openclaw.gateway_ws_url", gwURL),
		widget.NewFormItem("gateway_token / bearer", gwTok),
		widget.NewFormItem("openclaw.model", model),
		widget.NewFormItem("听写", container.NewVBox(chEn, chInj)),
	)

	w := r.app.NewWindow("连接设置")
	if res := AppIconResource(); res != nil {
		w.SetIcon(res)
	}
	hint := widget.NewLabel("日常修改配置用本页；不是首次向导，无需「上一步」。点「取消」或关闭窗口即可回到运行状态。")
	hint.Wrapping = fyne.TextWrapWord

	cancel := widget.NewButton("取消", func() { w.Close() })
	save := widget.NewButton("保存并重连", func() {
		if strings.TrimSpace(srv.Text) == "" {
			dialog.ShowInformation("填写商家云", "请填写商家云地址。", w)
			return
		}
		if err := deviceID.Validate(); err != nil {
			dialog.ShowInformation("填写设备信息", err.Error(), w)
			return
		}
		nc := buildConfigFromForm(c, chOpen, chTel, srv, deviceID, mode, baseURL, gwURL, gwTok, model, chEn, chInj)
		if err := config.Save(r.cfgPath, nc); err != nil {
			dialog.ShowError(err, w)
			return
		}
		loaded, err := config.Load(r.cfgPath)
		if err != nil {
			dialog.ShowError(err, w)
			return
		}
		r.setCfg(loaded)
		r.refreshStatusServerURL()
		dictation.InitSubtitleUI(r.app, loaded.DictationSubtitle())
		r.setConnStatus("正在连接商家云…", "配置已保存，正在连接新商家云…")
		r.triggerReconnect()
		dialog.ShowInformation("已保存", "配置已写入并已请求重连。", w)
		w.Close()
	})
	actions := container.NewBorder(nil, nil, cancel, save, layout.NewSpacer())
	scroll := container.NewScroll(form)
	scroll.SetMinSize(fyne.NewSize(520, 420))
	w.SetContent(container.NewBorder(actions, nil, nil, nil, container.NewBorder(nil, hint, nil, nil, scroll)))
	w.Resize(fyne.NewSize(580, 600))
	w.Show()
	r.showStatusWindow(c)
}

func buildConfigFromForm(
	base *config.Config,
	chOpen, chTel *widget.Check,
	srv *widget.Entry,
	deviceID *DeviceIDFields,
	mode *widget.Select,
	baseURL, gwURL, gwTok, model *widget.Entry,
	chEn, chInj *widget.Check,
) *config.Config {
	nc := *base
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
	nc.ServerURL = strings.TrimSpace(srv.Text)
	nc.Token = ""
	deviceID.Apply(&nc)
	nc.OpenClaw.Mode = mode.Selected
	nc.OpenClaw.BaseURL = baseURL.Text
	nc.OpenClaw.GatewayWSURL = gwURL.Text
	nc.OpenClaw.GatewayToken = gwTok.Text
	nc.OpenClaw.BearerToken = ""
	nc.OpenClaw.Model = model.Text
	en := chEn.Checked && chTel.Checked
	nc.Dictation.Enabled = &en
	inj := chInj.Checked
	nc.Dictation.Inject = &inj
	nc.Dictation.Subtitle = &f // 产品默认：仅 inject，无悬浮字幕
	return &nc
}
