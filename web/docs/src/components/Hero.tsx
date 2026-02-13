import React from "react";
import { Icon } from "@iconify/react";

const stats = [
  { value: "500+", label: "GitHub Stars", icon: "mdi:star" },
  { value: "6k+", label: "Downloads", icon: "mdi:download" },
  { value: "20+", label: "Integrations", icon: "mdi:puzzle" },
  { value: "MIT", label: "Licensed", icon: "mdi:license" },
];

export default function Hero(): JSX.Element {
  return (
    <header className="relative pt-8 pb-20 sm:pt-12 sm:pb-28 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900 gradient-mesh-hero overflow-hidden">
      <div className="absolute inset-0 dot-pattern opacity-40 pointer-events-none" />

      <div className="relative max-w-5xl mx-auto text-center">
        <div data-animate data-animate-delay="1">
          <img
            src="/img/marmot.svg"
            alt="Marmot logo"
            className="max-w-[8rem] sm:max-w-[9rem] mx-auto mb-8 animate-float drop-shadow-lg"
          />
        </div>

        <h1
          data-animate
          data-animate-delay="2"
          className="text-4xl sm:text-5xl lg:text-6xl font-extrabold text-gray-900 dark:text-white mb-6 tracking-tight leading-[1.1]"
        >
          Discover any data asset
          <br />
          across your entire org in{" "}
          <span className="gradient-text">seconds</span>
        </h1>

        <p
          data-animate
          data-animate-delay="3"
          className="text-lg sm:text-xl text-gray-500 dark:text-gray-400 max-w-2xl mx-auto mb-12 leading-relaxed"
        >
          The open-source data catalog for modern teams. Search tables, topics,
          queues, buckets, APIs, and more - all in one place.
        </p>

        <div
          data-animate
          data-animate-delay="4"
          className="flex flex-col sm:flex-row items-center justify-center gap-4"
        >
          <span className="sparkle-container">
            <a
              href="/docs/introduction"
              className="group inline-flex items-center justify-center w-full sm:w-auto px-8 py-4 text-base font-semibold rounded-xl text-white bg-gradient-to-r from-earthy-terracotta-700 to-earthy-terracotta-600 hover:from-earthy-terracotta-800 hover:to-earthy-terracotta-700 shadow-lg shadow-earthy-terracotta-700/20 hover:shadow-xl hover:shadow-earthy-terracotta-700/30 transition-all duration-300 hover:-translate-y-0.5"
            >
              Get Started
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
            <span className="sparkle sparkle-1"></span>
            <span className="sparkle sparkle-2"></span>
            <span className="sparkle sparkle-3"></span>
          </span>
          <a
            href="https://demo.marmotdata.io"
            target="_blank"
            rel="noopener noreferrer"
            className="group inline-flex items-center justify-center w-full sm:w-auto px-8 py-4 text-base font-semibold rounded-xl text-earthy-terracotta-700 dark:text-earthy-terracotta-400 bg-white/60 dark:bg-gray-800/50 backdrop-blur-md border border-white/40 dark:border-white/10 hover:bg-white/80 dark:hover:bg-gray-800/80 transition-all duration-300 hover:-translate-y-0.5"
          >
            <svg
              className="w-5 h-5 mr-2 transition-transform duration-200 group-hover:scale-110"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M15 15l-2 5L9 9l11 4-5 2zm0 0l5 5M7.188 2.239l.777 2.897M5.136 7.965l-2.898-.777M13.95 4.05l-2.122 2.122m-5.657 5.656l-2.12 2.122"
              />
            </svg>
            Live Demo
          </a>
        </div>

        <div
          data-animate
          data-animate-delay="5"
          className="mt-16 flex flex-wrap items-center justify-center divide-x divide-gray-300/30 dark:divide-gray-700"
        >
          {stats.map((stat, index) => (
            <div key={index} className="px-6 sm:px-10 py-2 text-center">
              <div className="flex items-center justify-center gap-2 mb-1">
                <Icon
                  icon={stat.icon}
                  className="w-4 h-4 text-gray-300 dark:text-gray-600"
                />
                <span className="text-2xl font-bold text-gray-400 dark:text-gray-500 tracking-tight">
                  {stat.value}
                </span>
              </div>
              <span className="text-xs uppercase tracking-wider font-medium text-gray-400 dark:text-gray-500">
                {stat.label}
              </span>
            </div>
          ))}
        </div>
      </div>
    </header>
  );
}
