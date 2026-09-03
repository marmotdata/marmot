import React from "react";
import { Icon } from "@iconify/react";
import { agents } from "./agents";

/* Two ways in, one catalog, many ways out.
 *
 * The diagram carries the shape: push from code on the left, Marmot in the
 * middle, engineers and agents on the right. The strip underneath carries the
 * breadth: the plugin ecosystem, grouped the way the plugin docs group it.
 *
 * Geometry note: the wire SVGs render at exactly WIRE_W x BODY_H with a
 * matching viewBox, and the chip columns are BODY_H tall with space-between,
 * so nothing scales and every curve endpoint lands on a chip centre by
 * construction. The connectors this replaces drifted because they stretched a
 * square viewBox across a wide box.
 */

const WIRE_W = 140;
const BODY_H = 380;
const CHIP_H = 52;

/* The ecosystem strip is capped at 62rem, and the wide layout only runs when
 * the figure is wider than that, so the strip is always exactly this wide
 * there. That lets the feed curves be fixed-size and still land dead on each
 * category's centre. */
const ECO_W = 1080;
const ECO_H = 80;

// Chip centres, derived from `space-between` over a BODY_H column.
const centres = (n: number) =>
  Array.from({ length: n }, (_, i) => i * ((BODY_H - CHIP_H) / (n - 1)) + CHIP_H / 2);

interface Chip {
  label: string;
  icon: string;
  iconImg?: string;
  /** Generic tool glyph rather than a brand mark: inherits ink, not colour. */
  glyph?: boolean;
  /** Brand colour for monochrome marks that would otherwise render as ink. */
  iconColor?: string;
  kafkaIcon?: boolean;
}

const pushFromCode: Chip[] = [
  { label: "CLI", icon: "mdi:console", glyph: true },
  { label: "Terraform", icon: "logos:terraform-icon" },
  { label: "Pulumi", icon: "logos:pulumi-icon" },
  { label: "REST API", icon: "mdi:api", glyph: true },
  { label: "SDK", icon: "mdi:code-braces", glyph: true },
];

const consumers: Chip[] = [
  { label: "MCP server", icon: "mdi:protocol", glyph: true },
  { label: "Web UI", icon: "mdi:monitor-dashboard", glyph: true },
  { label: "REST API", icon: "mdi:api", glyph: true },
  { label: "CLI", icon: "mdi:console", glyph: true },
];

const stores = [
  { label: "Assets", icon: "mdi:database-outline" },
  { label: "Lineage", icon: "mdi:graph-outline" },
  { label: "Ownership", icon: "mdi:account-outline" },
  { label: "Glossary", icon: "mdi:book-open-page-variant-outline" },
];

/** Grouped the same way the plugin docs group them. */
const categories: { label: string; items: Chip[] }[] = [
  {
    label: "Databases",
    items: [
      { label: "PostgreSQL", icon: "devicon:postgresql" },
      { label: "MySQL", icon: "logos:mysql-icon" },
      { label: "MongoDB", icon: "logos:mongodb-icon" },
      { label: "ClickHouse", icon: "devicon:clickhouse" },
      { label: "Redis", icon: "logos:redis" },
    ],
  },
  {
    label: "Warehouses & lakes",
    items: [
      { label: "BigQuery", icon: "simple-icons:googlebigquery", iconColor: "#4386fa" },
      { label: "Snowflake", icon: "logos:snowflake-icon" },
      { label: "S3", icon: "logos:aws-s3" },
      { label: "Iceberg", icon: "", iconImg: "/img/iceberg.svg" },
      { label: "Delta Lake", icon: "", iconImg: "/img/deltalake.svg" },
    ],
  },
  {
    label: "Streaming & pipelines",
    items: [
      { label: "Kafka", icon: "devicon:apachekafka", kafkaIcon: true },
      { label: "Redpanda", icon: "", iconImg: "/img/redpanda.svg" },
      { label: "NATS", icon: "logos:nats-icon" },
      { label: "Airflow", icon: "logos:airflow-icon" },
      { label: "dbt", icon: "logos:dbt-icon" },
    ],
  },
  {
    label: "Compute",
    items: [
      { label: "Kubernetes", icon: "logos:kubernetes" },
      { label: "AWS", icon: "simple-icons:amazonwebservices" },
      { label: "Google Cloud", icon: "logos:google-cloud" },
      { label: "Azure", icon: "logos:microsoft-azure" },
    ],
  },
];

// One feed curve per category, converging on the hub above.
const ECO_X = categories.map(
  (_, i) => ((i * 2 + 1) * ECO_W) / (categories.length * 2)
);

function ChipIcon({ chip, className }: { chip: Chip; className: string }) {
  if (chip.iconImg) {
    return <img src={chip.iconImg} alt="" className={`${className} object-contain`} aria-hidden="true" />;
  }
  return (
    <Icon
      icon={chip.icon}
      className={`${className}${chip.kafkaIcon ? " kafka-icon" : ""}${chip.glyph ? " cl-glyph" : ""}`}
      style={chip.iconColor ? { color: chip.iconColor } : undefined}
      aria-hidden="true"
    />
  );
}

function ChipRow({ chip }: { chip: Chip }) {
  return (
    <div className="cl-chip">
      <ChipIcon chip={chip} className="cl-chip-icon" />
      <span className="cl-chip-label">{chip.label}</span>
    </div>
  );
}

function Hub() {
  return (
    <div className="cl-hub">
      <div className="cl-hub-head">
        <img src="/img/marmot.svg" alt="" className="cl-hub-mascot" aria-hidden="true" />
        <img src="/img/marmot-text.svg" alt="Marmot" className="cl-hub-wordmark dark:hidden" />
        <img
          src="/img/marmot-text-light.svg"
          alt=""
          aria-hidden="true"
          className="cl-hub-wordmark hidden dark:inline"
        />
      </div>
      <ul className="cl-hub-list m-0">
        {stores.map((s) => (
          <li key={s.label} className="cl-hub-item">
            <Icon icon={s.icon} className="cl-hub-item-icon" aria-hidden="true" />
            <span className="cl-hub-item-title">{s.label}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

function Wires({ n, dir }: { n: number; dir: "in" | "out" }) {
  const ys = centres(n);
  const inbound = dir === "in";
  const mid = BODY_H / 2;
  return (
    <svg
      width={WIRE_W}
      height={BODY_H}
      viewBox={`0 0 ${WIRE_W} ${BODY_H}`}
      className="cl-wire"
      aria-hidden="true"
    >
      <clipPath id={`cl-wipe-${dir}`}>
        <rect className={`cl-wipe cl-wipe-${dir}`} x="0" y="0" width={WIRE_W} height={BODY_H} />
      </clipPath>
      <g clipPath={`url(#cl-wipe-${dir})`}>
        {ys.map((y, i) => {
          const d = inbound
            ? `M0,${y} C${WIRE_W * 0.45},${y} ${WIRE_W * 0.55},${mid} ${WIRE_W},${mid}`
            : `M0,${mid} C${WIRE_W * 0.45},${mid} ${WIRE_W * 0.55},${y} ${WIRE_W},${y}`;
          return (
            <g key={i}>
              <path d={d} className="cl-line-base" fill="none" stroke="var(--cl-accent)" strokeWidth={1.75} strokeLinecap="round" />
              <path
                d={d}
                className="cl-line-flow"
                fill="none"
                stroke="var(--cl-accent)"
                strokeWidth={2.25}
                strokeLinecap="round"
                style={{ animationDelay: `${(i * 0.42).toFixed(2)}s` }}
              />
            </g>
          );
        })}
        {inbound ? (
          <path d={`M${WIRE_W - 8},${mid - 5.5} L${WIRE_W},${mid} L${WIRE_W - 8},${mid + 5.5} z`} fill="var(--cl-accent)" />
        ) : (
          ys.map((y, i) => (
            <path key={i} d={`M${WIRE_W - 8},${y - 5.5} L${WIRE_W},${y} L${WIRE_W - 8},${y + 5.5} z`} fill="var(--cl-accent)" />
          ))
        )}
      </g>
    </svg>
  );
}

function Ecosystem() {
  return (
    <div className="cl-eco">
      {/* These sync into Marmot too, so each category feeds the hub with the
          same converging curve the code paths use. */}
      <svg
        className="cl-eco-feed"
        width={ECO_W}
        height={ECO_H}
        viewBox={`0 0 ${ECO_W} ${ECO_H}`}
        aria-hidden="true"
      >
        {ECO_X.map((x, i) => {
          const d =
            x === ECO_W / 2
              ? `M${x},${ECO_H} V16`
              : `M${x},${ECO_H} C${x},56 ${ECO_W / 2},48 ${ECO_W / 2},16`;
          return (
            <g key={i}>
              <path d={d} className="cl-line-base" fill="none" stroke="var(--cl-accent)" strokeWidth={1.75} strokeLinecap="round" />
              <path
                d={d}
                className="cl-line-flow"
                fill="none"
                stroke="var(--cl-accent)"
                strokeWidth={2.25}
                strokeLinecap="round"
                style={{ animationDelay: `${(i * 0.5).toFixed(2)}s` }}
              />
            </g>
          );
        })}
        <path
          d={`M${ECO_W / 2 - 5.5},17 L${ECO_W / 2},5 L${ECO_W / 2 + 5.5},17 z`}
          fill="var(--cl-accent)"
        />
      </svg>

      <div className="cl-eco-grid">
        {categories.map((c) => (
          <div key={c.label} className="cl-eco-group">
            <p className="cl-eco-label m-0">{c.label}</p>
            <div className="cl-eco-icons">
              {c.items.map((item) => (
                <span key={item.label} className="cl-eco-icon" title={item.label}>
                  <ChipIcon chip={item} className="cl-eco-glyph" />
                </span>
              ))}
            </div>
          </div>
        ))}
      </div>
      {/* Narrow: one arrow down into the hub, which sits below the strip once
          stacked. Wide uses the converging curves above instead. */}
      <svg className="cl-eco-feed-narrow" width="16" height="46" viewBox="0 0 16 46" aria-hidden="true">
        <path d="M8,0 V32" stroke="var(--cl-accent)" strokeWidth="1.75" fill="none" strokeLinecap="round" opacity="0.9" />
        <path d="M2.5,31 L8,43 L13.5,31 z" fill="var(--cl-accent)" />
      </svg>
    </div>
  );
}

export default function ContextDiagram(): JSX.Element {
  return (
    <section className="pt-2 pb-16 sm:pt-4 sm:pb-20 px-4 sm:px-6 lg:px-8">
      <figure data-animate className="cl-figure">
        {/* Wide layout: push paths converge, interfaces diverge. */}
        <div className="cl-grid">
          <p className="cl-eyebrow cl-head-in m-0">Manage via</p>
          <p className="cl-eyebrow cl-head-out m-0">Query via</p>

          <div className="cl-col">
            {pushFromCode.map((c) => (
              <ChipRow key={c.label} chip={c} />
            ))}
          </div>

          <Wires n={pushFromCode.length} dir="in" />
          <Hub />
          <Wires n={consumers.length} dir="out" />

          <div className="cl-col cl-col-out">
            <div className="cl-row">
              <ChipRow chip={consumers[0]} />
              <span className="cl-agent-tick" aria-hidden="true" />
              <div className="cl-agents">
                {agents.map((a) => (
                  <span key={a.label} className="agent-mark" title={a.label}>
                    <ChipIcon chip={a} className="agent-mark-icon" />
                  </span>
                ))}
              </div>
            </div>
            {consumers.slice(1).map((c) => (
              <ChipRow key={c.label} chip={c} />
            ))}
          </div>
        </div>

        {/* Narrow layout, split around the ecosystem strip. Plugins are an
            input, so stacked they belong above the hub with the code paths,
            not below the query column. */}
        <div className="cl-stack cl-stack-in">
          <p className="cl-eyebrow m-0">Manage via</p>
          <div className="cl-stack-chips">
            {pushFromCode.map((c) => (
              <ChipRow key={c.label} chip={c} />
            ))}
          </div>
        </div>

        <Ecosystem />

        <div className="cl-stack cl-stack-out">
          <Hub />
          <span className="cl-stack-conn" aria-hidden="true" />
          <p className="cl-eyebrow m-0">Query via</p>
          <div className="cl-stack-chips">
            {consumers.map((c) => (
              <ChipRow key={c.label} chip={c} />
            ))}
          </div>
          <div className="cl-agents cl-agents-stack">
            {agents.map((a) => (
              <span key={a.label} className="agent-mark" title={a.label}>
                <ChipIcon chip={a} className="agent-mark-icon" />
              </span>
            ))}
          </div>
          <p className="cl-agents-note m-0">via MCP</p>
        </div>
      </figure>
    </section>
  );
}
