import React from "react";
import { Icon } from "@iconify/react";
import IconAPI from "~icons/material-symbols/api-rounded";
import IconTerminal from "~icons/material-symbols/terminal";

const capabilities = [
  {
    icon: "mdi:magnify",
    title: "Discover",
    description:
      "One place for AI and engineers to find every table, topic, queue and API.",
  },
  {
    icon: "mdi:graph-outline",
    title: "Understand",
    description:
      "Trace how data flows and what depends on what with lineage.",
  },
  {
    icon: "mdi:tag-text-outline",
    title: "Contextualize",
    description:
      "Ownership, business definitions and custom fields that give AI the full picture.",
  },
  {
    icon: "mdi:share-variant-outline",
    title: "Share",
    description:
      "Expose certified context through MCP, the API and the UI.",
  },
];

const populateMethods = [
  {
    title: "Terraform",
    href: "/docs/populating/terraform",
    icon: (
      <img
        src="/img/terraform.svg"
        alt="Terraform"
        className="w-6 h-6 object-contain"
      />
    ),
  },
  {
    title: "Pulumi",
    href: "/docs/populating/pulumi",
    icon: (
      <img
        src="/img/pulumi.svg"
        alt="Pulumi"
        className="w-6 h-6 object-contain"
      />
    ),
  },
  {
    title: "API",
    href: "/docs/populating/api",
    icon: <IconAPI className="w-6 h-6" />,
  },
  {
    title: "CLI",
    href: "/docs/populating/cli",
    icon: <IconTerminal className="w-6 h-6" />,
  },
];

export default function BenefitsShowcase(): JSX.Element {
  return (
    <section className="py-16 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-800">
      <div className="max-w-6xl mx-auto">
        <div data-animate className="text-center mb-10">
          <h2 className="text-2xl sm:text-3xl font-extrabold text-gray-900 dark:text-white mb-3 tracking-tight">
            Context for AI and engineers
          </h2>
          <p className="text-base text-gray-500 dark:text-gray-400 max-w-2xl mx-auto">
            Catalog every data asset, enrich it with the context that matters
            and make it accessible to your team and your AI tools.
          </p>
        </div>

        <div className="grid grid-cols-1 lg:grid-cols-5 gap-8 mb-8 items-center">
          <div className="lg:col-span-3">
            <iframe
              width="100%"
              height="100%"
              style={{ aspectRatio: '16 / 9', border: 'none' }}
              className="rounded-xl shadow-lg"
              src="https://www.youtube.com/embed/_JBcQGj_bFU"
              title="Marmot Demo"
              allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture"
              allowFullScreen
            ></iframe>
          </div>

          <div className="lg:col-span-2 grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-1 gap-3">
            {capabilities.map((cap, index) => (
              <div
                key={cap.title}
                data-animate
                data-animate-delay={String(index + 1)}
                className="rounded-xl p-4 bg-earthy-brown-50/60 dark:bg-gray-900/40 border border-gray-100 dark:border-gray-700/40 flex items-start gap-3"
              >
                <div className="w-9 h-9 rounded-lg bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 flex items-center justify-center shrink-0">
                  <Icon
                    icon={cap.icon}
                    className="w-4 h-4 text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
                  />
                </div>
                <div>
                  <h3 className="text-sm font-bold text-gray-900 dark:text-white mb-0.5">
                    {cap.title}
                  </h3>
                  <p className="text-xs leading-relaxed text-gray-500 dark:text-gray-400">
                    {cap.description}
                  </p>
                </div>
              </div>
            ))}
          </div>
        </div>

        <div data-animate data-animate-delay="5">
          <p className="text-center text-xs font-semibold text-gray-400 dark:text-gray-500 uppercase tracking-widest mb-4">
            Populate with
          </p>
          <div className="flex items-center justify-center gap-3 sm:gap-4">
            {populateMethods.map((method) => (
              <a
                key={method.title}
                href={method.href}
                className="group flex items-center gap-2 px-4 py-2 rounded-lg border border-gray-100 dark:border-gray-700/50 bg-gray-50/80 dark:bg-gray-900/40 hover:border-earthy-terracotta-200 dark:hover:border-earthy-terracotta-700 hover:shadow-sm transition-all duration-200"
              >
                <span className="transition-transform duration-200 group-hover:scale-110">
                  {method.icon}
                </span>
                <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                  {method.title}
                </span>
              </a>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
