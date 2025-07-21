# OpenCode Integration

This document describes how OpenCode integration works in AgentAPI.

## Architecture

OpenCode uses a different integration approach compared to other agents:

- **Other agents** (Claude Code, Goose, Aider, Codex): TUI parsing approach
- **OpenCode**: REST API approach using "opencode serve"

## How it works

1. **Service Management**: AgentAPI starts "opencode serve" as a background process on a separate port (AgentAPI port + 1000)
2. **REST API Communication**: All interactions happen via HTTP requests to OpenCode's REST API
3. **Session Management**: Creates and manages OpenCode sessions via `/session` endpoints
4. **Message Handling**: Sends messages via POST `/session/:id/message` instead of terminal I/O
5. **Transparent Interface**: External clients see the same AgentAPI interface regardless of backend

## Components

### OpenCode Client (`lib/opencode/client.go`)
- HTTP client for OpenCode's REST API
- Handles session creation, message sending, provider configuration
- Manages connection lifecycle and error handling

### OpenCode Service (`lib/opencode/service.go`)
- Manages "opencode serve" process lifecycle  
- Handles graceful startup/shutdown
- Monitors service health

### OpenCode Conversation (`lib/opencode/conversation.go`)
- Adapts OpenCode REST API to match existing screentracker interface
- Manages conversation state via HTTP instead of terminal snapshots
- Handles async message processing

### OpenCode Server (`lib/httpapi/opencode_server.go`)
- Provides same AgentAPI HTTP interface but backed by OpenCode REST API
- Reuses existing event system and routes
- Transparent to external clients

## Benefits

1. **More Reliable**: No TUI parsing issues or terminal emulation complexity
2. **Better Performance**: Direct API communication is faster than screen scraping
3. **Consistent**: Uses OpenCode's official API instead of reverse-engineering TUI output
4. **Future-Proof**: Won't break when OpenCode updates its TUI
5. **Feature Complete**: Access to all OpenCode capabilities via proper API

## Usage

Usage remains identical to other agents:

```bash
# Start AgentAPI with OpenCode
agentapi server -- opencode

# Or with explicit type
agentapi server --type opencode

# Same API endpoints work
curl http://localhost:3284/status
curl -X POST http://localhost:3284/message -d '{"type":"user","content":"Hello"}'
```

The only difference is that OpenCode runs as a REST service instead of a TUI process.