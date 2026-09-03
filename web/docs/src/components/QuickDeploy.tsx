import React from "react";
import { Icon } from "@iconify/react";

const scale = [
  { value: "500k+", label: "Assets" },
  { value: "100+", label: "Concurrent users" },
  { value: "<50ms", label: "Avg response time" },
];

export default function QuickDeploy(): JSX.Element {
  return (
    <section className="py-20 sm:py-24 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900 border-t border-dashed border-earthy-brown-200/70 dark:border-gray-800/70">
      <div className="max-w-5xl mx-auto">
        <div
          data-animate
          className="flex flex-col lg:flex-row-reverse items-center gap-10 lg:gap-14"
        >
          <div className="lg:w-1/2 text-center lg:text-left">
            <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white mb-4 tracking-tight">
              Up in minutes
            </h2>
            <p className="text-lg text-gray-500 dark:text-gray-400 mb-6">
              Marmot is one service and a Postgres database you already run.
              Most catalog projects spend their first quarter standing up
              infrastructure. Marmot spends it on your data.
            </p>
            <a
              href="/docs/quick-start"
              className="inline-flex items-center gap-1 text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:text-earthy-terracotta-800 dark:hover:text-earthy-terracotta-300 font-semibold transition-colors"
            >
              Quick start
              <Icon icon="mdi:arrow-right" className="w-4 h-4" />
            </a>
          </div>

          <div data-animate data-animate-delay="1" className="lg:w-1/2 w-full">
            <div className="quick-deploy-terminal rounded-2xl overflow-hidden shadow-2xl shadow-black/20 dark:shadow-black/50">
              <div className="flex items-center justify-between px-5 py-3.5 bg-[#1c1c1e] dark:bg-[#111113] border-b border-white/5">
                <div className="flex gap-2">
                  <div className="w-3 h-3 rounded-full bg-[#ff5f57]" />
                  <div className="w-3 h-3 rounded-full bg-[#febc2e]" />
                  <div className="w-3 h-3 rounded-full bg-[#28c840]" />
                </div>
                <span className="text-[11px] text-gray-500 terminal-mono tracking-wide">
                  ~/marmot
                </span>
                <div className="w-[52px]" />
              </div>

              <div className="bg-[#1c1c1e] dark:bg-[#111113] px-6 py-5 terminal-mono text-[15px] leading-relaxed">
                <div className="flex items-center gap-2">
                  <span className="text-emerald-400 select-none">$</span>
                  <span className="text-gray-200">docker compose up -d</span>
                </div>

                <div className="mt-4 space-y-1 text-[13.5px] text-gray-500">
                  <div>
                    <span className="text-gray-600">[+]</span> Container
                    postgres-1{" "}
                    <span className="text-emerald-500/70">Started</span>
                  </div>
                  <div>
                    <span className="text-gray-600">[+]</span> Container
                    marmot-1{" "}
                    <span className="text-emerald-500/70">Started</span>
                  </div>
                </div>

                <div className="pt-3 mt-3 border-t border-white/5">
                  <div className="flex items-center gap-2">
                    <Icon
                      icon="mdi:check-circle"
                      className="w-4 h-4 text-emerald-400"
                    />
                    <span className="text-emerald-400/90 text-[13.5px]">
                      Marmot is running at{" "}
                      <span className="underline decoration-emerald-400/30">
                        http://localhost:8080
                      </span>
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>

        <div data-animate data-animate-delay="2" className="section-divider mt-14" />

        <p
          data-animate
          data-animate-delay="2"
          className="mt-8 text-center text-sm text-gray-500 dark:text-gray-400"
        >
          Load tested with a single Postgres and a single Marmot process:
        </p>

        <div
          data-animate
          data-animate-delay="3"
          className="mt-5 flex flex-col sm:flex-row items-center justify-center gap-6 sm:gap-12"
        >
          {scale.map((s) => (
            <div key={s.label} className="flex items-baseline gap-2">
              <span className="text-xl font-bold text-gray-900 dark:text-white tracking-tight">
                {s.value}
              </span>
              <span className="text-sm text-gray-500 dark:text-gray-400">
                {s.label}
              </span>
            </div>
          ))}
          <a
            href="/blog/postgres-one-database-to-rule-them-all"
            className="text-sm text-earthy-terracotta-600 dark:text-earthy-terracotta-400 hover:underline"
          >
            How we load tested it
          </a>
        </div>
      </div>
    </section>
  );
}
