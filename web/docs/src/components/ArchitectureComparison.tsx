import React from "react";
import { Icon } from "@iconify/react";

const traditionalRows = [
  [
    { name: "Elasticsearch", icon: "simple-icons:elasticsearch", role: "Search" },
    { name: "Kafka", icon: "simple-icons:apachekafka", role: "Events" },
  ],
  [
    { name: "Frontend", icon: "mdi:application-outline", role: "UI" },
    { name: "API", icon: "mdi:api", role: "Backend" },
    { name: "Neo4j", icon: "simple-icons:neo4j", role: "Graph" },
  ],
  [
    { name: "MySQL", icon: "simple-icons:mysql", role: "Metadata" },
    { name: "Airflow", icon: "simple-icons:apacheairflow", role: "Orchestration" },
  ],
];

export default function ArchitectureComparison(): JSX.Element {
  return (
    <section className="py-24 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-800">
      <div className="max-w-6xl mx-auto">
        <div
          data-animate
          className="flex flex-col-reverse lg:flex-row items-start gap-10 lg:gap-16"
        >
          {/* Left: comparison cards */}
          <div className="lg:w-3/5 w-full flex flex-col gap-3">
            <div
              data-animate
              data-animate-delay="1"
              className="relative rounded-xl p-4 pb-3 bg-gradient-to-br from-gray-50 to-gray-100 dark:from-gray-900 dark:to-gray-900/80 border border-gray-200/80 dark:border-gray-700/60"
            >
              <div className="flex items-center gap-2 mb-3">
                <div className="w-2 h-2 rounded-full bg-gray-300 dark:bg-gray-600" />
                <span className="text-[10px] font-semibold text-gray-400 dark:text-gray-500 uppercase tracking-widest">
                  Traditional catalog
                </span>
              </div>

              <div className="space-y-2">
                {traditionalRows.map((row, i) => (
                  <div key={i} className="flex justify-around">
                    {row.map((svc) => (
                      <div key={svc.name} className="flex flex-col items-center">
                        <div className="w-8 h-8 rounded-lg bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 shadow-sm flex items-center justify-center mb-1">
                          <Icon icon={svc.icon} className="w-3.5 h-3.5 text-gray-400 dark:text-gray-500" />
                        </div>
                        <span className="text-[10px] font-medium text-gray-500 dark:text-gray-400">{svc.name}</span>
                        <span className="text-[8px] text-gray-400 dark:text-gray-600">{svc.role}</span>
                      </div>
                    ))}
                  </div>
                ))}
              </div>

              <div className="mt-2.5 pt-2.5 border-t border-gray-200/60 dark:border-gray-700/40 flex items-center gap-4 text-[11px] text-gray-400 dark:text-gray-500">
                <span className="flex items-center gap-1">
                  <Icon icon="mdi:server" className="w-3 h-3" />
                  7+ services
                </span>
                <span className="flex items-center gap-1">
                  <Icon icon="mdi:clock-outline" className="w-3 h-3" />
                  Hours to deploy
                </span>
              </div>
            </div>

            <div
              data-animate
              data-animate-delay="2"
              className="relative rounded-xl p-4 pb-3 arch-marmot-card flex flex-col"
            >
              <div className="absolute -inset-px rounded-xl bg-gradient-to-br from-earthy-terracotta-400/8 via-transparent to-transparent pointer-events-none" />

              <div className="relative flex flex-col flex-1">
                <div className="flex items-center gap-2 mb-3">
                  <div className="w-2 h-2 rounded-full bg-emerald-400 arch-pulse" />
                  <span className="text-[10px] font-semibold text-earthy-terracotta-600 dark:text-earthy-terracotta-400 uppercase tracking-widest">
                    Marmot
                  </span>
                </div>

                <div className="flex items-center justify-center flex-1 gap-5">
                  <div className="flex flex-col items-center">
                    <div className="w-12 h-12 rounded-xl bg-white dark:bg-gray-800 border border-earthy-terracotta-200/50 dark:border-earthy-terracotta-800/30 shadow-md shadow-earthy-terracotta-500/5 flex items-center justify-center mb-1.5">
                      <img
                        src="/img/marmot.svg"
                        alt="Marmot"
                        className="w-6 h-6"
                      />
                    </div>
                    <span className="text-xs font-semibold text-gray-800 dark:text-gray-200">
                      Marmot
                    </span>
                    <span className="text-[9px] text-gray-400 dark:text-gray-500 mt-0.5">
                      Single binary
                    </span>
                  </div>

                  <div className="w-10 h-px bg-gradient-to-r from-earthy-terracotta-300/60 via-earthy-terracotta-400/40 to-earthy-terracotta-300/60 dark:from-earthy-terracotta-600/40 dark:via-earthy-terracotta-500/30 dark:to-earthy-terracotta-600/40" />

                  <div className="flex flex-col items-center">
                    <div className="w-12 h-12 rounded-xl bg-white dark:bg-gray-800 border border-earthy-terracotta-200/50 dark:border-earthy-terracotta-800/30 shadow-md shadow-earthy-terracotta-500/5 flex items-center justify-center mb-1.5">
                      <Icon
                        icon="simple-icons:postgresql"
                        className="w-6 h-6 text-[#336791]"
                      />
                    </div>
                    <span className="text-xs font-semibold text-gray-800 dark:text-gray-200">
                      PostgreSQL
                    </span>
                    <span className="text-[9px] text-gray-400 dark:text-gray-500 mt-0.5">
                      Search, storage & graphs
                    </span>
                  </div>
                </div>

                <div className="mt-3 pt-2.5 border-t border-earthy-terracotta-200/30 dark:border-earthy-terracotta-800/20 flex items-center gap-4 text-[11px] text-gray-400 dark:text-gray-500">
                  <span className="flex items-center gap-1">
                    <Icon icon="mdi:server" className="w-3 h-3" />
                    2 services
                  </span>
                  <span className="flex items-center gap-1">
                    <Icon icon="mdi:clock-outline" className="w-3 h-3" />
                    Minutes to deploy
                  </span>
                </div>
              </div>
            </div>
          </div>

          {/* Right: copy */}
          <div className="lg:w-2/5 text-center lg:text-left">
            <h2 className="text-3xl sm:text-4xl font-extrabold text-gray-900 dark:text-white mb-4 tracking-tight">
              Less infrastructure, same power
            </h2>
            <p className="text-lg text-gray-500 dark:text-gray-400 mb-6">
              Traditional data catalogs need an entire platform team. Marmot
              needs a database you probably already run.
            </p>
            <a
              href="/docs/Quick%20Start/"
              className="inline-flex items-center gap-1 text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:text-earthy-terracotta-800 dark:hover:text-earthy-terracotta-300 font-semibold transition-colors"
            >
              Quick start
              <svg
                className="w-4 h-4"
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
        </div>
      </div>
    </section>
  );
}
