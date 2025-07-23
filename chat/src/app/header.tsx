"use client";

import { useChat } from "@/components/chat-provider";
import { ModeToggle } from "../components/mode-toggle";

export function Header() {
  const { serverStatus } = useChat();

  return (
    <header className="relative bg-gradient-to-r from-blue-50 via-indigo-50 to-purple-50 dark:from-gray-900 dark:via-gray-800 dark:to-gray-900 border-b border-border/50 backdrop-blur-sm">
      {/* Gradient overlay for depth */}
      <div className="absolute inset-0 bg-gradient-to-r from-blue-500/5 via-indigo-500/5 to-purple-500/5 dark:from-blue-500/10 dark:via-indigo-500/10 dark:to-purple-500/10" />
      
      <div className="relative px-6 py-4 flex items-center justify-between">
        <div className="flex items-center gap-3">
          {/* Modern logo/icon placeholder */}
          <div className="w-8 h-8 rounded-lg bg-gradient-to-br from-blue-500 to-indigo-600 dark:from-blue-400 dark:to-indigo-500 flex items-center justify-center shadow-sm">
            <svg className="w-5 h-5 text-white" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 9l3 3-3 3m5 0h3" />
            </svg>
          </div>
          
          <div className="flex flex-col">
            <h1 className="text-lg font-semibold bg-gradient-to-r from-gray-900 to-gray-700 dark:from-white dark:to-gray-200 bg-clip-text text-transparent">
              AgentAPI Chat
            </h1>
            <p className="text-xs text-muted-foreground">
              Control coding agents with HTTP API
            </p>
          </div>
        </div>

        <div className="flex items-center gap-4">
          {serverStatus !== "unknown" && (
            <div className="flex items-center gap-2 px-3 py-1.5 rounded-full bg-white/50 dark:bg-gray-800/50 border border-border/50 backdrop-blur-sm">
              <span
                className={`w-2 h-2 rounded-full transition-all duration-200 ${
                  ["offline", "unknown"].includes(serverStatus)
                    ? "bg-red-500 shadow-lg shadow-red-500/50 animate-pulse"
                    : "bg-green-500 shadow-lg shadow-green-500/50"
                }`}
              />
              <span className="text-xs font-medium text-muted-foreground first-letter:uppercase">
                {serverStatus}
              </span>
            </div>
          )}
          <ModeToggle />
        </div>
      </div>
    </header>
  );
}
