package httpapi

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/agentapi/lib/opencode"
	mf "github.com/coder/agentapi/lib/msgfmt"
	st "github.com/coder/agentapi/lib/screentracker"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"golang.org/x/xerrors"
)

// OpenCodeServer represents an HTTP server for OpenCode
type OpenCodeServer struct {
	router       chi.Router
	api          huma.API
	port         int
	srv          *http.Server
	mu           sync.RWMutex
	logger       *slog.Logger
	conversation *opencode.Conversation
	emitter      *EventEmitter
}

// ConversationAdapter adapts OpenCode conversation to match screentracker interface
type ConversationAdapter struct {
	conv *opencode.Conversation
}

func (ca *ConversationAdapter) Status() st.ConversationStatus {
	return ca.conv.Status()
}

func (ca *ConversationAdapter) Messages() []st.ConversationMessage {
	return ca.conv.Messages()
}

func (ca *ConversationAdapter) Screen() string {
	return ca.conv.Screen()
}

func (ca *ConversationAdapter) SendMessage(userInput ...string) error {
	return ca.conv.SendMessage(userInput...)
}

func (ca *ConversationAdapter) StartSnapshotLoop(ctx context.Context) {
	ca.conv.StartSnapshotLoop(ctx)
}

// NewOpenCodeServer creates a new server instance for OpenCode
func NewOpenCodeServer(ctx context.Context, conversation *opencode.Conversation, port int, chatBasePath string, logger *slog.Logger) *OpenCodeServer {
	router := chi.NewMux()

	corsMiddleware := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	})
	router.Use(corsMiddleware.Handler)

	humaConfig := huma.DefaultConfig("AgentAPI", "0.2.3")
	humaConfig.Info.Description = "HTTP API for OpenCode.\n\nhttps://github.com/coder/agentapi"
	api := humachi.New(router, humaConfig)

	emitter := NewEventEmitter(1024)
	s := &OpenCodeServer{
		router:       router,
		api:          api,
		port:         port,
		conversation: conversation,
		logger:       logger,
		emitter:      emitter,
	}

	// Register API routes
	s.registerRoutes(chatBasePath)

	return s
}

func (s *OpenCodeServer) GetOpenAPI() string {
	// Reuse the existing server's OpenAPI generation logic
	tempServer := &Server{
		api: s.api,
	}
	return tempServer.GetOpenAPI()
}

func (s *OpenCodeServer) StartSnapshotLoop(ctx context.Context) {
	s.conversation.StartSnapshotLoop(ctx)
	go func() {
		adapter := &ConversationAdapter{conv: s.conversation}
		for {
			s.emitter.UpdateStatusAndEmitChanges(adapter.Status())
			s.emitter.UpdateMessagesAndEmitChanges(adapter.Messages())
			s.emitter.UpdateScreenAndEmitChanges(adapter.Screen())
			time.Sleep(25 * time.Millisecond)
		}
	}()
}

// registerRoutes sets up all API endpoints
func (s *OpenCodeServer) registerRoutes(chatBasePath string) {
	// GET /status endpoint
	huma.Get(s.api, "/status", s.getStatus, func(o *huma.Operation) {
		o.Description = "Returns the current status of the agent."
	})

	// GET /messages endpoint
	huma.Get(s.api, "/messages", s.getMessages, func(o *huma.Operation) {
		o.Description = "Returns a list of messages representing the conversation history with the agent."
	})

	// POST /message endpoint
	huma.Post(s.api, "/message", s.createMessage, func(o *huma.Operation) {
		o.Description = "Send a message to the agent. For messages of type 'user', the agent's status must be 'stable' for the operation to complete successfully. Otherwise, this endpoint will return an error."
	})

	// GET /events endpoint
	sse.Register(s.api, huma.Operation{
		OperationID: "subscribeEvents",
		Method:      http.MethodGet,
		Path:        "/events",
		Summary:     "Subscribe to events",
		Description: "The events are sent as Server-Sent Events (SSE). Initially, the endpoint returns a list of events needed to reconstruct the current state of the conversation and the agent's status. After that, it only returns events that have occurred since the last event was sent.\n\nNote: When an agent is running, the last message in the conversation history is updated frequently, and the endpoint sends a new message update event each time.",
	}, map[string]any{
		// Mapping of event type name to Go struct for that event.
		"message_update": MessageUpdateBody{},
		"status_change":  StatusChangeBody{},
	}, s.subscribeEvents)

	sse.Register(s.api, huma.Operation{
		OperationID: "subscribeScreen",
		Method:      http.MethodGet,
		Path:        "/internal/screen",
		Summary:     "Subscribe to screen",
		Hidden:      true,
	}, map[string]any{
		"screen": ScreenUpdateBody{},
	}, s.subscribeScreen)

	s.router.Handle("/", http.HandlerFunc(s.redirectToChat))

	// Serve static files for the chat interface under /chat
	s.registerStaticFileRoutes(chatBasePath)
}

// getStatus handles GET /status
func (s *OpenCodeServer) getStatus(ctx context.Context, input *struct{}) (*StatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := s.conversation.Status()
	agentStatus := convertStatus(status)

	resp := &StatusResponse{}
	resp.Body.Status = agentStatus

	return resp, nil
}

// getMessages handles GET /messages
func (s *OpenCodeServer) getMessages(ctx context.Context, input *struct{}) (*MessagesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	resp := &MessagesResponse{}
	messages := s.conversation.Messages()
	resp.Body.Messages = make([]Message, len(messages))
	for i, msg := range messages {
		resp.Body.Messages[i] = Message{
			Id:      msg.Id,
			Role:    msg.Role,
			Content: msg.Message,
			Time:    msg.Time,
		}
	}

	return resp, nil
}

// createMessage handles POST /message
func (s *OpenCodeServer) createMessage(ctx context.Context, input *MessageRequest) (*MessageResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch input.Body.Type {
	case MessageTypeUser:
		messageParts := FormatMessage(mf.AgentTypeOpenCode, input.Body.Content)
		// Convert MessageParts to strings
		var messageStrings []string
		for _, part := range messageParts {
			messageStrings = append(messageStrings, part.String())
		}
		if err := s.conversation.SendMessage(messageStrings...); err != nil {
			return nil, xerrors.Errorf("failed to send message: %w", err)
		}
	case MessageTypeRaw:
		// For OpenCode, raw messages are treated the same as user messages since we use REST API
		if err := s.conversation.SendMessage(input.Body.Content); err != nil {
			return nil, xerrors.Errorf("failed to send message: %w", err)
		}
	}

	resp := &MessageResponse{}
	resp.Body.Ok = true

	return resp, nil
}

// subscribeEvents is an SSE endpoint that sends events to the client
func (s *OpenCodeServer) subscribeEvents(ctx context.Context, input *struct{}, send sse.Sender) {
	subscriberId, ch, stateEvents := s.emitter.Subscribe()
	defer s.emitter.Unsubscribe(subscriberId)
	s.logger.Info("New subscriber", "subscriberId", subscriberId)
	for _, event := range stateEvents {
		if event.Type == EventTypeScreenUpdate {
			continue
		}
		if err := send.Data(event.Payload); err != nil {
			s.logger.Error("Failed to send event", "subscriberId", subscriberId, "error", err)
			return
		}
	}
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				s.logger.Info("Channel closed", "subscriberId", subscriberId)
				return
			}
			if event.Type == EventTypeScreenUpdate {
				continue
			}
			if err := send.Data(event.Payload); err != nil {
				s.logger.Error("Failed to send event", "subscriberId", subscriberId, "error", err)
				return
			}
		case <-ctx.Done():
			s.logger.Info("Context done", "subscriberId", subscriberId)
			return
		}
	}
}

func (s *OpenCodeServer) subscribeScreen(ctx context.Context, input *struct{}, send sse.Sender) {
	subscriberId, ch, stateEvents := s.emitter.Subscribe()
	defer s.emitter.Unsubscribe(subscriberId)
	s.logger.Info("New screen subscriber", "subscriberId", subscriberId)
	for _, event := range stateEvents {
		if event.Type != EventTypeScreenUpdate {
			continue
		}
		if err := send.Data(event.Payload); err != nil {
			s.logger.Error("Failed to send screen event", "subscriberId", subscriberId, "error", err)
			return
		}
	}
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				s.logger.Info("Screen channel closed", "subscriberId", subscriberId)
				return
			}
			if event.Type != EventTypeScreenUpdate {
				continue
			}
			if err := send.Data(event.Payload); err != nil {
				s.logger.Error("Failed to send screen event", "subscriberId", subscriberId, "error", err)
				return
			}
		case <-ctx.Done():
			s.logger.Info("Screen context done", "subscriberId", subscriberId)
			return
		}
	}
}

// Start starts the HTTP server
func (s *OpenCodeServer) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	s.srv = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	return s.srv.ListenAndServe()
}

// Stop gracefully stops the HTTP server
func (s *OpenCodeServer) Stop(ctx context.Context) error {
	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}
	return nil
}

// registerStaticFileRoutes sets up routes for serving static files
func (s *OpenCodeServer) registerStaticFileRoutes(chatBasePath string) {
	chatHandler := FileServerWithIndexFallback(chatBasePath)

	// Mount the file server at /chat
	s.router.Handle("/chat", http.StripPrefix("/chat", chatHandler))
	s.router.Handle("/chat/*", http.StripPrefix("/chat", chatHandler))
}

func (s *OpenCodeServer) redirectToChat(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/chat/embed", http.StatusTemporaryRedirect)
}