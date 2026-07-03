//go:build linux

package dictation

import (
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/atotto/clipboard"
)

func clearInjectField(targetApp string) error {
	_ = targetApp
	if err := clipboard.WriteAll(""); err != nil {
		return err
	}
	time.Sleep(45 * time.Millisecond)
	if err := exec.Command("xdotool", "key", "--clearmodifiers", "ctrl+a", "BackSpace").Run(); err == nil {
		return nil
	}
	if _, err := exec.LookPath("wtype"); err == nil {
		if err := exec.Command("wtype", "-M", "ctrl", "a").Run(); err == nil {
			time.Sleep(30 * time.Millisecond)
			return exec.Command("wtype", "BackSpace").Run()
		}
	}
	return fmt.Errorf("dictation clear inject: need xdotool or wtype")
}

func replaceFocusedField(text string, targetApp string, commit bool) error {
	_ = commit
	_ = targetApp
	if strings.TrimSpace(text) == "" {
		return clearInjectField(targetApp)
	}
	if err := clipboard.WriteAll(text); err != nil {
		return err
	}
	time.Sleep(45 * time.Millisecond)

	xdotErr := exec.Command("xdotool", "key", "--clearmodifiers", "ctrl+a", "ctrl+v").Run()
	if xdotErr == nil {
		return nil
	}

	if _, err := exec.LookPath("wtype"); err == nil {
		if err := exec.Command("wtype", "-M", "ctrl", "a").Run(); err == nil {
			time.Sleep(30 * time.Millisecond)
			if err := exec.Command("wtype", "-M", "ctrl", "v").Run(); err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("dictation inject: need xdotool (X11) or wtype (Wayland) in PATH: xdotool: %w", xdotErr)
}

func injectSendEnterKey(targetApp string) error {
	_ = targetApp
	if err := exec.Command("xdotool", "key", "--clearmodifiers", "Return").Run(); err == nil {
		return nil
	}
	if _, err := exec.LookPath("wtype"); err == nil {
		return exec.Command("wtype", "\n").Run()
	}
	return fmt.Errorf("dictation inject enter: xdotool/wtype failed")
}
