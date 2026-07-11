import React from "react";
import { Icon } from "@iconify/react";

const todayItems = [
  "Answers live in people's heads, Slack threads and stale wiki pages",
  "Schemas pasted into prompts that quietly go out of date",
  "Nothing tells the AI what GMV means or which orders table is the real one, so it guesses",
  "Every team wires up the same context by hand, separately, over and over",
];

const marmotItems = [
  "Catalog every service, database, dashboard and pipeline once",
  "Ownership, definitions and lineage attached to every asset",
  "One MCP endpoint every assistant shares, from Claude to Copilot",
  "Always live, never a stale copy",
];

export default function ContextProblem(): JSX.Element {
  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-5xl mx-auto">
        <div data-animate className="text-center mb-10">
          <p className="text-xs font-bold uppercase tracking-widest text-earthy-terracotta-600 dark:text-earthy-terracotta-400 mb-3">
            Why a context layer
          </p>
          <h2 className="text-2xl sm:text-3xl font-extrabold text-gray-900 dark:text-white mb-3 tracking-tight">
            When AI doesn't know, it guesses
          </h2>
          <p className="text-base text-gray-500 dark:text-gray-400 max-w-2xl mx-auto">
            When someone doesn't know what a column means or where a database
            lives, they ask a teammate. An agent can't do that. It only knows
            what it can reach, so the knowledge people carry in their heads
            needs a place both humans and agents can actually reach.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-2 gap-5 items-stretch">
          {/* Today */}
          <div
            data-animate
            data-animate-delay="1"
            className="rounded-2xl p-7 bg-gradient-to-br from-gray-50 to-gray-100 dark:from-gray-900 dark:to-gray-900/70 border border-gray-200/80 dark:border-gray-700/60 flex flex-col"
          >
            <div className="flex items-center gap-2 mb-5">
              <span className="w-2 h-2 rounded-full bg-gray-300 dark:bg-gray-600" />
              <span className="text-[11px] font-semibold text-gray-400 dark:text-gray-500 uppercase tracking-widest">
                Hardcoded today
              </span>
            </div>
            <ul className="space-y-3.5">
              {todayItems.map((item) => (
                <li key={item} className="flex items-start gap-3">
                  <span className="mt-0.5 w-5 h-5 rounded-full bg-gray-200 dark:bg-gray-700/60 flex items-center justify-center flex-shrink-0">
                    <Icon
                      icon="mdi:close"
                      className="w-3 h-3 text-gray-400 dark:text-gray-500"
                    />
                  </span>
                  <span className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
                    {item}
                  </span>
                </li>
              ))}
            </ul>
          </div>

          {/* With Marmot */}
          <div
            data-animate
            data-animate-delay="2"
            className="relative rounded-2xl p-7 bg-white dark:bg-gray-800 border border-earthy-terracotta-200 dark:border-earthy-terracotta-700/50 shadow-sm flex flex-col overflow-hidden"
          >
            <div className="absolute -inset-px rounded-2xl bg-gradient-to-br from-earthy-terracotta-400/8 via-transparent to-transparent pointer-events-none" />
            <div className="relative flex flex-col flex-1">
              <div className="flex items-center gap-2 mb-5">
                <span className="w-2 h-2 rounded-full bg-emerald-400" />
                <span className="text-[11px] font-semibold text-earthy-terracotta-600 dark:text-earthy-terracotta-400 uppercase tracking-widest">
                  With Marmot
                </span>
              </div>
              <ul className="space-y-3.5">
                {marmotItems.map((item) => (
                  <li key={item} className="flex items-start gap-3">
                    <span className="mt-0.5 w-5 h-5 rounded-full bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/30 flex items-center justify-center flex-shrink-0">
                      <Icon
                        icon="mdi:check"
                        className="w-3 h-3 text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
                      />
                    </span>
                    <span className="text-sm text-gray-700 dark:text-gray-300 leading-relaxed">
                      {item}
                    </span>
                  </li>
                ))}
              </ul>
            </div>
          </div>
        </div>

        <p
          data-animate
          data-animate-delay="3"
          className="text-center text-base text-gray-500 dark:text-gray-400 mt-9 max-w-2xl mx-auto"
        >
          You don't move your data into Marmot. You stop hardcoding the
          context around it.{" "}
          <a
            href="/docs/Agents/"
            className="text-earthy-terracotta-700 dark:text-earthy-terracotta-400 font-semibold hover:underline whitespace-nowrap"
          >
            See Marmot for agents
          </a>
        </p>
      </div>
    </section>
  );
}
