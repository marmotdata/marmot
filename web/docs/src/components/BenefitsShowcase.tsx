import React, { useState, useEffect } from "react";
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

  // Preload all images on mount
  useEffect(() => {
    const imagesToPreload = [
      "/img/discovery.png",
      "/img/discovery-dark.png",
      "/img/lineage.png",
      "/img/lineage-dark.png",
      "/img/metrics.png",
      "/img/metrics-dark.png",
    ];

    imagesToPreload.forEach((src) => {
      const img = new Image();
      img.src = src;
    });
  }, []);

  const benefits: Benefit[] = [
    {
      id: "search",
      icon: "mdi:magnify",
      title: "Search everything",
      description:
        "Find exactly what you're looking for with a powerful query language that works across all your data assets.",
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
        "Understand dependencies and assess impact before making changes. Built-in support for OpenLineage.",
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
        "Get insights into your data landscape. Track which assets are most used and monitor technology adoption trends.",
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
      <div className="max-w-7xl mx-auto">
        <div className="flex flex-col lg:flex-row gap-8 items-start">
          <div className="flex flex-col gap-6 lg:w-1/3 shrink-0">
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
                  className={`group block p-6 rounded-xl transition-all duration-200 ${isActive
                      ? "bg-gradient-to-br from-earthy-terracotta-50 to-earthy-terracotta-100 dark:from-earthy-terracotta-900/20 dark:to-earthy-terracotta-900/20 shadow-lg"
                      : "bg-gray-50 dark:bg-gray-900 hover:bg-gray-100 dark:hover:bg-gray-800 hover:shadow-md"
                    }`}
                >
                  <div
                    className={`inline-flex items-center justify-center w-12 h-12 mb-4 rounded-lg transition-all duration-200 ${isActive
                        ? "bg-earthy-terracotta-700 text-white shadow-md"
                        : "bg-white dark:bg-gray-800 text-earthy-terracotta-700 dark:text-earthy-terracotta-500 group-hover:bg-earthy-terracotta-50 dark:group-hover:bg-earthy-terracotta-900/30"
                      }`}
                  >
                    <Icon icon={benefit.icon} className="text-2xl" />
                  </div>
                  <h3
                    className={`text-lg font-bold mb-2 transition-colors ${isActive
                        ? "text-earthy-terracotta-900 dark:text-earthy-terracotta-100"
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

          <div className="flex-1">
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
      </div>
    </section>
  );
}
