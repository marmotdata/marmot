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
    <section className="py-10 px-4 bg-earthy-brown-50 dark:bg-gray-900 border-t border-gray-200 dark:border-gray-800">
      <div className="max-w-4xl mx-auto">
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-8 text-center">
          {stats.map((stat, index) => (
            <div key={index} className="flex flex-col items-center gap-1">
              <Icon
                icon={stat.icon}
                className="w-5 h-5 text-gray-400 dark:text-gray-500 mb-1"
              />
              <span className="text-2xl font-semibold text-gray-400 dark:text-gray-500">
                {stat.value}
              </span>
              <span className="text-xs uppercase tracking-wider text-gray-500 dark:text-gray-400">
                {stat.label}
              </span>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
