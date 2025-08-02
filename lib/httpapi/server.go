package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/coder/agentapi/lib/logctx"
	mf "github.com/coder/agentapi/lib/msgfmt"
	st "github.com/coder/agentapi/lib/screentracker"
	"github.com/coder/agentapi/lib/termexec"
	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humachi"
	"github.com/danielgtaylor/huma/v2/sse"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"golang.org/x/xerrors"
)

// Server represents the HTTP server
type Server struct {
	router         chi.Router
	api            huma.API
	port           int
	srv            *http.Server
	mu             sync.RWMutex
	logger         *slog.Logger
	conversation   *st.Conversation
	agentio        *termexec.Process
	opencodeClient *OpencodeClient
	agentType      mf.AgentType
	emitter        *EventEmitter
}

func (s *Server) GetOpenAPI() string {
	jsonBytes, err := s.api.OpenAPI().MarshalJSON()
	if err != nil {
		return ""
	}
	// unmarshal the json and pretty print it
	var jsonObj any
	if err := json.Unmarshal(jsonBytes, &jsonObj); err != nil {
		return ""
	}
	prettyJSON, err := json.MarshalIndent(jsonObj, "", "  ")
	if err != nil {
		return ""
	}
	return string(prettyJSON)
}

// That's about 40 frames per second. It's slightly less
// because the action of taking a snapshot takes time too.
const snapshotInterval = 25 * time.Millisecond

// NewServer creates a new server instance
func NewServer(ctx context.Context, agentType mf.AgentType, process *termexec.Process, port int, chatBasePath string) *Server {
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
	humaConfig.Info.Description = "HTTP API for Claude Code, Goose, Aider, and Opencode.\n\nhttps://github.com/coder/agentapi"
	api := humachi.New(router, humaConfig)

	logger := logctx.From(ctx)
	emitter := NewEventEmitter(1024)

	s := &Server{
		router:    router,
		api:       api,
		port:      port,
		logger:    logger,
		agentio:   process,
		agentType: agentType,
		emitter:   emitter,
	}

	if agentType == mf.AgentTypeOpencode {
		// For opencode, create opencode client instead of conversation
		opencodeClient, err := NewOpencodeClient(ctx, logger)
		if err != nil {
			logger.Error("Failed to create opencode client", "error", err)
			logger.Warn("Opencode client unavailable - ensure opencode daemon is running")
			// Continue with nil client, will provide helpful error in message endpoints
		}
		s.opencodeClient = opencodeClient
	} else {
		// For terminal-based agents, create conversation tracker
		formatMessage := func(message string, userInput string) string {
			return mf.FormatAgentMessage(agentType, message, userInput)
		}
		conversation := st.NewConversation(ctx, st.ConversationConfig{
			AgentIO: process,
			GetTime: func() time.Time {
				return time.Now()
			},
			SnapshotInterval:      snapshotInterval,
			ScreenStabilityLength: 2 * time.Second,
			FormatMessage:         formatMessage,
		})
		s.conversation = conversation
	}

	// Register API routes
	s.registerRoutes(chatBasePath)

	return s
}

func (s *Server) StartSnapshotLoop(ctx context.Context) {
	if s.agentType == mf.AgentTypeOpencode {
		// For opencode, start a loop to emit changes from opencode client
		go func() {
			for {
				if s.opencodeClient != nil {
					s.emitter.UpdateStatusAndEmitChanges(s.opencodeClient.Status())
					s.emitter.UpdateMessagesAndEmitChanges(s.opencodeClient.Messages())
					s.emitter.UpdateScreenAndEmitChanges(s.opencodeClient.Screen())
				}
				time.Sleep(snapshotInterval)
			}
		}()
	} else {
		// For terminal-based agents, use conversation tracker
		if s.conversation != nil {
			s.conversation.StartSnapshotLoop(ctx)
		}
		go func() {
			for {
				if s.conversation != nil {
					s.emitter.UpdateStatusAndEmitChanges(s.conversation.Status())
					s.emitter.UpdateMessagesAndEmitChanges(s.conversation.Messages())
					s.emitter.UpdateScreenAndEmitChanges(s.conversation.Screen())
				}
				time.Sleep(snapshotInterval)
			}
		}()
	}
}

// registerRoutes sets up all API endpoints
func (s *Server) registerRoutes(chatBasePath string) {
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
func (s *Server) getStatus(ctx context.Context, input *struct{}) (*StatusResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var status st.ConversationStatus
	if s.agentType == mf.AgentTypeOpencode && s.opencodeClient != nil {
		status = s.opencodeClient.Status()
	} else if s.conversation != nil {
		status = s.conversation.Status()
	} else {
		status = st.ConversationStatusStable // Default status
	}
	
	agentStatus := convertStatus(status)

	resp := &StatusResponse{}
	resp.Body.Status = agentStatus

	return resp, nil
}

// getMessages handles GET /messages
func (s *Server) getMessages(ctx context.Context, input *struct{}) (*MessagesResponse, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var messages []st.ConversationMessage
	if s.agentType == mf.AgentTypeOpencode && s.opencodeClient != nil {
		messages = s.opencodeClient.Messages()
	} else if s.conversation != nil {
		messages = s.conversation.Messages()
	}

	resp := &MessagesResponse{}
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
func (s *Server) createMessage(ctx context.Context, input *MessageRequest) (*MessageResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	switch input.Body.Type {
	case MessageTypeUser:
		if s.agentType == mf.AgentTypeOpencode && s.opencodeClient != nil {
			// Use opencode REST API
			if err := s.opencodeClient.SendMessage(ctx, input.Body.Content); err != nil {
				return nil, xerrors.Errorf("failed to send message to opencode: %w", err)
			}
		} else if s.agentType == mf.AgentTypeOpencode && s.opencodeClient == nil {
			return nil, xerrors.Errorf("opencode client unavailable - ensure opencode daemon is running and properly configured")
		} else if s.conversation != nil {
			// Use terminal-based agent
			if err := s.conversation.SendMessage(FormatMessage(s.agentType, input.Body.Content)...); err != nil {
				return nil, xerrors.Errorf("failed to send message: %w", err)
			}
		} else {
			return nil, xerrors.Errorf("no agent available")
		}
	case MessageTypeRaw:
		if s.agentType == mf.AgentTypeOpencode {
			return nil, xerrors.Errorf("raw message type not supported for opencode")
		}
		if s.agentio == nil {
			return nil, xerrors.Errorf("no terminal agent available")
		}
		if _, err := s.agentio.Write([]byte(input.Body.Content)); err != nil {
			return nil, xerrors.Errorf("failed to send message: %w", err)
		}
	}

	resp := &MessageResponse{}
	resp.Body.Ok = true

	return resp, nil
}

// subscribeEvents is an SSE endpoint that sends events to the client
func (s *Server) subscribeEvents(ctx context.Context, input *struct{}, send sse.Sender) {
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

func (s *Server) subscribeScreen(ctx context.Context, input *struct{}, send sse.Sender) {
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
func (s *Server) Start() error {
	addr := fmt.Sprintf(":%d", s.port)
	s.srv = &http.Server{
		Addr:    addr,
		Handler: s.router,
	}

	return s.srv.ListenAndServe()
}

// Stop gracefully stops the HTTP server
func (s *Server) Stop(ctx context.Context) error {
	if s.srv != nil {
		return s.srv.Shutdown(ctx)
	}
	return nil
}

// registerStaticFileRoutes sets up routes for serving static files
func (s *Server) registerStaticFileRoutes(chatBasePath string) {
	chatHandler := FileServerWithIndexFallback(chatBasePath)

	// Mount the file server at /chat
	s.router.Handle("/chat", http.StripPrefix("/chat", chatHandler))
	s.router.Handle("/chat/*", http.StripPrefix("/chat", chatHandler))
}

func (s *Server) redirectToChat(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/chat/embed", http.StatusTemporaryRedirect)
}
