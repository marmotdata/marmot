import React from "react";

export default function PerformanceProof(): JSX.Element {
  return (
    <section className="py-16 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-5xl mx-auto">
        <div data-animate className="text-center mb-8">
          <h2 className="text-3xl sm:text-4xl font-extrabold text-gray-900 dark:text-white mb-4 tracking-tight">
            Built to scale
          </h2>
          <p className="text-lg text-gray-500 dark:text-gray-400 max-w-2xl mx-auto">
            Simple architecture doesn't mean limited. Marmot handles real
            workloads on modest infrastructure.
          </p>
        </div>

        <div
          data-animate
          data-animate-delay="1"
          className="max-w-3xl mx-auto"
        >
          <div className="text-xs font-semibold uppercase tracking-widest text-earthy-terracotta-500 dark:text-earthy-terracotta-400 text-center mb-6">
            Load tested on real infrastructure
          </div>
          <div className="flex flex-col sm:flex-row items-center justify-center gap-6 sm:gap-14">
            <div className="text-center">
              <div className="text-5xl sm:text-6xl font-extrabold tracking-tight gradient-text">
                500k+
              </div>
              <div className="mt-2 text-sm font-medium text-gray-500 dark:text-gray-400">
                Assets
              </div>
            </div>
            <div className="hidden sm:block w-px h-16 bg-gray-200 dark:bg-gray-700" />
            <div className="text-center">
              <div className="text-5xl sm:text-6xl font-extrabold tracking-tight gradient-text">
                100+
              </div>
              <div className="mt-2 text-sm font-medium text-gray-500 dark:text-gray-400">
                Concurrent users
              </div>
            </div>
            <div className="hidden sm:block w-px h-16 bg-gray-200 dark:bg-gray-700" />
            <div className="text-center">
              <div className="text-5xl sm:text-6xl font-extrabold tracking-tight gradient-text">
                &lt;50ms
              </div>
              <div className="mt-2 text-sm font-medium text-gray-500 dark:text-gray-400">
                Avg response time
              </div>
            </div>
          </div>
          <div className="mt-6 text-center">
            <a
              href="/blog/postgres-one-database-to-rule-them-all"
              className="text-sm font-medium text-earthy-terracotta-600 dark:text-earthy-terracotta-400 hover:underline"
            >
              Read more
            </a>
          </div>
        </div>
      </div>
    </section>
  );
}
