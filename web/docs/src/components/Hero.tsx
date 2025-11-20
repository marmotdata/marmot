import React from "react";

export default function Hero(): JSX.Element {
  return (
    <header className="py-16 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-7xl mx-auto text-center">
        <img
          src="/img/marmot.svg"
          alt="Marmot logo"
          className="max-w-[12rem] mx-auto mb-6"
        />

        <h1 className="text-5xl font-bold text-gray-900 dark:text-white mb-6 tracking-tight">
          Discover any data asset
          <br />
          across your entire org in{" "}
          <span className="text-earthy-terracotta-700 dark:text-earthy-terracotta-600">seconds</span>
        </h1>

        <p className="text-xl text-gray-600 dark:text-gray-300 max-w-3xl mx-auto">
          Open-source catalog for all your data assets. Search everything -
          tables, topics, queues, buckets, and more.
        </p>

        <div className="mt-8 flex justify-center gap-4">
          <span className="sparkle-container">
            <a
              href="/docs/introduction"
              className="inline-flex items-center px-6 py-3 border border-transparent text-base font-medium rounded-md text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800"
            >
              Get Started
            </a>
            <span className="sparkle sparkle-1"></span>
            <span className="sparkle sparkle-2"></span>
            <span className="sparkle sparkle-3"></span>
          </span>
          <a
            href="https://github.com/marmotdata/marmot"
            className="inline-flex items-center px-6 py-3 border border-gray-300 dark:border-gray-600 text-base font-medium rounded-md text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-800 hover:bg-gray-50 dark:hover:bg-gray-700"
            target="_blank"
            rel="noopener noreferrer"
          >
            <svg
              className="w-5 h-5 mr-2"
              fill="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                fillRule="evenodd"
                d="M12 2C6.477 2 2 6.484 2 12.017c0 4.425 2.865 8.18 6.839 9.504.5.092.682-.217.682-.483 0-.237-.008-.868-.013-1.703-2.782.605-3.369-1.343-3.369-1.343-.454-1.158-1.11-1.466-1.11-1.466-.908-.62.069-.608.069-.608 1.003.07 1.531 1.032 1.531 1.032.892 1.53 2.341 1.088 2.91.832.092-.647.35-1.088.636-1.338-2.22-.253-4.555-1.113-4.555-4.951 0-1.093.39-1.988 1.029-2.688-.103-.253-.446-1.272.098-2.65 0 0 .84-.27 2.75 1.026A9.564 9.564 0 0112 6.844c.85.004 1.705.115 2.504.337 1.909-1.296 2.747-1.027 2.747-1.027.546 1.379.202 2.398.1 2.651.64.7 1.028 1.595 1.028 2.688 0 3.848-2.339 4.695-4.566 4.943.359.309.678.92.678 1.855 0 1.338-.012 2.419-.012 2.747 0 .268.18.58.688.482A10.019 10.019 0 0022 12.017C22 6.484 17.522 2 12 2z"
                clipRule="evenodd"
              />
            </svg>
            View on GitHub
          </a>
        </div>
      </div>
    </header>
  );
}
