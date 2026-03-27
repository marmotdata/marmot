import React, { useEffect, useRef } from "react";

function MagnifyIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" className={className} fill="currentColor">
      <path d="M9.5 3A6.5 6.5 0 0 1 16 9.5c0 1.61-.59 3.09-1.56 4.23l.27.27h.79l5 5l-1.5 1.5l-5-5v-.79l-.27-.27A6.52 6.52 0 0 1 9.5 16A6.5 6.5 0 0 1 3 9.5A6.5 6.5 0 0 1 9.5 3m0 2C7 5 5 7 5 9.5S7 14 9.5 14S14 12 14 9.5S12 5 9.5 5" />
    </svg>
  );
}

function AccountSearchIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" className={className} fill="currentColor">
      <path d="M10 13c-.35.59-.64 1.24-.81 1.93C6.5 15.16 3.9 16.42 3.9 17v1.1h5.3c.17.68.45 1.32.8 1.9H2v-3c0-2.66 5.33-4 8-4m0-9a4 4 0 0 1 4 4c0 .91-.31 1.75-.82 2.43c-.86.32-1.63.83-2.27 1.47L10 12a4 4 0 0 1-4-4a4 4 0 0 1 4-4m0 1.9A2.1 2.1 0 0 0 7.9 8a2.1 2.1 0 0 0 2.1 2.1A2.1 2.1 0 0 0 12.1 8A2.1 2.1 0 0 0 10 5.9m5.5 6.1c2.5 0 4.5 2 4.5 4.5c0 .88-.25 1.71-.69 2.4l3.08 3.1L21 23.39l-3.12-3.07c-.69.43-1.51.68-2.38.68c-2.5 0-4.5-2-4.5-4.5s2-4.5 4.5-4.5m0 2a2.5 2.5 0 0 0-2.5 2.5a2.5 2.5 0 0 0 2.5 2.5a2.5 2.5 0 0 0 2.5-2.5a2.5 2.5 0 0 0-2.5-2.5" />
    </svg>
  );
}

function BookOpenIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" className={className} fill="currentColor">
      <path d="m19 1l-5 5v11l5-4.5zm2 4v13.5c-1.1-.35-2.3-.5-3.5-.5c-1.7 0-4.15.65-5.5 1.5V6c-1.45-1.1-3.55-1.5-5.5-1.5S2.45 4.9 1 6v14.65c0 .25.25.5.5.5c.1 0 .15-.05.25-.05C3.1 20.45 5.05 20 6.5 20c1.95 0 4.05.4 5.5 1.5c1.35-.85 3.8-1.5 5.5-1.5c1.65 0 3.35.3 4.75 1.05c.1.05.15.05.25.05c.25 0 .5-.25.5-.5V6c-.6-.45-1.25-.75-2-1M10 18.41C8.75 18.09 7.5 18 6.5 18c-1.06 0-2.32.19-3.5.5V7.13c.91-.4 2.14-.63 3.5-.63s2.59.23 3.5.63z" />
    </svg>
  );
}

const iconMap: Record<string, React.FC<{ className?: string }>> = {
  "mdi:magnify": MagnifyIcon,
  "mdi:account-search-outline": AccountSearchIcon,
  "mdi:book-open-page-variant-outline": BookOpenIcon,
};

interface ChatMessage {
  role: "user" | "assistant";
  text: string;
  tool?: { name: string; icon: string };
}

const messages: ChatMessage[] = [
  {
    role: "user",
    text: "What tables do we have related to customer orders?",
  },
  {
    role: "assistant",
    text: 'I found 3 assets matching "customer orders": the orders table in the warehouse, an orders_raw Kafka topic, and a daily_orders_summary view.',
    tool: { name: "discover_data", icon: "mdi:magnify" },
  },
  {
    role: "user",
    text: "Who owns the orders table?",
  },
  {
    role: "assistant",
    text: "The orders table is owned by the Data Platform team. Sarah Chen is the primary contact.",
    tool: {
      name: "find_ownership",
      icon: "mdi:account-search-outline",
    },
  },
  {
    role: "user",
    text: 'What does "GMV" mean in the order_gmv column?',
  },
  {
    role: "assistant",
    text: "GMV stands for Gross Merchandise Value — the total sales revenue before deductions.",
    tool: {
      name: "lookup_term",
      icon: "mdi:book-open-page-variant-outline",
    },
  },
];

const TYPING_DELAY = 1800;
const USER_DELAY = 1000;
const FIRST_DELAY = 800;

export default function MCPShowcase(): JSX.Element {
  const chatRef = useRef<HTMLDivElement>(null);
  const msgRefs = useRef<(HTMLDivElement | null)[]>([]);
  const timeoutsRef = useRef<ReturnType<typeof setTimeout>[]>([]);

  useEffect(() => {
    const chat = chatRef.current;
    if (!chat) return;

    // Hide messages now that JS is ready to animate them.
    // Without JS, messages stay visible (progressive enhancement).
    chat.classList.add("chat-animated");

    // Lock the container height so typing collapse doesn't shift layout
    const messagesEl = chat.querySelector(".chat-messages") as HTMLElement;
    if (messagesEl) {
      messagesEl.style.minHeight = messagesEl.offsetHeight + "px";
    }

    function runAnimation() {
      let elapsed = FIRST_DELAY;

      messages.forEach((msg, i) => {
        if (msg.role === "assistant") {
          // Show typing dots in-place
          const showTyping = setTimeout(() => {
            const el = msgRefs.current[i];
            if (el) el.classList.add("chat-msg-typing");
          }, elapsed);
          timeoutsRef.current.push(showTyping);
          elapsed += TYPING_DELAY;

          // Swap dots for content
          const showMsg = setTimeout(() => {
            const el = msgRefs.current[i];
            if (el) {
              el.classList.remove("chat-msg-typing");
              el.classList.add("chat-msg-visible");
            }
          }, elapsed);
          timeoutsRef.current.push(showMsg);
          elapsed += USER_DELAY;
        } else {
          const showMsg = setTimeout(() => {
            const el = msgRefs.current[i];
            if (el) el.classList.add("chat-msg-visible");
          }, elapsed);
          timeoutsRef.current.push(showMsg);
          elapsed += USER_DELAY;
        }
      });
    }

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            observer.disconnect();
            runAnimation();
            return;
          }
        }
      },
      { threshold: 0.3 },
    );
    observer.observe(chat);

    return () => {
      observer.disconnect();
      timeoutsRef.current.forEach(clearTimeout);
      timeoutsRef.current = [];
    };
  }, []);

  return (
    <section className="py-24 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-6xl mx-auto">
        <div
          data-animate
          className="flex flex-col lg:flex-row items-start gap-10 lg:gap-16"
        >
          {/* Left: copy */}
          <div className="lg:w-2/5 text-center lg:text-left">
            <h2 className="text-3xl sm:text-4xl font-extrabold text-gray-900 dark:text-white mb-4 tracking-tight">
              Built-in MCP server
            </h2>
            <p className="text-lg text-gray-500 dark:text-gray-400 mb-6">
              Ask your AI assistant about your business and get answers backed
              by your actual catalog.
            </p>
            <p className="text-sm text-gray-400 dark:text-gray-500 mb-6">
              Works with any MCP-compatible client.
            </p>
            <a
              href="/docs/MCP/"
              className="inline-flex items-center gap-1 text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:text-earthy-terracotta-800 dark:hover:text-earthy-terracotta-300 font-semibold transition-colors"
            >
              Set up MCP
              <svg
                className="w-4 h-4"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <path
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  strokeWidth={2}
                  d="M13 7l5 5m0 0l-5 5m5-5H6"
                />
              </svg>
            </a>
          </div>

          {/* Right: chat */}
          <div
            ref={chatRef}
            className="chat-window lg:w-3/5 w-full"
          >
            <div className="chat-messages px-4 py-6 flex flex-col gap-3">
              {messages.map((msg, i) => (
                <div
                  key={i}
                  ref={(el) => {
                    msgRefs.current[i] = el;
                  }}
                  className={`chat-msg ${msg.role === "user" ? "chat-msg-user" : "chat-msg-assistant"}`}
                >
                  {/* Typing dots (assistant only, shown during chat-msg-typing) */}
                  {msg.role === "assistant" && (
                    <div className="chat-dots">
                      <span className="typing-dot" />
                      <span className="typing-dot" />
                      <span className="typing-dot" />
                    </div>
                  )}
                  {/* Actual content (shown during chat-msg-visible) */}
                  <div className="chat-content">
                    {msg.tool && (
                      <div className="chat-tool-badge">
                        {(() => {
                          const IconComponent = iconMap[msg.tool.icon];
                          return IconComponent ? <IconComponent className="w-3.5 h-3.5" /> : null;
                        })()}
                        <span>{msg.tool.name}</span>
                      </div>
                    )}
                    <p className="text-sm leading-relaxed m-0">{msg.text}</p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
