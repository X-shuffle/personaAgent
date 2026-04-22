//go:build darwin

package main

/*
#cgo CFLAGS: -x objective-c -fobjc-arc
#cgo LDFLAGS: -framework Cocoa

#import <Cocoa/Cocoa.h>
#import <dispatch/dispatch.h>

static NSStatusItem *personaStatusItem = nil;
static NSMenu *personaStatusMenu = nil;
static id personaStatusBarTarget = nil;

@interface PersonaStatusBarTarget : NSObject
- (void)statusItemClicked:(id)sender;
- (void)toggleLauncher:(id)sender;
- (void)quitApp:(id)sender;
@end

@implementation PersonaStatusBarTarget

- (void)statusItemClicked:(id)sender {
	NSEvent *event = [NSApp currentEvent];
	if (event != nil && event.type == NSEventTypeRightMouseUp && personaStatusMenu != nil) {
		[personaStatusItem popUpStatusItemMenu:personaStatusMenu];
		return;
	}
	[self toggleLauncher:sender];
}

- (void)toggleLauncher:(id)sender {
	// 直接操作主窗口，实现菜单栏点击与全局快捷键一致的显隐行为。
	NSArray<NSWindow *> *windows = [NSApp windows];
	if (windows.count == 0) {
		return;
	}

	NSWindow *window = [NSApp keyWindow];
	if (window == nil) {
		window = windows.firstObject;
	}
	if (window == nil) {
		return;
	}

	if (window.isVisible) {
		[window orderOut:nil];
		return;
	}

	[NSApp activateIgnoringOtherApps:YES];

	NSScreen *screen = window.screen;
	if (screen == nil) {
		screen = NSScreen.mainScreen;
	}
	if (screen != nil) {
		NSRect visibleFrame = screen.visibleFrame;
		NSRect frame = window.frame;
		CGFloat x = NSMidX(visibleFrame) - frame.size.width / 2.0;
		CGFloat y = NSMaxY(visibleFrame) - frame.size.height - 72.0;
		if (y < NSMinY(visibleFrame)) {
			y = NSMinY(visibleFrame);
		}
		[window setFrameOrigin:NSMakePoint(x, y)];
	}

	[window makeKeyAndOrderFront:nil];
}

- (void)quitApp:(id)sender {
	[NSApp terminate:nil];
}

@end

static NSImage *personaStatusBarIcon() {
	NSImage *image = [[NSImage alloc] initWithSize:NSMakeSize(18, 18)];
	[image lockFocus];

	[[NSColor blackColor] setStroke];
	[[NSColor blackColor] setFill];

	NSBezierPath *bubble = [NSBezierPath bezierPathWithRoundedRect:NSMakeRect(2.0, 4.5, 11.0, 8.0) xRadius:3.0 yRadius:3.0];
	[bubble setLineWidth:1.8];
	[bubble stroke];

	NSBezierPath *tail = [NSBezierPath bezierPath];
	[tail moveToPoint:NSMakePoint(6.2, 4.6)];
	[tail lineToPoint:NSMakePoint(5.0, 2.6)];
	[tail lineToPoint:NSMakePoint(8.0, 4.6)];
	[tail closePath];
	[tail fill];

	NSBezierPath *sparkle = [NSBezierPath bezierPath];
	[sparkle setLineWidth:1.6];
	[sparkle moveToPoint:NSMakePoint(14.6, 12.5)];
	[sparkle lineToPoint:NSMakePoint(14.6, 16.0)];
	[sparkle moveToPoint:NSMakePoint(12.9, 14.2)];
	[sparkle lineToPoint:NSMakePoint(16.3, 14.2)];
	[sparkle stroke];

	[image unlockFocus];
	[image setTemplate:YES];
	return image;
}

// 在主线程创建菜单栏图标与右键菜单。
static void startPersonaStatusBarOnMain() {
	@autoreleasepool {
		// Accessory 模式仅显示菜单栏入口，不在 Dock 常驻图标。
		[NSApp setActivationPolicy:NSApplicationActivationPolicyAccessory];
		if (personaStatusItem != nil) {
			return;
		}

		personaStatusBarTarget = [PersonaStatusBarTarget new];
		personaStatusItem = [[NSStatusBar systemStatusBar] statusItemWithLength:NSVariableStatusItemLength];

		NSButton *button = [personaStatusItem button];
		if (button != nil) {
			[button setImage:personaStatusBarIcon()];
			[button setTitle:@""];
			[button setToolTip:@"Persona Agent"];
			[button setTarget:personaStatusBarTarget];
			[button setAction:@selector(statusItemClicked:)];
			[button sendActionOn:(NSEventMaskLeftMouseUp | NSEventMaskRightMouseUp)];
		}

		personaStatusMenu = [[NSMenu alloc] initWithTitle:@"Persona Agent"];

		NSMenuItem *toggleItem = [[NSMenuItem alloc] initWithTitle:@"Toggle Launcher" action:@selector(toggleLauncher:) keyEquivalent:@""];
		[toggleItem setTarget:personaStatusBarTarget];
		[personaStatusMenu addItem:toggleItem];

		[personaStatusMenu addItem:[NSMenuItem separatorItem]];

		NSMenuItem *quitItem = [[NSMenuItem alloc] initWithTitle:@"Quit" action:@selector(quitApp:) keyEquivalent:@"q"];
		[quitItem setTarget:personaStatusBarTarget];
		[personaStatusMenu addItem:quitItem];
	}
}

static void stopPersonaStatusBarOnMain() {
	@autoreleasepool {
		if (personaStatusItem == nil) {
			return;
		}
		[[NSStatusBar systemStatusBar] removeStatusItem:personaStatusItem];
		personaStatusItem = nil;
		personaStatusMenu = nil;
		personaStatusBarTarget = nil;
	}
}

static void startPersonaStatusBar() {
	if ([NSThread isMainThread]) {
		startPersonaStatusBarOnMain();
		return;
	}
	dispatch_sync(dispatch_get_main_queue(), ^{
		startPersonaStatusBarOnMain();
	});
}

static void stopPersonaStatusBar() {
	if ([NSThread isMainThread]) {
		stopPersonaStatusBarOnMain();
		return;
	}
	dispatch_sync(dispatch_get_main_queue(), ^{
		stopPersonaStatusBarOnMain();
	});
}
*/
import "C"

func (a *App) startStatusBar() {
	C.startPersonaStatusBar()
}

func (a *App) stopStatusBar() {
	C.stopPersonaStatusBar()
}
