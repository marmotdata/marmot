import React from "react";
import { Icon } from "@iconify/react";

export default function ValueProps(): JSX.Element {
  const props = [
    {
      title: "Find data for your agents",
      description: "Give AI agents instant context about your data assets",
      icon: "mdi:robot-outline",
    },
    {
      title: "Integration made simple",
      description: "Flexible API, CLI and Terraform support",
      icon: "mdi:code-braces",
    },
    {
      title: "Deploy anywhere",
      description: "Completely open source and self-hosted. Your data stays yours",
      icon: "mdi:cloud-lock-outline",
    },
  ];

  return (
    <section className="py-16 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-4xl mx-auto">
        <div className="grid grid-cols-1 md:grid-cols-3 gap-8">
          {props.map((prop, index) => (
            <div key={index} className="text-center">
              <div className="flex justify-center mb-4">
                <Icon
                  icon={prop.icon}
                  className="w-10 h-10 text-earthy-terracotta-700 dark:text-earthy-terracotta-500"
                />
              </div>
              <h3 className="text-lg font-bold text-gray-900 dark:text-white mb-2">
                {prop.title}
              </h3>
              <p className="text-sm text-gray-600 dark:text-gray-400">
                {prop.description}
              </p>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
