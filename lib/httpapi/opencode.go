package httpapi

import (
	st "github.com/coder/agentapi/lib/screentracker"
	mf "github.com/coder/agentapi/lib/msgfmt"
)

// FormatMessageForOpenCode formats the message to be sent to the OpenCode agent.
// OpenCode requires similar formatting as other CLI agents.
func FormatMessageForOpenCode(message string) []st.MessagePart {
	message = mf.TrimWhitespace(message)
	return formatClaudeCodeMessage(message)
}