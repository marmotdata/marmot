import React from "react";
import { Icon } from "@iconify/react";

interface Node {
  label: string;
  icon?: string;
  iconImg?: string;
  kafkaIcon?: boolean;
}

interface Group {
  label: string;
  items: Node[];
}

const sourceGroups: Group[] = [
  {
    label: "Databases",
    items: [
      { label: "PostgreSQL", icon: "devicon:postgresql" },
      { label: "MySQL", icon: "devicon:mysql" },
      { label: "MongoDB", icon: "devicon:mongodb" },
      { label: "ClickHouse", icon: "devicon:clickhouse" },
      { label: "Redis", icon: "devicon:redis" },
    ],
  },
  {
    label: "Warehouses & lakes",
    items: [
      { label: "BigQuery", icon: "devicon:googlecloud" },
      { label: "Snowflake", icon: "simple-icons:snowflake" },
      { label: "S3", icon: "logos:aws-s3" },
      { label: "Iceberg", iconImg: "/img/iceberg.svg" },
      { label: "Delta Lake", iconImg: "/img/deltalake.svg" },
    ],
  },
  {
    label: "Streaming & pipelines",
    items: [
      { label: "Kafka", icon: "devicon:apachekafka", kafkaIcon: true },
      { label: "Redpanda", iconImg: "/img/redpanda.svg" },
      { label: "NATS", icon: "devicon:nats" },
      { label: "Airflow", icon: "logos:airflow-icon" },
      { label: "dbt", icon: "logos:dbt-icon" },
    ],
  },
  {
    label: "Push from code",
    items: [
      { label: "CLI", icon: "mdi:console" },
      { label: "Terraform", icon: "simple-icons:terraform" },
      { label: "Pulumi", icon: "simple-icons:pulumi" },
      { label: "REST API", icon: "mdi:api" },
      { label: "SDK", icon: "mdi:code-braces" },
    ],
  },
];

const consumerGroups: Group[] = [
  {
    label: "AI agents",
    items: [
      { label: "Claude", icon: "simple-icons:anthropic" },
      { label: "Cursor", icon: "simple-icons:cursor" },
      { label: "Copilot", icon: "simple-icons:githubcopilot" },
      { label: "ChatGPT", icon: "simple-icons:openai" },
    ],
  },
  {
    label: "Engineers",
    items: [
      { label: "Web UI", icon: "mdi:monitor-dashboard" },
      { label: "REST API", icon: "mdi:api" },
      { label: "CLI", icon: "mdi:console" },
      { label: "Slack", icon: "simple-icons:slack" },
    ],
  },
];

const stores = [
  {
    label: "Assets",
    icon: "mdi:database-outline",
    desc: "Every table, topic, bucket",
  },
  {
    label: "Lineage",
    icon: "mdi:graph-outline",
    desc: "How data connects",
  },
  {
    label: "Ownership",
    icon: "mdi:account-outline",
    desc: "Who to ask",
  },
  {
    label: "Glossary",
    icon: "mdi:book-open-page-variant-outline",
    desc: "What the terms mean",
  },
];

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

function Chip({ node }: { node: Node }) {
  return (
    <div className="inline-flex items-center gap-1.5 px-2 py-1 rounded-md bg-white/80 dark:bg-gray-900/50 border border-earthy-brown-200/60 dark:border-gray-700">
      <NodeIcon node={node} className="w-3.5 h-3.5 flex-shrink-0" />
      <span className="text-[11px] font-medium text-gray-700 dark:text-gray-300 whitespace-nowrap">
        {node.label}
      </span>
    </div>
  );
}

function IconChip({ node }: { node: Node }) {
  return (
    <div
      className="inline-flex items-center justify-center w-6 h-6 rounded-md bg-white/80 dark:bg-gray-900/50 border border-earthy-brown-200/60 dark:border-gray-700"
      title={node.label}
    >
      <NodeIcon node={node} className="w-3.5 h-3.5" />
    </div>
  );
}

function GroupCard({
  group,
  iconsOnly,
}: {
  group: Group;
  iconsOnly?: boolean;
}) {
  return (
    <div className="rounded-xl border border-earthy-brown-200/70 dark:border-gray-700 bg-white/60 dark:bg-gray-800/50 backdrop-blur-sm px-3 py-2 shadow-sm shadow-earthy-terracotta-900/5">
      <p className="text-[9px] font-semibold uppercase tracking-[0.16em] text-earthy-terracotta-700 dark:text-earthy-terracotta-400 mb-1.5">
        {group.label}
      </p>
      <div
        className={`flex flex-wrap items-center ${iconsOnly ? "gap-1" : "gap-1.5"}`}
      >
        {group.items.map((item) =>
          iconsOnly ? (
            <IconChip key={item.label} node={item} />
          ) : (
            <Chip key={item.label} node={item} />
          )
        )}
      </div>
    </div>
  );
}

function RowLabel({ children }: { children: React.ReactNode }) {
  return (
    <div className="hidden sm:flex items-center">
      <span className="text-[10px] font-semibold uppercase tracking-[0.16em] text-gray-400 dark:text-gray-500 leading-tight">
        {children}
      </span>
    </div>
  );
}

function MarmotHub() {
  return (
    <div className="relative rounded-xl border-2 border-earthy-terracotta-400/70 dark:border-earthy-terracotta-500/60 bg-white dark:bg-gray-800 p-4 sm:p-5 shadow-sm">
      <div className="flex items-center gap-3 mb-4">
        <img
          src="/img/marmot.svg"
          alt="Marmot"
          className="w-10 h-10 flex-shrink-0"
        />
        <div className="flex-1 min-w-0">
          <p className="text-[10px] font-semibold uppercase tracking-[0.16em] text-earthy-terracotta-600 dark:text-earthy-terracotta-400 leading-tight">
            Context layer
          </p>
          <h3 className="m-0 text-base sm:text-lg font-bold text-gray-900 dark:text-white leading-tight">
            Marmot
          </h3>
        </div>
      </div>
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-2">
        {stores.map((s) => (
          <div
            key={s.label}
            className="rounded-md bg-earthy-brown-50 dark:bg-gray-900/50 border border-earthy-brown-200/60 dark:border-gray-700 px-3 py-2.5"
          >
            <div className="flex items-center gap-1.5 mb-1">
              <Icon
                icon={s.icon}
                className="w-3.5 h-3.5 text-earthy-terracotta-600 dark:text-earthy-terracotta-400"
              />
              <span className="text-[12px] font-semibold text-gray-800 dark:text-gray-200">
                {s.label}
              </span>
            </div>
            <p className="m-0 text-[10px] text-gray-500 dark:text-gray-400 leading-tight">
              {s.desc}
            </p>
          </div>
        ))}
      </div>
    </div>
  );
}

/** Vertical connectors between Marmot and a row of groups. Straight dashed
 * lines with small anchor dots at each end, and a traveling "packet" dot
 * rising up each line — staggered per line so packets arrive in a cascade
 * rather than in lock-step. */
function VerticalConnectors({
  count,
}: {
  count: number;
  mode: "consumers" | "sources";
}) {
  return (
    <svg
      className="absolute inset-0 w-full h-full text-earthy-terracotta-400/70 dark:text-earthy-terracotta-500/50 overflow-visible"
      viewBox="0 0 100 100"
      preserveAspectRatio="none"
      aria-hidden="true"
    >
      {Array.from({ length: count }, (_, i) => {
        const x = ((i + 0.5) / count) * 100;
        // Stagger packet start times & durations per line for organic flow.
        const dur = 2.8 + i * 0.6;
        const delay = -i * 0.9;

        return (
          <g key={i}>
            {/* Base dashed line */}
            <line
              x1={x}
              y1={100}
              x2={x}
              y2={0}
              stroke="currentColor"
              strokeWidth={1}
              strokeDasharray="5 4"
              vectorEffect="non-scaling-stroke"
              className="animate-context-flow"
            />
            {/* Anchor dots at endpoints — small enough not to look like
                horizontal elements when the viewBox is stretched. */}
            <circle cx={x} cy={4} r={1.2} fill="currentColor" opacity={0.9} />
            <circle cx={x} cy={96} r={1.2} fill="currentColor" opacity={0.9} />
            {/* Traveling packet — rises bottom → top, fades in/out */}
            <circle cx={x} r={1.8} fill="currentColor">
              <animate
                attributeName="cy"
                values="96;4"
                dur={`${dur}s`}
                begin={`${delay}s`}
                repeatCount="indefinite"
              />
              <animate
                attributeName="opacity"
                values="0;1;1;0"
                keyTimes="0;0.15;0.85;1"
                dur={`${dur}s`}
                begin={`${delay}s`}
                repeatCount="indefinite"
              />
            </circle>
          </g>
        );
      })}
    </svg>
  );
}

export default function ContextDiagram(): JSX.Element {
  return (
    <section className="pt-2 pb-16 sm:pt-4 sm:pb-20 px-4 sm:px-6 lg:px-8">
      <div
        data-animate
        className="max-w-6xl mx-auto rounded-2xl border border-earthy-brown-200/70 dark:border-gray-700/70 bg-white/50 dark:bg-gray-900/40 backdrop-blur-sm shadow-sm shadow-earthy-terracotta-900/5 p-5 sm:p-8"
      >
        <div className="grid grid-cols-1 sm:grid-cols-[5rem_1fr] lg:grid-cols-[7rem_1fr] gap-x-6 gap-y-3">
          {/* Row: Consumers */}
          <RowLabel>Interfaces &amp; agents</RowLabel>
          <div className="grid grid-cols-1 sm:grid-cols-4 gap-3 items-stretch">
            {consumerGroups.map((g) => (
              <div key={g.label} className="sm:col-span-2">
                <GroupCard group={g} />
              </div>
            ))}
          </div>

          {/* Connectors: consumers → marmot */}
          <div />
          <div className="relative w-full h-10">
            <VerticalConnectors
              count={consumerGroups.length}
              mode="consumers"
            />
          </div>

          {/* Row: Marmot (full-width) */}
          <RowLabel>Marmot catalog</RowLabel>
          <MarmotHub />

          {/* Connectors: sources → marmot */}
          <div />
          <div className="relative w-full h-10">
            <VerticalConnectors count={sourceGroups.length} mode="sources" />
          </div>

          {/* Row: Sources */}
          <RowLabel>Sources &amp; ingest</RowLabel>
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 items-stretch">
            {sourceGroups.map((g) => (
              <GroupCard key={g.label} group={g} iconsOnly />
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
