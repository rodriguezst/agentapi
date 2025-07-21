# Testing OpenCode Integration

This document describes how to test the OpenCode integration with AgentAPI using the provided test script.

## Overview

The OpenCode integration allows AgentAPI to communicate with OpenCode via REST API instead of parsing terminal output. This provides a more robust and reliable integration.

## Architecture

- **AgentAPI Server**: Runs on port 3284 (default) and provides the standard AgentAPI endpoints
- **OpenCode Server**: Runs on port 4284 (default) and provides OpenCode's REST API
- **Integration**: AgentAPI forwards messages to OpenCode via HTTP API calls and retrieves responses

## Test Script

The `test_opencode_integration.sh` script provides comprehensive testing of the integration by:

1. Checking that both AgentAPI and OpenCode servers are running
2. Sending a test message via AgentAPI's `/message` endpoint
3. Retrieving messages from both AgentAPI's `/messages` endpoint and OpenCode's session endpoints
4. Comparing the responses to verify they contain the same content

## Prerequisites

1. **OpenCode installed**: Install via `npm i -g opencode-ai@latest` or `curl -fsSL https://opencode.ai/install | bash`
2. **OpenCode configured**: Create `~/.config/opencode/opencode.json` with your provider configuration
3. **Dependencies**: The script requires `curl` and optionally `jq` for JSON parsing

### Example OpenCode Configuration

For testing purposes, you can use the mockgpt provider:

```json
{
  "$schema": "https://opencode.ai/config.json",
  "theme": "opencode",
  "model": "mockgpt/gpt-3.5-turbo",
  "provider": {
    "mockgpt": {
      "npm": "@ai-sdk/openai-compatible",
      "name": "mockgpt",
      "options": {
        "baseURL": "https://mockgpt.wiremockapi.cloud/v1",
        "apiKey": "sk-k01o5e0prys87i2sam8qegvxc5vyy5mu"
      },
      "models": {
        "gpt-3.5-turbo": {}
      }
    }
  }
}
```

## Running the Test

1. **Start AgentAPI with OpenCode**:
   ```bash
   agentapi server -- opencode
   ```

2. **In another terminal, run the test script**:
   ```bash
   ./test_opencode_integration.sh
   ```

## Test Script Options

The script supports various command-line options:

```bash
./test_opencode_integration.sh [OPTIONS]

Options:
  --agentapi-port PORT    AgentAPI port (default: 3284)
  --opencode-port PORT    OpenCode port (default: 4284)
  --message MESSAGE       Test message to send
  --max-retries COUNT     Maximum retries for service checks (default: 30)
  --retry-delay SECONDS   Delay between retries (default: 2)
  --session-id ID         Manually specify OpenCode session ID
  --help                  Show help message
```

## Example Usage

```bash
# Basic test
./test_opencode_integration.sh

# Custom message and session ID
./test_opencode_integration.sh --message "Custom test message" --session-id ses_abc123

# Different ports
./test_opencode_integration.sh --agentapi-port 8080 --opencode-port 8081

# Quick test with shorter timeouts
./test_opencode_integration.sh --max-retries 5 --retry-delay 1
```

## Expected Output

Successful test run:
```
[INFO] Starting OpenCode Integration Test
[INFO] ==================================
[INFO] Waiting for AgentAPI to be ready...
[SUCCESS] AgentAPI is running
[INFO] Waiting for OpenCode to be ready...
[SUCCESS] OpenCode is running
[INFO] Using OpenCode session ID: ses_7d2d0ecb3ffejWeavxq5MAcrkD
[INFO] Sending message via AgentAPI: 'Hello, this is a test message from the integration script'
[SUCCESS] Message sent via AgentAPI
[INFO] Waiting 2s for message processing...
[INFO] Getting messages from AgentAPI...
[SUCCESS] Retrieved messages from AgentAPI
[INFO] Getting messages from OpenCode session: ses_7d2d0ecb3ffejWeavxq5MAcrkD
[SUCCESS] Retrieved messages from OpenCode
[INFO] Comparing messages from both endpoints...
[SUCCESS] Both endpoints contain the test message
[SUCCESS] Integration test PASSED: Messages are consistent between endpoints
[SUCCESS] Integration test completed successfully!
```

## Troubleshooting

### Services Not Running
If you get "service not running" errors:
- Ensure AgentAPI is started with `agentapi server -- opencode`
- Check that ports 3284 and 4284 are not in use by other processes
- Verify OpenCode is properly installed and configured

### Session ID Not Found
If the script can't find the OpenCode session ID:
- Check the AgentAPI server logs for session creation messages
- Manually specify the session ID using `--session-id` option
- Ensure OpenCode server is responding to API calls

### Message Comparison Failures
If messages don't match between endpoints:
- Check that both services are processing messages correctly
- Verify the OpenCode configuration is working (test with `opencode run` first)
- Review the server logs for any error messages

### Dependencies Missing
- Install `curl`: Most systems have this pre-installed
- Install `jq`: `sudo apt-get install jq` (Ubuntu/Debian) or `brew install jq` (macOS)

## Manual Testing

You can also test the integration manually using curl:

```bash
# Check AgentAPI status
curl http://localhost:3284/status

# Check OpenCode status  
curl http://localhost:4284/status

# Send a message via AgentAPI
curl -X POST -H "Content-Type: application/json" \
  -d '{"type": "user", "message": "Hello world"}' \
  http://localhost:3284/message

# Get messages from AgentAPI
curl http://localhost:3284/messages

# Get OpenCode session (replace with actual session ID)
curl http://localhost:4284/session/ses_YOUR_SESSION_ID
```

This allows you to debug specific parts of the integration if the automated test fails.