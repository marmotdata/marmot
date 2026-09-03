import React, { useEffect, useRef } from "react";
import { Icon } from "@iconify/react";
import { agents } from "./agents";

function MagnifyIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" className={className} fill="currentColor">
      <path d="M9.5 3A6.5 6.5 0 0 1 16 9.5c0 1.61-.59 3.09-1.56 4.23l.27.27h.79l5 5l-1.5 1.5l-5-5v-.79l-.27-.27A6.52 6.52 0 0 1 9.5 16A6.5 6.5 0 0 1 3 9.5A6.5 6.5 0 0 1 9.5 3m0 2C7 5 5 7 5 9.5S7 14 9.5 14S14 12 14 9.5S12 5 9.5 5" />
    </svg>
  );
}

function LineageIcon({ className }: { className?: string }) {
  return (
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" className={className} fill="none" stroke="currentColor" strokeWidth="2">
      <circle cx="5" cy="6" r="2.2" />
      <circle cx="5" cy="18" r="2.2" />
      <circle cx="19" cy="12" r="2.2" />
      <path d="M7 6.8 16.8 11.2 M7 17.2 16.8 12.8" />
    </svg>
  );
}

const iconMap: Record<string, React.FC<{ className?: string }>> = {
  "mdi:magnify": MagnifyIcon,
  "custom:lineage": LineageIcon,
};

interface ChatMessage {
  role: "user" | "assistant";
  text: string;
  tool?: { name: string; icon: string };
}

const messages: ChatMessage[] = [
  {
    role: "user",
    text: "Where do we hold customer PII?",
  },
  {
    role: "assistant",
    text: "Four assets are tagged pii: the customers, orders and support_tickets tables in Postgres, plus the user_events topic in Kafka. All four are owned by the Data Platform team.",
    tool: { name: "discover_data", icon: "mdi:magnify" },
  },
  {
    role: "user",
    text: "What breaks if we rename the order_gmv column?",
  },
  {
    role: "assistant",
    text: "The daily_orders_summary view and the Revenue Overview dashboard both depend on that column. The table is owned by the Data Platform team, so check with Sarah Chen first.",
    tool: {
      name: "trace_lineage",
      icon: "custom:lineage",
    },
  },
];

const TYPING_DELAY = 1200;
const USER_DELAY = 700;
const FIRST_DELAY = 600;

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
    <section className="py-20 sm:py-24 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900 border-t border-dashed border-earthy-brown-200/70 dark:border-gray-800/70">
      <div className="max-w-5xl mx-auto">
        <div
          data-animate
          className="flex flex-col lg:flex-row items-center gap-10 lg:gap-14"
        >
          {/* Left: copy */}
          <div className="lg:w-2/5 text-center lg:text-left">
            <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white mb-4 tracking-tight">
              Everyone answers their own questions
            </h2>
            <p className="text-lg text-gray-500 dark:text-gray-400 mb-4">
              Where a number came from. Whether a column is safe to change. Who
              owns this table. Questions that used to sit in the data team's
              queue. Marmot answers them without anyone needing to know where
              to look.
            </p>
            <p className="text-lg text-gray-500 dark:text-gray-400 mb-6">
              Same goes for agents. They are limited to whatever context you
              paste into the prompt. With Marmot they can go and find it
              themselves.
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

            <div className="mcp-clients">
              {agents.map((a) => (
                <span key={a.label} className="agent-mark" title={a.label}>
                  <Icon icon={a.icon} className="agent-mark-icon" aria-hidden="true" />
                </span>
              ))}
            </div>
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
