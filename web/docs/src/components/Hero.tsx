import React from "react";
import { Icon } from "@iconify/react";

const assistants = [
  { label: "Claude", icon: "simple-icons:anthropic" },
  { label: "Cursor", icon: "simple-icons:cursor" },
  { label: "Copilot", icon: "simple-icons:githubcopilot" },
  { label: "Gemini", icon: "simple-icons:googlegemini" },
  { label: "ChatGPT", icon: "simple-icons:openai" },
  { label: "Windsurf", icon: "simple-icons:codeium" },
];

export default function Hero(): JSX.Element {
  return (
    <header className="relative pt-28 pb-20 sm:pt-32 sm:pb-24 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900 gradient-mesh-hero overflow-hidden">
      <div className="relative max-w-4xl mx-auto text-center">
        <h1
          data-animate
          className="text-4xl sm:text-5xl lg:text-6xl font-extrabold text-gray-900 dark:text-white tracking-tight leading-[1.05]"
        >
          {/* Inline on small screens so the line wraps naturally; two
              deliberate lines once there is room for them. */}
          <span className="sm:block">AI agents are only as good as</span>{" "}
          <span className="gradient-text sm:block">
            the context they can reach.
          </span>
        </h1>

        <p
          data-animate
          data-animate-delay="2"
          className="mt-6 max-w-2xl mx-auto text-lg sm:text-xl text-gray-500 dark:text-gray-400 leading-relaxed"
        >
          Marmot is the open source context layer. It catalogs what you run,
          what it means and who owns it, then serves that to your team in the
          UI and to every assistant over one MCP server.
        </p>

        <div
          data-animate
          data-animate-delay="3"
          className="mt-9 flex flex-row items-center justify-center gap-3"
        >
          <a
            href="/docs/introduction"
            className="group inline-flex items-center justify-center px-7 py-3.5 text-sm font-semibold rounded-xl text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 shadow-sm hover:shadow-md transition-all duration-200 hover:-translate-y-0.5"
          >
            Get started
            <svg
              className="w-4 h-4 ml-2 transition-transform duration-200 group-hover:translate-x-0.5"
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
          <a
            href="https://demo.marmotdata.io"
            target="_blank"
            rel="noopener noreferrer"
            className="group demo-btn inline-flex items-center justify-center px-7 py-3.5 text-sm font-semibold rounded-xl text-gray-700 dark:text-gray-300 bg-white/70 dark:bg-gray-800/50 transition-all duration-200 hover:-translate-y-0.5"
          >
            Live demo
          </a>
        </div>
      </div>

      <div
        data-animate
        data-animate-delay="4"
        className="relative max-w-3xl mx-auto mt-16 sm:mt-20"
      >
        <p className="text-[10px] font-semibold uppercase tracking-[0.18em] text-gray-400 dark:text-gray-500 text-center mb-5">
          Works with
        </p>
        <div className="flex flex-wrap items-center justify-center gap-x-8 gap-y-4">
          {assistants.map((a) => (
            <div
              key={a.label}
              className="flex items-center gap-2 text-gray-400 dark:text-gray-500"
            >
              <Icon icon={a.icon} className="w-5 h-5" />
              <span className="text-sm font-medium">{a.label}</span>
            </div>
          ))}
        </div>
      </div>
    </header>
  );
}
