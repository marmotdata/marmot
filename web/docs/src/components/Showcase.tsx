import React, { useState } from "react";
import { FontAwesomeIcon } from "@fortawesome/react-fontawesome";
import { IconDefinition } from "@fortawesome/fontawesome-svg-core";

interface Feature {
  id: string;
  label: string;
  icon?: IconDefinition | React.ComponentType<{ className?: string }>;
  title: string;
  description: string;
  features: string[];
  imageSrc: string;
  imageAlt: string;
}

interface FeatureShowcaseProps {
  subtitle: string;
  options: Feature[];
}

export default function FeatureShowcase({
  subtitle,
  options,
}: FeatureShowcaseProps): JSX.Element {
  const [activeFeature, setActiveFeature] = useState(options[0]?.id || "");
  const currentFeature =
    options.find((opt) => opt.id === activeFeature) || options[0];

  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-7xl mx-auto">
        <div className="flex flex-wrap justify-center gap-3 mb-12">
          {options.map((option) => (
            <a
              key={option.id}
              href="#"
              onClick={(e) => {
                e.preventDefault();
                setActiveFeature(option.id);
              }}
              className={`
                px-6 py-2 rounded-full text-sm font-medium transition-all duration-200 cursor-pointer inline-flex items-center gap-2
                ${activeFeature === option.id
                  ? "bg-amber-600 text-white"
                  : "bg-amber-100 dark:bg-amber-900/30 text-amber-800 dark:text-amber-300 hover:bg-amber-200 dark:hover:bg-amber-900/50"
                }
              `}
            >
              {option.icon &&
                (typeof option.icon === "object" &&
                  "iconName" in option.icon ? (
                  <FontAwesomeIcon icon={option.icon} className="w-4 h-4" />
                ) : (
                  React.createElement(
                    option.icon as React.ComponentType<{ className?: string }>,
                    { className: "w-4 h-4" },
                  )
                ))}
              {option.label}
            </a>
          ))}
        </div>
        <div className="text-center max-w-4xl mx-auto mb-12">
          <h2 className="text-4xl font-bold text-gray-900 dark:text-white mb-6">
            {currentFeature.title}
          </h2>
          <p className="text-xl text-gray-600 dark:text-gray-300 mb-8">
            {currentFeature.description}
          </p>
          <div className="flex flex-wrap justify-center gap-x-8 gap-y-4">
            {currentFeature.features.map((feature, index) => (
              <div key={index} className="flex items-center">
                <div className="flex-shrink-0">
                  <CheckIcon />
                </div>
                <p className="ml-3 text-base text-gray-600 dark:text-gray-300">
                  {feature}
                </p>
              </div>
            ))}
          </div>
        </div>
        <div className="max-w-5xl mx-auto">
          <div className="relative rounded-lg overflow-hidden shadow-xl border border-gray-200 dark:border-gray-700 transition-all duration-300">
            <img
              src={currentFeature.imageSrc}
              alt={currentFeature.imageAlt}
              className="w-full h-auto dark:hidden"
              key={currentFeature.id}
            />
            <img
              src={currentFeature.imageSrc.replace(/\.png$/, "-dark.png")}
              alt={currentFeature.imageAlt}
              className="w-full h-auto hidden dark:block"
              key={`${currentFeature.id}-dark`}
            />
          </div>
        </div>
      </div>
    </section>
  );
}

function CheckIcon(): JSX.Element {
  return (
    <svg
      className="h-6 w-6 text-amber-600"
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M5 13l4 4L19 7"
      />
    </svg>
  );
}
