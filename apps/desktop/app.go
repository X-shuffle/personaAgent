package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.design/x/hotkey"
)

const (
	focusInputEventName = "launcher:focus-input"
)

// App struct
type App struct {
	ctx context.Context

	mu          sync.Mutex
	visible     bool
	hotkey      *hotkey.Hotkey
	hotkeyLabel string
	stopHotkey  chan struct{}
	doneHotkey  chan struct{}
}

// NewApp creates a new App application struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.mu.Lock()
	a.ctx = ctx
	a.mu.Unlock()

	runtime.WindowSetAlwaysOnTop(ctx, true)
	_ = a.HideLauncher()
	a.registerGlobalHotkeyWithFallback()
}

func (a *App) shutdown(context.Context) {
	a.unregisterGlobalHotkey()
}

// ShowLauncher shows, restores and focuses the launcher window.
func (a *App) ShowLauncher() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.ctx == nil {
		return fmt.Errorf("app context not initialized")
	}

	a.showLocked()
	return nil
}

// HideLauncher hides the launcher window.
func (a *App) HideLauncher() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.ctx == nil {
		return fmt.Errorf("app context not initialized")
	}

	a.hideLocked()
	return nil
}

// ToggleLauncher toggles the launcher window visibility.
func (a *App) ToggleLauncher() error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.ctx == nil {
		return fmt.Errorf("app context not initialized")
	}

	if a.visible {
		a.hideLocked()
		return nil
	}

	a.showLocked()
	return nil
}

func (a *App) showLocked() {
	runtime.Show(a.ctx)
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
	runtime.WindowCenter(a.ctx)
	runtime.EventsEmit(a.ctx, focusInputEventName)
	a.visible = true
}

func (a *App) hideLocked() {
	runtime.WindowHide(a.ctx)
	runtime.Hide(a.ctx)
	a.visible = false
}

func (a *App) registerGlobalHotkeyWithFallback() {
	a.unregisterGlobalHotkey()

	primary := hotkey.New([]hotkey.Modifier{hotkey.ModOption}, hotkey.KeySpace)
	if err := primary.Register(); err == nil {
		a.attachHotkey(primary, "Option+Space")
		runtime.LogInfof(a.ctx, "global hotkey registered: %s", a.hotkeyLabel)
		return
	}

	fallback := hotkey.New([]hotkey.Modifier{hotkey.ModCmd, hotkey.ModShift}, hotkey.KeySpace)
	if err := fallback.Register(); err == nil {
		a.attachHotkey(fallback, "Cmd+Shift+Space")
		runtime.LogWarningf(a.ctx, "primary hotkey unavailable, fallback active: %s", a.hotkeyLabel)
		return
	}

	runtime.LogWarning(a.ctx, "failed to register both primary and fallback global hotkeys")
}

func (a *App) attachHotkey(hk *hotkey.Hotkey, label string) {
	a.mu.Lock()
	a.hotkey = hk
	a.hotkeyLabel = label
	a.stopHotkey = make(chan struct{})
	a.doneHotkey = make(chan struct{})
	stopHotkey := a.stopHotkey
	doneHotkey := a.doneHotkey
	a.mu.Unlock()

	go func(h *hotkey.Hotkey, stop <-chan struct{}, done chan<- struct{}) {
		defer close(done)
		for {
			select {
			case <-h.Keydown():
				if err := a.ToggleLauncher(); err != nil {
					runtime.LogErrorf(a.ctx, "toggle launcher failed: %v", err)
				}
			case <-stop:
				return
			}
		}
	}(hk, stopHotkey, doneHotkey)
}

func (a *App) unregisterGlobalHotkey() {
	a.mu.Lock()
	hk := a.hotkey
	stop := a.stopHotkey
	done := a.doneHotkey
	a.hotkey = nil
	a.hotkeyLabel = ""
	a.stopHotkey = nil
	a.doneHotkey = nil
	a.mu.Unlock()

	if stop != nil {
		close(stop)
	}
	if done != nil {
		<-done
	}
	if hk != nil {
		_ = hk.Unregister()
	}
}
