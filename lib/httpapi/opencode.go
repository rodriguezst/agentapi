package httpapi

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/sst/opencode-sdk-go"
	st "github.com/coder/agentapi/lib/screentracker"
)

// OpencodeClient wraps the opencode SDK and provides agentapi-compatible interface
type OpencodeClient struct {
	client    *opencode.Client
	sessionID string
	logger    *slog.Logger
	mu        sync.RWMutex
	messages  []st.ConversationMessage
	status    st.ConversationStatus
}

// NewOpencodeClient creates a new opencode client
func NewOpencodeClient(ctx context.Context, logger *slog.Logger) (*OpencodeClient, error) {
	client := opencode.NewClient()
	
	// Create a new session
	session, err := client.Session.New(ctx)
	if err != nil {
		return nil, err
	}

	oc := &OpencodeClient{
		client:    client,
		sessionID: session.ID,
		logger:    logger,
		messages:  []st.ConversationMessage{},
		status:    st.ConversationStatusStable,
	}

	// Initialize the session
	err = oc.initSession(ctx)
	if err != nil {
		return nil, err
	}

	return oc, nil
}

// initSession initializes the opencode session
func (oc *OpencodeClient) initSession(ctx context.Context) error {
	_, err := oc.client.Session.Init(ctx, oc.sessionID, opencode.SessionInitParams{
		MessageID:  opencode.F("init_msg_001"),
		ProviderID: opencode.F("mockgpt"),
		ModelID:    opencode.F("gpt-3.5-turbo"),
	})
	if err != nil {
		return err
	}

	// Add initial system message
	oc.mu.Lock()
	defer oc.mu.Unlock()
	
	oc.messages = append(oc.messages, st.ConversationMessage{
		Id:      1,
		Role:    st.ConversationRoleAgent,
		Message: "Opencode session initialized. Ready for your requests.",
		Time:    time.Now(),
	})

	return nil
}

// SendMessage sends a message to opencode
func (oc *OpencodeClient) SendMessage(ctx context.Context, content string) error {
	oc.mu.Lock()
	oc.status = st.ConversationStatusChanging
	oc.mu.Unlock()

	// Add user message
	userMsg := st.ConversationMessage{
		Id:      oc.getNextMessageID(),
		Role:    st.ConversationRoleUser,
		Message: content,
		Time:    time.Now(),
	}
	
	oc.mu.Lock()
	oc.messages = append(oc.messages, userMsg)
	oc.mu.Unlock()

	// Send to opencode
	oc.logger.Info("Sending message to opencode", "content", content)
	_, err := oc.client.Session.Chat(ctx, oc.sessionID, opencode.SessionChatParams{
		ModelID: opencode.F("gpt-3.5-turbo"), // Default model, could be configurable
		ProviderID: opencode.F("mockgpt"), // Default provider, could be configurable
		Parts: opencode.F([]opencode.SessionChatParamsPartUnion{
			opencode.TextPartInputParam{
				Type: opencode.F(opencode.TextPartInputTypeText),
				Text: opencode.F(content),
			},
		}),
	})

	oc.mu.Lock()
	defer oc.mu.Unlock()
	
	if err != nil {
		oc.status = st.ConversationStatusStable
		// Add error message
		oc.messages = append(oc.messages, st.ConversationMessage{
			Id:      oc.getNextMessageIDUnsafe(),
			Role:    st.ConversationRoleAgent,
			Message: "Error: " + err.Error(),
			Time:    time.Now(),
		})
		return err
	}

	// Add a small delay to allow the response to be processed
	time.Sleep(1 * time.Second)

	// Get the latest messages to find the response
	oc.logger.Info("Getting session messages from opencode")
	sessionMessages, err := oc.client.Session.Messages(ctx, oc.sessionID)
	if err != nil {
		oc.logger.Error("Error getting session messages", "error", err)
		oc.status = st.ConversationStatusStable
		oc.messages = append(oc.messages, st.ConversationMessage{
			Id:      oc.getNextMessageIDUnsafe(),
			Role:    st.ConversationRoleAgent,
			Message: "Error getting response: " + err.Error(),
			Time:    time.Now(),
		})
		return err
	}

	// Find the assistant responses and add them as messages
	if sessionMessages != nil {
		oc.logger.Info("Processing session messages", "count", len(*sessionMessages))
		for i, sessionMsg := range *sessionMessages {
			oc.logger.Info("Session message", "index", i, "role", sessionMsg.Info.Role, "parts_count", len(sessionMsg.Parts))
			if sessionMsg.Info.Role == opencode.MessageRoleAssistant {
				var responseContent string
				for j, part := range sessionMsg.Parts {
					oc.logger.Info("Message part", "index", j, "type", part.Type, "text", part.Text)
					// Capture all text content, not just text type
					if part.Type == opencode.PartTypeText || part.Text != "" {
						responseContent += part.Text
					}
				}

				if responseContent != "" {
					oc.logger.Info("Adding assistant response", "content", responseContent)
					oc.messages = append(oc.messages, st.ConversationMessage{
						Id:      oc.getNextMessageIDUnsafe(),
						Role:    st.ConversationRoleAgent,
						Message: responseContent,
						Time:    time.Now(),
					})
				}
			}
		}
	} else {
		oc.logger.Warn("No session messages returned")
	}

	oc.status = st.ConversationStatusStable
	return nil
}

// Messages returns the current conversation messages
func (oc *OpencodeClient) Messages() []st.ConversationMessage {
	oc.mu.RLock()
	defer oc.mu.RUnlock()
	
	// Create a copy to avoid race conditions
	messages := make([]st.ConversationMessage, len(oc.messages))
	copy(messages, oc.messages)
	return messages
}

// Status returns the current status
func (oc *OpencodeClient) Status() st.ConversationStatus {
	oc.mu.RLock()
	defer oc.mu.RUnlock()
	return oc.status
}

// Screen returns empty screen content since opencode doesn't use terminal
func (oc *OpencodeClient) Screen() string {
	return ""
}

// Close cleans up the opencode session
func (oc *OpencodeClient) Close(ctx context.Context) error {
	if oc.sessionID != "" {
		_, err := oc.client.Session.Delete(ctx, oc.sessionID)
		return err
	}
	return nil
}

// getNextMessageID generates the next message ID (thread-safe)
func (oc *OpencodeClient) getNextMessageID() int {
	oc.mu.RLock()
	defer oc.mu.RUnlock()
	return oc.getNextMessageIDUnsafe()
}

// getNextMessageIDUnsafe generates the next message ID (not thread-safe, must hold lock)
func (oc *OpencodeClient) getNextMessageIDUnsafe() int {
	maxID := 0
	for _, msg := range oc.messages {
		if msg.Id > maxID {
			maxID = msg.Id
		}
	}
	return maxID + 1
}