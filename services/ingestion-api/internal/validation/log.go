package validation

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type InferenceLogPayload struct {
	RequestID      string `json:"request_id"`
	SessionID      string `json:"session_id"`
	ConversationID string `json:"conversation_id"`
	MessageID      string `json:"message_id"`
	Model          string `json:"model"`
	Provider       string `json:"provider"`
	LatencyMs      int    `json:"latency_ms"`
	InputTokens    int    `json:"input_tokens"`
	OutputTokens   int    `json:"output_tokens"`
	Status         string `json:"status"`
	ErrorCode      string `json:"error_code"`
	InputPreview   string `json:"input_preview"`
	OutputPreview  string `json:"output_preview"`
	Timestamp      string `json:"timestamp"`
}

type Result struct {
	Valid   bool
	Errors  []ValidationError
	Payload *InferenceLogPayload
}

func ValidateInferenceLog(raw []byte) Result {
	result := Result{}

	var payload InferenceLogPayload
	if err := json.Unmarshal(raw, &payload); err != nil {
		result.Errors = append(result.Errors, ValidationError{Field: "body", Message: fmt.Sprintf("invalid JSON: %v", err)})
		return result
	}

	payload.RequestID = strings.TrimSpace(payload.RequestID)
	payload.SessionID = strings.TrimSpace(payload.SessionID)
	payload.ConversationID = strings.TrimSpace(payload.ConversationID)
	payload.Model = strings.TrimSpace(payload.Model)
	payload.Provider = strings.TrimSpace(payload.Provider)
	payload.Status = strings.TrimSpace(payload.Status)
	payload.Timestamp = strings.TrimSpace(payload.Timestamp)

	if payload.RequestID == "" {
		result.Errors = append(result.Errors, ValidationError{Field: "request_id", Message: "request_id is required"})
	}
	if payload.SessionID == "" {
		result.Errors = append(result.Errors, ValidationError{Field: "session_id", Message: "session_id is required"})
	}
	if payload.ConversationID == "" {
		result.Errors = append(result.Errors, ValidationError{Field: "conversation_id", Message: "conversation_id is required"})
	}
	if payload.Model == "" {
		result.Errors = append(result.Errors, ValidationError{Field: "model", Message: "model is required"})
	}
	if payload.Provider == "" {
		result.Errors = append(result.Errors, ValidationError{Field: "provider", Message: "provider is required"})
	}
	if payload.Status == "" {
		result.Errors = append(result.Errors, ValidationError{Field: "status", Message: "status is required"})
	}
	if payload.Status != "success" && payload.Status != "error" && payload.Status != "timeout" {
		result.Errors = append(result.Errors, ValidationError{Field: "status", Message: "status must be 'success', 'error', or 'timeout'"})
	}
	if payload.LatencyMs < 0 {
		result.Errors = append(result.Errors, ValidationError{Field: "latency_ms", Message: "latency_ms must be non-negative"})
	}
	if payload.InputTokens < 0 {
		result.Errors = append(result.Errors, ValidationError{Field: "input_tokens", Message: "input_tokens must be non-negative"})
	}
	if payload.OutputTokens < 0 {
		result.Errors = append(result.Errors, ValidationError{Field: "output_tokens", Message: "output_tokens must be non-negative"})
	}
	if payload.Timestamp != "" {
		if _, err := time.Parse(time.RFC3339, payload.Timestamp); err != nil {
			if _, err := time.Parse(time.RFC3339Nano, payload.Timestamp); err != nil {
				result.Errors = append(result.Errors, ValidationError{Field: "timestamp", Message: "timestamp must be RFC3339 format"})
			}
		}
	}

	if len(result.Errors) == 0 {
		result.Valid = true
		result.Payload = &payload
	}

	return result
}
