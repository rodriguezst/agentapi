package opencode

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenCodeIntegration(t *testing.T) {
	// Create a mock OpenCode server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/config":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"theme": "opencode",
				"model": "mockgpt/gpt-3.5-turbo",
			})

		case r.Method == "GET" && r.URL.Path == "/config/providers":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"providers": []map[string]interface{}{
					{
						"id":   "mockgpt",
						"name": "MockGPT",
						"models": map[string]interface{}{
							"gpt-3.5-turbo": map[string]interface{}{
								"id":   "gpt-3.5-turbo",
								"name": "GPT-3.5 Turbo",
							},
						},
					},
				},
				"default": map[string]interface{}{
					"mockgpt": "gpt-3.5-turbo",
				},
			})

		case r.Method == "POST" && r.URL.Path == "/session":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"id":        "test_session_123",
				"createdAt": time.Now().Format(time.RFC3339),
				"updatedAt": time.Now().Format(time.RFC3339),
				"title":     "Test Session",
				"shared":    false,
			})

		case r.Method == "POST" && strings.HasPrefix(r.URL.Path, "/session/") && strings.HasSuffix(r.URL.Path, "/message"):
			// Validate the request format
			var req SendMessageRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("Failed to decode request: %v", err)
				w.WriteHeader(400)
				return
			}

			// Verify required fields are present
			if req.ProviderID == "" {
				t.Error("Missing providerID")
				w.WriteHeader(400)
				return
			}
			if req.ModelID == "" {
				t.Error("Missing modelID")
				w.WriteHeader(400)
				return
			}
			if len(req.Parts) == 0 {
				t.Error("Missing parts")
				w.WriteHeader(400)
				return
			}

			// Verify parts structure
			for _, part := range req.Parts {
				if part.Type != "text" {
					t.Errorf("Expected part type 'text', got '%s'", part.Type)
					w.WriteHeader(400)
					return
				}
				if part.Text == "" {
					t.Error("Missing text in part")
					w.WriteHeader(400)
					return
				}
			}

			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"message": map[string]interface{}{
					"id":   "msg_test_123",
					"role": "assistant",
					"parts": []map[string]interface{}{
						{
							"type": "text",
							"text": "Test response: " + req.Parts[0].Text,
						},
					},
				},
			})

		default:
			w.WriteHeader(404)
		}
	}))
	defer server.Close()

	// Test the client
	client := NewClient(server.URL)
	logger := slog.Default()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Create conversation
	conv, err := NewConversation(ctx, client, logger)
	if err != nil {
		t.Fatalf("Failed to create conversation: %v", err)
	}

	// Send message
	err = conv.SendMessage("Hello, test message!")
	if err != nil {
		t.Fatalf("Failed to send message: %v", err)
	}

	// Wait for async response
	time.Sleep(100 * time.Millisecond)

	// Check messages
	messages := conv.Messages()
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}

	if messages[0].Role != "user" {
		t.Errorf("Expected first message role 'user', got '%s'", messages[0].Role)
	}
	if messages[0].Message != "Hello, test message!" {
		t.Errorf("Expected first message content 'Hello, test message!', got '%s'", messages[0].Message)
	}

	if messages[1].Role != "agent" {
		t.Errorf("Expected second message role 'agent', got '%s'", messages[1].Role)
	}
	if !strings.Contains(messages[1].Message, "Test response: Hello, test message!") {
		t.Errorf("Expected second message to contain response, got '%s'", messages[1].Message)
	}
}

func TestSendMessageRequestFormat(t *testing.T) {
	// Test that the request format matches OpenCode API expectations
	req := SendMessageRequest{
		ProviderID: "test_provider",
		ModelID:    "test_model",
		Mode:       "code",
		Parts: []MessagePart{
			{
				Type: "text",
				Text: "test message",
			},
		},
	}

	// Serialize to JSON and verify structure
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	// Verify required fields
	if parsed["providerID"] != "test_provider" {
		t.Errorf("Expected providerID 'test_provider', got '%v'", parsed["providerID"])
	}
	if parsed["modelID"] != "test_model" {
		t.Errorf("Expected modelID 'test_model', got '%v'", parsed["modelID"])
	}
	if parsed["mode"] != "code" {
		t.Errorf("Expected mode 'code', got '%v'", parsed["mode"])
	}

	parts, ok := parsed["parts"].([]interface{})
	if !ok || len(parts) != 1 {
		t.Fatalf("Expected parts array with 1 element, got %v", parsed["parts"])
	}

	part, ok := parts[0].(map[string]interface{})
	if !ok {
		t.Fatalf("Expected part to be object, got %v", parts[0])
	}

	if part["type"] != "text" {
		t.Errorf("Expected part type 'text', got '%v'", part["type"])
	}
	if part["text"] != "test message" {
		t.Errorf("Expected part text 'test message', got '%v'", part["text"])
	}

	// Verify that "content" field is NOT present (this was the bug)
	if _, hasContent := part["content"]; hasContent {
		t.Error("Part should not have 'content' field, should use 'text' field")
	}
}