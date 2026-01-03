import React, { useState } from "react";
import { Icon } from "@iconify/react";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";

interface Plugin {
  name: string;
  description: string;
  href: string;
  icon: string;
  useLocalIcon?: boolean;
}

const plugins: Plugin[] = [
  {
    name: "Airflow",
    description: "Ingest DAGs, tasks, and dataset lineage from Apache Airflow",
    href: "/docs/Plugins/Airflow",
    icon: "logos:airflow-icon",
  },
  {
    name: "AsyncAPI",
    description: "Discover services, topics, and queues from AsyncAPI specifications",
    href: "/docs/Plugins/AsyncAPI",
    icon: "asyncapi",
    useLocalIcon: true,
  },
  {
    name: "Azure Blob Storage",
    description: "Discover containers from Azure Blob Storage accounts",
    href: "/docs/Plugins/Azure%20Blob%20Storage",
    icon: "logos:azure-icon",
  },
  {
    name: "BigQuery",
    description: "Catalog datasets, tables, and views from Google BigQuery projects",
    href: "/docs/Plugins/BigQuery",
    icon: "devicon:googlecloud",
  },
  {
    name: "ClickHouse",
    description: "Discover databases, tables, and views from ClickHouse instances",
    href: "/docs/Plugins/ClickHouse",
    icon: "simple-icons:clickhouse",
  },
  {
    name: "DBT",
    description: "Ingest models, sources, seeds, and lineage from dbt projects",
    href: "/docs/Plugins/DBT",
    icon: "simple-icons:dbt",
  },
  {
    name: "Kafka",
    description: "Catalog topics from Apache Kafka clusters with Schema Registry integration",
    href: "/docs/Plugins/Kafka",
    icon: "devicon:apachekafka",
  },
  {
    name: "MongoDB",
    description: "Discover databases and collections from MongoDB instances",
    href: "/docs/Plugins/MongoDB",
    icon: "devicon:mongodb",
  },
  {
    name: "MySQL",
    description: "Discover databases and tables from MySQL instances",
    href: "/docs/Plugins/MySQL",
    icon: "devicon:mysql",
  },
  {
    name: "OpenAPI",
    description: "Discover services and endpoints from OpenAPI v3 specifications",
    href: "/docs/Plugins/OpenAPI",
    icon: "simple-icons:openapiinitiative",
  },
  {
    name: "PostgreSQL",
    description: "Discover tables, views, and relationships from PostgreSQL databases",
    href: "/docs/Plugins/PostgreSQL",
    icon: "devicon:postgresql",
  },
  {
    name: "S3",
    description: "Catalog buckets from Amazon S3",
    href: "/docs/Plugins/S3",
    icon: "logos:aws-s3",
  },
  {
    name: "SNS",
    description: "Catalog topics from Amazon SNS",
    href: "/docs/Plugins/SNS",
    icon: "logos:aws-sns",
  },
  {
    name: "SQS",
    description: "Discover queues from Amazon SQS",
    href: "/docs/Plugins/SQS",
    icon: "logos:aws-sqs",
  },
];

function PluginIcon({ plugin, isDarkTheme }: { plugin: Plugin; isDarkTheme: boolean }) {
  if (plugin.useLocalIcon) {
    const iconSrc = isDarkTheme ? `/img/dark-${plugin.icon}.svg` : `/img/${plugin.icon}.svg`;
    return <img src={iconSrc} alt={`${plugin.name} icon`} className="w-8 h-8" />;
  }

  return (
    <Icon
      icon={plugin.icon}
      className={`w-8 h-8 ${plugin.name === "Kafka" ? "kafka-icon" : ""}`}
    />
  );
}

export default function PluginCards(): JSX.Element {
  const [search, setSearch] = useState("");
  const { siteConfig } = useDocusaurusContext();

  // Check for dark theme
  const isDarkTheme = typeof document !== "undefined" &&
    document.documentElement.getAttribute("data-theme") === "dark";

  const filteredPlugins = plugins.filter(
    (plugin) =>
      plugin.name.toLowerCase().includes(search.toLowerCase()) ||
      plugin.description.toLowerCase().includes(search.toLowerCase())
  );

  return (
    <div className="mt-6">
      <div className="relative mb-5">
        <Icon
          icon="mdi:magnify"
          className="absolute left-3 top-1/2 -translate-y-1/2 w-4 h-4 text-gray-400 dark:text-gray-500"
        />
        <input
          type="text"
          placeholder="Search plugins..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="w-full pl-9 pr-4 py-2 text-sm bg-white dark:bg-gray-900 border border-gray-200 dark:border-gray-700 rounded-lg text-gray-900 dark:text-white placeholder-gray-400 dark:placeholder-gray-500 focus:outline-none focus:border-[var(--ifm-color-primary)] focus:ring-1 focus:ring-[var(--ifm-color-primary)] transition-colors"
        />
      </div>

      {filteredPlugins.length === 0 ? (
        <div className="text-center py-8 text-gray-500 dark:text-gray-400">
          No plugins found matching "{search}"
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {filteredPlugins.map((plugin) => (
            <a
              key={plugin.name}
              href={plugin.href}
              className="group block p-5 bg-gray-50 dark:bg-gray-800 rounded-xl border border-gray-200 dark:border-gray-700 hover:border-[var(--ifm-color-primary)] dark:hover:border-[var(--ifm-color-primary)] hover:shadow-lg transition-all no-underline"
            >
              <div className="flex items-start gap-4">
                <div className="flex-shrink-0 p-2 bg-white dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 group-hover:border-[var(--ifm-color-primary)] transition-colors">
                  <PluginIcon plugin={plugin} isDarkTheme={isDarkTheme} />
                </div>
                <div className="flex-1 min-w-0">
                  <h3 className="text-base font-semibold text-gray-900 dark:text-white m-0 group-hover:text-[var(--ifm-color-primary)] transition-colors">
                    {plugin.name}
                  </h3>
                  <p className="mt-1 text-sm text-gray-600 dark:text-gray-400 m-0 line-clamp-2">
                    {plugin.description}
                  </p>
                </div>
              </div>
            </a>
          ))}
        </div>
      )}
    </div>
  );
}
