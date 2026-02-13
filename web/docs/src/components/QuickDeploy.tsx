import React from "react";
import { Icon } from "@iconify/react";

export default function QuickDeploy(): JSX.Element {
  return (
    <section className="py-24 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-6xl mx-auto">
        <div
          data-animate
          className="flex flex-col lg:flex-row items-center gap-10 lg:gap-16"
        >
          <div className="lg:w-2/5 text-center lg:text-left">
            <h2 className="text-3xl sm:text-4xl font-extrabold text-gray-900 dark:text-white mb-4 tracking-tight">
              Deploy in under five minutes
            </h2>
            <p className="text-lg text-gray-500 dark:text-gray-400 mb-6">
              Marmot runs as a single binary backed by PostgreSQL - the only
              dependency you need to start cataloging your data.
            </p>
            <a
              href="/docs/quick-start"
              className="inline-flex items-center gap-2 text-sm font-medium text-earthy-terracotta-600 dark:text-earthy-terracotta-400 hover:underline"
            >
              Follow the Quick Start guide
              <Icon icon="mdi:arrow-right" className="w-4 h-4" />
            </a>
          </div>

          <div data-animate data-animate-delay="1" className="lg:w-3/5 w-full">
            <div className="quick-deploy-terminal rounded-2xl overflow-hidden shadow-2xl shadow-black/20 dark:shadow-black/50">
              <div className="flex items-center justify-between px-5 py-3.5 bg-[#1c1c1e] dark:bg-[#111113] border-b border-white/5">
                <div className="flex gap-2">
                  <div className="w-3 h-3 rounded-full bg-[#ff5f57]" />
                  <div className="w-3 h-3 rounded-full bg-[#febc2e]" />
                  <div className="w-3 h-3 rounded-full bg-[#28c840]" />
                </div>
                <span className="text-[11px] text-gray-500 font-mono tracking-wide">
                  ~ / marmot
                </span>
                <div className="w-[52px]" />
              </div>

              <div className="bg-[#1c1c1e] dark:bg-[#111113] px-6 py-5 font-mono text-[13px] leading-relaxed">
                <div className="flex items-center gap-2">
                  <span className="text-emerald-400 select-none">$</span>
                  <span className="text-gray-200">docker compose up -d</span>
                </div>

                <div className="mt-4 space-y-1 text-xs text-gray-500">
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
                    <span className="text-emerald-400/90 text-xs">
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
      </div>
    </section>
  );
}
