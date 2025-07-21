package opencode

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	st "github.com/coder/agentapi/lib/screentracker"
)

// Conversation manages OpenCode conversation state via REST API
type Conversation struct {
	mu               sync.RWMutex
	client           *Client
	sessionID        string
	messages         []st.ConversationMessage
	status           st.ConversationStatus
	logger           *slog.Logger
	defaultProvider  string
	defaultModel     string
}

// NewConversation creates a new OpenCode conversation
func NewConversation(ctx context.Context, client *Client, logger *slog.Logger) (*Conversation, error) {
	conv := &Conversation{
		client:   client,
		logger:   logger,
		status:   st.ConversationStatusStable,
		messages: []st.ConversationMessage{},
	}

	// Create a session
	session, err := client.CreateSession(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create session: %w", err)
	}
	conv.sessionID = session.ID

	// Get providers to set defaults
	if err := conv.setupDefaults(ctx); err != nil {
		logger.Warn("Failed to setup defaults from providers", "error", err)
		// Set fallback defaults that match our mockgpt config
		conv.defaultProvider = "mockgpt"
		conv.defaultModel = "gpt-3.5-turbo"
		logger.Info("Using fallback defaults", "provider", conv.defaultProvider, "model", conv.defaultModel)
	}

	logger.Info("Created OpenCode conversation", "sessionID", conv.sessionID)
	return conv, nil
}

// setupDefaults configures default provider and model
func (c *Conversation) setupDefaults(ctx context.Context) error {
	providers, err := c.client.GetProviders(ctx)
	if err != nil {
		return err
	}

	// Look for providers in the response structure that matches OpenCode API
	if providersData, ok := providers["providers"].([]interface{}); ok && len(providersData) > 0 {
		// Use the first available provider
		if provider, ok := providersData[0].(map[string]interface{}); ok {
			if providerID, ok := provider["id"].(string); ok {
				c.defaultProvider = providerID
				c.logger.Info("Using provider", "providerID", providerID)
			}
			if models, ok := provider["models"].(map[string]interface{}); ok {
				// Use the first available model
				for modelID := range models {
					c.defaultModel = modelID
					c.logger.Info("Using model", "modelID", modelID)
					break
				}
			}
		}
	}

	// Try to get defaults from the "default" field if available
	if defaultMap, ok := providers["default"].(map[string]interface{}); ok {
		if c.defaultProvider != "" {
			if defaultModel, ok := defaultMap[c.defaultProvider].(string); ok {
				c.defaultModel = defaultModel
				c.logger.Info("Using default model for provider", "providerID", c.defaultProvider, "modelID", defaultModel)
			}
		}
	}

	// If still no provider/model found, check the config
	if c.defaultProvider == "" || c.defaultModel == "" {
		c.logger.Warn("No providers found in API response, using fallback from config")
		c.defaultProvider = "mockgpt"  // Match the config we set up
		c.defaultModel = "gpt-3.5-turbo"
	}

	c.logger.Info("Setup complete", "provider", c.defaultProvider, "model", c.defaultModel)

	return nil
}

// SendMessage sends a message to the OpenCode session
func (c *Conversation) SendMessage(userInput ...string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.status == st.ConversationStatusChanging {
		return fmt.Errorf("agent is currently running")
	}

	// Combine all input into a single message
	content := strings.Join(userInput, " ")
	if content == "" {
		return fmt.Errorf("message content cannot be empty")
	}

	c.status = st.ConversationStatusChanging

	// Add user message to local messages
	userMsg := st.ConversationMessage{
		Id:      len(c.messages),
		Role:    st.ConversationRoleUser,
		Message: content,
		Time:    time.Now(),
	}
	c.messages = append(c.messages, userMsg)

	// Send message via OpenCode API
	go c.sendMessageAsync(content)

	return nil
}

// sendMessageAsync sends the message to OpenCode and updates conversation state
func (c *Conversation) sendMessageAsync(content string) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	req := SendMessageRequest{
		ProviderID: c.defaultProvider,
		ModelID:    c.defaultModel,
		Parts: []MessagePart{
			{
				Type: "text",
				Text: content,
			},
		},
	}

	resp, err := c.client.SendMessage(ctx, c.sessionID, req)
	if err != nil {
		c.logger.Error("Failed to send message to OpenCode", "error", err)
		c.mu.Lock()
		c.status = st.ConversationStatusStable
		c.mu.Unlock()
		return
	}

	// Update messages with assistant response
	c.mu.Lock()
	defer c.mu.Unlock()

	// Convert OpenCode response to our message format
	assistantMsg := st.ConversationMessage{
		Id:      len(c.messages),
		Role:    st.ConversationRoleAgent,
		Message: c.formatMessageParts(resp.Message.Parts),
		Time:    time.Now(),
	}
	c.messages = append(c.messages, assistantMsg)
	c.status = st.ConversationStatusStable
}

// formatMessageParts converts OpenCode message parts to a single string
func (c *Conversation) formatMessageParts(parts []MessagePart) string {
	var result strings.Builder
	for i, part := range parts {
		if i > 0 {
			result.WriteString("\n")
		}
		result.WriteString(part.Text)
	}
	return result.String()
}

// Status returns the current conversation status
func (c *Conversation) Status() st.ConversationStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

// Messages returns the current messages
func (c *Conversation) Messages() []st.ConversationMessage {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return append([]st.ConversationMessage{}, c.messages...) // Return a copy
}

// Screen returns empty string since OpenCode doesn't use terminal screen
func (c *Conversation) Screen() string {
	return ""
}

// StartSnapshotLoop is a no-op for OpenCode since we don't need screen snapshots
func (c *Conversation) StartSnapshotLoop(ctx context.Context) {
	// No-op: OpenCode uses REST API, no screen snapshots needed
}

// generateID generates a random ID for messages
func generateID() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}