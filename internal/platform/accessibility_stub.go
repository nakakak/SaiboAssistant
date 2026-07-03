//go:build !darwin

package platform

// AccessibilityTrusted is always true on non-macOS (inject uses other mechanisms).
func AccessibilityTrusted() bool {
	return true
}

// RequestAccessibilityPrompt is a no-op off macOS.
func RequestAccessibilityPrompt() bool {
	return true
}
