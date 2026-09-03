import React from "react";
import { Icon } from "@iconify/react";
import { plugins, type Plugin } from "./PluginCards";

const REGISTRY = "https://plugins.marmotdata.io";

/* Registry slugs are the doc name lowercased with spaces removed, except for
   these three. Verified against the slug list plugins.marmotdata.io serves. */
const registrySlugOverrides: Record<string, string> = {
  "Plugins/Azure Blob Storage": "azureblob",
  "Plugins/Confluent Cloud": "confluent",
  "Plugins/Google Cloud Storage": "gcs",
};

/* #usage selects the Usage tab on the registry page, which is the part
   someone clicking a plugin from the homepage actually wants. */
const registryHref = (docId: string): string => {
  const slug =
    registrySlugOverrides[docId] ??
    docId.replace(/^Plugins\//, "").toLowerCase().replace(/\s+/g, "");
  return `${REGISTRY}/marmotdata/${slug}#usage`;
};

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
            Covering the databases, warehouses, lakes, streams{" "}
            <br className="hidden sm:inline" />
            and orchestrators your team already uses, and counting.
          </p>
        </div>

        <div
          data-animate
          data-animate-delay="1"
          className="mt-10 flex flex-wrap items-center justify-center gap-2.5"
        >
          {chips.map((plugin) => (
            <a
              key={plugin.docId}
              href={registryHref(plugin.docId)}
              target="_blank"
              rel="noopener noreferrer"
              className="plugin-chip group inline-flex items-center gap-2 px-3.5 py-2 rounded-lg bg-white dark:bg-gray-900/40 no-underline"
            >
              <ChipIcon plugin={plugin} isDarkTheme={isDarkTheme} />
              <span className="text-sm font-medium text-gray-700 dark:text-gray-300 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors">
                {displayNameOverrides[plugin.docId] ?? plugin.name}
              </span>
            </a>
          ))}
        </div>

        <p
          data-animate
          data-animate-delay="2"
          className="mt-9 text-center text-lg text-gray-500 dark:text-gray-400"
        >
          <a
            href={REGISTRY}
            target="_blank"
            rel="noopener noreferrer"
            className="text-earthy-terracotta-700 dark:text-earthy-terracotta-400 font-semibold hover:underline"
          >
            View all plugins
          </a>{" "}
          or{" "}
          <a
            href="https://github.com/marmotdata/marmot/issues/new"
            target="_blank"
            rel="noopener noreferrer"
            className="text-earthy-terracotta-700 dark:text-earthy-terracotta-400 font-semibold hover:underline"
          >
            ask for one
          </a>{" "}
          that isn't here yet.
        </p>
      </div>
    </section>
  );
}
