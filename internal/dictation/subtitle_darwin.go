//go:build darwin

package dictation

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>

// 高于普通应用窗口，避免 inject activate Chrome 后字幕被盖住。
static const NSWindowLevel kSubtitleWindowLevel = NSPopUpMenuWindowLevel;

static void applySubtitleMouseAndLevel(NSWindow *w) {
	if (w == nil) return;
	[w setLevel:kSubtitleWindowLevel];
	[w setHidesOnDeactivate:NO];
	[w setCanHide:YES];
	// 必须可点、可拖；每次置顶后都要重新设置（Fyne/系统有时会改回穿透）
	[w setIgnoresMouseEvents:NO];
	[w setAcceptsMouseMovedEvents:YES];
	[w setMovableByWindowBackground:YES];
	[w setCollectionBehavior:(NSWindowCollectionBehaviorCanJoinAllSpaces |
		NSWindowCollectionBehaviorStationary |
		NSWindowCollectionBehaviorFullScreenAuxiliary |
		NSWindowCollectionBehaviorIgnoresCycle)];
	// 不用 NonactivatingPanel：会导致部分系统上无法点关闭/拖动
	NSWindowStyleMask mask = [w styleMask];
	mask &= ~NSWindowStyleMaskNonactivatingPanel;
	[w setStyleMask:mask];
	if ([w isKindOfClass:[NSPanel class]]) {
		NSPanel *p = (NSPanel *)w;
		[p setFloatingPanel:YES];
		[p setWorksWhenModal:YES];
		[p setBecomesKeyOnlyIfNeeded:NO];
	}
}

void configureSubtitleNSWindow(uintptr_t wptr) {
	NSWindow *w = (NSWindow *)wptr;
	if (w == nil) return;
	applySubtitleMouseAndLevel(w);
}

void raiseSubtitleNSWindow(uintptr_t wptr) {
	NSWindow *w = (NSWindow *)wptr;
	if (w == nil) return;
	applySubtitleMouseAndLevel(w);
	[w orderFrontRegardless];
}

// 注入前：字幕窗不挡鼠标，点击可落到下方 Chrome/输入框。
void subtitleSetInjectPassthrough(uintptr_t wptr, int on) {
	NSWindow *w = (NSWindow *)wptr;
	if (w == nil) return;
	if (on) {
		[w setIgnoresMouseEvents:YES];
		[w orderBack:nil];
	} else {
		applySubtitleMouseAndLevel(w);
		if ([w isVisible]) {
			[w orderFrontRegardless];
		}
	}
}
*/
import "C"

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/driver"
)

func applySubtitleNSWindowPtr(ptr uintptr) {
	if ptr == 0 {
		return
	}
	subtitleMu.Lock()
	subtitleNSWindowPtr = ptr
	subtitleMu.Unlock()
	C.configureSubtitleNSWindow(C.uintptr_t(ptr))
	C.raiseSubtitleNSWindow(C.uintptr_t(ptr))
}

func configureSubtitleWindow(win fyne.Window) {
	nw, ok := win.(driver.NativeWindow)
	if !ok || win == nil {
		return
	}
	nw.RunNative(func(ctx any) {
		mac, ok := ctx.(driver.MacWindowContext)
		if !ok || mac.NSWindow == 0 {
			return
		}
		applySubtitleNSWindowPtr(mac.NSWindow)
	})
}

func raiseSubtitleWindow(win fyne.Window) {
	subtitleMu.Lock()
	ptr := subtitleNSWindowPtr
	subtitleMu.Unlock()
	if ptr != 0 {
		C.raiseSubtitleNSWindow(C.uintptr_t(ptr))
		return
	}
	nw, ok := win.(driver.NativeWindow)
	if !ok || win == nil {
		return
	}
	nw.RunNative(func(ctx any) {
		mac, ok := ctx.(driver.MacWindowContext)
		if !ok || mac.NSWindow == 0 {
			return
		}
		applySubtitleNSWindowPtr(mac.NSWindow)
	})
}

func setSubtitleInjectPassthrough(on bool) {
	subtitleMu.Lock()
	win := subtitleWindow
	visible := subtitleVisible
	pinned := subtitleSessionPinned
	subtitleMu.Unlock()
	if win == nil || (!visible && !pinned) {
		return
	}
	flag := 0
	if on {
		flag = 1
	}
	// 须在 Fyne 主线程调用 RunNative；云端 WS 线程会触发 inject/clear。
	runOnFyneMain(func() {
		subtitleMu.Lock()
		ptr := subtitleNSWindowPtr
		subtitleMu.Unlock()
		if ptr != 0 {
			C.subtitleSetInjectPassthrough(C.uintptr_t(ptr), C.int(flag))
			return
		}
		nw, ok := win.(driver.NativeWindow)
		if !ok {
			return
		}
		nw.RunNative(func(ctx any) {
			mac, ok := ctx.(driver.MacWindowContext)
			if !ok || mac.NSWindow == 0 {
				return
			}
			applySubtitleNSWindowPtr(mac.NSWindow)
			C.subtitleSetInjectPassthrough(C.uintptr_t(mac.NSWindow), C.int(flag))
		})
	})
}

// duringInjectTargetFocus 注入/清空时暂时让悬浮窗穿透鼠标，避免挡住目标输入框。
func duringInjectTargetFocus(fn func()) {
	setSubtitleInjectPassthrough(true)
	defer setSubtitleInjectPassthrough(false)
	fn()
}
