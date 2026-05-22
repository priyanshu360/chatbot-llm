package pkg

import "time"

type InferenceLog struct {
	RequestID      string `json:"request_id"`
	SessionID      string `json:"session_id"`
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id,omitempty"`
	Model          string `json:"model"`
	Provider       string `json:"provider"`
	LatencyMs      int    `json:"latency_ms"`
	InputTokens    int    `json:"input_tokens,omitempty"`
	OutputTokens   int    `json:"output_tokens,omitempty"`
	Status         string `json:"status"`
	ErrorCode      string `json:"error_code,omitempty"`
	InputPreview   string `json:"input_preview,omitempty"`
	OutputPreview  string `json:"output_preview,omitempty"`
	Timestamp      string `json:"timestamp"`
}

type ChatRequest struct {
	ConversationID string `json:"conversation_id,omitempty"`
	Provider       string `json:"provider"`
	Model          string `json:"model"`
	Message        string `json:"message"`
}

type ChatResponse struct {
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
}

type SSEEvent struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

type Conversation struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Provider  string    `json:"provider"`
	Model     string    `json:"model"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Message struct {
	ID              string    `json:"id"`
	ConversationID  string    `json:"conversation_id"`
	Role            string    `json:"role"`
	Content         string    `json:"content"`
	Seq             int       `json:"seq"`
	CreatedAt       time.Time `json:"created_at"`
}

type CreateConversationRequest struct {
	Title    string `json:"title"`
	Provider string `json:"provider"`
	Model    string `json:"model"`
}

type UpdateConversationRequest struct {
	Status string `json:"status"`
}

type IngestionResponse struct {
	Accepted int `json:"accepted"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
