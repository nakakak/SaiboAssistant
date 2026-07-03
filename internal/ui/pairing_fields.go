package ui

import (
	"fmt"
	"strings"
	"unicode"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
	"openclaw-connector/internal/config"
)

const (
	deviceIDByPair = "连接码（6 位数字）"
	deviceIDByMAC  = "设备 MAC"
)

// DeviceIDFields 连接码与设备 MAC 二选一。
type DeviceIDFields struct {
	Radio  *widget.RadioGroup
	Pair   *widget.Entry
	MAC    *widget.Entry
	inner  *fyne.Container
	Widget fyne.CanvasObject
}

func NewDeviceIDFields(initialMAC, initialPair string) *DeviceIDFields {
	pair := widget.NewEntry()
	pair.SetText(strings.TrimSpace(initialPair))
	pair.SetPlaceHolder("设备或管理台上的 6 位数字")

	mac := widget.NewEntry()
	mac.SetText(strings.TrimSpace(initialMAC))
	mac.SetPlaceHolder("例如 98:88:e0:13:85:38")

	d := &DeviceIDFields{Pair: pair, MAC: mac, inner: container.NewVBox()}
	d.Radio = widget.NewRadioGroup([]string{deviceIDByPair, deviceIDByMAC}, func(string) {
		d.refresh()
	})
	if strings.TrimSpace(initialMAC) != "" && strings.TrimSpace(initialPair) == "" {
		d.Radio.SetSelected(deviceIDByMAC)
	} else {
		d.Radio.SetSelected(deviceIDByPair)
	}
	d.Widget = container.NewVBox(d.Radio, d.inner)
	d.refresh()
	return d
}

func (d *DeviceIDFields) refresh() {
	d.inner.RemoveAll()
	if d.Radio.Selected == deviceIDByMAC {
		d.inner.Add(widget.NewForm(widget.NewFormItem(deviceIDByMAC, d.MAC)))
	} else {
		d.inner.Add(widget.NewForm(widget.NewFormItem(deviceIDByPair, d.Pair)))
	}
}

func digitsOnlyPairCode(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

// Validate 检查二选一已填且格式正确。
func (d *DeviceIDFields) Validate() error {
	if d.Radio.Selected == deviceIDByMAC {
		if strings.TrimSpace(d.MAC.Text) == "" {
			return fmt.Errorf("请填写设备 MAC")
		}
		return nil
	}
	code := digitsOnlyPairCode(d.Pair.Text)
	if code == "" {
		return fmt.Errorf("请填写 6 位连接码")
	}
	if len(code) != 6 {
		return fmt.Errorf("连接码须为 6 位数字")
	}
	return nil
}

// Apply 写入配置：只保留所选方式对应字段，清空另一项。
func (d *DeviceIDFields) Apply(nc *config.Config) {
	if d.Radio.Selected == deviceIDByMAC {
		nc.DeviceMAC = strings.TrimSpace(d.MAC.Text)
		nc.PairCode = ""
	} else {
		nc.PairCode = strings.TrimSpace(d.Pair.Text)
		nc.DeviceMAC = ""
	}
}
