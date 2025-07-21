package opencode

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// Client represents an OpenCode HTTP client
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new OpenCode client
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Session represents an OpenCode session
type Session struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Title     string    `json:"title"`
	Shared    bool      `json:"shared"`
}

// Message represents an OpenCode message
type Message struct {
	ID    string      `json:"id"`
	Role  string      `json:"role"`
	Parts []MessagePart `json:"parts"`
}

// MessagePart represents a part of an OpenCode message
type MessagePart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// CreateSessionRequest represents the request to create a session
type CreateSessionRequest struct{}

// CreateSessionResponse represents the response from creating a session
type CreateSessionResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
	Title     string    `json:"title"`
	Shared    bool      `json:"shared"`
}

// SendMessageRequest represents the request to send a message
type SendMessageRequest struct {
	MessageID  string        `json:"messageID,omitempty"`
	ProviderID string        `json:"providerID"`
	ModelID    string        `json:"modelID"`
	Parts      []MessagePart `json:"parts"`
}

// SendMessageResponse represents the response from sending a message
type SendMessageResponse struct {
	Message Message `json:"message"`
}

// CreateSession creates a new OpenCode session
func (c *Client) CreateSession(ctx context.Context) (*CreateSessionResponse, error) {
	req := CreateSessionRequest{}
	var resp CreateSessionResponse
	
	if err := c.post(ctx, "/session", req, &resp); err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	
	return &resp, nil
}

// SendMessage sends a message to an OpenCode session
func (c *Client) SendMessage(ctx context.Context, sessionID string, req SendMessageRequest) (*SendMessageResponse, error) {
	var resp SendMessageResponse
	
	if err := c.post(ctx, fmt.Sprintf("/session/%s/message", sessionID), req, &resp); err != nil {
		return nil, fmt.Errorf("failed to send message: %w", err)
	}
	
	return &resp, nil
}

// GetMessages retrieves messages from an OpenCode session
func (c *Client) GetMessages(ctx context.Context, sessionID string) ([]Message, error) {
	var resp struct {
		Messages []struct {
			Info  Message `json:"info"`
			Parts []MessagePart `json:"parts"`
		} `json:"messages"`
	}
	
	if err := c.get(ctx, fmt.Sprintf("/session/%s/message", sessionID), &resp); err != nil {
		return nil, fmt.Errorf("failed to get messages: %w", err)
	}
	
	messages := make([]Message, len(resp.Messages))
	for i, msg := range resp.Messages {
		messages[i] = Message{
			ID:    msg.Info.ID,
			Role:  msg.Info.Role,
			Parts: msg.Parts,
		}
	}
	
	return messages, nil
}

// GetProviders retrieves available providers
func (c *Client) GetProviders(ctx context.Context) (map[string]interface{}, error) {
	var resp map[string]interface{}
	
	if err := c.get(ctx, "/config/providers", &resp); err != nil {
		return nil, fmt.Errorf("failed to get providers: %w", err)
	}
	
	return resp, nil
}

// GetConfig retrieves the OpenCode configuration
func (c *Client) GetConfig(ctx context.Context) (map[string]interface{}, error) {
	var resp map[string]interface{}
	
	if err := c.get(ctx, "/config", &resp); err != nil {
		return nil, fmt.Errorf("failed to get config: %w", err)
	}
	
	return resp, nil
}

// WaitForReady waits for the OpenCode server to be ready
func (c *Client) WaitForReady(ctx context.Context, maxWait time.Duration) error {
	deadline := time.Now().Add(maxWait)
	
	for time.Now().Before(deadline) {
		if err := c.get(ctx, "/config", nil); err == nil {
			return nil
		}
		
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(100 * time.Millisecond):
			// Continue trying
		}
	}
	
	return fmt.Errorf("opencode server not ready after %v", maxWait)
}

// post performs a POST request
func (c *Client) post(ctx context.Context, path string, reqBody interface{}, respBody interface{}) error {
	return c.request(ctx, "POST", path, reqBody, respBody)
}

// get performs a GET request
func (c *Client) get(ctx context.Context, path string, respBody interface{}) error {
	return c.request(ctx, "GET", path, nil, respBody)
}

// request performs an HTTP request
func (c *Client) request(ctx context.Context, method, path string, reqBody interface{}, respBody interface{}) error {
	url := c.baseURL + path
	
	var body io.Reader
	var jsonData []byte
	if reqBody != nil {
		var err error
		jsonData, err = json.Marshal(reqBody)
		if err != nil {
			return fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(jsonData)
	}
	
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}
	
	if reqBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode >= 400 {
		// Read the response body for better error messages
		respBodyBytes, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("request failed with status %d, url: %s, request body: %s, response: %s", 
			resp.StatusCode, url, string(jsonData), string(respBodyBytes))
	}
	
	if respBody != nil {
		if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
			return fmt.Errorf("failed to decode response body: %w", err)
		}
	}
	
	return nil
}