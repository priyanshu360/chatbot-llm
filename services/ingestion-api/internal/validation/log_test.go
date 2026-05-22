package validation

import (
	"encoding/json"
	"testing"
)

func validPayload() []byte {
	return []byte(`{
		"request_id": "550e8400-e29b-41d4-a716-446655440000",
		"session_id": "session-1",
		"conversation_id": "conv-1",
		"model": "gpt-4o",
		"provider": "openai",
		"latency_ms": 1200,
		"input_tokens": 150,
		"output_tokens": 200,
		"status": "success",
		"timestamp": "2025-01-15T10:30:00Z"
	}`)
}

func TestValidateInferenceLog_Valid(t *testing.T) {
	result := ValidateInferenceLog(validPayload())
	if !result.Valid {
		t.Fatalf("expected valid, got errors: %v", result.Errors)
	}
	if result.Payload == nil {
		t.Fatal("expected payload, got nil")
	}
	if result.Payload.RequestID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("unexpected request_id: %s", result.Payload.RequestID)
	}
}

func TestValidateInferenceLog_InvalidJSON(t *testing.T) {
	result := ValidateInferenceLog([]byte(`not json`))
	if result.Valid {
		t.Fatal("expected invalid")
	}
	if len(result.Errors) == 0 {
		t.Fatal("expected at least one error")
	}
	if result.Errors[0].Field != "body" {
		t.Errorf("expected field 'body', got %q", result.Errors[0].Field)
	}
}

func TestValidateInferenceLog_RequiredFields(t *testing.T) {
	missing := map[string]string{
		"request_id":      "request_id",
		"session_id":      "session_id",
		"conversation_id": "conversation_id",
		"model":           "model",
		"provider":        "provider",
		"status":          "status",
	}

	for field, expected := range missing {
		t.Run("missing "+field, func(t *testing.T) {
			var raw map[string]interface{}
			json.Unmarshal(validPayload(), &raw)
			delete(raw, field)
			b, _ := json.Marshal(raw)

			result := ValidateInferenceLog(b)
			if result.Valid {
				t.Fatal("expected invalid")
			}
			found := false
			for _, e := range result.Errors {
				if e.Field == expected {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("expected error for field %q, got errors: %v", expected, result.Errors)
			}
		})
	}
}

func TestValidateInferenceLog_WhitespaceTrimmed(t *testing.T) {
	raw := []byte(`{
		"request_id": "  abc  ",
		"session_id": "s1",
		"conversation_id": "c1",
		"model": "gpt-4",
		"provider": "openai",
		"latency_ms": 100,
		"input_tokens": 10,
		"output_tokens": 20,
		"status": "success"
	}`)
	result := ValidateInferenceLog(raw)
	if !result.Valid {
		t.Fatalf("expected valid, got: %v", result.Errors)
	}
	if result.Payload.RequestID != "abc" {
		t.Errorf("expected trimmed 'abc', got %q", result.Payload.RequestID)
	}
}

func TestValidateInferenceLog_StatusEnum(t *testing.T) {
	valid := []string{"success", "error", "timeout"}
	for _, s := range valid {
		t.Run("valid status "+s, func(t *testing.T) {
			var raw map[string]interface{}
			json.Unmarshal(validPayload(), &raw)
			raw["status"] = s
			b, _ := json.Marshal(raw)
			result := ValidateInferenceLog(b)
			if !result.Valid {
				t.Errorf("expected valid for status %q, got: %v", s, result.Errors)
			}
		})
	}

	t.Run("invalid status", func(t *testing.T) {
		var raw map[string]interface{}
		json.Unmarshal(validPayload(), &raw)
		raw["status"] = "pending"
		b, _ := json.Marshal(raw)
		result := ValidateInferenceLog(b)
		if result.Valid {
			t.Fatal("expected invalid")
		}
	})
}

func TestValidateInferenceLog_NegativeInts(t *testing.T) {
	fields := []struct {
		key   string
		value int
	}{
		{"latency_ms", -1},
		{"input_tokens", -5},
		{"output_tokens", -10},
	}
	for _, f := range fields {
		t.Run("negative "+f.key, func(t *testing.T) {
			var raw map[string]interface{}
			json.Unmarshal(validPayload(), &raw)
			raw[f.key] = f.value
			b, _ := json.Marshal(raw)
			result := ValidateInferenceLog(b)
			if result.Valid {
				t.Fatal("expected invalid")
			}
		})
	}
}

func TestValidateInferenceLog_TimestampFormat(t *testing.T) {
	t.Run("RFC3339 valid", func(t *testing.T) {
		var raw map[string]interface{}
		json.Unmarshal(validPayload(), &raw)
		raw["timestamp"] = "2025-01-15T10:30:00Z"
		b, _ := json.Marshal(raw)
		result := ValidateInferenceLog(b)
		if !result.Valid {
			t.Fatalf("expected valid, got: %v", result.Errors)
		}
	})

	t.Run("RFC3339Nano valid", func(t *testing.T) {
		var raw map[string]interface{}
		json.Unmarshal(validPayload(), &raw)
		raw["timestamp"] = "2025-01-15T10:30:00.123456Z"
		b, _ := json.Marshal(raw)
		result := ValidateInferenceLog(b)
		if !result.Valid {
			t.Fatalf("expected valid, got: %v", result.Errors)
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		var raw map[string]interface{}
		json.Unmarshal(validPayload(), &raw)
		raw["timestamp"] = "2025/01/15"
		b, _ := json.Marshal(raw)
		result := ValidateInferenceLog(b)
		if result.Valid {
			t.Fatal("expected invalid")
		}
	})

	t.Run("empty timestamp is valid", func(t *testing.T) {
		var raw map[string]interface{}
		json.Unmarshal(validPayload(), &raw)
		raw["timestamp"] = ""
		b, _ := json.Marshal(raw)
		result := ValidateInferenceLog(b)
		if !result.Valid {
			t.Fatalf("expected valid, got: %v", result.Errors)
		}
	})
}

func TestValidateInferenceLog_WhitespaceOnlyField(t *testing.T) {
	t.Run("whitespace treated as empty", func(t *testing.T) {
		var raw map[string]interface{}
		json.Unmarshal(validPayload(), &raw)
		raw["request_id"] = "  "
		b, _ := json.Marshal(raw)
		result := ValidateInferenceLog(b)
		if result.Valid {
			t.Fatal("expected invalid for whitespace-only request_id")
		}
	})
}
