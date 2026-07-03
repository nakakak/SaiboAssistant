//go:build darwin

package platform

/*
#cgo LDFLAGS: -framework ApplicationServices -framework CoreFoundation
#include <ApplicationServices/ApplicationServices.h>
#include <CoreFoundation/CoreFoundation.h>
#include <stdbool.h>

static bool ax_is_trusted(bool show_prompt) {
	const void *keys[] = { kAXTrustedCheckOptionPrompt };
	const void *vals[] = { show_prompt ? kCFBooleanTrue : kCFBooleanFalse };
	CFDictionaryRef opts = CFDictionaryCreate(
		NULL, keys, vals, 1,
		&kCFTypeDictionaryKeyCallBacks,
		&kCFTypeDictionaryValueCallBacks);
	if (!opts) {
		return AXIsProcessTrusted() ? true : false;
	}
	Boolean ok = AXIsProcessTrustedWithOptions(opts);
	CFRelease(opts);
	return ok ? true : false;
}
*/
import "C"

// AccessibilityTrusted reports whether this process is allowed to post accessibility events.
func AccessibilityTrusted() bool {
	return bool(C.ax_is_trusted(false))
}

// RequestAccessibilityPrompt asks macOS to show the standard Accessibility consent dialog
// (and opens Privacy settings guidance). Returns whether the app is already trusted.
func RequestAccessibilityPrompt() bool {
	return bool(C.ax_is_trusted(true))
}
