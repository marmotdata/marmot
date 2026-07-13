import React from "react";

export default function CTA(): JSX.Element {
  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-900">
      <div className="max-w-2xl mx-auto text-center">
        <div className="w-12 h-px bg-earthy-terracotta-300 dark:bg-earthy-terracotta-600 mx-auto mb-10" />

        <h2
          data-animate
          className="text-2xl sm:text-3xl font-bold text-gray-900 dark:text-white mb-3 tracking-tight"
        >
          One context for people and agents.
        </h2>
        <p
          data-animate
          data-animate-delay="1"
          className="text-base text-gray-500 dark:text-gray-400 mb-8 leading-relaxed"
        >
          Giving your employees and agents a context layer is a huge boost in
          autonomy and productivity. Explore the live demo, deploy it for
          free, or talk to us when you're ready to scale your context layer.
        </p>

        <div
          data-animate
          data-animate-delay="2"
          className="flex flex-row justify-center items-center gap-4"
        >
          <a
            href="https://demo.marmotdata.io"
            target="_blank"
            rel="noopener noreferrer"
            className="group inline-flex items-center justify-center px-6 py-3 text-sm font-semibold rounded-lg text-white bg-earthy-terracotta-600 hover:bg-earthy-terracotta-700 shadow-sm hover:shadow-md transition-all duration-200"
          >
            View Live Demo
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
            href="mailto:support@marmotdata.io"
            className="group demo-btn inline-flex items-center justify-center px-6 py-3 text-sm font-semibold rounded-lg text-gray-600 dark:text-gray-300 hover:text-gray-900 dark:hover:text-white transition-all duration-200"
          >
            Talk to Us
            <svg
              className="w-4 h-4 ml-1.5 transition-transform duration-200 group-hover:translate-x-0.5"
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

        <p
          data-animate
          data-animate-delay="3"
          className="mt-6 text-sm text-gray-400 dark:text-gray-500"
        >
          Prefer to explore first?{" "}
          <a
            href="/docs/introduction"
            className="text-earthy-terracotta-600 dark:text-earthy-terracotta-400 hover:underline"
          >
            Read the docs
          </a>
        </p>
      </div>
    </section>
  );
}
