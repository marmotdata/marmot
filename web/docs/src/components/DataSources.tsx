import React from "react";
import { Icon } from "@iconify/react";
import Link from "@docusaurus/Link";
import { useDocsVersionCandidates } from "@docusaurus/plugin-content-docs/client";
import { plugins, type Plugin } from "./PluginCards";

// Order of chips shown on the homepage. Curated across categories
// (databases, warehouses/lakes, cloud storage, streaming, orchestration,
// compute, search) so the grid reads as breadth. Every entry must exist
// in the plugins array exported from PluginCards.
const homepageDocIds = [
  "Plugins/PostgreSQL",
  "Plugins/MySQL",
  "Plugins/MongoDB",
  "Plugins/ClickHouse",
  "Plugins/DynamoDB",
  "Plugins/BigQuery",
  "Plugins/DuckDB",
  "Plugins/Trino",
  "Plugins/Iceberg",
  "Plugins/Delta Lake",
  "Plugins/S3",
  "Plugins/Google Cloud Storage",
  "Plugins/Azure Blob Storage",
  "Plugins/Kafka",
  "Plugins/Redpanda",
  "Plugins/NATS",
  "Plugins/Confluent Cloud",
  "Plugins/Airflow",
  "Plugins/DBT",
  "Plugins/Lambda",
  "Plugins/Kubernetes",
  "Plugins/EKS",
  "Plugins/Elasticsearch",
  "Plugins/OpenSearch",
];

const displayNameOverrides: Record<string, string> = {
  "Plugins/DBT": "dbt",
  "Plugins/EKS": "EKS",
};

function ChipIcon({ plugin, isDarkTheme }: { plugin: Plugin; isDarkTheme: boolean }) {
  if (plugin.useLocalIcon) {
    const ext = plugin.icon.includes(".") ? "" : ".svg";
    const useDark = isDarkTheme && plugin.hasDarkIcon;
    const iconSrc = useDark ? `/img/dark-${plugin.icon}${ext}` : `/img/${plugin.icon}${ext}`;
    return <img src={iconSrc} alt="" className="w-5 h-5" />;
  }

  return (
    <Icon
      icon={plugin.icon}
      className={`w-5 h-5 ${plugin.name === "Kafka" ? "kafka-icon" : ""}`}
    />
  );
}

export default function DataSources(): JSX.Element {
  const versionCandidates = useDocsVersionCandidates("default");

  const resolveHref = (docId: string): string => {
    for (const version of versionCandidates) {
      const doc = version.docs.find((d) => d.id === docId);
      if (doc) return doc.path;
    }
    return "#";
  };

  const isDarkTheme =
    typeof document !== "undefined" &&
    document.documentElement.getAttribute("data-theme") === "dark";

  const chips = homepageDocIds
    .map((docId) => plugins.find((p) => p.docId === docId))
    .filter((p): p is Plugin => Boolean(p));

  return (
    <section className="py-20 sm:py-24 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900 border-t border-dashed border-earthy-brown-200/70 dark:border-gray-800/70">
      <div className="max-w-4xl mx-auto">
        <div data-animate className="text-center">
          <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white tracking-tight">
            Plugins for what you already run
          </h2>
          <p className="mt-4 text-lg text-gray-500 dark:text-gray-400">
            {plugins.length} of them and counting, covering the databases,
            warehouses, lakes, streams and orchestrators your team already uses.
          </p>
        </div>

        <div
          data-animate
          data-animate-delay="1"
          className="mt-10 flex flex-wrap items-center justify-center gap-2.5"
        >
          {chips.map((plugin) => (
            <Link
              key={plugin.docId}
              to={resolveHref(plugin.docId)}
              className="plugin-chip group inline-flex items-center gap-2 px-3.5 py-2 rounded-lg bg-white dark:bg-gray-900/40 no-underline"
            >
              <ChipIcon plugin={plugin} isDarkTheme={isDarkTheme} />
              <span className="text-sm font-medium text-gray-700 dark:text-gray-300 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors">
                {displayNameOverrides[plugin.docId] ?? plugin.name}
              </span>
            </Link>
          ))}
        </div>

        <p
          data-animate
          data-animate-delay="2"
          className="mt-9 text-center text-sm text-gray-500 dark:text-gray-400"
        >
          <a
            href="/docs/Plugins/"
            className="text-earthy-terracotta-700 dark:text-earthy-terracotta-400 font-semibold hover:underline"
          >
            View all {plugins.length} plugins
          </a>{" "}
          or{" "}
          <a
            href="https://github.com/marmotdata/marmot/issues/new"
            target="_blank"
            rel="noopener noreferrer"
            className="text-earthy-terracotta-600 dark:text-earthy-terracotta-400 hover:underline"
          >
            ask for one
          </a>{" "}
          that isn't here yet.
        </p>
      </div>
    </section>
  );
}
