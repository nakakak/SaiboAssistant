//go:build !darwin

package dictation

// injectRequiresExplicitTarget：Windows/Linux 注入当前键盘焦点，无需 macOS 式 target 捕获。
func injectRequiresExplicitTarget() bool { return false }

func hasSessionInjectTarget() bool {
	return false
}

func captureOrPreserveInjectTarget(existing, _ string) string {
	return existing
}

func captureInjectTarget() string {
	return ""
}

func refreshInjectTargetOnPageEnter(existing string) string {
	return existing
}

func normalizeAppName(name string) string {
	return name
}

func resolveInjectSessionApp(sessionApp string) string {
	return sessionApp
}

func appNameAtSavedMouse() string { return "" }

func refreshInjectTargetFromSavedMouse(existing string) string { return existing }

func logSavedInjectPoint(string) {}

func resetInjectFocus() {}
