import React from "react";
import { Icon } from "@iconify/react";

const assurances = [
  {
    icon: "mdi:database-eye-off-outline",
    title: "Metadata only",
    description:
      "Schemas, ownership and lineage. Your rows, messages and payloads never enter Marmot.",
  },
  {
    icon: "mdi:cloud-lock-outline",
    title: "Self-host or Cloud",
    description:
      "Run it inside your own VPC under your own controls, or let us run it for you.",
  },
  {
    icon: "mdi:source-branch-check",
    title: "MIT licensed",
    description:
      "Built in the open. Read exactly what it collects and what it stores.",
  },
];

export default function CTA(): JSX.Element {
  return (
    <section className="py-20 sm:py-24 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-4xl mx-auto">
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-8">
          {assurances.map((a, index) => (
            <div
              key={a.title}
              data-animate
              data-animate-delay={String(index + 1)}
            >
              <Icon
                icon={a.icon}
                className="w-5 h-5 text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
              />
              <h3 className="mt-3 text-sm font-bold text-gray-900 dark:text-white">
                {a.title}
              </h3>
              <p className="mt-1 text-sm text-gray-500 dark:text-gray-400 leading-relaxed">
                {a.description}
              </p>
            </div>
          ))}
        </div>

        <div className="section-divider mt-14" />

        <div className="pt-14 max-w-2xl mx-auto text-center">
          <h2
            data-animate
            className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white tracking-tight"
          >
            One context for people and agents
          </h2>
          <p
            data-animate
            data-animate-delay="1"
            className="mt-4 text-lg text-gray-500 dark:text-gray-400"
          >
            Try the live demo, or deploy it yourself in a few minutes.
          </p>

          <div
            data-animate
            data-animate-delay="2"
            className="mt-8 flex flex-row justify-center items-center gap-3"
          >
            <a
              href="https://demo.marmotdata.io"
              target="_blank"
              rel="noopener noreferrer"
              className="group inline-flex items-center justify-center px-7 py-3.5 text-sm font-semibold rounded-xl text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 shadow-sm hover:shadow-md transition-all duration-200 hover:-translate-y-0.5"
            >
              Live demo
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
              className="group demo-btn inline-flex items-center justify-center px-7 py-3.5 text-sm font-semibold rounded-xl text-gray-700 dark:text-gray-300 bg-white/70 dark:bg-gray-800/50 transition-all duration-200 hover:-translate-y-0.5"
            >
              Talk to us
            </a>
          </div>

          <p
            data-animate
            data-animate-delay="3"
            className="mt-12 text-sm text-gray-400 dark:text-gray-500 leading-relaxed"
          >
            Built by engineers from HashiCorp, Adidas, Just Eat Takeaway.com and
            Traefik, who help maintain Kubernetes, Terraform, Redpanda and the
            Cloud Native Computing Foundation.
          </p>
        </div>
      </div>
    </section>
  );
}
