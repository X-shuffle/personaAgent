package model

// ChatRequest is the request payload for POST /chat.
type ChatRequest struct {
	SessionID string `json:"session_id"`
	Message   string `json:"message"`
}

// ChatResponse is the response payload for POST /chat.
type ChatResponse struct {
	Response string `json:"response"`
}

// IngestRequest is the request payload for POST /ingest JSON mode.
type IngestRequest struct {
	SessionID string `json:"session_id"`
	Source    string `json:"source"`
	Format    string `json:"format"`
	Data      string `json:"data"`
	DryRun    bool   `json:"dry_run"`
}

// IngestResponse is the response payload for POST /ingest.
type IngestResponse struct {
	Status    string   `json:"status"`
	SessionID string   `json:"session_id"`
	Source    string   `json:"source"`
	Accepted  int      `json:"accepted"`
	Rejected  int      `json:"rejected"`
	Segments  int      `json:"segments"`
	Stored    int      `json:"stored"`
	DryRun    bool     `json:"dry_run"`
	Warnings  []string `json:"warnings,omitempty"`
}

// Persona defines style constraints for response generation.
type Persona struct {
	Tone    string   `json:"tone"`
	Style   string   `json:"style"`
	Values  []string `json:"values"`
	Phrases []string `json:"phrases"`
}

// LLMMessage is one message sent to the model.
type LLMMessage struct {
	Role       string        `json:"role"`
	Content    string        `json:"content,omitempty"`
	Name       string        `json:"name,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	ToolCalls  []LLMToolCall `json:"tool_calls,omitempty"`
}

type LLMFunctionSpec struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type LLMTool struct {
	Type     string          `json:"type"`
	Function LLMFunctionSpec `json:"function"`
}

type LLMFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type LLMToolCall struct {
	ID       string          `json:"id"`
	Type     string          `json:"type"`
	Function LLMFunctionCall `json:"function"`
}

// LLMRequest is a generic chat-completion request.
type LLMRequest struct {
	Messages    []LLMMessage `json:"messages"`
	Temperature float64      `json:"temperature,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Tools       []LLMTool    `json:"tools,omitempty"`
	ToolChoice  string       `json:"tool_choice,omitempty"`
}

// LLMResponse is a generic model response.
type LLMResponse struct {
	Text         string        `json:"text"`
	Model        string        `json:"model,omitempty"`
	ToolCalls    []LLMToolCall `json:"tool_calls,omitempty"`
	FinishReason string        `json:"finish_reason,omitempty"`
}

// EmotionState is the detected emotional state for the current user turn.
type EmotionState struct {
	Label     string  `json:"label"`
	Intensity float64 `json:"intensity"`
}

type MemoryType string

const (
	MemoryTypeEpisodic MemoryType = "episodic"
	MemoryTypeSemantic MemoryType = "semantic"
	MemoryTypeSummary  MemoryType = "summary"
)

// Memory is one memory unit used for retrieval and storage.
type Memory struct {
	ID         string     `json:"id"`
	SessionID  string     `json:"session_id"`
	Type       MemoryType `json:"type"`
	Content    string     `json:"content"`
	Embedding  []float64  `json:"embedding"`
	Emotion    string     `json:"emotion"`
	Timestamp  int64      `json:"timestamp"`
	Importance float64    `json:"importance"`
}

// MemorySearchQuery represents vector retrieval constraints.
type MemorySearchQuery struct {
	SessionID      string
	QueryEmbedding []float64
	TopK           int
	MinImportance  float64
}

// MemoryMatch includes memory and similarity score.
type MemoryMatch struct {
	Memory Memory
	Score  float64
}
