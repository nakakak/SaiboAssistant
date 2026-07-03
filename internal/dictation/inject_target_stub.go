//go:build !darwin

package dictation

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
