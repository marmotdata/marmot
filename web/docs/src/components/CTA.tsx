import React from "react";

export default function CTA(): JSX.Element {
  return (
    <section className="py-16 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900 border-t border-gray-200 dark:border-gray-700">
      <div className="max-w-4xl mx-auto text-center">
        <h2 className="text-3xl font-bold text-gray-900 dark:text-white mb-4">
          Ready to get started?
        </h2>
        <p className="text-lg text-gray-600 dark:text-gray-300 mb-8">
          Try the live demo or explore the open source, self-hostable solution.
          MIT licensed with a flexible API.
        </p>
        <div className="flex flex-col sm:flex-row justify-center gap-4">
          <span className="sparkle-container">
            <a
              href="https://demo.marmotdata.io"
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center justify-center px-8 py-3 text-base font-semibold rounded-lg text-white bg-amber-600 hover:bg-amber-700 shadow-lg hover:shadow-xl transition-all"
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
              View Live Demo
            </a>
            <span className="sparkle sparkle-1"></span>
            <span className="sparkle sparkle-2"></span>
            <span className="sparkle sparkle-3"></span>
          </span>
          <a
            href="/docs/introduction"
            className="inline-flex items-center justify-center px-8 py-3 text-base font-semibold rounded-lg text-gray-700 dark:text-gray-200 bg-white dark:bg-gray-800 border border-gray-300 dark:border-gray-600 hover:bg-gray-50 dark:hover:bg-gray-700 transition-all"
          >
            Read the Docs
          </a>
        </div>
      </div>
    </section>
  );
}
