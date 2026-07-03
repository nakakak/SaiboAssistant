//go:build !darwin && !windows && !linux

package dictation

func clearInjectField(targetApp string) error {
	_ = targetApp
	return nil
}

func replaceFocusedField(text string, targetApp string, commit bool) error {
	_, _ = commit
	_, _ = text, targetApp
	return nil
}

func injectSendEnterKey(targetApp string) error {
	_ = targetApp
	return nil
}

func LogInjectCapability(injectEnabled bool) {
	_ = injectEnabled
}

func clearSessionInjectTarget() {}

func hasSessionInjectTarget() bool {
	return false
}
