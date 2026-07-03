//go:build windows

package dictation

import (
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/micmonay/keybd_event"
)

func clearInjectField(targetApp string) error {
	_ = targetApp
	if err := keyComboCtrl(keybd_event.VK_A); err != nil {
		return err
	}
	time.Sleep(40 * time.Millisecond)
	kb, err := keybd_event.NewKeyBonding()
	if err != nil {
		return err
	}
	kb.SetKeys(keybd_event.VK_DELETE)
	return kb.Launching()
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
	time.Sleep(50 * time.Millisecond)
	if err := keyComboCtrl(keybd_event.VK_A); err != nil {
		return err
	}
	time.Sleep(40 * time.Millisecond)
	return keyComboCtrl(keybd_event.VK_V)
}

func keyComboCtrl(key int) error {
	kb, err := keybd_event.NewKeyBonding()
	if err != nil {
		return err
	}
	kb.HasCTRL(true)
	kb.SetKeys(key)
	return kb.Launching()
}

func injectSendEnterKey(targetApp string) error {
	_ = targetApp
	kb, err := keybd_event.NewKeyBonding()
	if err != nil {
		return err
	}
	kb.SetKeys(keybd_event.VK_ENTER)
	return kb.Launching()
}
