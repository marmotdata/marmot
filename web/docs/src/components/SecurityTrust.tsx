import React from "react";
import { Icon } from "@iconify/react";

const pillars = [
  {
    icon: "mdi:database-eye-off-outline",
    title: "Metadata, not your data",
    description:
      "Marmot catalogs schemas, ownership, descriptions, lineage and statistics. The rows, messages and payloads inside your systems never enter Marmot.",
  },
  {
    icon: "mdi:cloud-lock-outline",
    title: "Deploy it your way",
    description:
      "Use managed Cloud, or run the open source build yourself. Run it yourself and even the metadata stays inside your own VPC and under your own controls.",
  },
  {
    icon: "mdi:source-branch-check",
    title: "Open source and auditable",
    description:
      "MIT licensed and built in the open. Read exactly what Marmot collects, how it connects to a source, and what it stores, line by line.",
  },
];

export default function SecurityTrust(): JSX.Element {
  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-6xl mx-auto">
        <div data-animate className="text-center mb-10">
          <div className="inline-flex items-center justify-center w-11 h-11 rounded-xl bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 mb-4">
            <Icon
              icon="mdi:shield-check-outline"
              className="w-5 h-5 text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
            />
          </div>
          <h2 className="text-2xl sm:text-3xl font-extrabold text-gray-900 dark:text-white mb-3 tracking-tight">
            Only metadata. Your data stays put.
          </h2>
          <p className="text-base text-gray-500 dark:text-gray-400 max-w-2xl mx-auto">
            A context layer needs to know about your assets, not to hold their
            contents. Marmot is built so the data itself never leaves your
            systems.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
          {pillars.map((p, index) => (
            <div
              key={p.title}
              data-animate
              data-animate-delay={String(index + 1)}
              className="glass-card rounded-2xl p-6 flex flex-col"
            >
              <div className="w-10 h-10 rounded-lg bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 flex items-center justify-center mb-4">
                <Icon
                  icon={p.icon}
                  className="w-5 h-5 text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
                />
              </div>
              <h3 className="text-base font-bold text-gray-900 dark:text-white mb-2">
                {p.title}
              </h3>
              <p className="text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
                {p.description}
              </p>
            </div>
          ))}
        </div>

        <p
          data-animate
          data-animate-delay="4"
          className="text-center text-sm text-gray-500 dark:text-gray-400 mt-9"
        >
          <a
            href="/pricing#contact"
            className="text-earthy-terracotta-700 dark:text-earthy-terracotta-400 font-semibold hover:underline"
          >
            Talk to us
          </a>
        </p>
      </div>
    </section>
  );
}
