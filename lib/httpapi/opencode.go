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
	_, err := oc.client.Session.Init(ctx, oc.sessionID, opencode.SessionInitParams{})
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

	// Get the latest messages to find the response
	sessionMessages, err := oc.client.Session.Messages(ctx, oc.sessionID)
	if err != nil {
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
		for _, sessionMsg := range *sessionMessages {
			if sessionMsg.Info.Role == opencode.MessageRoleAssistant {
				var responseContent string
				for _, part := range sessionMsg.Parts {
					if part.Type == opencode.PartTypeText {
						responseContent += part.Text
					}
				}

				if responseContent != "" {
					oc.messages = append(oc.messages, st.ConversationMessage{
						Id:      oc.getNextMessageIDUnsafe(),
						Role:    st.ConversationRoleAgent,
						Message: responseContent,
						Time:    time.Now(),
					})
				}
			}
		}
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