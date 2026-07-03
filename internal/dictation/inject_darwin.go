//go:build darwin

package dictation

/*
#cgo darwin CFLAGS: -x objective-c
#cgo darwin LDFLAGS: -framework ApplicationServices -framework CoreFoundation -framework AppKit
#include <ApplicationServices/ApplicationServices.h>
#include <AppKit/AppKit.h>
#include <string.h>
#include <unistd.h>

static void clickAtPoint(double x, double y) {
	CGPoint p = CGPointMake(x, y);
	CGEventRef down = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseDown, p, kCGMouseButtonLeft);
	CGEventRef up = CGEventCreateMouseEvent(NULL, kCGEventLeftMouseUp, p, kCGMouseButtonLeft);
	CGEventPost(kCGHIDEventTap, down);
	CGEventPost(kCGHIDEventTap, up);
	CFRelease(down);
	CFRelease(up);
}

static void clickAtCurrentMouse(void) {
	CGEventRef ev = CGEventCreate(NULL);
	if (ev == NULL) return;
	CGPoint p = CGEventGetLocation(ev);
	CFRelease(ev);
	clickAtPoint(p.x, p.y);
}

static void readCurrentMouse(double *x, double *y) {
	CGEventRef ev = CGEventCreate(NULL);
	if (ev == NULL) {
		*x = 0;
		*y = 0;
		return;
	}
	CGPoint p = CGEventGetLocation(ev);
	*x = p.x;
	*y = p.y;
	CFRelease(ev);
}

static void postCmdKey(int keyCode) {
	CGEventRef down = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keyCode, true);
	CGEventRef up = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keyCode, false);
	CGEventSetFlags(down, kCGEventFlagMaskCommand);
	CGEventSetFlags(up, kCGEventFlagMaskCommand);
	CGEventPost(kCGHIDEventTap, down);
	CGEventPost(kCGHIDEventTap, up);
	CFRelease(down);
	CFRelease(up);
}

static void postKey(int keyCode) {
	CGEventRef down = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keyCode, true);
	CGEventRef up = CGEventCreateKeyboardEvent(NULL, (CGKeyCode)keyCode, false);
	CGEventPost(kCGHIDEventTap, down);
	CGEventPost(kCGHIDEventTap, up);
	CFRelease(down);
	CFRelease(up);
}

static void injectSelectAllPasteDeselect(void) {
	postCmdKey(0);
	usleep(50000);
	postCmdKey(9);
	usleep(30000);
	postKey(124);
}

static void injectPasteOnly(void) {
	postCmdKey(9);
}

// 光标到文末再 ⌘V，用于流式追加增量，避免 ⌘A 全选蓝底。
static void injectPasteAtEnd(void) {
	postCmdKey(125);
	usleep(20000);
	postCmdKey(9);
}

static void injectSelectAllDelete(void) {
	postCmdKey(0);
	usleep(40000);
	postKey(51);
}

// 直接 AX 写值，无 ⌘A 全选高亮（不经过 AppleScript，避免中文系统语法问题）。
static int axSetFocusedValueUTF8(const char *text) {
	AXUIElementRef sw = AXUIElementCreateSystemWide();
	if (sw == NULL) {
		return -1;
	}
	CFTypeRef ref = NULL;
	AXError err = AXUIElementCopyAttributeValue(sw, kAXFocusedUIElementAttribute, &ref);
	CFRelease(sw);
	if (err != kAXErrorSuccess || ref == NULL) {
		return -2;
	}
	AXUIElementRef el = (AXUIElementRef)ref;
	const char *s = (text != NULL) ? text : "";
	CFStringRef cf = CFStringCreateWithCString(kCFAllocatorDefault, s, kCFStringEncodingUTF8);
	if (cf == NULL) {
		CFRelease(el);
		return -3;
	}
	err = AXUIElementSetAttributeValue(el, kAXValueAttribute, cf);
	if (err != kAXErrorSuccess) {
		err = AXUIElementSetAttributeValue(el, CFSTR("AXText"), cf);
	}
	CFRelease(cf);
	CFRelease(el);
	return (err == kAXErrorSuccess) ? 0 : -4;
}

// 鼠标位置下的 App（Connector 前台时仍能定位 Chrome 等）。
static int copyAppNameAtPoint(double x, double y, char *buf, int buflen) {
	if (buf == NULL || buflen <= 0) {
		return -1;
	}
	buf[0] = '\0';
	AXUIElementRef system = AXUIElementCreateSystemWide();
	if (system == NULL) {
		return -2;
	}
	AXUIElementRef el = NULL;
	AXError err = AXUIElementCopyElementAtPosition(system, (CGFloat)x, (CGFloat)y, &el);
	CFRelease(system);
	if (err != kAXErrorSuccess || el == NULL) {
		return -3;
	}
	pid_t pid = 0;
	if (AXUIElementGetPid(el, &pid) != kAXErrorSuccess) {
		CFRelease(el);
		return -4;
	}
	CFRelease(el);
	if (pid <= 0) {
		return -5;
	}
	NSRunningApplication *app = [NSRunningApplication runningApplicationWithProcessIdentifier:pid];
	if (app == nil) {
		return -6;
	}
	NSString *name = app.localizedName;
	if (name == nil || name.length == 0) {
		return -7;
	}
	const char *utf8 = [name UTF8String];
	if (utf8 == NULL) {
		return -8;
	}
	strncpy(buf, utf8, (size_t)buflen - 1);
	buf[buflen - 1] = '\0';
	return 0;
}

static void focusPressAtPoint(double x, double y) {
	clickAtPoint(x, y);
	usleep(60000);
	AXUIElementRef sw = AXUIElementCreateSystemWide();
	if (sw == NULL) {
		return;
	}
	AXUIElementRef el = NULL;
	if (AXUIElementCopyElementAtPosition(sw, (CGFloat)x, (CGFloat)y, &el) == kAXErrorSuccess && el != NULL) {
		AXUIElementPerformAction(el, kAXPressAction);
		CFRelease(el);
	}
	CFRelease(sw);
	usleep(40000);
}

static int axUtf8FromElement(AXUIElementRef el, char *buf, int buflen) {
	if (el == NULL || buf == NULL || buflen <= 0) {
		return -1;
	}
	buf[0] = '\0';
	CFTypeRef ref = NULL;
	AXError err = AXUIElementCopyAttributeValue(el, kAXValueAttribute, &ref);
	if (err != kAXErrorSuccess || ref == NULL) {
		err = AXUIElementCopyAttributeValue(el, CFSTR("AXText"), &ref);
	}
	if (err != kAXErrorSuccess || ref == NULL) {
		return -2;
	}
	if (CFGetTypeID(ref) != CFStringGetTypeID()) {
		CFRelease(ref);
		return -3;
	}
	CFStringRef s = (CFStringRef)ref;
	if (!CFStringGetCString(s, buf, buflen, kCFStringEncodingUTF8)) {
		CFRelease(ref);
		return -4;
	}
	CFRelease(ref);
	return 0;
}

static int axTextVisibleAtElement(AXUIElementRef el, const char *expected) {
	char got[8192];
	if (axUtf8FromElement(el, got, (int)sizeof(got)) != 0) {
		return 0;
	}
	if (expected == NULL) {
		return got[0] == '\0';
	}
	if (strcmp(got, expected) == 0) {
		return 1;
	}
	if (strstr(got, expected) != NULL) {
		return 1;
	}
	size_t elen = strlen(expected);
	size_t glen = strlen(got);
	if (elen > 0 && glen > 0 && elen <= glen) {
		return strncmp(got, expected, elen) == 0;
	}
	return 0;
}

static int axSetValueOnElement(AXUIElementRef el, const char *text) {
	if (el == NULL) {
		return -1;
	}
	const char *s = (text != NULL) ? text : "";
	CFStringRef cf = CFStringCreateWithCString(kCFAllocatorDefault, s, kCFStringEncodingUTF8);
	if (cf == NULL) {
		return -2;
	}
	AXError err = AXUIElementSetAttributeValue(el, kAXValueAttribute, cf);
	if (err != kAXErrorSuccess) {
		err = AXUIElementSetAttributeValue(el, CFSTR("AXText"), cf);
	}
	CFRelease(cf);
	if (err != kAXErrorSuccess) {
		return -3;
	}
	usleep(30000);
	return axTextVisibleAtElement(el, s) ? 0 : -4;
}

static int axSetValueAtPoint(double x, double y, const char *text) {
	AXUIElementRef sw = AXUIElementCreateSystemWide();
	if (sw == NULL) {
		return -1;
	}
	AXUIElementRef el = NULL;
	AXError err = AXUIElementCopyElementAtPosition(sw, (CGFloat)x, (CGFloat)y, &el);
	CFRelease(sw);
	if (err != kAXErrorSuccess || el == NULL) {
		return -2;
	}
	AXUIElementPerformAction(el, kAXPressAction);
	usleep(50000);
	int rc = axSetValueOnElement(el, text);
	CFRelease(el);
	return rc;
}

static int axSetFocusedValueUTF8Verified(const char *text) {
	AXUIElementRef sw = AXUIElementCreateSystemWide();
	if (sw == NULL) {
		return -1;
	}
	CFTypeRef ref = NULL;
	AXError err = AXUIElementCopyAttributeValue(sw, kAXFocusedUIElementAttribute, &ref);
	CFRelease(sw);
	if (err != kAXErrorSuccess || ref == NULL) {
		return -2;
	}
	AXUIElementRef el = (AXUIElementRef)ref;
	int rc = axSetValueOnElement(el, text);
	CFRelease(el);
	return rc;
}
*/
import "C"

import (
	"bytes"
	"fmt"
	"log"
	"os/exec"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/atotto/clipboard"
	"openclaw-connector/internal/platform"
)

var axPromptOnce sync.Once

var (
	injectFocusMu    sync.Mutex
	injectFocusReady bool
	injectFocusAt    time.Time
	sessionMouseMu   sync.Mutex
	sessionMouseX    float64
	sessionMouseY    float64
	sessionMouseValid bool
)

// captureSessionMousePoint 在「开始传输」时记录鼠标位置，后续注入/发送都点回该输入框。
func captureSessionMousePoint() {
	var x, y C.double
	C.readCurrentMouse(&x, &y)
	sessionMouseMu.Lock()
	sessionMouseX = float64(x)
	sessionMouseY = float64(y)
	sessionMouseValid = true
	sessionMouseMu.Unlock()
}

func clearSessionMousePoint() {
	sessionMouseMu.Lock()
	sessionMouseValid = false
	sessionMouseMu.Unlock()
}

func hasSessionInjectTarget() bool {
	sessionMouseMu.Lock()
	valid := sessionMouseValid
	sessionMouseMu.Unlock()
	return valid
}

func clearSessionInjectTarget() {
	clearSessionMousePoint()
	resetInjectFocus()
}

func resetInjectFocus() {
	injectFocusMu.Lock()
	injectFocusReady = false
	injectFocusAt = time.Time{}
	injectFocusMu.Unlock()
	resetStreamInjectCache()
}

func injectPasteDeltaAtEnd(delta string) error {
	delta = strings.TrimSpace(delta)
	if delta == "" {
		return nil
	}
	cmd := exec.Command("pbcopy")
	cmd.Stdin = bytes.NewReader([]byte(delta))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pbcopy delta: %w: %s", err, strings.TrimSpace(string(out)))
	}
	duringInjectTargetFocus(func() {
		C.injectPasteAtEnd()
	})
	return nil
}

func markInjectFocusReady() {
	injectFocusMu.Lock()
	injectFocusReady = true
	injectFocusAt = time.Now()
	injectFocusMu.Unlock()
}

func needsInjectPrepare() bool {
	injectFocusMu.Lock()
	ok := injectFocusReady && time.Since(injectFocusAt) < 90*time.Second
	injectFocusMu.Unlock()
	return !ok
}

func clickInjectTarget() {
	sessionMouseMu.Lock()
	valid := sessionMouseValid
	x, y := sessionMouseX, sessionMouseY
	sessionMouseMu.Unlock()
	if valid {
		C.focusPressAtPoint(C.double(x), C.double(y))
	} else {
		C.clickAtCurrentMouse()
	}
}

func isBrowserApp(name string) bool {
	n := strings.ToLower(normalizeAppName(name))
	for _, b := range []string{"chrome", "safari", "firefox", "arc", "edge", "brave", "opera", "vivaldi"} {
		if strings.Contains(n, b) {
			return true
		}
	}
	return false
}

// needsCGEventInject Electron/IDE 下 AX 常误报成功但 Monaco 等编辑器不显示，须用 ⌘A⌘V。
func needsCGEventInject(name string) bool {
	n := strings.ToLower(normalizeAppName(name))
	for _, p := range []string{
		"cursor", "visual studio code", "vscode", "code", "electron",
		"windsurf", "zed", "sublime", "nova", "fleet", "webstorm", "pycharm",
		"intellij", "idea", "android studio", "xcode", "slack", "discord",
		"notion", "figma", "wechat", "微信",
	} {
		if strings.Contains(n, p) {
			return true
		}
	}
	return false
}

func normalizeAppName(name string) string {
	if i := strings.IndexByte(name, 0); i >= 0 {
		name = name[:i]
	}
	return strings.TrimSpace(name)
}

func getAppNameAtMousePoint() (string, error) {
	var x, y C.double
	C.readCurrentMouse(&x, &y)
	return appNameAtPoint(float64(x), float64(y))
}

func appNameAtPoint(x, y float64) (string, error) {
	var cbuf [256]C.char
	rc := C.copyAppNameAtPoint(C.double(x), C.double(y), &cbuf[0], C.int(len(cbuf)))
	if rc != 0 {
		return "", fmt.Errorf("app at point: rc=%d", int(rc))
	}
	name := normalizeAppName(C.GoString(&cbuf[0]))
	if name == "" {
		return "", fmt.Errorf("app at point: empty")
	}
	return name, nil
}

func appNameAtSavedMouse() string {
	sessionMouseMu.Lock()
	valid := sessionMouseValid
	x, y := sessionMouseX, sessionMouseY
	sessionMouseMu.Unlock()
	if !valid {
		return ""
	}
	name, err := appNameAtPoint(x, y)
	if err != nil || isConnectorProcess(name) {
		return ""
	}
	return name
}

func refreshInjectTargetFromSavedMouse(existing string) string {
	existing = normalizeAppName(existing)
	if at := appNameAtSavedMouse(); at != "" {
		return at
	}
	return existing
}

func resolveInjectSessionApp(sessionApp string) string {
	if at := appNameAtSavedMouse(); at != "" {
		return at
	}
	sessionApp = normalizeAppName(sessionApp)
	if sessionApp != "" && !isConnectorProcess(sessionApp) {
		return sessionApp
	}
	if at, err := getAppNameAtMousePoint(); err == nil && !isConnectorProcess(at) {
		return at
	}
	return sessionApp
}

func logSavedInjectPoint(reason string) {
	sessionMouseMu.Lock()
	valid := sessionMouseValid
	x, y := sessionMouseX, sessionMouseY
	sessionMouseMu.Unlock()
	if !valid {
		log.Printf("dictation: inject point not set (%s)", reason)
		return
	}
	app := appNameAtSavedMouse()
	log.Printf("dictation: inject point (%.0f,%.0f) app=%q (%s)", x, y, app, reason)
}

// prepareInjectTarget 在「开始传输」时记录的屏幕坐标点击并聚焦，再激活该点下的 App。
func prepareInjectTarget(sessionApp string) {
	if !hasSessionInjectTarget() {
		clickInjectTarget()
		time.Sleep(120 * time.Millisecond)
		return
	}
	// 先点保存的坐标（鼠标所在输入框），再按该点下的 App 激活
	clickInjectTarget()
	time.Sleep(100 * time.Millisecond)
	sessionApp = resolveInjectSessionApp(sessionApp)
	if sessionApp != "" && !isConnectorProcess(sessionApp) {
		if err := activateApplication(sessionApp); err != nil {
			log.Printf("dictation: activate session app: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	clickInjectTarget()
	time.Sleep(80 * time.Millisecond)
}

// prepareInjectTargetQuick 清空/轻量注入：一次点击 + 短等待。
func prepareInjectTargetQuick(sessionApp string) {
	if !hasSessionInjectTarget() {
		clickInjectTarget()
		time.Sleep(50 * time.Millisecond)
		return
	}
	clickInjectTarget()
	time.Sleep(50 * time.Millisecond)
	sessionApp = resolveInjectSessionApp(sessionApp)
	if sessionApp != "" && !isConnectorProcess(sessionApp) {
		_ = activateApplication(sessionApp)
		time.Sleep(60 * time.Millisecond)
	}
	clickInjectTarget()
	time.Sleep(60 * time.Millisecond)
}

var connectorProcessNames = []string{
	"openclaw-connector",
	"openclaw_connector",
	"听写",
	"Connector",
	"openclaw",
	"赛搏小小助手",
	"SaiboAssistant",
	"saiboassistant",
}

func isConnectorProcess(name string) bool {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, p := range connectorProcessNames {
		if strings.Contains(n, strings.ToLower(p)) {
			return true
		}
	}
	return false
}

func getFrontmostAppName() (string, error) {
	script := `tell application "System Events" to name of first application process whose frontmost is true`
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func activateApplication(name string) error {
	name = normalizeAppName(name)
	if name == "" {
		return fmt.Errorf("empty app name")
	}
	escaped := strings.ReplaceAll(name, `\`, `\\`)
	escaped = strings.ReplaceAll(escaped, `"`, `\"`)
	script := fmt.Sprintf(`tell application "%s" to activate`, escaped)
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("activate %q: %w: %s", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func pasteViaNativeAX(text string) error {
	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))
	if C.axSetFocusedValueUTF8Verified(ctext) != 0 {
		return fmt.Errorf("native ax set value failed")
	}
	return nil
}

func pasteViaNativeAXAtMouse(text string) error {
	sessionMouseMu.Lock()
	valid := sessionMouseValid
	x, y := sessionMouseX, sessionMouseY
	sessionMouseMu.Unlock()
	if !valid {
		return fmt.Errorf("no saved mouse point")
	}
	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))
	if C.axSetValueAtPoint(C.double(x), C.double(y), ctext) != 0 {
		return fmt.Errorf("native ax at mouse failed")
	}
	return nil
}

// injectTextPreferAX 直接写 AX 值，无 ⌘A 全选蓝底；调用前须已 prepare 焦点。
func injectTextPreferAX(text string) bool {
	var ok bool
	duringInjectTargetFocus(func() {
		if pasteViaNativeAXAtMouse(text) == nil || pasteViaNativeAX(text) == nil {
			ok = true
		}
	})
	return ok
}

// AppleScript 写 AX（部分环境可用，作备用）。
func pasteViaAXValue() error {
	script := `tell application "System Events"
	tell (first application process whose frontmost is true)
		try
			set w to front window
			set f to value of attribute "AXFocusedUIElement" of w
			if f is missing value then error "no focused element"
			set value of f to (the clipboard as text)
		end try
	end tell
end tell`
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("ax set value: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// 回退：⌘A ⌘V 后按右方向键取消选中（避免蓝底高亮）。
func pasteViaSystemEventsKeysForApp(sessionApp string) error {
	sessionApp = normalizeAppName(sessionApp)
	if sessionApp != "" && !isConnectorProcess(sessionApp) {
		escaped := strings.ReplaceAll(sessionApp, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		script := fmt.Sprintf(`tell application "%s" to activate
delay 0.06
tell application "System Events"
	tell process "%s"
		keystroke "a" using command down
		delay 0.03
		keystroke "v" using command down
		delay 0.02
		key code 124
	end tell
end tell`, escaped, escaped)
		out, err := exec.Command("osascript", "-e", script).CombinedOutput()
		if err == nil {
			return nil
		}
		log.Printf("dictation: CmdAV in process %q: %v", sessionApp, strings.TrimSpace(string(out)))
	}
	script := `tell application "System Events"
	keystroke "a" using command down
	delay 0.05
	keystroke "v" using command down
	delay 0.03
	key code 124
end tell`
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("keystroke paste: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

func pasteViaSystemEventsKeys() error {
	return pasteViaSystemEventsKeysForApp("")
}

func pasteViaCGEvent() error {
	C.injectSelectAllPasteDeselect()
	return nil
}

func pasteClearViaCGEvent() error {
	C.injectSelectAllDelete()
	return nil
}

func clearInjectField(sessionApp string) error {
	if !platform.AccessibilityTrusted() {
		return fmt.Errorf("辅助功能未授权")
	}
	sessionApp = resolveInjectSessionApp(sessionApp)
	log.Printf("dictation: clear inject field via session app %q", sessionApp)
	duringInjectTargetFocus(func() {
		prepareInjectTargetQuick(sessionApp)
		ensureFrontmostForInject(sessionApp)
	})
	_ = clipboard.WriteAll("")

	resetInjectFocus()
	if e := pasteClearViaCGEvent(); e == nil {
		log.Printf("dictation: clear inject path=CGEvent-CmdA-Backspace")
		return nil
	}
	if e := pasteClearViaKeysForApp(sessionApp); e == nil {
		log.Printf("dictation: clear inject path=CmdA-Backspace")
		return nil
	}
	return fmt.Errorf("clear failed for app %q", sessionApp)
}

func ensureFrontmostForInject(sessionApp string) {
	sessionApp = normalizeAppName(sessionApp)
	if sessionApp == "" || isConnectorProcess(sessionApp) {
		return
	}
	if err := activateApplication(sessionApp); err != nil {
		log.Printf("dictation: ensure frontmost %q: %v", sessionApp, err)
		return
	}
	time.Sleep(120 * time.Millisecond)
	front, err := getFrontmostAppName()
	if err != nil {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(front), sessionApp) {
		log.Printf("dictation: frontmost is %q, wanted %q for inject/clear", front, sessionApp)
	}
}

func pasteClearViaKeysForApp(sessionApp string) error {
	sessionApp = strings.TrimSpace(sessionApp)
	if sessionApp != "" && !isConnectorProcess(sessionApp) {
		escaped := strings.ReplaceAll(sessionApp, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		script := fmt.Sprintf(`tell application "%s" to activate
delay 0.05
tell application "System Events"
	tell process "%s"
		keystroke "a" using command down
		delay 0.02
		key code 51
	end tell
end tell`, escaped, escaped)
		out, err := exec.Command("osascript", "-e", script).CombinedOutput()
		if err == nil {
			return nil
		}
		log.Printf("dictation: clear keys in process %q: %v", sessionApp, strings.TrimSpace(string(out)))
	}
	script := `tell application "System Events"
	keystroke "a" using command down
	delay 0.05
	key code 51
end tell`
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("clear keys: %w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// replaceFocusedField commit=false：流式优先 AX 直写（无全选闪烁），失败再 CGEvent；commit=true：句末/segment_end。
func replaceFocusedField(text string, sessionApp string, commit bool) error {
	if strings.TrimSpace(text) == "" {
		return clearInjectField(sessionApp)
	}

	if !platform.AccessibilityTrusted() {
		axPromptOnce.Do(func() { platform.RequestAccessibilityPrompt() })
		_ = clipboard.WriteAll(text)
		return fmt.Errorf("辅助功能未授权；文本已复制到剪贴板，可手动 ⌘V")
	}

	cmd := exec.Command("pbcopy")
	cmd.Stdin = bytes.NewReader([]byte(text))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pbcopy: %w: %s", err, strings.TrimSpace(string(out)))
	}

	sessionApp = resolveInjectSessionApp(sessionApp)
	if sessionApp == "" {
		return fmt.Errorf("no inject target: move mouse into Mac input, then tap 开始传输 on device")
	}
	if commit {
		logSavedInjectPoint("inject")
		log.Printf("dictation: inject via session app %q (commit)", sessionApp)
	} else if needsInjectPrepare() {
		log.Printf("dictation: inject stream via %q (focus)", sessionApp)
	}

	if needsInjectPrepare() {
		duringInjectTargetFocus(func() {
			prepareInjectTarget(sessionApp)
			ensureFrontmostForInject(sessionApp)
			time.Sleep(50 * time.Millisecond)
		})
		markInjectFocusReady()
	} else {
		_ = activateApplication(sessionApp)
		time.Sleep(25 * time.Millisecond)
	}

	// 流式听写：始终用整段 show 覆盖输入框，避免 append-at-end 在光标/AX 不一致时叠字（如「明天」+「明天星期几」）
	if !commit {
		streamInjectMu.Lock()
		prev := lastStreamInjectedText
		streamInjectMu.Unlock()
		if prev == text {
			markInjectFocusReady()
			return nil
		}
	}
	if injectTextPreferAX(text) {
		if !commit {
			log.Printf("dictation: inject path=AX-direct (stream)")
		} else if isBrowserApp(sessionApp) {
			log.Printf("dictation: inject path=AX-direct (commit, browser)")
		} else {
			log.Printf("dictation: inject path=AX-direct (commit)")
		}
		noteStreamInjected(text)
		markInjectFocusReady()
		return nil
	}
	if !commit {
		log.Printf("dictation: ax unavailable, fallback CGEvent (may flash select-all)")
	}

	if !commit {
		var streamErr error
		duringInjectTargetFocus(func() {
			if e := pasteViaCGEvent(); e != nil {
				streamErr = e
				return
			}
			if isBrowserApp(sessionApp) {
				log.Printf("dictation: inject path=Browser-CGEvent (stream)")
			} else {
				log.Printf("dictation: inject path=CGEvent-CmdAV (stream)")
			}
		})
		if streamErr == nil {
			noteStreamInjected(text)
			markInjectFocusReady()
			return nil
		}
		return streamErr
	}

	var injectErr error
	duringInjectTargetFocus(func() {
		if e := pasteViaCGEvent(); e != nil {
			injectErr = e
			return
		}
		if isBrowserApp(sessionApp) {
			log.Printf("dictation: inject path=Browser-CGEvent (commit)")
		} else {
			log.Printf("dictation: inject path=CGEvent-CmdAV (commit)")
		}
	})
	noteStreamInjected(text)
	if injectErr == nil {
		return nil
	}
	duringInjectTargetFocus(func() {
		if e := pasteViaSystemEventsKeysForApp(sessionApp); e != nil {
			injectErr = e
			return
		}
		log.Printf("dictation: inject path=CmdAV (commit fallback)")
		injectErr = nil
	})
	if injectErr == nil {
		return nil
	}
	_ = clipboard.WriteAll(text)
	return fmt.Errorf("%w（文本已在剪贴板，可手动 ⌘V）", injectErr)
}

func injectSendEnterKey(sessionApp string) error {
	if !platform.AccessibilityTrusted() {
		return fmt.Errorf("辅助功能未授权")
	}
	sessionApp = resolveInjectSessionApp(sessionApp)
	prepareInjectTarget(sessionApp)
	ensureFrontmostForInject(sessionApp)
	var script string
	if sessionApp != "" && !isConnectorProcess(sessionApp) {
		escaped := strings.ReplaceAll(sessionApp, `\`, `\\`)
		escaped = strings.ReplaceAll(escaped, `"`, `\"`)
		script = fmt.Sprintf(`tell application "System Events"
	tell process "%s"
		key code 36
	end tell
end tell`, escaped)
	} else {
		script = `tell application "System Events" to key code 36`
	}
	out, err := exec.Command("osascript", "-e", script).CombinedOutput()
	if err != nil {
		return fmt.Errorf("enter: %w: %s", err, strings.TrimSpace(string(out)))
	}
	// 发送后焦点常会离开输入框；点回记录位置，便于下一轮继续注入
	time.Sleep(120 * time.Millisecond)
	prepareInjectTarget(sessionApp)
	log.Printf("dictation: enter sent, refocused inject target")
	return nil
}

func LogInjectCapability(injectEnabled bool) {
	if !injectEnabled {
		log.Println("dictation: inject disabled in config")
		return
	}
	if platform.AccessibilityTrusted() {
		log.Println("dictation: inject enabled (NativeAX / CmdAV fallback; clear targets session app)")
	} else {
		log.Println("dictation: inject enabled, accessibility NOT trusted")
		platform.RequestAccessibilityPrompt()
	}
}
