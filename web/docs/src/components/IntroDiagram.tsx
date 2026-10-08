import React from "react";

/* A small, quiet picture of the shape: sources on the left, one catalog in
 * the middle, people and agents on the right. Inline SVG so it stays crisp
 * and takes its colours from the theme. */

const W = 720;
const H = 200;
const BOX_W = 160;
const BOX_H = 34;
const GAP = 12;
const LEFT_X = 20;
const RIGHT_X = W - 20 - BOX_W;
const HUB_X = W / 2 - 90;
const HUB_W = 180;
const HUB_H = 128;
const MID = H / 2;

const sources = ["Plugins", "Terraform and Pulumi", "CLI, SDK and API"];
const consumers = ["Web UI", "REST API and CLI", "MCP for agents"];

function rowY(i: number, n: number): number {
  const total = n * BOX_H + (n - 1) * GAP;
  return MID - total / 2 + i * (BOX_H + GAP) + BOX_H / 2;
}

function Box({ x, y, label }: { x: number; y: number; label: string }) {
  return (
    <g>
      <rect
        x={x}
        y={y - BOX_H / 2}
        width={BOX_W}
        height={BOX_H}
        rx={8}
        fill="var(--ifm-background-surface-color)"
        stroke="var(--ifm-color-emphasis-300)"
      />
      <text
        x={x + BOX_W / 2}
        y={y}
        textAnchor="middle"
        dominantBaseline="central"
        fontSize={12.5}
        fill="var(--ifm-font-color-base)"
      >
        {label}
      </text>
    </g>
  );
}

function wire(x1: number, y1: number, x2: number, y2: number): string {
  const c = (x1 + x2) / 2;
  return `M${x1},${y1} C${c},${y1} ${c},${y2} ${x2},${y2}`;
}

export default function IntroDiagram(): JSX.Element {
  const inX = LEFT_X + BOX_W;
  const outX = RIGHT_X;
  const hubL = HUB_X;
  const hubR = HUB_X + HUB_W;
  const stroke = "var(--ifm-color-primary)";
  return (
    <figure style={{ margin: "1.5rem auto", maxWidth: `${W}px` }}>
      <svg
        viewBox={`0 0 ${W} ${H}`}
        width="100%"
        role="img"
        aria-label="Sources feed the Marmot catalog; people and agents query it through the UI, the API and MCP"
        fontFamily="var(--ifm-font-family-base)"
      >
        <defs>
          <marker id="intro-arrow" viewBox="0 0 8 8" refX="7" refY="4" markerWidth="6" markerHeight="6" orient="auto">
            <path d="M0,0 L8,4 L0,8 z" fill={stroke} />
          </marker>
        </defs>

        <text x={LEFT_X + BOX_W / 2} y={16} textAnchor="middle" fontSize={11} fontWeight={600} letterSpacing="0.08em" fill="var(--ifm-color-emphasis-600)">
          POPULATE
        </text>
        <text x={RIGHT_X + BOX_W / 2} y={16} textAnchor="middle" fontSize={11} fontWeight={600} letterSpacing="0.08em" fill="var(--ifm-color-emphasis-600)">
          QUERY
        </text>

        {sources.map((s, i) => {
          const y = rowY(i, sources.length);
          return (
            <g key={s}>
              <Box x={LEFT_X} y={y} label={s} />
              <path d={wire(inX, y, hubL - 2, MID + (i - 1) * 18)} fill="none" stroke={stroke} strokeWidth={1.25} opacity={0.7} markerEnd="url(#intro-arrow)" />
            </g>
          );
        })}

        {consumers.map((c, i) => {
          const y = rowY(i, consumers.length);
          return (
            <g key={c}>
              <path d={wire(hubR, MID + (i - 1) * 18, outX - 2, y)} fill="none" stroke={stroke} strokeWidth={1.25} opacity={0.7} markerEnd="url(#intro-arrow)" />
              <Box x={RIGHT_X} y={y} label={c} />
            </g>
          );
        })}

        <g>
          <rect
            x={HUB_X}
            y={MID - HUB_H / 2}
            width={HUB_W}
            height={HUB_H}
            rx={12}
            fill="var(--ifm-background-surface-color)"
            stroke={stroke}
            strokeWidth={1.5}
          />
          <text x={W / 2} y={MID - HUB_H / 2 + 26} textAnchor="middle" fontSize={15} fontWeight={700} fill="var(--ifm-font-color-base)">
            Marmot
          </text>
          <line x1={HUB_X + 16} x2={HUB_X + HUB_W - 16} y1={MID - HUB_H / 2 + 40} y2={MID - HUB_H / 2 + 40} stroke="var(--ifm-color-emphasis-300)" />
          {["Assets and schemas", "Ownership and glossary", "Lineage", "Tags and custom fields"].map((t, i) => (
            <text key={t} x={W / 2} y={MID - HUB_H / 2 + 60 + i * 18} textAnchor="middle" fontSize={12} fill="var(--ifm-color-emphasis-800)">
              {t}
            </text>
          ))}
        </g>

        <text x={W / 2} y={H - 8} textAnchor="middle" fontSize={11} fill="var(--ifm-color-emphasis-600)">
          One Go binary, backed by PostgreSQL
        </text>
      </svg>
    </figure>
  );
}
