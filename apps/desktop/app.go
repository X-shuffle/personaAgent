package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"desktop/backend/chat"
	"desktop/backend/history"
	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.design/x/hotkey"
)

const (
	focusInputEventName  = "launcher:focus-input"
	defaultChatBaseURL   = "http://localhost:8080"
	defaultHistoryDBEnv  = "DESKTOP_HISTORY_DB_PATH"
	fixedSessionID       = "desktop-default-session"
	defaultHistorySubDir = ".persona-agent/desktop"
	defaultHistoryDBName = "history.sqlite"
	defaultSessionTitle  = ""

	launcherWidthRatio  = 0.35
	launcherHeightRatio = 0.7
	launcherMinWidth    = 760
	launcherMaxWidth    = 1160
	launcherMinHeight   = 420
	launcherMaxHeight   = 860

	launcherTopOffsetRatio = 0.07
	launcherMinTopOffsetPx = 56
	launcherMaxTopOffsetPx = 96
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

	statusbarDone chan struct{}

	chatClient   *chat.Client
	historyStore   *history.Store
	historyDB      string
	sessionID      string
}

type ChatResult struct {
	Response string      `json:"response,omitempty"`
	Error    *chat.Error `json:"error,omitempty"`
}

type HistorySearchItem struct {
	MessageID    int64  `json:"message_id"`
	SessionID    string `json:"session_id"`
	SessionTitle string `json:"session_title"`
	Role         string `json:"role"`
	Content      string `json:"content"`
	Status       string `json:"status"`
	ErrorCode    string `json:"error_code"`
	CreatedAt    int64  `json:"created_at"`
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
	a.sessionID = fixedSessionID
	baseURL := strings.TrimSpace(os.Getenv("DESKTOP_CHAT_BASE_URL"))
	if baseURL == "" {
		baseURL = defaultChatBaseURL
		runtime.LogWarningf(ctx, "DESKTOP_CHAT_BASE_URL is not set, fallback to %s", defaultChatBaseURL)
	}
	a.chatClient = chat.NewClient(baseURL)
	a.mu.Unlock()

	store, path, err := openHistoryStore(ctx)
	if err != nil {
		runtime.LogWarningf(ctx, "history disabled: %v", err)
	} else {
		// 历史库可用时，固定绑定到同一个 desktop session，便于后续搜索/回跳。
		a.mu.Lock()
		a.historyStore = store
		a.historyDB = path
		a.mu.Unlock()
		if upsertErr := store.UpsertSession(ctx, fixedSessionID, defaultSessionTitle); upsertErr != nil {
			runtime.LogWarningf(ctx, "initialize history session failed: %v", upsertErr)
		}
		runtime.LogInfof(ctx, "history store ready: %s", path)
	}

	runtime.WindowSetAlwaysOnTop(ctx, true)
	a.startStatusBar()
	_ = a.HideLauncher()
	a.registerGlobalHotkeyWithFallback()
}

func (a *App) shutdown(context.Context) {
	a.unregisterGlobalHotkey()
	a.stopStatusBar()

	a.mu.Lock()
	store := a.historyStore
	a.historyStore = nil
	a.historyDB = ""
	a.mu.Unlock()

	if store != nil {
		if err := store.Close(); err != nil && a.ctx != nil {
			runtime.LogWarningf(a.ctx, "close history store failed: %v", err)
		}
	}
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

// SendChat sends message to backend /chat and returns either response or structured error.
func (a *App) SendChat(message string) ChatResult {
	a.mu.Lock()
	client := a.chatClient
	store := a.historyStore
	sessionID := a.sessionID
	ctx := a.ctx
	a.mu.Unlock()

	if client == nil {
		return ChatResult{Error: &chat.Error{Code: "config_error", Message: "chat client is not initialized"}}
	}
	if ctx == nil {
		ctx = context.Background()
	}

	trimmedMessage := strings.TrimSpace(message)
	if trimmedMessage == "" {
		return ChatResult{Error: &chat.Error{StatusCode: 422, Code: "invalid_argument", Message: "message is required"}}
	}

	resp, appErr := client.Send(ctx, chat.ChatRequest{SessionID: sessionID, Message: trimmedMessage})
	if appErr != nil {
		return ChatResult{Error: appErr}
	}

	if store != nil {
		// 仅在后端成功后落盘，避免把失败请求写入历史。
		if err := store.UpsertSession(ctx, sessionID, defaultSessionTitle); err != nil {
			a.logHistoryWarning("upsert session after success failed", err)
		}
		if _, err := store.PersistUserTurn(ctx, sessionID, trimmedMessage); err != nil {
			a.logHistoryWarning("persist user turn failed", err)
		}
		if _, err := store.PersistAssistantTurn(ctx, sessionID, resp.Response); err != nil {
			a.logHistoryWarning("persist assistant turn failed", err)
		}
	}
	return ChatResult{Response: resp.Response}
}

// SearchHistory exposes history full-text-like query for desktop frontend calls.
func (a *App) SearchHistory(keyword string, limit int, offset int) ([]HistorySearchItem, error) {
	a.mu.Lock()
	store := a.historyStore
	ctx := a.ctx
	a.mu.Unlock()

	if store == nil {
		return nil, fmt.Errorf("history store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	hits, err := store.SearchMessages(ctx, keyword, limit, offset)
	if err != nil {
		return nil, err
	}

	items := make([]HistorySearchItem, 0, len(hits))
	for _, hit := range hits {
		items = append(items, HistorySearchItem{
			MessageID:    hit.MessageID,
			SessionID:    hit.SessionID,
			SessionTitle: hit.SessionTitle,
			Role:         hit.Role,
			Content:      hit.Content,
			Status:       hit.Status,
			ErrorCode:    hit.ErrorCode,
			CreatedAt:    hit.CreatedAt.Unix(),
		})
	}
	return items, nil
}

// LoadMessageContext loads hit message with its adjacent QA context for desktop jump.
func (a *App) LoadMessageContext(messageID int64) ([]HistorySearchItem, error) {
	a.mu.Lock()
	store := a.historyStore
	ctx := a.ctx
	a.mu.Unlock()

	if store == nil {
		return nil, fmt.Errorf("history store is not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	messages, err := store.LoadMessageContext(ctx, messageID)
	if err != nil {
		return nil, err
	}

	items := make([]HistorySearchItem, 0, len(messages))
	for _, msg := range messages {
		items = append(items, HistorySearchItem{
			MessageID: msg.ID,
			SessionID: msg.SessionID,
			Role:      msg.Role,
			Content:   msg.Content,
			Status:    msg.Status,
			ErrorCode: msg.ErrorCode,
			CreatedAt: msg.CreatedAt.Unix(),
		})
	}
	return items, nil
}

// showLocked 在持锁状态下显示并激活窗口，同时触发前端输入框聚焦事件。
func (a *App) showLocked() {
	runtime.Show(a.ctx)
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
	a.positionLauncherTopCenterLocked()
	runtime.EventsEmit(a.ctx, focusInputEventName)
	a.visible = true
}

// positionLauncherTopCenterLocked 将窗口定位到“当前屏幕顶部居中”，并限制在可见区域内。
func (a *App) positionLauncherTopCenterLocked() {
	screenWidth, screenHeight, ok := a.currentScreenSizeLocked()
	if ok {
		targetWidth := clampInt(int(float64(screenWidth)*launcherWidthRatio), launcherMinWidth, launcherMaxWidth)
		targetHeight := clampInt(int(float64(screenHeight)*launcherHeightRatio), launcherMinHeight, launcherMaxHeight)
		runtime.WindowSetSize(a.ctx, targetWidth, targetHeight)
	}

	runtime.WindowCenter(a.ctx)

	centerX, centerY := runtime.WindowGetPosition(a.ctx)
	_, windowHeight := runtime.WindowGetSize(a.ctx)
	if windowHeight <= 0 {
		return
	}

	if !ok {
		_, screenHeight, ok = a.currentScreenSizeLocked()
		if !ok || screenHeight <= 0 {
			return
		}
	}

	originY := centerY - (screenHeight-windowHeight)/2
	topOffset := clampInt(int(float64(screenHeight)*launcherTopOffsetRatio), launcherMinTopOffsetPx, launcherMaxTopOffsetPx)
	targetY := originY + topOffset
	maxY := originY + (screenHeight - windowHeight)
	if maxY < originY {
		maxY = originY
	}
	if targetY > maxY {
		targetY = maxY
	}
	if targetY < originY {
		targetY = originY
	}

	runtime.WindowSetPosition(a.ctx, centerX, targetY)
}

func (a *App) currentScreenSizeLocked() (int, int, bool) {
	screens, err := runtime.ScreenGetAll(a.ctx)
	if err != nil || len(screens) == 0 {
		return 0, 0, false
	}

	for _, screen := range screens {
		if !screen.IsCurrent {
			continue
		}
		width, height := screenSize(screen)
		if width > 0 && height > 0 {
			return width, height, true
		}
	}

	width, height := screenSize(screens[0])
	if width > 0 && height > 0 {
		return width, height, true
	}

	return 0, 0, false
}

func screenSize(screen runtime.Screen) (int, int) {
	width := screen.Size.Width
	if width <= 0 {
		width = screen.Width
	}
	height := screen.Size.Height
	if height <= 0 {
		height = screen.Height
	}
	return width, height
}

func clampInt(value int, min int, max int) int {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

// hideLocked 在持锁状态下隐藏窗口，并同步可见性状态。
func (a *App) hideLocked() {
	runtime.WindowHide(a.ctx)
	a.visible = false
}

// registerGlobalHotkeyWithFallback 注册全局快捷键，主键失败时自动回退到备选组合。
func (a *App) registerGlobalHotkeyWithFallback() {
	a.unregisterGlobalHotkey()

	// 主快捷键优先使用 Option+Space，冲突时自动降级到 Cmd+Shift+Space。
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

// attachHotkey 绑定热键监听协程，把按键事件转成窗口显隐切换。
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

// unregisterGlobalHotkey 停止热键监听并释放系统级快捷键注册。
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

// logHistoryWarning 在上下文可用时记录历史模块告警日志。
func (a *App) logHistoryWarning(prefix string, err error) {
	if err == nil {
		return
	}
	a.mu.Lock()
	ctx := a.ctx
	a.mu.Unlock()
	if ctx != nil {
		runtime.LogWarningf(ctx, "%s: %v", prefix, err)
	}
}

// openHistoryStore 打开历史数据库并执行迁移，失败时确保回收资源。
func openHistoryStore(ctx context.Context) (*history.Store, string, error) {
	path, err := resolveHistoryDBPath()
	if err != nil {
		return nil, "", err
	}

	store, err := history.Open(path)
	if err != nil {
		return nil, "", err
	}

	if err := store.Migrate(ctx); err != nil {
		_ = store.Close()
		return nil, "", err
	}

	return store, path, nil
}

// resolveHistoryDBPath 解析历史库路径：优先环境变量，否则落到用户目录默认位置。
func resolveHistoryDBPath() (string, error) {
	if fromEnv := strings.TrimSpace(os.Getenv(defaultHistoryDBEnv)); fromEnv != "" {
		return fromEnv, nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve user home: %w", err)
	}

	dir := filepath.Join(home, defaultHistorySubDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("create history directory: %w", err)
	}

	return filepath.Join(dir, defaultHistoryDBName), nil
}

