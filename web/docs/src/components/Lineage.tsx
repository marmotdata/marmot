import React from "react";
import Icon from "./Icon";

interface LineageNodeProps {
  title: string;
  type: string;
  isCurrent?: boolean;
}

function LineageNode({
  title,
  type,
  isCurrent = false,
}: LineageNodeProps): JSX.Element {
  return (
    <div
      className={`
        node p-4 rounded-lg cursor-pointer min-w-[150px] border-solid
        ${isCurrent
          ? "bg-orange-50 border-2 border-orange-600 dark:bg-[#4d4d4d]"
          : "bg-[#fefdf8] border border-[#dfdfdf] dark:bg-[#2e2e2e] dark:border-[#4d4d4d]"
        }
        transition-all duration-150
      `}
      style={{
        borderStyle: "solid",
      }}
    >
      <div className="text-gray-900 dark:text-gray-100 font-medium text-center">
        {title}
      </div>
      <div className="flex justify-center mt-2">
        <div className="p-2">
          <div className="flex flex-col items-center gap-2">
            <Icon type={type} />
            <span className="font-medium text-gray-900 dark:text-gray-100 text-center">
              {type}
            </span>
          </div>
        </div>
      </div>
    </div>
  );
}

export default function Lineage(): JSX.Element {
  return (
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-7xl mx-auto">
        <div className="lg:grid lg:grid-cols-2 lg:gap-8 lg:items-center">
          <div className="order-2 lg:order-1">
            {/* Static Lineage Diagram */}
            <div className="relative flex justify-center items-center gap-12 py-8 overflow-hidden">
              {/* Animated Connector Lines */}
              <div className="absolute left-[calc(50%-240px)] right-[calc(50%-240px)] top-1/2 -translate-y-1/2 z-0">
                <svg width="100%" height="2">
                  <line
                    x1="0"
                    y1="1"
                    x2="100%"
                    y2="1"
                    stroke="#94a3b8"
                    strokeWidth="2"
                    strokeDasharray="4 4"
                    className="animate-flow"
                  />
                </svg>
              </div>

              {/* Node Container - higher z-index than line */}
              <div className="relative z-10 flex justify-center items-center gap-20">
                {/* Kafka Node */}
                <div>
                  <LineageNode title="orders" type="Kafka" />
                </div>

                {/* OrderAPI Node */}
                <div>
                  <LineageNode
                    title="orderapi"
                    type="AsyncAPI"
                    isCurrent={true}
                  />
                </div>

                {/* SNS Node */}
                <div>
                  <LineageNode title="order-placed" type="SNS" />
                </div>
              </div>

              {/* Gradient overlays for left/right edges */}
              <div
                className="absolute inset-y-0 left-0 w-20 pointer-events-none dark:hidden"
                style={{
                  background:
                    "linear-gradient(to right, rgba(254, 253, 248, 1) 0%, rgba(254, 253, 248, 0) 100%)",
                  zIndex: 20,
                }}
              />
              <div
                className="absolute inset-y-0 right-0 w-20 pointer-events-none dark:hidden"
                style={{
                  background:
                    "linear-gradient(to left, rgba(254, 253, 248, 1) 0%, rgba(254, 253, 248, 0) 100%)",
                  zIndex: 20,
                }}
              />
              <div
                className="absolute inset-y-0 left-0 w-20 pointer-events-none hidden dark:block"
                style={{
                  background:
                    "linear-gradient(to right, rgba(26, 26, 26, 1) 0%, rgba(26, 26, 26, 0) 100%)",
                  zIndex: 20,
                }}
              />
              <div
                className="absolute inset-y-0 right-0 w-20 pointer-events-none hidden dark:block"
                style={{
                  background:
                    "linear-gradient(to left, rgba(26, 26, 26, 1) 0%, rgba(26, 26, 26, 0) 100%)",
                  zIndex: 20,
                }}
              />
            </div>
          </div>

          <div className="mb-10 lg:mb-0 order-1 lg:order-2">
            <h2 className="text-3xl font-extrabold text-gray-900 dark:text-white">
              Lineage Visualization
            </h2>
            <p className="mt-4 text-lg text-gray-600 dark:text-gray-300">
              Understand data flow and dependencies with interactive lineage
              graphs. Track data throughout your entire stack, seeing exactly
              where you data is being used.
            </p>

            <ul className="mt-8 space-y-4">
              {[
                "Impact analysis for data changes",
                "Data flow visualization",
                "Dependency tracking",
              ].map((feature, index) => (
                <li key={index} className="flex items-start">
                  <div className="flex-shrink-0">
                    <CheckIcon />
                  </div>
                  <p className="ml-3 text-base text-gray-600 dark:text-gray-300">
                    {feature}
                  </p>
                </li>
              ))}
            </ul>
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
