import React from "react";
import { Icon } from "@iconify/react";

const stats = [
  { value: "500+", label: "GitHub Stars", icon: "mdi:star" },
  { value: "6k+", label: "Downloads", icon: "mdi:download" },
  { value: "20+", label: "Integrations", icon: "mdi:puzzle" },
  { value: "MIT", label: "Licensed", icon: "mdi:license" },
];

export default function Stats(): React.ReactElement {
  return (
    <section className="py-12 px-4 bg-earthy-brown-50 dark:bg-gray-900">
      <div
        data-animate
        className="max-w-3xl mx-auto flex flex-wrap items-center justify-center divide-x divide-gray-200 dark:divide-gray-700"
      >
        {stats.map((stat, index) => (
          <div key={index} className="px-6 sm:px-10 py-2 text-center">
            <div className="flex items-center justify-center gap-2 mb-1">
              <Icon
                icon={stat.icon}
                className="w-4 h-4 text-gray-300 dark:text-gray-600"
              />
              <span className="text-2xl font-bold text-gray-400 dark:text-gray-500 tracking-tight">
                {stat.value}
              </span>
            </div>
            <span className="text-xs uppercase tracking-wider font-medium text-gray-400 dark:text-gray-500">
              {stat.label}
            </span>
          </div>
        ))}
      </div>
    </section>
  );
}
