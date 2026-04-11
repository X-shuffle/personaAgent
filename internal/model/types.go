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

// Persona defines style constraints for response generation.
type Persona struct {
	Tone    string   `json:"tone"`
	Style   string   `json:"style"`
	Values  []string `json:"values"`
	Phrases []string `json:"phrases"`
}

// LLMMessage is one message sent to the model.
type LLMMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// LLMRequest is a generic chat-completion request.
type LLMRequest struct {
	Messages    []LLMMessage `json:"messages"`
	Temperature float64      `json:"temperature,omitempty"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
}

// LLMResponse is a generic model response.
type LLMResponse struct {
	Text  string `json:"text"`
	Model string `json:"model,omitempty"`
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
