"use client";

import { useLayoutEffect, useRef, useEffect, useCallback } from "react";

interface Message {
  role: string;
  content: string;
  id: number;
}

// Draft messages are used to optmistically update the UI
// before the server responds.
interface DraftMessage extends Omit<Message, "id"> {
  id?: number;
}

interface MessageListProps {
  messages: (Message | DraftMessage)[];
}

export default function MessageList({ messages }: MessageListProps) {
  const scrollAreaRef = useRef<HTMLDivElement>(null);
  // Avoid the message list to change its height all the time. It causes some
  // flickering in the screen because some messages, as the ones displaying
  // progress statuses, are changing the content(the number of lines) and size
  // constantily. To minimize it, we keep track of the biggest scroll height of
  // the content, and use that as the min height of the scroll area.
  const contentMinHeight = useRef(0);

  // Track if user is at bottom - default to true for initial scroll
  const isAtBottomRef = useRef(true);
  // Track the last known scroll height to detect new content
  const lastScrollHeightRef = useRef(0);

  const checkIfAtBottom = useCallback(() => {
    if (!scrollAreaRef.current) return false;
    const { scrollTop, scrollHeight, clientHeight } = scrollAreaRef.current;
    return scrollTop + clientHeight >= scrollHeight - 10; // 10px tolerance
  }, []);

  // Update isAtBottom on scroll
  useEffect(() => {
    const scrollContainer = scrollAreaRef.current;
    if (!scrollContainer) return;

    const handleScroll = () => {
      isAtBottomRef.current = checkIfAtBottom();
    };

    // Initial check
    handleScroll();

    scrollContainer.addEventListener("scroll", handleScroll);
    return () => scrollContainer.removeEventListener("scroll", handleScroll);
  }, [checkIfAtBottom]);

  // Handle auto-scrolling when messages change
  useLayoutEffect(() => {
    if (!scrollAreaRef.current) return;

    const scrollContainer = scrollAreaRef.current;
    const currentScrollHeight = scrollContainer.scrollHeight;

    // Check if this is new content (scroll height increased)
    const hasNewContent = currentScrollHeight > lastScrollHeightRef.current;
    const isFirstRender = lastScrollHeightRef.current === 0;
    const isNewUserMessage =
      messages.length > 0 && messages[messages.length - 1].role === "user";

    // Update content min height if needed
    if (currentScrollHeight > contentMinHeight.current) {
      contentMinHeight.current = currentScrollHeight;
    }

    // Auto-scroll only if:
    // 1. It's the first render, OR
    // 2. There's new content AND user was at the bottom, OR
    // 3. The user sent a new message
    if (
      hasNewContent &&
      (isFirstRender || isAtBottomRef.current || isNewUserMessage)
    ) {
      scrollContainer.scrollTo({
        top: currentScrollHeight,
        behavior: isFirstRender ? "instant" : "smooth",
      });
      // After scrolling, we're at the bottom
      isAtBottomRef.current = true;
    }

    // Update the last known scroll height
    lastScrollHeightRef.current = currentScrollHeight;
  }, [messages]);

  // If no messages, show a more appealing welcome screen
  if (messages.length === 0) {
    return (
      <div className="flex-1 p-6 flex items-center justify-center">
        <div className="text-center max-w-md mx-auto">
          <div className="w-16 h-16 rounded-full bg-gradient-to-br from-blue-100 to-indigo-100 dark:from-blue-900/30 dark:to-indigo-900/30 flex items-center justify-center mb-4 mx-auto">
            <svg className="w-8 h-8 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z" />
            </svg>
          </div>
          <h3 className="text-lg font-semibold text-foreground mb-2">
            Welcome to AgentAPI Chat
          </h3>
          <p className="text-muted-foreground text-sm leading-relaxed">
            Start a conversation with your coding agent. Type a message below or use Control mode to send terminal commands directly.
          </p>
        </div>
      </div>
    );
  }

  return (
    <div className="overflow-y-auto flex-1 bg-gradient-to-b from-gray-50/30 to-white dark:from-gray-900/30 dark:to-background" ref={scrollAreaRef}>
      <div
        className="p-6 flex flex-col gap-6 max-w-4xl mx-auto"
        style={{ minHeight: contentMinHeight.current }}
      >
        {messages.map((message) => (
          <div
            key={message.id ?? "draft"}
            className={`flex ${message.role === "user" ? "justify-end" : "justify-start"} animate-in fade-in slide-in-from-bottom-4 duration-300`}
          >
            <div className="flex gap-3 max-w-[85%]">
              {/* Avatar for assistant messages */}
              {message.role !== "user" && (
                <div className="w-8 h-8 rounded-full bg-gradient-to-br from-gray-100 to-gray-200 dark:from-gray-700 dark:to-gray-800 flex items-center justify-center flex-shrink-0 mt-1">
                  <svg className="w-4 h-4 text-gray-600 dark:text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                  </svg>
                </div>
              )}
              
              <div
                className={`relative group ${
                  message.role === "user"
                    ? "bg-gradient-to-br from-blue-500 to-blue-600 text-white shadow-lg shadow-blue-500/25 rounded-2xl rounded-br-sm"
                    : "bg-white dark:bg-gray-800 border border-border/50 shadow-sm hover:shadow-md transition-shadow duration-200 rounded-2xl rounded-bl-sm"
                } px-4 py-3 ${message.id === undefined ? "animate-pulse" : ""}`}
              >
                {/* Message content */}
                <div
                  className={`whitespace-pre-wrap break-words text-sm leading-relaxed ${
                    message.role === "user" 
                      ? "text-white" 
                      : "text-foreground font-mono text-xs"
                  }`}
                >
                  {message.role !== "user" && message.content === "" ? (
                    <LoadingDots />
                  ) : (
                    message.content.trim()
                  )}
                </div>
                
                {/* Timestamp on hover */}
                <div className="opacity-0 group-hover:opacity-100 transition-opacity duration-200 absolute -bottom-6 right-0 text-xs text-muted-foreground">
                  {message.id && new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}
                </div>
              </div>
              
              {/* Avatar for user messages */}
              {message.role === "user" && (
                <div className="w-8 h-8 rounded-full bg-gradient-to-br from-blue-100 to-blue-200 dark:from-blue-900 dark:to-blue-800 flex items-center justify-center flex-shrink-0 mt-1">
                  <svg className="w-4 h-4 text-blue-600 dark:text-blue-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M16 7a4 4 0 11-8 0 4 4 0 018 0zM12 14a7 7 0 00-7 7h14a7 7 0 00-7-7z" />
                  </svg>
                </div>
              )}
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

const LoadingDots = () => (
  <div className="flex items-center space-x-2 py-2">
    <div className="flex space-x-1">
      <div
        aria-hidden="true"
        className="w-2 h-2 rounded-full bg-muted-foreground animate-bounce [animation-delay:0ms]"
      />
      <div
        aria-hidden="true"
        className="w-2 h-2 rounded-full bg-muted-foreground animate-bounce [animation-delay:150ms]"
      />
      <div
        aria-hidden="true"
        className="w-2 h-2 rounded-full bg-muted-foreground animate-bounce [animation-delay:300ms]"
      />
    </div>
    <span className="text-xs text-muted-foreground ml-2">Agent is thinking...</span>
    <span className="sr-only">Loading...</span>
  </div>
);
