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
    <section className="py-16 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-800 overflow-hidden">
      <div className="max-w-5xl mx-auto">
        <div data-animate className="text-center mb-12">
          <h2 className="text-3xl sm:text-4xl font-extrabold text-gray-900 dark:text-white mb-4 tracking-tight">
            Less infrastructure, same power
          </h2>
          <p className="text-lg text-gray-500 dark:text-gray-400 max-w-2xl mx-auto">
            Traditional data catalogs need an entire platform team. Marmot needs
            a database you probably already run.
          </p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 lg:gap-10 items-stretch">
          <div
            data-animate
            data-animate-delay="1"
            className="relative rounded-2xl p-6 pb-5 bg-gradient-to-br from-gray-50 to-gray-100 dark:from-gray-900 dark:to-gray-900/80 border border-gray-200/80 dark:border-gray-700/60"
          >
            <div className="flex items-center gap-2.5 mb-5">
              <div className="w-2.5 h-2.5 rounded-full bg-gray-300 dark:bg-gray-600" />
              <span className="text-xs font-semibold text-gray-400 dark:text-gray-500 uppercase tracking-widest">
                Traditional catalog
              </span>
            </div>

            <div className="space-y-4 mb-2">
              {traditionalRows.map((row, i) => (
                <div key={i} className="flex justify-around">
                  {row.map((svc) => (
                    <div key={svc.name} className="flex flex-col items-center">
                      <div className="w-10 h-10 rounded-xl bg-white dark:bg-gray-800 border border-gray-200 dark:border-gray-700 shadow-sm flex items-center justify-center mb-1.5">
                        <Icon icon={svc.icon} className="w-4 h-4 text-gray-400 dark:text-gray-500" />
                      </div>
                      <span className="text-[11px] font-medium text-gray-500 dark:text-gray-400">{svc.name}</span>
                      <span className="text-[9px] text-gray-400 dark:text-gray-600">{svc.role}</span>
                    </div>
                  ))}
                </div>
              ))}
            </div>

            <div className="mt-4 pt-4 border-t border-gray-200/60 dark:border-gray-700/40 flex items-center gap-5 text-xs text-gray-400 dark:text-gray-500">
              <span className="flex items-center gap-1.5">
                <Icon icon="mdi:server" className="w-3.5 h-3.5" />
                7+ services
              </span>
              <span className="flex items-center gap-1.5">
                <Icon icon="mdi:clock-outline" className="w-3.5 h-3.5" />
                Hours to deploy
              </span>
            </div>
          </div>

          <div
            data-animate
            data-animate-delay="2"
            className="relative rounded-2xl p-6 pb-5 arch-marmot-card flex flex-col"
          >
            <div className="absolute -inset-px rounded-2xl bg-gradient-to-br from-earthy-terracotta-400/10 via-transparent to-transparent pointer-events-none" />

            <div className="relative flex flex-col flex-1">
              <div className="flex items-center gap-2.5 mb-5">
                <div className="w-2.5 h-2.5 rounded-full bg-emerald-400 arch-pulse" />
                <span className="text-xs font-semibold text-earthy-terracotta-600 dark:text-earthy-terracotta-400 uppercase tracking-widest">
                  Marmot
                </span>
              </div>

              <div className="flex items-center justify-center flex-1 gap-6">
                <div className="flex flex-col items-center">
                  <div className="w-20 h-20 rounded-2xl bg-white dark:bg-gray-800 border border-earthy-terracotta-200/50 dark:border-earthy-terracotta-800/30 shadow-lg shadow-earthy-terracotta-500/5 flex items-center justify-center mb-3">
                    <img
                      src="/img/marmot.svg"
                      alt="Marmot"
                      className="w-10 h-10"
                    />
                  </div>
                  <span className="text-sm font-semibold text-gray-800 dark:text-gray-200">
                    Marmot
                  </span>
                  <span className="text-[10px] text-gray-400 dark:text-gray-500 mt-0.5">
                    Single binary
                  </span>
                </div>

                <div className="w-16 h-px bg-gradient-to-r from-earthy-terracotta-300/60 via-earthy-terracotta-400/40 to-earthy-terracotta-300/60 dark:from-earthy-terracotta-600/40 dark:via-earthy-terracotta-500/30 dark:to-earthy-terracotta-600/40" />

                <div className="flex flex-col items-center">
                  <div className="w-20 h-20 rounded-2xl bg-white dark:bg-gray-800 border border-earthy-terracotta-200/50 dark:border-earthy-terracotta-800/30 shadow-lg shadow-earthy-terracotta-500/5 flex items-center justify-center mb-3">
                    <Icon
                      icon="simple-icons:postgresql"
                      className="w-10 h-10 text-[#336791]"
                    />
                  </div>
                  <span className="text-sm font-semibold text-gray-800 dark:text-gray-200">
                    PostgreSQL
                  </span>
                  <span className="text-[10px] text-gray-400 dark:text-gray-500 mt-0.5">
                    Search, storage & graphs
                  </span>
                </div>
              </div>

              <div className="mt-6 pt-4 border-t border-earthy-terracotta-200/30 dark:border-earthy-terracotta-800/20 flex items-center gap-5 text-xs text-gray-400 dark:text-gray-500">
                <span className="flex items-center gap-1.5">
                  <Icon icon="mdi:server" className="w-3.5 h-3.5" />
                  2 services
                </span>
                <span className="flex items-center gap-1.5">
                  <Icon icon="mdi:clock-outline" className="w-3.5 h-3.5" />
                  Minutes to deploy
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
