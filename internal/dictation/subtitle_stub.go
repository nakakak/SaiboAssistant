//go:build !darwin

package dictation

import "fyne.io/fyne/v2"

func configureSubtitleWindow(_ fyne.Window) {}

func raiseSubtitleWindow(_ fyne.Window) {}

func setSubtitleInjectPassthrough(_ bool) {}

func duringInjectTargetFocus(fn func()) { fn() }
