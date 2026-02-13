import React, { useEffect } from "react";
import { Icon } from "@iconify/react";

interface Benefit {
  icon: string;
  title: string;
  description: string;
  features: string[];
  imageSrcLight: string;
  imageSrcDark: string;
  imageAlt: string;
}

const benefits: Benefit[] = [
  {
    icon: "mdi:magnify",
    title: "Search everything",
    description:
      "Find exactly what you're looking for with a powerful query language that works across all your data assets.",
    features: [
      "Full-text search across all metadata",
      "Powerful built-in query language",
      "Save and share search queries",
    ],
    imageSrcLight: "/img/search-light.png",
    imageSrcDark: "/img/search-dark.png",
    imageAlt: "Marmot search interface showing filters and search results",
  },
  {
    icon: "mdi:graph-outline",
    title: "Track lineage",
    description:
      "Understand dependencies and assess impact before making changes. Built-in support for OpenLineage.",
    features: [
      "Impact analysis for data changes",
      "Data flow visualization",
      "OpenLineage support",
    ],
    imageSrcLight: "/img/lineage-light.png",
    imageSrcDark: "/img/lineage-dark.png",
    imageAlt: "Interactive lineage graph showing data flow and dependencies",
  },
  {
    icon: "mdi:database-outline",
    title: "Rich metadata",
    description:
      "Store comprehensive metadata for any asset type. From tables and topics to APIs and dashboards.",
    features: [
      "Rich metadata for any asset type",
      "Custom fields and properties",
      "Comprehensive documentation",
    ],
    imageSrcLight: "/img/home-light.png",
    imageSrcDark: "/img/home-dark.png",
    imageAlt: "Asset detail page showing rich metadata and documentation",
  },
  {
    icon: "mdi:account-group",
    title: "Team collaboration",
    description:
      "Assign ownership, document business context, and create glossaries. Keep your team aligned.",
    features: [
      "Team ownership management",
      "Business glossaries",
      "Centralised documentation",
    ],
    imageSrcLight: "/img/team-light.png",
    imageSrcDark: "/img/team-dark.png",
    imageAlt:
      "Team management interface showing ownership and collaboration features",
  },
];

export { benefits };

export function BenefitRow({
  benefit,
  reversed,
}: {
  benefit: Benefit;
  reversed: boolean;
}) {
  return (
    <div
      className={`flex flex-col ${
        reversed ? "lg:flex-row-reverse" : "lg:flex-row"
      } items-center gap-10 lg:gap-16`}
    >
      <div data-animate className="w-full lg:w-3/5 flex-shrink-0">
        <div className="relative rounded-2xl overflow-hidden shadow-2xl shadow-gray-900/10 dark:shadow-black/30 image-glow">
          <img
            src={benefit.imageSrcLight}
            alt={benefit.imageAlt}
            className="block w-full h-auto dark:hidden"
          />
          <img
            src={benefit.imageSrcDark}
            alt={benefit.imageAlt}
            className="block w-full h-auto hidden dark:block"
          />
        </div>
      </div>

      <div data-animate className="w-full lg:w-2/5 space-y-5">
        <h3 className="text-2xl sm:text-3xl font-bold text-gray-900 dark:text-white tracking-tight">
          {benefit.title}
        </h3>
        <p className="text-base leading-relaxed text-gray-500 dark:text-gray-400">
          {benefit.description}
        </p>
        <div className="space-y-3 pt-2">
          {benefit.features.map((feature, index) => (
            <div key={index} className="flex items-center gap-3">
              <div className="flex-shrink-0 w-5 h-5 rounded-md bg-earthy-terracotta-50 dark:bg-earthy-terracotta-900/20 flex items-center justify-center">
                <Icon
                  icon="mdi:check"
                  className="w-3.5 h-3.5 text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
                />
              </div>
              <span className="text-sm font-medium text-gray-700 dark:text-gray-300">
                {feature}
              </span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

export default function BenefitsShowcase(): JSX.Element {
  useEffect(() => {
    benefits.forEach((b) => {
      new Image().src = b.imageSrcLight;
      new Image().src = b.imageSrcDark;
    });
  }, []);

  return (
    <section className="py-24 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-800">
      <div className="max-w-7xl mx-auto">
        <div className="space-y-24 lg:space-y-32">
          {benefits.map((benefit, index) => (
            <BenefitRow
              key={benefit.title}
              benefit={benefit}
              reversed={index % 2 !== 0}
            />
          ))}
        </div>
      </div>
    </section>
  );
}
