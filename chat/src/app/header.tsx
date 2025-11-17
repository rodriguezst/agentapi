"use client";

import {AgentType, useChat} from "@/components/chat-provider";
import {ModeToggle} from "@/components/mode-toggle";

export function Header() {
  const {serverStatus, agentType} = useChat();

  return (
    <header className="backdrop-blur-sm bg-background/80 border-b px-6 py-4 sticky top-0 z-10">
      <div className="max-w-7xl mx-auto flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-primary to-primary/60 flex items-center justify-center shadow-sm">
              <svg className="w-5 h-5 text-primary-foreground" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 10h.01M12 10h.01M16 10h.01M9 16H5a2 2 0 01-2-2V6a2 2 0 012-2h14a2 2 0 012 2v8a2 2 0 01-2 2h-5l-5 5v-5z" />
              </svg>
            </div>
            <h1 className="font-semibold text-lg tracking-tight">AgentAPI Chat</h1>
          </div>
        </div>

        <div className="flex items-center gap-4">
          {serverStatus !== "unknown" && (
            <div className="flex items-center gap-2.5 px-3 py-1.5 rounded-full bg-muted/50 border border-border/50">
              <div className="relative flex items-center justify-center">
                <span
                  className={`w-2 h-2 rounded-full transition-all duration-300 ${
                    ["offline", "unknown"].includes(serverStatus)
                      ? "bg-red-500"
                      : "bg-green-500 animate-pulse"
                  }`}
                />
                <span
                  className={`absolute w-2 h-2 rounded-full ${
                    ["offline", "unknown"].includes(serverStatus)
                      ? "bg-red-500/30 ring-2 ring-red-500/20"
                      : "bg-green-500/30 ring-2 ring-green-500/20 animate-ping"
                  }`}
                />
              </div>
              <span className="sr-only">Status:</span>
              <span className="text-xs font-medium first-letter:uppercase">{serverStatus}</span>
            </div>
          )}

          {agentType !== "unknown" && (
            <div className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-primary/10 border border-primary/20">
              <span className="text-xs font-medium text-primary">{AgentType[agentType].displayName}</span>
            </div>
          )}

          <div className="h-5 w-px bg-border" />
          <ModeToggle/>
        </div>
      </div>
    </header>
  );
}
