import React, { useEffect } from "react";
import Wordmark from "@marmotdata/tokens/logos/marmot-wordmark.svg";
import Terminal from "~icons/material-symbols/terminal";
import Api from "~icons/material-symbols/api";
import DataObject from "~icons/material-symbols/data-object";
import Hub from "~icons/material-symbols/hub-outline";
import Monitor from "~icons/material-symbols/desktop-windows-outline";
import Sql from "~icons/material-symbols/database-outline";
import Table from "~icons/material-symbols/table-outline";
import Lineage from "~icons/material-symbols/account-tree-outline";
import Person from "~icons/material-symbols/person-outline";
import Book from "~icons/material-symbols/menu-book-outline";
import Shield from "~icons/material-symbols/shield-outline";
import Terraform from "~icons/logos/terraform-icon";
import Pulumi from "~icons/logos/pulumi-icon";
import Anthropic from "~icons/simple-icons/anthropic";
import OpenAI from "~icons/simple-icons/openai";
import Gemini from "~icons/simple-icons/googlegemini";
import Cursor from "~icons/simple-icons/cursor";
import Copilot from "~icons/simple-icons/githubcopilot";

/* Two ways in, one catalog, many ways out: code pushes on the left, Marmot in the middle, people
 * and agents on the right, and underneath the plugins that keep it current.
 *
 * Geometry: the wires render at exactly WIRE_W × BODY_H with a matching viewBox, and the chip
 * columns are BODY_H tall with space-between, so nothing scales and every curve lands on a chip's
 * centre by construction. The plugin strip is ECO_W wide in the wide layout for the same reason.
 */
const WIRE_W = 140;
const BODY_H = 380;
const CHIP_H = 52;
const ECO_W = 1080;
const ECO_H = 80;

const centres = (n: number) => Array.from({ length: n }, (_, i) => i * ((BODY_H - CHIP_H) / (n - 1)) + CHIP_H / 2);

type IconType = React.ComponentType<{ "aria-hidden"?: string; className?: string }>;
interface Chip {
  label: string;
  Icon: IconType;
  /** A tool glyph rather than a brand mark: drawn in ink-2, so only real products carry colour. */
  glyph?: boolean;
}

// Four a side, so the picture stays calm and the assistants fit in the gap under MCP.
const manage: Chip[] = [
  { label: "CLI", Icon: Terminal, glyph: true },
  { label: "Terraform", Icon: Terraform },
  { label: "Pulumi", Icon: Pulumi },
  { label: "SDK", Icon: DataObject, glyph: true },
];

const query: Chip[] = [
  { label: "MCP server", Icon: Hub, glyph: true },
  { label: "Web app", Icon: Monitor, glyph: true },
  { label: "SQL", Icon: Sql, glyph: true },
  { label: "REST API", Icon: Api, glyph: true },
];

const holds: { label: string; Icon: IconType }[] = [
  { label: "Assets", Icon: Table },
  { label: "Lineage", Icon: Lineage },
  { label: "Ownership", Icon: Person },
  { label: "Glossary", Icon: Book },
  { label: "Access and audit", Icon: Shield },
];

// The assistants that read the catalog over MCP. Monochrome marks, so they sit quietly in ink.
const agents: { label: string; Icon: IconType }[] = [
  { label: "Claude", Icon: Anthropic },
  { label: "ChatGPT", Icon: OpenAI },
  { label: "Gemini", Icon: Gemini },
  { label: "Cursor", Icon: Cursor },
  { label: "Copilot", Icon: Copilot },
];

// Grouped the way the plugin docs group them; provider keys from @marmotdata/icons.
const plugins: { label: string; items: [string, string][] }[] = [
  { label: "Databases", items: [["postgresql", "PostgreSQL"], ["mysql", "MySQL"], ["mongodb", "MongoDB"], ["clickhouse", "ClickHouse"]] },
  { label: "Warehouses and lakes", items: [["snowflake", "Snowflake"], ["bigquery", "BigQuery"], ["s3", "S3"], ["iceberg", "Iceberg"]] },
  { label: "Streams and pipelines", items: [["kafka", "Kafka"], ["redpanda", "Redpanda"], ["airflow", "Airflow"], ["dbt", "dbt"]] },
  { label: "Cloud and compute", items: [["kubernetes", "Kubernetes"], ["lambda", "Lambda"], ["cloudrun", "Cloud Run"], ["gke", "GKE"]] },
];
const ECO_X = plugins.map((_, i) => ((i * 2 + 1) * ECO_W) / (plugins.length * 2));

function ChipRow({ chip }: { chip: Chip }) {
  return (
    <div className="mkt-dia__chip">
      <chip.Icon aria-hidden="true" className={chip.glyph ? "mkt-dia__glyph" : "mkt-dia__mark"} />
      <span className="mkt-dia__chip-label">{chip.label}</span>
    </div>
  );
}

function Agents({ className = "" }: { className?: string }) {
  return (
    <ul className={`mkt-dia__agents ${className}`} aria-label="Assistants that connect over MCP">
      {agents.map(({ label, Icon }) => (
        <li key={label} title={label}>
          <Icon aria-hidden="true" />
          <span className="mm-visually-hidden">{label}</span>
        </li>
      ))}
    </ul>
  );
}

function HubCard() {
  return (
    <div className="mkt-dia__hub">
      <div className="mkt-dia__hub-head">
        <span className="mm-wordmark" role="img" aria-label="Marmot" style={{ "--mm-wordmark-size": "32px" } as React.CSSProperties}>
          <Wordmark aria-hidden="true" />
        </span>
      </div>
      <ul className="mkt-dia__hub-list">
        {holds.map(({ label, Icon }) => (
          <li key={label}>
            <Icon aria-hidden="true" />
            <span>{label}</span>
          </li>
        ))}
      </ul>
    </div>
  );
}

/** Curves between a column of chips and the hub's middle: converging inwards, diverging out. */
function Wires({ n, dir }: { n: number; dir: "in" | "out" }) {
  const ys = centres(n);
  const mid = BODY_H / 2;
  return (
    <svg width={WIRE_W} height={BODY_H} viewBox={`0 0 ${WIRE_W} ${BODY_H}`} className="mkt-dia__wire" aria-hidden="true">
      {ys.map((y, i) => {
        const d =
          dir === "in"
            ? `M0,${y} C${WIRE_W * 0.45},${y} ${WIRE_W * 0.55},${mid} ${WIRE_W},${mid}`
            : `M0,${mid} C${WIRE_W * 0.45},${mid} ${WIRE_W * 0.55},${y} ${WIRE_W},${y}`;
        return (
          <g key={i}>
            <path d={d} className="mkt-dia__line" />
            <path d={d} className="mkt-dia__flow" pathLength={100} style={{ animationDelay: `${(i * 0.42).toFixed(2)}s` }} />
            {/* A dot where each line leaves its chip, as a leader line on a drawing has. */}
            <circle cx={dir === "in" ? 0 : WIRE_W} cy={y} r={2.5} className="mkt-dia__dot" />
          </g>
        );
      })}
      {dir === "in" ? (
        <path d={`M${WIRE_W - 7},${mid - 4.5} L${WIRE_W},${mid} L${WIRE_W - 7},${mid + 4.5} z`} className="mkt-dia__head" />
      ) : null}
    </svg>
  );
}

function Plugins() {
  return (
    <div className="mkt-dia__eco">
      <svg className="mkt-dia__feed" width={ECO_W} height={ECO_H} viewBox={`0 0 ${ECO_W} ${ECO_H}`} aria-hidden="true">
        {ECO_X.map((x, i) => {
          const d = `M${x},${ECO_H} C${x},56 ${ECO_W / 2},48 ${ECO_W / 2},14`;
          return (
            <g key={i}>
              <path d={d} className="mkt-dia__line" />
              <path d={d} className="mkt-dia__flow" pathLength={100} style={{ animationDelay: `${(i * 0.5).toFixed(2)}s` }} />
            </g>
          );
        })}
        <path d={`M${ECO_W / 2 - 4.5},15 L${ECO_W / 2},7 L${ECO_W / 2 + 4.5},15 z`} className="mkt-dia__head" />
      </svg>
      <div className="mkt-dia__eco-grid">
        {plugins.map((g) => (
          <div key={g.label} className="mkt-dia__eco-group">
            <p className="mkt-dia__eco-label">{g.label}</p>
            <ul className="mkt-dia__eco-icons">
              {g.items.map(([key, name]) => (
                <li key={key} title={name}>
                  <mm-provider-logo provider={key} size="24" decorative></mm-provider-logo>
                  <span className="mm-visually-hidden">{name}</span>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
      <p className="mkt-dia__eco-more">
        and more than 60 plugins
      </p>
    </div>
  );
}

export default function ContextMap(): JSX.Element {
  useEffect(() => {
    void import("@marmotdata/icons/define.js");
  }, []);
  return (
    <figure
      className="mkt-dia"
      aria-label="How Marmot works: you manage it from code, plugins keep it current, and people and agents query it"
    >
      {/* Wide: the code paths converge on Marmot, the ways out diverge from it. */}
      <div className="mkt-dia__grid">
        <p className="mkt-dia__label mkt-dia__label--in">Manage via</p>
        <p className="mkt-dia__label mkt-dia__label--out">Query via</p>
        <div className="mkt-dia__col">
          {manage.map((c) => (
            <ChipRow key={c.label} chip={c} />
          ))}
        </div>
        <Wires n={manage.length} dir="in" />
        <HubCard />
        <Wires n={query.length} dir="out" />
        <div className="mkt-dia__col mkt-dia__col--out">
          <div className="mkt-dia__row">
            <ChipRow chip={query[0]!} />
            <Agents className="mkt-dia__agents--under" />
          </div>
          {query.slice(1).map((c) => (
            <ChipRow key={c.label} chip={c} />
          ))}
        </div>
      </div>

      {/* Narrow: the same story top to bottom. Plugins are an input, so they sit above the hub. */}
      <div className="mkt-dia__stack">
        <p className="mkt-dia__label">Manage via</p>
        <div className="mkt-dia__stack-chips">
          {manage.map((c) => (
            <ChipRow key={c.label} chip={c} />
          ))}
        </div>
      </div>

      <Plugins />

      <div className="mkt-dia__stack">
        <HubCard />
        <span className="mkt-dia__stack-line" aria-hidden="true" />
        <p className="mkt-dia__label">Query via</p>
        <div className="mkt-dia__stack-chips">
          {query.map((c) => (
            <ChipRow key={c.label} chip={c} />
          ))}
        </div>
        <Agents />
      </div>
    </figure>
  );
}
