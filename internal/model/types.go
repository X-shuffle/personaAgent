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
