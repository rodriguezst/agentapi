package httpapi_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	"github.com/coder/agentapi/lib/httpapi"
	"github.com/coder/agentapi/lib/logctx"
	"github.com/coder/agentapi/lib/msgfmt"
	"github.com/stretchr/testify/require"
)

func TestOpencodeIntegration(t *testing.T) {
	t.Parallel()

	ctx := logctx.WithLogger(context.Background(), slog.New(slog.NewTextHandler(os.Stdout, nil)))
	
	// Test that opencode server can be created (even if opencode daemon isn't running)
	srv := httpapi.NewServer(ctx, msgfmt.AgentTypeOpencode, nil, 0, "/chat")
	require.NotNil(t, srv)

	// Test getting OpenAPI schema for opencode
	schema := srv.GetOpenAPI()
	require.NotEmpty(t, schema)
	require.Contains(t, schema, "Opencode")
}

func TestOpencodeMessageFormat(t *testing.T) {
	// Test that opencode message formatting works correctly
	testMessage := "Hello, this is a test message with some formatting.\n\nMultiple lines."
	formatted := msgfmt.FormatAgentMessage(msgfmt.AgentTypeOpencode, testMessage, "")
	
	// For opencode, the message should be trimmed but not heavily processed like terminal agents
	require.Equal(t, "Hello, this is a test message with some formatting.\n\nMultiple lines.", formatted)
}