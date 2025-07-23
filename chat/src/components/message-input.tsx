"use client";

import { useState, FormEvent, KeyboardEvent, useEffect, useRef } from "react";
import { Button } from "./ui/button";
import {
  ArrowDownIcon,
  ArrowLeftIcon,
  ArrowRightIcon,
  ArrowUpIcon,
  CornerDownLeftIcon,
  DeleteIcon,
  SendIcon,
} from "lucide-react";
import { Tabs, TabsList, TabsTrigger } from "./ui/tabs";

interface MessageInputProps {
  onSendMessage: (message: string, type: "user" | "raw") => void;
  disabled?: boolean;
}

interface SentChar {
  char: string;
  id: number;
  timestamp: number;
}

export default function MessageInput({
  onSendMessage,
  disabled = false,
}: MessageInputProps) {
  const [message, setMessage] = useState("");
  const [inputMode, setInputMode] = useState("text");
  const [sentChars, setSentChars] = useState<SentChar[]>([]);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const nextCharId = useRef(0);
  const [controlAreaFocused, setControlAreaFocused] = useState(false);

  const handleSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (message.trim() && !disabled) {
      onSendMessage(message, "user");
      setMessage("");
    }
  };

  // Remove sent characters after they expire (2 seconds)
  useEffect(() => {
    if (sentChars.length === 0) return;

    const interval = setInterval(() => {
      const now = Date.now();
      setSentChars((chars) =>
        chars.filter((char) => now - char.timestamp < 2000)
      );
    }, 100);

    return () => clearInterval(interval);
  }, [sentChars]);

  const addSentChar = (char: string) => {
    const newChar: SentChar = {
      char,
      id: nextCharId.current++,
      timestamp: Date.now(),
    };
    setSentChars((prev) => [...prev, newChar]);
  };

  const handleKeyDown = (e: KeyboardEvent<HTMLTextAreaElement>) => {
    // In control mode, send special keys as raw messages
    if (inputMode === "control" && !disabled) {
      // List of keys to send as raw input when in control mode
      const specialKeys: Record<string, string> = {
        ArrowUp: "\x1b[A", // Escape sequence for up arrow
        ArrowDown: "\x1b[B", // Escape sequence for down arrow
        ArrowRight: "\x1b[C", // Escape sequence for right arrow
        ArrowLeft: "\x1b[D", // Escape sequence for left arrow
        Escape: "\x1b", // Escape key
        Tab: "\t", // Tab key
        Delete: "\x1b[3~", // Delete key
        Home: "\x1b[H", // Home key
        End: "\x1b[F", // End key
        PageUp: "\x1b[5~", // Page Up
        PageDown: "\x1b[6~", // Page Down
        Backspace: "\b", // Backspace key
      };

      // Check if the pressed key is in our special keys map
      if (specialKeys[e.key]) {
        e.preventDefault();
        addSentChar(e.key);
        onSendMessage(specialKeys[e.key], "raw");
        return;
      }

      // Handle Enter as raw newline when in control mode
      if (e.key === "Enter" && !e.shiftKey) {
        e.preventDefault();
        addSentChar("⏎");
        onSendMessage("\r", "raw");
        return;
      }

      // Handle Ctrl+key combinations
      if (e.ctrlKey) {
        const ctrlMappings: Record<string, string> = {
          c: "\x03", // Ctrl+C (SIGINT)
          d: "\x04", // Ctrl+D (EOF)
          z: "\x1A", // Ctrl+Z (SIGTSTP)
          l: "\x0C", // Ctrl+L (clear screen)
          a: "\x01", // Ctrl+A (beginning of line)
          e: "\x05", // Ctrl+E (end of line)
          w: "\x17", // Ctrl+W (delete word)
          u: "\x15", // Ctrl+U (clear line)
          r: "\x12", // Ctrl+R (reverse history search)
        };

        if (ctrlMappings[e.key.toLowerCase()]) {
          e.preventDefault();
          addSentChar(`Ctrl+${e.key.toUpperCase()}`);
          onSendMessage(ctrlMappings[e.key.toLowerCase()], "raw");
          return;
        }
      }

      // If it's a printable character (length 1), send it as raw input
      if (e.key.length === 1) {
        e.preventDefault();
        addSentChar(e.key);
        onSendMessage(e.key, "raw");
        return;
      }
    } else if (e.key === "Enter" && !e.shiftKey) {
      // Normal Enter handling for text mode with non-empty message
      e.preventDefault();
      handleSubmit(e);
    }
  };

  return (
    <Tabs value={inputMode} onValueChange={setInputMode}>
      <div className="max-w-4xl mx-auto w-full p-6 pt-4 bg-gradient-to-t from-white via-white to-transparent dark:from-background dark:via-background dark:to-transparent">
        <form
          onSubmit={handleSubmit}
          className="relative group rounded-2xl border border-border/50 bg-white dark:bg-gray-800/50 shadow-lg shadow-black/5 dark:shadow-black/20 backdrop-blur-sm text-base placeholder:text-muted-foreground focus-within:outline-none focus-within:ring-2 focus-within:ring-blue-500/20 focus-within:border-blue-500/50 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm transition-all duration-200 hover:shadow-xl hover:shadow-black/10 dark:hover:shadow-black/30"
        >
          <div className="flex flex-col">
            <div className="flex relative">
              {inputMode === "control" && !disabled ? (
                <div
                  // eslint-disable-next-line @typescript-eslint/no-explicit-any
                  ref={textareaRef as any}
                  tabIndex={0}
                  // eslint-disable-next-line @typescript-eslint/no-explicit-any
                  onKeyDown={handleKeyDown as any}
                  onFocus={() => setControlAreaFocused(true)}
                  onBlur={() => setControlAreaFocused(false)}
                  className="cursor-text p-6 h-20 text-muted-foreground flex items-center justify-center w-full outline-none text-sm bg-gradient-to-br from-amber-50/50 to-orange-50/50 dark:from-amber-900/10 dark:to-orange-900/10 rounded-t-2xl border-b border-border/30"
                >
                  <div className="text-center">
                    <div className="text-amber-600 dark:text-amber-400 font-medium mb-1">
                      {controlAreaFocused ? "🎛️ Control Mode Active" : "🎛️ Terminal Control"}
                    </div>
                    <div className="text-xs">
                      {controlAreaFocused
                        ? "Press any key to send to terminal (arrows, Ctrl+C, Ctrl+R, etc.)"
                        : "Click or focus this area to send keystrokes to terminal"}
                    </div>
                  </div>
                </div>
              ) : (
                <textarea
                  autoFocus
                  ref={textareaRef}
                  value={message}
                  onChange={(e) => setMessage(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder="Type a message..."
                  className="resize-none w-full text-sm outline-none p-6 h-20 bg-transparent rounded-t-2xl placeholder:text-muted-foreground/70"
                />
              )}
              
              {/* Floating gradient indicator */}
              <div className="absolute top-2 right-2 w-2 h-2 rounded-full bg-gradient-to-r from-blue-500 to-indigo-500 opacity-0 group-focus-within:opacity-100 transition-opacity duration-200" />
            </div>

            <div className="flex items-center justify-between p-4 pt-2 bg-gray-50/50 dark:bg-gray-900/20 rounded-b-2xl border-t border-border/30">
              <div className="flex items-center gap-2">
                <TabsList className="bg-white dark:bg-gray-800 shadow-sm border border-border/50">
                  <TabsTrigger
                    value="text"
                    onClick={() => {
                      textareaRef.current?.focus();
                    }}
                    className="data-[state=active]:bg-blue-500 data-[state=active]:text-white data-[state=active]:shadow-md transition-all duration-200"
                  >
                    💬 Text
                  </TabsTrigger>
                  <TabsTrigger
                    value="control"
                    onClick={() => {
                      textareaRef.current?.focus();
                    }}
                    className="data-[state=active]:bg-amber-500 data-[state=active]:text-white data-[state=active]:shadow-md transition-all duration-200"
                  >
                    🎛️ Control
                  </TabsTrigger>
                </TabsList>
                
                {/* Character count for text mode */}
                {inputMode === "text" && message.length > 0 && (
                  <div className="text-xs text-muted-foreground">
                    {message.length} chars
                  </div>
                )}
              </div>

              {inputMode === "text" && (
                <Button
                  type="submit"
                  disabled={disabled || !message.trim()}
                  size="icon"
                  className="rounded-full bg-gradient-to-r from-blue-500 to-indigo-600 hover:from-blue-600 hover:to-indigo-700 shadow-lg shadow-blue-500/25 disabled:shadow-none transition-all duration-200 hover:scale-105 active:scale-95"
                >
                  <SendIcon className="h-4 w-4" />
                  <span className="sr-only">Send</span>
                </Button>
              )}

              {inputMode === "control" && !disabled && (
                <div className="flex items-center gap-2">
                  {sentChars.map((char) => (
                    <span
                      key={char.id}
                      className="min-w-9 h-9 px-2 rounded-lg border border-amber-200 dark:border-amber-800 bg-amber-50 dark:bg-amber-900/20 font-mono font-medium text-xs flex items-center justify-center animate-pulse shadow-sm"
                    >
                      <Char char={char.char} />
                    </span>
                  ))}
                </div>
              )}
            </div>
          </div>
        </form>

        <div className="text-center mt-3">
          <span className="text-xs text-muted-foreground/80 bg-white dark:bg-gray-800 px-3 py-1 rounded-full border border-border/30">
            {inputMode === "text" ? (
              <>
                💬 Chat with your agent • Switch to <span className="font-medium text-amber-600 dark:text-amber-400">Control</span> mode for terminal commands
              </>
            ) : (
              <>🎛️ Terminal control mode • All keystrokes sent directly to agent</>
            )}
          </span>
        </div>
      </div>
    </Tabs>
  );
}

function Char({ char }: { char: string }) {
  switch (char) {
    case "ArrowUp":
      return <ArrowUpIcon className="h-4 w-4" />;
    case "ArrowDown":
      return <ArrowDownIcon className="h-4 w-4" />;
    case "ArrowRight":
      return <ArrowRightIcon className="h-4 w-4" />;
    case "ArrowLeft":
      return <ArrowLeftIcon className="h-4 w-4" />;
    case "⏎":
      return <CornerDownLeftIcon className="h-4 w-4" />;
    case "Backspace":
      return <DeleteIcon className="h-4 w-4" />;
    default:
      return char;
  }
}
