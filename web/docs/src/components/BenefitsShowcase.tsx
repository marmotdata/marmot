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
      "/img/search-light.png",
      "/img/search-dark.png",
      "/img/lineage-light.png",
      "/img/lineage-dark.png",
      "/img/home-light.png",
      "/img/home-dark.png",
      "/img/team-light.png",
      "/img/team-dark.png",
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
      imageSrc: "/img/search-light.png",
      imageAlt: "Marmot search interface showing filters and search results",
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
      imageSrc: "/img/lineage-light.png",
      imageAlt: "Interactive lineage graph showing data flow and dependencies",
    },
    {
      id: "metadata",
      icon: "mdi:database-outline",
      title: "Rich metadata",
      description:
        "Store comprehensive metadata for any asset type. From tables and topics to APIs and dashboards.",
      features: [
        "Rich metadata for any asset type",
        "Custom fields and properties",
        "Comprehensive documentation",
      ],
      imageSrc: "/img/home-light.png",
      imageAlt: "Asset detail page showing rich metadata and documentation",
    },
    {
      id: "team",
      icon: "mdi:account-group",
      title: "Team collaboration",
      description:
        "Assign ownership, document business context, and create glossaries. Keep your team aligned.",
      features: [
        "Team ownership management",
        "Business glossaries",
        "Centralised documentation",
      ],
      imageSrc: "/img/team-light.png",
      imageAlt: "Team management interface showing ownership and collaboration features",
    },
  ];

  const currentBenefit =
    benefits.find((b) => b.id === activeBenefit) || benefits[0];

  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-800">
      <div className="max-w-7xl mx-auto">
        <div className="flex flex-col items-center gap-8">
          {/* Row 1: Horizontal navigation links - centered */}
          <div className="flex flex-wrap gap-3 justify-center">
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
                  className={`inline-flex items-center gap-2 px-5 py-2.5 rounded-full font-medium text-sm transition-all duration-200 ${
                    isActive
                      ? "bg-earthy-terracotta-600 text-white shadow-lg shadow-earthy-terracotta-500/30"
                      : "bg-white dark:bg-gray-800 text-gray-700 dark:text-gray-300 border border-gray-200 dark:border-gray-700 hover:border-earthy-terracotta-300 dark:hover:border-earthy-terracotta-700 hover:shadow-md"
                  }`}
                >
                  <Icon icon={benefit.icon} className="text-lg" />
                  {benefit.title}
                </a>
              );
            })}
          </div>

          {/* Row 2: Centered, shrunk screenshot */}
          <div className="relative rounded-xl overflow-hidden border border-gray-200 dark:border-gray-700 shadow-lg max-w-4xl w-full">
            <img
              src={currentBenefit.imageSrc}
              alt={currentBenefit.imageAlt}
              className="w-full h-auto dark:hidden"
              key={currentBenefit.id}
            />
            <img
              src={currentBenefit.imageSrc.replace("-light.png", "-dark.png")}
              alt={currentBenefit.imageAlt}
              className="w-full h-auto hidden dark:block"
              key={`${currentBenefit.id}-dark`}
            />
          </div>

          {/* Row 3: Split layout - description left, features right - same width as image */}
          <div className="grid grid-cols-1 lg:grid-cols-5 gap-8 max-w-4xl w-full">
            {/* Left: Description */}
            <div className="lg:col-span-3 space-y-4">
              <h2 className="text-2xl font-bold text-gray-900 dark:text-white">
                {currentBenefit.title}
              </h2>
              <p className="text-base leading-relaxed text-gray-600 dark:text-gray-400">
                {currentBenefit.description}
              </p>
              <p className="text-sm leading-relaxed text-gray-500 dark:text-gray-500">
                Marmot helps you discover and understand your data assets quickly.
                Whether you're searching across tables, tracking lineage, or collaborating
                with your team, everything is designed to be simple and intuitive.
              </p>
            </div>

            {/* Right: Feature points */}
            <div className="lg:col-span-2 flex flex-col gap-4">
              <h3 className="text-lg font-semibold text-gray-900 dark:text-white mb-2">
                Key Features
              </h3>
              {currentBenefit.features.map((feature, index) => (
                <div key={index} className="flex items-start gap-3">
                  <div className="flex-shrink-0">
                    <Icon
                      icon="mdi:check-circle"
                      className="w-6 h-6 text-earthy-terracotta-600 dark:text-earthy-terracotta-500"
                    />
                  </div>
                  <p className="text-base text-gray-900 dark:text-white font-medium leading-relaxed">
                    {feature}
                  </p>
                </div>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
