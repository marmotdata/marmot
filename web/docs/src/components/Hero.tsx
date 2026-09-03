import React from "react";
import ContextDiagram from "./ContextDiagram";

export default function Hero(): JSX.Element {
  return (
    <header className="relative pt-32 pb-8 sm:pt-40 sm:pb-10 lg:pt-44 lg:pb-12 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900 hero-glow overflow-hidden">
      <div className="relative max-w-6xl mx-auto text-center">
        <h1
          data-animate
          className="text-4xl sm:text-5xl md:text-6xl lg:text-7xl font-extrabold text-gray-900 dark:text-white tracking-tight leading-[1.05]"
          style={{ textWrap: "balance" }}
        >
          AI agents are only as good as{" "}
          <span className="gradient-text">the context they can reach.</span>
        </h1>

        <p
          data-animate
          data-animate-delay="2"
          className="mt-8 text-lg sm:text-xl text-gray-500 dark:text-gray-400 max-w-3xl mx-auto leading-relaxed"
        >
          Marmot is an open source context layer for engineers and AI agents.
          It tracks schemas, ownership and lineage across your entire stack.
        </p>

        <div
          data-animate
          data-animate-delay="3"
          className="mt-10 flex flex-row items-center justify-center gap-3"
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
            href="https://github.com/marmotdata/marmot"
            target="_blank"
            rel="noopener noreferrer"
            className="group inline-flex items-center justify-center px-7 py-3.5 text-sm font-semibold rounded-xl text-gray-700 dark:text-gray-300 bg-white/70 dark:bg-gray-800/50 border border-earthy-brown-200/70 dark:border-gray-700/70 transition-all duration-200 hover:-translate-y-0.5"
          >
            View on GitHub
          </a>
        </div>
      </div>

      {/* The figure is part of the same header unit: the headline makes the
          claim, the figure is the evidence. */}
      <div className="relative mt-12">
        <ContextDiagram />
      </div>
    </header>
  );
}
