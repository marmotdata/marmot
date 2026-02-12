import React from "react";
import { Icon } from "@iconify/react";

const props = [
  {
    title: "Find data for your agents",
    description:
      "Give AI agents instant context about your data assets via MCP",
    icon: "mdi:robot-outline",
  },
  {
    title: "Integration made simple",
    description: "Flexible API, CLI, Terraform and Pulumi support",
    icon: "mdi:code-braces",
  },
  {
    title: "Deploy anywhere",
    description:
      "Completely open source and self-hosted. Your data stays yours",
    icon: "mdi:cloud-lock-outline",
  },
];

export default function ValueProps(): JSX.Element {
  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-5xl mx-auto">
        <div className="text-center mb-12">
          <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white mb-4">
            Built for how you work
          </h2>
          <p className="text-lg text-gray-600 dark:text-gray-300 max-w-2xl mx-auto">
            Whether you're a data engineer, platform team, or building AI agents
            — Marmot fits into your workflow.
          </p>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
          {props.map((prop, index) => (
            <div
              key={index}
              className="text-center bg-white dark:bg-gray-800 rounded-xl p-8 border border-gray-200 dark:border-gray-700 hover:shadow-lg hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-600 transition-all"
            >
              <div className="flex justify-center mb-5">
                <div className="inline-flex items-center justify-center w-12 h-12 rounded-xl bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900">
                  <Icon
                    icon={prop.icon}
                    className="w-7 h-7 text-earthy-terracotta-700 dark:text-earthy-terracotta-400"
                  />
                </div>
              </div>
              <h3 className="text-lg font-bold text-gray-900 dark:text-white mb-2">
                {prop.title}
              </h3>
              <p className="text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
                {prop.description}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
