import React, { useState } from "react";
import { Icon } from "@iconify/react";
import Link from "@docusaurus/Link";
import useDocusaurusContext from "@docusaurus/useDocusaurusContext";
import { useDocsVersionCandidates } from "@docusaurus/plugin-content-docs/client";

interface Plugin {
  name: string;
  description: string;
  docId: string;
  icon: string;
  useLocalIcon?: boolean;
  hasDarkIcon?: boolean;
}

const plugins: Plugin[] = [
  {
    name: "Airflow",
    description: "Ingest DAGs, tasks, and dataset lineage from Apache Airflow",
    docId: "Plugins/Airflow",
    icon: "logos:airflow-icon",
  },
  {
    name: "AsyncAPI",
    description: "Discover services, topics, and queues from AsyncAPI specifications",
    docId: "Plugins/AsyncAPI",
    icon: "asyncapi",
    useLocalIcon: true,
    hasDarkIcon: true,
  },
  {
    name: "Azure Blob Storage",
    description: "Discover containers from Azure Blob Storage accounts",
    docId: "Plugins/Azure Blob Storage",
    icon: "logos:azure-icon",
  },
  {
    name: "BigQuery",
    description: "Catalog datasets, tables, and views from Google BigQuery projects",
    docId: "Plugins/BigQuery",
    icon: "devicon:googlecloud",
  },
  {
    name: "ClickHouse",
    description: "Discover databases, tables, and views from ClickHouse instances",
    docId: "Plugins/ClickHouse",
    icon: "devicon:clickhouse",
  },
  {
    name: "Confluent Cloud",
    description: "Discover Kafka topics from Confluent Cloud clusters",
    docId: "Plugins/Confluent Cloud",
    icon: "confluent.png",
    useLocalIcon: true,
  },
  {
    name: "DBT",
    description: "Ingest models, sources, seeds, and lineage from dbt projects",
    docId: "Plugins/DBT",
    icon: "logos:dbt-icon",
  },
  {
    name: "Delta Lake",
    description: "Discover tables from Delta Lake transaction logs on local filesystems",
    docId: "Plugins/Delta Lake",
    icon: "deltalake",
    useLocalIcon: true,
  },
  {
    name: "DuckDB",
    description: "Discover schemas, tables, views, and relationships from DuckDB database files",
    docId: "Plugins/DuckDB",
    icon: "devicon:duckdb",
  },
  {
    name: "DynamoDB",
    description: "Discover tables from Amazon DynamoDB",
    docId: "Plugins/DynamoDB",
    icon: "logos:aws-dynamodb",
  },
  {
    name: "Elasticsearch",
    description: "Discover indices, data streams, and aliases from Elasticsearch clusters",
    docId: "Plugins/Elasticsearch",
    icon: "logos:elasticsearch",
  },
  {
    name: "Google Cloud Storage",
    description: "Discover buckets from Google Cloud Storage",
    docId: "Plugins/Google Cloud Storage",
    icon: "logos:google-cloud",
  },
  {
    name: "Glue",
    description: "Discover jobs, databases, tables and crawlers from AWS Glue",
    docId: "Plugins/Glue",
    icon: "logos:aws-glue",
  },
  {
    name: "Iceberg",
    description: "Discover namespaces, tables and views from Iceberg catalogs (REST and AWS Glue)",
    docId: "Plugins/Iceberg",
    icon: "iceberg",
    useLocalIcon: true,
  },
  {
    name: "Lambda",
    description: "Discover functions from AWS Lambda",
    docId: "Plugins/Lambda",
    icon: "logos:aws-lambda",
  },
  {
    name: "Kafka",
    description: "Catalog topics from Apache Kafka clusters with Schema Registry integration",
    docId: "Plugins/Kafka",
    icon: "devicon:apachekafka",
  },
  {
    name: "Kubernetes",
    description: "Discover namespaces, services, workloads, and cron jobs from self-managed Kubernetes clusters",
    docId: "Plugins/Kubernetes",
    icon: "devicon:kubernetes",
  },
  {
    name: "Amazon EKS",
    description: "Discover namespaces, services, workloads, and cron jobs from Amazon EKS clusters",
    docId: "Plugins/EKS",
    icon: "logos:aws-eks",
  },
  {
    name: "Google GKE",
    description: "Discover namespaces, services, workloads, and cron jobs from Google GKE clusters",
    docId: "Plugins/GKE",
    icon: "logos:google-icon",
  },
  {
    name: "MongoDB",
    description: "Discover databases and collections from MongoDB instances",
    docId: "Plugins/MongoDB",
    icon: "devicon:mongodb",
  },
  {
    name: "MySQL",
    description: "Discover databases and tables from MySQL instances",
    docId: "Plugins/MySQL",
    icon: "devicon:mysql",
  },
  {
    name: "NATS",
    description: "Discover JetStream streams from NATS servers",
    docId: "Plugins/NATS",
    icon: "devicon:nats",
  },
  {
    name: "OpenSearch",
    description: "Discover indices, data streams, and aliases from OpenSearch clusters",
    docId: "Plugins/OpenSearch",
    icon: "logos:opensearch-icon",
  },
  {
    name: "OpenAPI",
    description: "Discover services and endpoints from OpenAPI v3 specifications",
    docId: "Plugins/OpenAPI",
    icon: "devicon:openapi",
  },
  {
    name: "PostgreSQL",
    description: "Discover tables, views, and relationships from PostgreSQL databases",
    docId: "Plugins/PostgreSQL",
    icon: "devicon:postgresql",
  },
  {
    name: "Redis",
    description: "Discover databases from Redis instances",
    docId: "Plugins/Redis",
    icon: "devicon:redis",
  },
  {
    name: "Redpanda",
    description: "Discover topics from Redpanda clusters",
    docId: "Plugins/Redpanda",
    icon: "redpanda",
    useLocalIcon: true,
  },
  {
    name: "S3",
    description: "Catalog buckets from Amazon S3",
    docId: "Plugins/S3",
    icon: "logos:aws-s3",
  },
  {
    name: "SNS",
    description: "Catalog topics from Amazon SNS",
    docId: "Plugins/SNS",
    icon: "logos:aws-sns",
  },
  {
    name: "SQS",
    description: "Discover queues from Amazon SQS",
    docId: "Plugins/SQS",
    icon: "logos:aws-sqs",
  },
  {
    name: "Trino",
    description: "Discover catalogs, schemas, tables and views from Trino clusters",
    docId: "Plugins/Trino",
    icon: "simple-icons:trino",
  },
];

function PluginIcon({ plugin, isDarkTheme }: { plugin: Plugin; isDarkTheme: boolean }) {
  if (plugin.useLocalIcon) {
    const ext = plugin.icon.includes('.') ? '' : '.svg';
    const useDark = isDarkTheme && plugin.hasDarkIcon;
    const iconSrc = useDark ? `/img/dark-${plugin.icon}${ext}` : `/img/${plugin.icon}${ext}`;
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
  const versionCandidates = useDocsVersionCandidates("default");

  const resolveHref = (docId: string): string => {
    for (const version of versionCandidates) {
      const doc = version.docs.find((d) => d.id === docId);
      if (doc) return doc.path;
    }
    return "#";
  };

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
            <Link
              key={plugin.name}
              to={resolveHref(plugin.docId)}
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
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}
