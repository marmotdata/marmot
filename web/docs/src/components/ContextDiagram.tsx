import React from "react";
import { Icon } from "@iconify/react";

interface Node {
  label: string;
  icon?: string;
  iconImg?: string;
  kafkaIcon?: boolean;
}

const sources: Node[] = [
  { label: "PostgreSQL", icon: "devicon:postgresql" },
  { label: "Kafka", icon: "devicon:apachekafka", kafkaIcon: true },
  { label: "S3", icon: "logos:aws-s3" },
  { label: "dbt", icon: "logos:dbt-icon" },
  { label: "Trino", icon: "simple-icons:trino" },
  { label: "Iceberg", iconImg: "/img/iceberg.svg" },
];

const surfaces: Node[] = [
  { label: "MCP server", icon: "mdi:transit-connection-variant" },
  { label: "Web UI", icon: "mdi:monitor-dashboard" },
  { label: "REST API", icon: "mdi:api" },
  { label: "CLI", icon: "mdi:console" },
  { label: "Slack", icon: "simple-icons:slack" },
];

const stores = ["Assets", "Lineage", "Ownership", "Glossary"];

function NodeIcon({ node, className }: { node: Node; className: string }) {
  if (node.iconImg) {
    return (
      <img
        src={node.iconImg}
        alt=""
        className={`${className} object-contain`}
        aria-hidden="true"
      />
    );
  }
  return (
    <Icon
      icon={node.icon!}
      className={`${className} ${node.kafkaIcon ? "kafka-icon" : ""}`}
      aria-hidden="true"
    />
  );
}

/**
 * Straight lines fanning from one edge of the box to a single point on the
 * other. `side` says which side of Marmot the fan sits on; either way the
 * lines are drawn towards Marmot, so the dashes travel that way too. Sources
 * push their metadata in, everything on the right reads back out of it.
 */
function Fan({ count, side }: { count: number; side: "left" | "right" }) {
  const lines = Array.from({ length: count }, (_, i) => {
    const y = ((i + 0.5) / count) * 100;
    return side === "left"
      ? { x1: 0, y1: y, x2: 100, y2: 50 }
      : { x1: 100, y1: y, x2: 0, y2: 50 };
  });

  return (
    <svg
      className="absolute inset-0 w-full h-full text-earthy-terracotta-400/80 dark:text-earthy-terracotta-500/60"
      viewBox="0 0 100 100"
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      {lines.map((l, i) => (
        <line
          key={i}
          x1={l.x1}
          y1={l.y1}
          x2={l.x2}
          y2={l.y2}
          stroke="currentColor"
          strokeWidth={1}
          strokeDasharray="3 3"
          vectorEffect="non-scaling-stroke"
          className="animate-flow"
        />
      ))}
    </svg>
  );
}

/** Arrowhead at the point where a fan meets Marmot, pointing at it. */
function Arrow({
  className,
  points,
}: {
  className: string;
  points: "left" | "right";
}) {
  return (
    <svg
      viewBox="0 0 8 10"
      className={`${className} w-2 h-2.5 text-earthy-terracotta-500 dark:text-earthy-terracotta-400`}
      aria-hidden="true"
    >
      <path
        d={points === "right" ? "M0 0 L8 5 L0 10 Z" : "M8 0 L0 5 L8 10 Z"}
        fill="currentColor"
      />
    </svg>
  );
}

function MarmotCard({ compact }: { compact?: boolean }) {
  return (
    <div
      className={`diagram-hub relative rounded-2xl bg-white dark:bg-gray-800 ${
        compact ? "px-5 py-4 w-full max-w-xs" : "px-8 py-7 w-[280px]"
      }`}
    >
      <div className="flex flex-col items-center">
        <img
          src="/img/marmot.svg"
          alt="Marmot"
          className={compact ? "w-9 h-9" : "w-11 h-11"}
        />
        <span className="mt-1.5 text-sm font-bold text-gray-800 dark:text-gray-200">
          Marmot
        </span>
        <span className="text-[9px] font-semibold uppercase tracking-[0.18em] text-earthy-terracotta-600 dark:text-earthy-terracotta-400">
          Context layer
        </span>
      </div>
      <div className="mt-4 grid grid-cols-2 gap-1.5">
        {stores.map((s) => (
          <span
            key={s}
            className="rounded-md bg-earthy-brown-100 dark:bg-gray-900/60 py-1.5 text-center text-[11px] font-medium text-gray-600 dark:text-gray-400"
          >
            {s}
          </span>
        ))}
      </div>
    </div>
  );
}

export default function ContextDiagram(): JSX.Element {
  return (
    <section className="py-20 sm:py-24 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-800">
      <div className="max-w-5xl mx-auto">
        <div data-animate className="max-w-2xl mx-auto text-center">
          <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white tracking-tight">
            One layer between your systems and everyone asking about them
          </h2>
          <p className="mt-4 text-lg text-gray-500 dark:text-gray-400">
            Plugins keep it filled. People and agents read from the same place.
          </p>
        </div>

        {/* Desktop */}
        <div
          data-animate
          data-animate-delay="1"
          className="hidden lg:flex items-center justify-center mt-16"
        >
          <div className="flex items-stretch">
            <ul className="m-0 list-none p-0">
              {sources.map((s) => (
                <li
                  key={s.label}
                  className="flex h-9 items-center justify-end gap-2.5"
                >
                  <span className="text-sm text-gray-600 dark:text-gray-400">
                    {s.label}
                  </span>
                  <NodeIcon node={s} className="w-5 h-5" />
                </li>
              ))}
            </ul>
            <div className="relative w-40">
              <Fan count={sources.length} side="left" />
              <Arrow
                points="right"
                className="absolute right-0 top-1/2 -translate-y-1/2"
              />
            </div>
          </div>

          <MarmotCard />

          <div className="flex items-stretch">
            <div className="relative w-40">
              <Fan count={surfaces.length} side="right" />
              <Arrow
                points="left"
                className="absolute left-0 top-1/2 -translate-y-1/2"
              />
            </div>
            <ul className="m-0 list-none p-0">
              {surfaces.map((s) => (
                <li key={s.label} className="flex h-9 items-center gap-2.5">
                  <NodeIcon
                    node={s}
                    className="w-5 h-5 text-gray-400 dark:text-gray-500"
                  />
                  <span className="text-sm text-gray-600 dark:text-gray-400">
                    {s.label}
                  </span>
                </li>
              ))}
            </ul>
          </div>
        </div>

        {/* Mobile */}
        <div
          data-animate
          data-animate-delay="1"
          className="lg:hidden mt-12 flex flex-col items-center"
        >
          <div className="flex flex-wrap items-center justify-center gap-x-5 gap-y-3">
            {sources.map((s) => (
              <div key={s.label} className="flex items-center gap-2">
                <NodeIcon node={s} className="w-5 h-5" />
                <span className="text-xs text-gray-600 dark:text-gray-400">
                  {s.label}
                </span>
              </div>
            ))}
          </div>

          <span className="diagram-rule-v my-4 h-8" />
          <MarmotCard compact />
          <span className="diagram-rule-v my-4 h-8" />

          <div className="flex flex-wrap items-center justify-center gap-x-5 gap-y-3">
            {surfaces.map((s) => (
              <div key={s.label} className="flex items-center gap-2">
                <NodeIcon
                  node={s}
                  className="w-5 h-5 text-gray-400 dark:text-gray-500"
                />
                <span className="text-xs text-gray-600 dark:text-gray-400">
                  {s.label}
                </span>
              </div>
            ))}
          </div>
        </div>

        <p
          data-animate
          data-animate-delay="2"
          className="mt-14 text-center text-sm text-gray-400 dark:text-gray-500 max-w-xl mx-auto"
        >
          Your rows never move. Marmot holds what exists, what it means and who
          owns it, and you can also push that in from{" "}
          <a
            href="/docs/Populating/Terraform"
            className="text-earthy-terracotta-600 dark:text-earthy-terracotta-400 hover:underline"
          >
            Terraform
          </a>
          ,{" "}
          <a
            href="/docs/Populating/Pulumi"
            className="text-earthy-terracotta-600 dark:text-earthy-terracotta-400 hover:underline"
          >
            Pulumi
          </a>{" "}
          or the{" "}
          <a
            href="/docs/populating/api"
            className="text-earthy-terracotta-600 dark:text-earthy-terracotta-400 hover:underline"
          >
            API
          </a>
          .
        </p>
      </div>
    </section>
  );
}
