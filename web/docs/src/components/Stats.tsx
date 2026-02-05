import React from "react";
import { Icon } from "@iconify/react";

const stats = [
  { value: "500+", label: "GitHub Stars", icon: "mdi:star" },
  { value: "6k+", label: "Downloads", icon: "mdi:download" },
  { value: "20+", label: "Integrations", icon: "mdi:puzzle" },
];

export default function Stats(): React.ReactElement {
  return (
    <section className="py-8 px-4 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-3xl mx-auto flex justify-center gap-12 text-gray-400 dark:text-gray-500">
        {stats.map((stat, index) => (
          <div key={index} className="flex items-center gap-2">
            <Icon icon={stat.icon} className="w-5 h-5" />
            <div>
              <span className="text-2xl font-semibold">{stat.value}</span>
              <span className="text-xs uppercase tracking-wide ml-1">{stat.label}</span>
            </div>
          </div>
        ))}
      </div>
    </section>
  );
}
