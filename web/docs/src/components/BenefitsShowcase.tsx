import React, { useState } from "react";
import { Icon } from "@iconify/react";

interface Benefit {
  id: string;
  icon: string;
  title: string;
  description: string;
  features: string[];
  imageSrc: string;
  imageAlt: string;
}

export default function BenefitsShowcase(): JSX.Element {
  const [activeBenefit, setActiveBenefit] = useState("search");

  const benefits: Benefit[] = [
    {
      id: "search",
      icon: "mdi:magnify",
      title: "Search everything",
      description:
        "Power query language across all your data assets. Find exactly what you're looking for.",
      features: [
        "Full-text search across all metadata",
        "Powerful built-in query language",
        "Save and share search queries",
      ],
      imageSrc: "/img/discovery.png",
      imageAlt: "Discovery interface",
    },
    {
      id: "lineage",
      icon: "mdi:graph-outline",
      title: "Track lineage",
      description:
        "See dependencies and impact before making changes. OpenLineage support built-in.",
      features: [
        "Impact analysis for data changes",
        "Data flow visualization",
        "OpenLineage support",
      ],
      imageSrc: "/img/lineage.png",
      imageAlt: "Lineage visualization",
    },
    {
      id: "metrics",
      icon: "mdi:chart-line",
      title: "Usage analytics",
      description:
        "Understand your data landscape. See which tables are most used, monitor technology adoption and gain insights into data trends.",
      features: [
        "Asset trends and popularity",
        "Overview of your data landscape",
        "Technology usage insights",
      ],
      imageSrc: "/img/metrics.png",
      imageAlt: "Usage metrics and analytics",
    },
  ];

  const currentBenefit =
    benefits.find((b) => b.id === activeBenefit) || benefits[0];

  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-800">
      <div className="max-w-6xl mx-auto">
        {/* Clickable Benefit Cards */}
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6 mb-16">
          {benefits.map((benefit) => {
            const isActive = activeBenefit === benefit.id;
            return (
              <a
                key={benefit.id}
                href="#"
                onClick={(e) => {
                  e.preventDefault();
                  setActiveBenefit(benefit.id);
                }}
                className={`group block p-8 rounded-xl transition-all duration-200 ${
                  isActive
                    ? "bg-gradient-to-br from-amber-50 to-orange-50 dark:from-amber-900/20 dark:to-orange-900/20 shadow-lg"
                    : "bg-gray-50 dark:bg-gray-900 hover:bg-gray-100 dark:hover:bg-gray-800 hover:shadow-md"
                }`}
              >
                <div
                  className={`inline-flex items-center justify-center w-14 h-14 mb-5 rounded-lg transition-all duration-200 ${
                    isActive
                      ? "bg-amber-600 text-white shadow-md"
                      : "bg-white dark:bg-gray-800 text-amber-600 dark:text-amber-400 group-hover:bg-amber-50 dark:group-hover:bg-amber-900/30"
                  }`}
                >
                  <Icon icon={benefit.icon} className="text-2xl" />
                </div>
                <h3
                  className={`text-xl font-bold mb-2 transition-colors ${
                    isActive
                      ? "text-amber-900 dark:text-amber-100"
                      : "text-gray-900 dark:text-white"
                  }`}
                >
                  {benefit.title}
                </h3>
                <p className="text-sm text-gray-600 dark:text-gray-400 leading-relaxed">
                  {benefit.description}
                </p>
              </a>
            );
          })}
        </div>

        {/* Screenshot */}
        <div className="max-w-4xl mx-auto">
          <div className="relative rounded-xl overflow-hidden border border-gray-200 dark:border-gray-700">
            <img
              src={currentBenefit.imageSrc}
              alt={currentBenefit.imageAlt}
              className="w-full h-auto dark:hidden"
              key={currentBenefit.id}
            />
            <img
              src={currentBenefit.imageSrc.replace(/\.png$/, "-dark.png")}
              alt={currentBenefit.imageAlt}
              className="w-full h-auto hidden dark:block"
              key={`${currentBenefit.id}-dark`}
            />
            <div className="absolute inset-x-0 bottom-0 h-1/2 bg-gradient-to-t from-white dark:from-gray-800 via-white/50 dark:via-gray-800/50 to-transparent pointer-events-none"></div>
          </div>
        </div>
      </div>
    </section>
  );
}
