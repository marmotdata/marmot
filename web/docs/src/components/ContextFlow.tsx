import React, { useEffect, useRef, useState } from "react";
import { Icon } from "@iconify/react";

interface FlowItem {
  label: string;
  icon?: string;
  iconImg?: string;
  kafkaIcon?: boolean;
}

interface FlowGroup {
  title: string;
  items: FlowItem[];
}

const sourceGroups: FlowGroup[] = [
  {
    title: "Plugins",
    items: [
      { label: "PostgreSQL", icon: "devicon:postgresql" },
      { label: "Trino", icon: "simple-icons:trino" },
      { label: "Kafka", icon: "devicon:apachekafka", kafkaIcon: true },
      { label: "S3", icon: "logos:aws-s3" },
      { label: "dbt", icon: "simple-icons:dbt" },
      { label: "Iceberg", icon: "simple-icons:apacheiceberg" },
    ],
  },
  {
    title: "Populate",
    items: [
      { label: "Terraform", iconImg: "/img/terraform.svg" },
      { label: "Pulumi", iconImg: "/img/pulumi.svg" },
      { label: "API", icon: "mdi:api" },
      { label: "CLI", icon: "mdi:console" },
    ],
  },
];

const consumerGroups: FlowGroup[] = [
  {
    title: "AI Agents",
    items: [
      { label: "Claude", icon: "simple-icons:anthropic" },
      { label: "Cursor", icon: "simple-icons:cursor" },
      { label: "Windsurf", icon: "simple-icons:codeium" },
      { label: "Copilot", icon: "simple-icons:githubcopilot" },
      { label: "Gemini", icon: "simple-icons:googlegemini" },
      { label: "ChatGPT", icon: "simple-icons:openai" },
    ],
  },
  {
    title: "Integrations",
    items: [
      { label: "REST API", icon: "mdi:api" },
      { label: "CLI", icon: "mdi:console" },
      { label: "Slack", icon: "simple-icons:slack" },
    ],
  },
];

const GroupCard: React.FC<{
  group: FlowGroup;
  visible: boolean;
  delay: number;
  iconsOnly?: boolean;
}> = ({ group, visible, delay, iconsOnly }) => {
  const gridCols = group.items.length > 4 ? "grid-cols-3" : "grid-cols-2";

  return (
    <div
      className={`rounded-xl border border-gray-200 dark:border-gray-700/40 bg-white/70 dark:bg-gray-800/50 ${iconsOnly ? "p-3" : "p-4"}`}
      style={{
        opacity: visible ? 1 : 0,
        transform: visible ? "translateY(0)" : "translateY(12px)",
        transition: "opacity 0.5s ease, transform 0.5s ease",
        transitionDelay: `${delay}ms`,
      }}
    >
      <p className={`font-semibold uppercase tracking-widest text-gray-400 dark:text-gray-500 ${iconsOnly ? "text-[9px] mb-2 text-center" : "text-[10px] mb-3"}`}>
        {group.title}
      </p>
      {iconsOnly ? (
        <div className="flex items-center justify-center gap-3 flex-wrap">
          {group.items.map((item) => (
            <div key={item.label} title={item.label}>
              {item.iconImg ? (
                <img
                  src={item.iconImg}
                  alt={item.label}
                  className="w-6 h-6 object-contain"
                />
              ) : (
                <Icon
                  icon={item.icon!}
                  className={`w-6 h-6 text-gray-500 dark:text-gray-400 ${item.kafkaIcon ? "kafka-icon" : ""}`}
                />
              )}
            </div>
          ))}
        </div>
      ) : (
        <div className={`grid ${gridCols} gap-3`}>
          {group.items.map((item) => (
            <div key={item.label} className="flex items-center gap-2">
              {item.iconImg ? (
                <img
                  src={item.iconImg}
                  alt={item.label}
                  className="w-5 h-5 object-contain"
                />
              ) : (
                <Icon
                  icon={item.icon!}
                  className={`w-5 h-5 ${item.kafkaIcon ? "kafka-icon" : ""}`}
                />
              )}
              <span className="text-xs text-gray-600 dark:text-gray-400 whitespace-nowrap">
                {item.label}
              </span>
            </div>
          ))}
        </div>
      )}
    </div>
  );
};

const leftPaths = [
  "M 0 30 C 60 30, 40 50, 100 50",
  "M 0 70 C 60 70, 40 50, 100 50",
];

const rightPaths = [
  "M 0 50 C 60 50, 40 30, 100 30",
  "M 0 50 C 60 50, 40 70, 100 70",
];

export default function ContextFlow(): JSX.Element {
  const wrapperRef = useRef<HTMLDivElement>(null);
  const timeoutsRef = useRef<ReturnType<typeof setTimeout>[]>([]);
  const [phase, setPhase] = useState(0);

  useEffect(() => {
    const el = wrapperRef.current;
    if (!el) return;

    const prefersReduced = window.matchMedia(
      "(prefers-reduced-motion: reduce)",
    ).matches;

    if (prefersReduced) {
      setPhase(5);
      return;
    }

    const observer = new IntersectionObserver(
      (entries) => {
        for (const entry of entries) {
          if (entry.isIntersecting) {
            observer.disconnect();
            const schedule = (ms: number, p: number) => {
              const id = setTimeout(() => setPhase(p), ms);
              timeoutsRef.current.push(id);
            };
            schedule(200, 1);
            schedule(900, 2);
            schedule(1400, 3);
            schedule(1800, 4);
            schedule(2300, 5);
            return;
          }
        }
      },
      { threshold: 0.3 },
    );
    observer.observe(el);

    return () => {
      observer.disconnect();
      timeoutsRef.current.forEach(clearTimeout);
      timeoutsRef.current = [];
    };
  }, []);

  return (
    <section className="pt-24 pb-24 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900 gradient-mesh-hero">
      <div ref={wrapperRef} className="max-w-7xl mx-auto">
        {/* Desktop layout */}
        <div className="hidden lg:flex items-center">
          {/* Left: source groups */}
          <div className="flex flex-col gap-4 flex-shrink-0">
            {sourceGroups.map((group, i) => (
              <GroupCard
                key={group.title}
                group={group}
                visible={phase >= 1}
                delay={i * 150}
              />
            ))}
          </div>

          {/* Left connector - converging lines */}
          <div
            className="flex-1 relative min-w-[4rem] self-stretch"
            style={{
              opacity: phase >= 2 ? 1 : 0,
              transition: "opacity 0.5s ease",
            }}
          >
            <svg
              className="absolute inset-0 w-full h-full text-earthy-terracotta-300/60 dark:text-earthy-terracotta-600/40"
              viewBox="0 0 100 100"
              preserveAspectRatio="none"
              overflow="visible"
            >
              {leftPaths.map((d, i) => (
                <React.Fragment key={i}>
                  {/* Glow */}
                  <path
                    d={d}
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={6}
                    strokeLinecap="round"
                    vectorEffect="non-scaling-stroke"
                    className="animate-flow-glow"
                    style={{ filter: "blur(4px)", animationDelay: `${i * 0.8}s` }}
                  />
                  {/* Dashed line */}
                  <path
                    d={d}
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={1.5}
                    strokeDasharray="4 3"
                    vectorEffect="non-scaling-stroke"
                    className="animate-flow"
                  />
                </React.Fragment>
              ))}
            </svg>
            <span className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 text-[10px] font-medium text-gray-400 dark:text-gray-500 bg-earthy-brown-50 dark:bg-gray-900 px-2 py-0.5 rounded whitespace-nowrap">
              Populate
            </span>
          </div>

          {/* Hub card */}
          <div
            className={`flex-shrink-0 self-center flex flex-col items-center gap-2 px-8 py-6 rounded-2xl border border-earthy-terracotta-200 dark:border-earthy-terracotta-700/50 bg-white dark:bg-gray-800 shadow-lg ${phase >= 3 ? "hub-pulse" : ""}`}
            style={{
              opacity: phase >= 3 ? 1 : 0,
              transform: phase >= 3 ? "scale(1)" : "scale(0.85)",
              transition: "opacity 0.5s ease, transform 0.5s ease",
            }}
          >
            <img src="/img/marmot.svg" alt="Marmot" className="w-14 h-14" />
            <span className="text-sm font-bold text-gray-700 dark:text-gray-300">
              Marmot
            </span>
            <span className="text-[10px] font-medium uppercase tracking-widest text-gray-400 dark:text-gray-500">
              Context layer
            </span>
          </div>

          {/* Right connector - diverging lines */}
          <div
            className="flex-1 relative min-w-[4rem] self-stretch"
            style={{
              opacity: phase >= 4 ? 1 : 0,
              transition: "opacity 0.5s ease",
            }}
          >
            <svg
              className="absolute inset-0 w-full h-full text-earthy-terracotta-300/60 dark:text-earthy-terracotta-600/40"
              viewBox="0 0 100 100"
              preserveAspectRatio="none"
              overflow="visible"
            >
              {rightPaths.map((d, i) => (
                <React.Fragment key={i}>
                  {/* Glow */}
                  <path
                    d={d}
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={6}
                    strokeLinecap="round"
                    vectorEffect="non-scaling-stroke"
                    className="animate-flow-glow"
                    style={{ filter: "blur(4px)", animationDelay: `${1.2 + i * 0.8}s` }}
                  />
                  {/* Dashed line */}
                  <path
                    d={d}
                    fill="none"
                    stroke="currentColor"
                    strokeWidth={1.5}
                    strokeDasharray="4 3"
                    vectorEffect="non-scaling-stroke"
                    className="animate-flow"
                  />
                </React.Fragment>
              ))}
            </svg>
            <span className="absolute top-1/2 left-1/2 -translate-x-1/2 -translate-y-1/2 text-[10px] font-medium text-gray-400 dark:text-gray-500 bg-earthy-brown-50 dark:bg-gray-900 px-2 py-0.5 rounded whitespace-nowrap">
              Discover
            </span>
          </div>

          {/* Right: consumer groups */}
          <div className="flex flex-col gap-4 flex-shrink-0">
            {consumerGroups.map((group, i) => (
              <GroupCard
                key={group.title}
                group={group}
                visible={phase >= 5}
                delay={i * 150}
              />
            ))}
          </div>
        </div>

        {/* Mobile layout — grouped cards, vertical flow */}
        <div className="lg:hidden flex flex-col items-center gap-0">
          {/* Source cards */}
          <div className="grid grid-cols-2 gap-3 w-full max-w-sm">
            {sourceGroups.map((group, i) => (
              <GroupCard
                key={group.title}
                group={group}
                visible={phase >= 1}
                delay={i * 150}
                iconsOnly
              />
            ))}
          </div>

          {/* Vertical connector + label */}
          <div
            className="flex flex-col items-center"
            style={{
              opacity: phase >= 2 ? 1 : 0,
              transition: "opacity 0.5s ease",
            }}
          >
            <svg width="2" height="20" overflow="visible" className="text-earthy-terracotta-300/60 dark:text-earthy-terracotta-600/40">
              <line x1="1" y1="0" x2="1" y2="20" stroke="currentColor" strokeWidth={1.5} strokeDasharray="4 3" className="animate-flow" />
              <line x1="1" y1="0" x2="1" y2="20" stroke="currentColor" strokeWidth={6} strokeLinecap="round" className="animate-flow-glow" style={{ filter: "blur(4px)" }} />
            </svg>
            <span className="text-[9px] font-medium text-gray-400 dark:text-gray-500 py-1">
              Populate
            </span>
            <svg width="2" height="20" overflow="visible" className="text-earthy-terracotta-300/60 dark:text-earthy-terracotta-600/40">
              <line x1="1" y1="0" x2="1" y2="20" stroke="currentColor" strokeWidth={1.5} strokeDasharray="4 3" className="animate-flow" />
              <line x1="1" y1="0" x2="1" y2="20" stroke="currentColor" strokeWidth={6} strokeLinecap="round" className="animate-flow-glow" style={{ filter: "blur(4px)", animationDelay: "0.8s" }} />
            </svg>
          </div>

          {/* Hub card */}
          <div
            className={`flex items-center gap-3 px-5 py-3 rounded-xl border border-earthy-terracotta-200 dark:border-earthy-terracotta-700/50 bg-white dark:bg-gray-800 shadow-md ${phase >= 3 ? "hub-pulse" : ""}`}
            style={{
              opacity: phase >= 3 ? 1 : 0,
              transform: phase >= 3 ? "scale(1)" : "scale(0.9)",
              transition: "opacity 0.4s ease, transform 0.4s ease",
            }}
          >
            <img src="/img/marmot.svg" alt="Marmot" className="w-10 h-10" />
            <div className="flex flex-col">
              <span className="text-sm font-bold text-gray-700 dark:text-gray-300 leading-tight">
                Marmot
              </span>
              <span className="text-[9px] font-medium uppercase tracking-widest text-gray-400 dark:text-gray-500">
                Context layer
              </span>
            </div>
          </div>

          {/* Vertical connector + label */}
          <div
            className="flex flex-col items-center"
            style={{
              opacity: phase >= 4 ? 1 : 0,
              transition: "opacity 0.5s ease",
            }}
          >
            <svg width="2" height="20" overflow="visible" className="text-earthy-terracotta-300/60 dark:text-earthy-terracotta-600/40">
              <line x1="1" y1="0" x2="1" y2="20" stroke="currentColor" strokeWidth={1.5} strokeDasharray="4 3" className="animate-flow" />
              <line x1="1" y1="0" x2="1" y2="20" stroke="currentColor" strokeWidth={6} strokeLinecap="round" className="animate-flow-glow" style={{ filter: "blur(4px)", animationDelay: "1.2s" }} />
            </svg>
            <span className="text-[9px] font-medium text-gray-400 dark:text-gray-500 py-1">
              Discover
            </span>
            <svg width="2" height="20" overflow="visible" className="text-earthy-terracotta-300/60 dark:text-earthy-terracotta-600/40">
              <line x1="1" y1="0" x2="1" y2="20" stroke="currentColor" strokeWidth={1.5} strokeDasharray="4 3" className="animate-flow" />
              <line x1="1" y1="0" x2="1" y2="20" stroke="currentColor" strokeWidth={6} strokeLinecap="round" className="animate-flow-glow" style={{ filter: "blur(4px)", animationDelay: "2s" }} />
            </svg>
          </div>

          {/* Consumer cards */}
          <div className="grid grid-cols-2 gap-3 w-full max-w-sm">
            {consumerGroups.map((group, i) => (
              <GroupCard
                key={group.title}
                group={group}
                visible={phase >= 5}
                delay={i * 150}
                iconsOnly
              />
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
