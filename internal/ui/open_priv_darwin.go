//go:build darwin

package ui

import "os/exec"

func openMacAccessibilityPrefs() error {
	return exec.Command("open", "x-apple.systempreferences:com.apple.preference.security?Privacy_Accessibility").Start()
}
