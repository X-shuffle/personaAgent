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

	chatClient   *chat.Client
	historyStore *history.Store
	historyDB    string
	sessionID    string
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
	_ = a.HideLauncher()
	a.registerGlobalHotkeyWithFallback()
}

func (a *App) shutdown(context.Context) {
	a.unregisterGlobalHotkey()

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

	if store != nil {
		if err := store.UpsertSession(ctx, sessionID, defaultSessionTitle); err != nil {
			a.logHistoryWarning("upsert session before send failed", err)
		}
		if _, err := store.PersistUserTurn(ctx, sessionID, trimmedMessage); err != nil {
			a.logHistoryWarning("persist user turn failed", err)
		}
	}

	resp, appErr := client.Send(ctx, chat.ChatRequest{SessionID: sessionID, Message: trimmedMessage})
	if appErr != nil {
		if store != nil {
			errContent := strings.TrimSpace(appErr.Message)
			if errContent == "" {
				errContent = "chat request failed"
			}
			if _, err := store.PersistAssistantError(ctx, sessionID, normalizeHistoryErrorCode(appErr), errContent); err != nil {
				a.logHistoryWarning("persist assistant error failed", err)
			}
		}
		return ChatResult{Error: appErr}
	}

	if store != nil {
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

func normalizeHistoryErrorCode(appErr *chat.Error) string {
	if appErr == nil {
		return "internal_error"
	}
	if code := strings.TrimSpace(appErr.Code); code != "" {
		return code
	}
	if appErr.StatusCode > 0 {
		return fmt.Sprintf("http_%d", appErr.StatusCode)
	}
	return "internal_error"
}
