import React from "react";
import { Icon } from "@iconify/react";

export default function GetStarted(): JSX.Element {
  return (
    <section className="py-16 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-7xl mx-auto">
        <div className="rounded-lg overflow-hidden shadow-xl bg-gradient-to-r from-earthy-terracotta-700/80 to-earthy-terracotta-500/80 dark:from-earthy-terracotta-800/80 dark:to-earthy-terracotta-600/80">
          <div className="px-8 py-12 md:p-12 lg:p-16">
            <div className="md:flex md:items-center md:justify-between">
              <div className="md:max-w-2xl">
                <h2 className="text-3xl font-bold text-white">
                  Ready to discover your data landscape?
                </h2>
                <p className="mt-4 text-lg text-white text-opacity-90">
                  Get started with Marmot in minutes. The documentation covers
                  everything from quick installation to advanced integration
                  patterns.
                </p>
                <div className="mt-8">
                  <a
                    href="/docs/introduction"
                    className="inline-flex items-center px-6 py-3 border border-transparent text-base font-medium rounded-md shadow-sm bg-white text-earthy-terracotta-700 hover:bg-gray-50 transition-colors duration-150"
                  >
                    Explore the docs
                    <Icon icon="mdi:arrow-right" className="ml-2 text-xl" />
                  </a>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
