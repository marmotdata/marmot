import React from "react";

export default function Hero(): JSX.Element {
  return (
    <header className="pt-20 pb-16 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-7xl mx-auto text-center">
        <img
          src="/img/marmot.svg"
          alt="Marmot logo"
          className="max-w-[10rem] mx-auto mb-6"
        />

        <h1 className="text-5xl sm:text-6xl font-bold text-gray-900 dark:text-white mb-6 tracking-tight">
          Discover any data asset
          <br />
          across your entire org in{" "}
          <span className="text-earthy-terracotta-700 dark:text-earthy-terracotta-500">
            seconds
          </span>
        </h1>

        <p className="text-xl text-gray-600 dark:text-gray-300 max-w-2xl mx-auto mb-10">
          The open-source data catalog for modern teams. Search tables, topics,
          queues, buckets, APIs, and more — all in one place.
        </p>

        <div className="flex flex-col sm:flex-row items-center justify-center gap-4">
          <span className="sparkle-container">
            <a
              href="/docs/introduction"
              className="inline-flex items-center justify-center w-full sm:w-auto px-7 py-3.5 border border-transparent text-base font-semibold rounded-lg text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 shadow-lg hover:shadow-xl transition-all"
            >
              Get Started
            </a>
            <span className="sparkle sparkle-1"></span>
            <span className="sparkle sparkle-2"></span>
            <span className="sparkle sparkle-3"></span>
          </span>
          <a
            href="https://demo.marmotdata.io"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center justify-center w-full sm:w-auto px-7 py-3.5 border border-earthy-terracotta-300 dark:border-earthy-terracotta-700 text-base font-semibold rounded-lg text-earthy-terracotta-700 dark:text-earthy-terracotta-400 bg-white dark:bg-gray-800 hover:bg-earthy-terracotta-50 dark:hover:bg-gray-700 transition-all"
          >
            <svg
              className="w-5 h-5 mr-2"
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
      </div>
    </header>
  );
}
