package ui

import (
	_ "embed"

	"fyne.io/fyne/v2"
)

//go:embed iconai.jpg
var embeddedIconJPG []byte

// AppIconResource 内置应用图标（JPEG），用于窗口与托盘；无数据时返回 nil。
func AppIconResource() fyne.Resource {
	if len(embeddedIconJPG) == 0 {
		return nil
	}
	return fyne.NewStaticResource("iconai.jpg", embeddedIconJPG)
}
