import React from "react";
import ContextDiagram from "./ContextDiagram";

export default function Hero(): JSX.Element {
  return (
    <header className="relative pt-32 pb-8 sm:pt-40 sm:pb-10 lg:pt-44 lg:pb-12 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900 gradient-mesh-hero overflow-hidden">
      <LineageBackdrop />

      {/* Warm breathing underglow at the bottom edge */}
      <div
        aria-hidden
        className="absolute inset-x-0 bottom-0 h-48 pointer-events-none animate-hero-warmth"
        style={{
          background:
            "radial-gradient(ellipse 60% 100% at 50% 100%, rgba(247,136,94,0.4) 0%, transparent 70%)",
        }}
      />

      <div className="relative max-w-6xl mx-auto text-center">
        <h1
          data-animate
          className="text-4xl sm:text-5xl md:text-6xl lg:text-7xl font-extrabold text-gray-900 dark:text-white tracking-tight leading-[1.05]"
          style={{ textWrap: "balance" }}
        >
          AI agents are only as good as{" "}
          <span className="gradient-text">the context they can reach.</span>
        </h1>

        <p
          data-animate
          data-animate-delay="2"
          className="mt-8 text-lg sm:text-xl text-gray-500 dark:text-gray-400 max-w-3xl mx-auto leading-relaxed"
        >
          Marmot is an open source context layer for engineers and AI agents.
          It tracks schemas, ownership and lineage across your entire stack.
        </p>

        <div
          data-animate
          data-animate-delay="3"
          className="mt-10 flex flex-row items-center justify-center gap-3"
        >
          <a
            href="/docs/introduction"
            className="group inline-flex items-center justify-center px-7 py-3.5 text-sm font-semibold rounded-xl text-white bg-earthy-terracotta-700 hover:bg-earthy-terracotta-800 shadow-sm hover:shadow-md transition-all duration-200 hover:-translate-y-0.5"
          >
            Get started
            <svg
              className="w-4 h-4 ml-2 transition-transform duration-200 group-hover:translate-x-0.5"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                strokeWidth={2}
                d="M13 7l5 5m0 0l-5 5m5-5H6"
              />
            </svg>
          </a>
          <a
            href="https://demo.marmotdata.io"
            target="_blank"
            rel="noopener noreferrer"
            className="group demo-btn inline-flex items-center justify-center px-7 py-3.5 text-sm font-semibold rounded-xl text-gray-700 dark:text-gray-300 bg-white/70 dark:bg-gray-800/50 transition-all duration-200 hover:-translate-y-0.5"
          >
            Live demo
          </a>
        </div>
      </div>

      {/* Architecture diagram, part of the same warm-bg header unit */}
      <div className="relative mt-8">
        <ContextDiagram />
      </div>
    </header>
  );
}

/* Scattered lineage-mesh backdrop. Hand-placed clusters of dots + monospace
   service labels + short dashed curves. Not on a grid, so it reads as an
   organic mesh rather than wallpaper. Faded to sit well behind the content. */
type Cluster = {
  dots: [number, number][];
  labels: [number, number, string][];
  edges?: string[];
  opacity?: number;
};

const backdropClusters: Cluster[] = [
  // Top band
  {
    dots: [[60, 40], [230, 60]],
    labels: [[68, 43, "postgres"], [238, 63, "kafka"]],
    edges: ["M 78 42 C 130 44, 175 55, 220 60"],
  },
  {
    dots: [[410, 30]],
    labels: [[418, 33, "s3"]],
    opacity: 0.55,
  },
  {
    dots: [[580, 78], [720, 55]],
    labels: [[588, 81, "dbt"], [728, 58, "bigquery"]],
    edges: ["M 594 76 C 640 65, 680 60, 712 58"],
  },
  {
    dots: [[1050, 75], [1220, 45]],
    labels: [[1058, 78, "airflow"], [1228, 48, "trino"]],
    edges: ["M 1066 73 C 1130 60, 1180 50, 1212 48"],
  },
  {
    dots: [[1320, 100]],
    labels: [[1328, 103, "redis"]],
    opacity: 0.5,
  },
  // Upper-middle band
  {
    dots: [[100, 220]],
    labels: [[108, 223, "mongo"]],
    opacity: 0.65,
  },
  {
    dots: [[280, 260], [450, 240]],
    labels: [[288, 263, "mysql"], [458, 243, "elastic"]],
    edges: ["M 298 258 C 350 250, 400 244, 442 242"],
  },
  {
    dots: [[1050, 230]],
    labels: [[1058, 233, "clickhouse"]],
    opacity: 0.55,
  },
  {
    dots: [[1220, 260], [1330, 220]],
    labels: [[1228, 263, "duckdb"], [1338, 223, "iceberg"]],
    edges: ["M 1238 258 C 1270 245, 1300 228, 1322 222"],
  },
  // Lower-middle band
  {
    dots: [[80, 440]],
    labels: [[88, 443, "nats"]],
    opacity: 0.6,
  },
  {
    dots: [[240, 480], [400, 460]],
    labels: [[248, 483, "glue"], [408, 463, "lambda"]],
    edges: ["M 258 478 C 310 470, 360 464, 392 462"],
  },
  {
    dots: [[900, 490]],
    labels: [[908, 493, "redpanda"]],
    opacity: 0.55,
  },
  {
    dots: [[1080, 460], [1240, 490]],
    labels: [[1088, 463, "postgres"], [1248, 493, "kafka"]],
    edges: ["M 1098 462 C 1150 476, 1200 486, 1232 490"],
  },
  // Bottom band
  {
    dots: [[150, 680], [320, 700]],
    labels: [[158, 683, "s3"], [328, 703, "dbt"]],
    edges: ["M 168 682 C 220 690, 270 698, 312 700"],
  },
  {
    dots: [[600, 720]],
    labels: [[608, 723, "bigquery"]],
    opacity: 0.5,
  },
  {
    dots: [[830, 690], [990, 720]],
    labels: [[838, 693, "snowflake"], [998, 723, "airflow"]],
    edges: ["M 848 690 C 900 702, 950 716, 982 720"],
  },
  {
    dots: [[1180, 700]],
    labels: [[1188, 703, "trino"]],
    opacity: 0.55,
  },
];

function LineageBackdrop(): JSX.Element {
  return (
    <div
      aria-hidden
      className="absolute inset-0 pointer-events-none text-earthy-terracotta-700/25 dark:text-earthy-terracotta-400/20"
      style={{
        maskImage:
          "radial-gradient(ellipse 55% 45% at 50% 42%, transparent 10%, black 85%)",
        WebkitMaskImage:
          "radial-gradient(ellipse 55% 45% at 50% 42%, transparent 10%, black 85%)",
      }}
    >
      <svg
        viewBox="0 0 1400 800"
        preserveAspectRatio="xMidYMid slice"
        className="w-full h-full"
      >
        {backdropClusters.map((c, i) => (
          <g key={i} opacity={c.opacity ?? 1}>
            {(c.edges ?? []).map((d, j) => (
              <path
                key={j}
                d={d}
                fill="none"
                stroke="currentColor"
                strokeWidth="0.7"
                strokeDasharray="3 3"
                strokeLinecap="round"
              />
            ))}
            {c.dots.map(([x, y], j) => (
              <circle key={j} cx={x} cy={y} r={1.6} fill="currentColor" />
            ))}
            {c.labels.map(([x, y, text], j) => (
              <text
                key={j}
                x={x}
                y={y}
                fill="currentColor"
                fontFamily="ui-monospace, SFMono-Regular, Menlo, Consolas, monospace"
                fontSize="8"
                fontWeight="500"
              >
                {text}
              </text>
            ))}
          </g>
        ))}
      </svg>
    </div>
  );
}
