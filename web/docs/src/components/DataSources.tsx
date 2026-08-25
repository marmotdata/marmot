import React from "react";
import { Icon } from "@iconify/react";

const sources = [
  { name: "PostgreSQL", href: "/docs/Plugins/PostgreSQL", icon: "devicon:postgresql" },
  { name: "MySQL", href: "/docs/Plugins/MySQL", icon: "devicon:mysql" },
  { name: "MongoDB", href: "/docs/Plugins/MongoDB", icon: "devicon:mongodb" },
  { name: "ClickHouse", href: "/docs/Plugins/ClickHouse", icon: "devicon:clickhouse" },
  { name: "BigQuery", href: "/docs/Plugins/BigQuery", icon: "devicon:googlecloud" },
  { name: "Kafka", href: "/docs/Plugins/Kafka", icon: "devicon:apachekafka" },
  { name: "Airflow", href: "/docs/Plugins/Airflow", icon: "logos:airflow-icon" },
  { name: "dbt", href: "/docs/Plugins/DBT", icon: "logos:dbt-icon" },
  { name: "S3", href: "/docs/Plugins/S3", icon: "logos:aws-s3" },
  { name: "Trino", href: "/docs/Plugins/Trino", icon: "simple-icons:trino" },
  { name: "DuckDB", href: "/docs/Plugins/DuckDB", icon: "devicon:duckdb" },
  { name: "Elasticsearch", href: "/docs/Plugins/Elasticsearch", icon: "devicon:elasticsearch" },
  { name: "Kubernetes", href: "/docs/Plugins/Kubernetes", icon: "devicon:kubernetes" },
  { name: "SQS", href: "/docs/Plugins/SQS", icon: "logos:aws-sqs" },
  { name: "Redis", href: "/docs/Plugins/Redis", icon: "devicon:redis" },
];

export default function DataSources(): JSX.Element {
  return (
    <section className="py-20 sm:py-24 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-800">
      <div className="max-w-4xl mx-auto">
        <div data-animate className="text-center">
          <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white tracking-tight">
            Plugins for what you already run
          </h2>
          <p className="mt-4 text-lg text-gray-500 dark:text-gray-400">
            More than 30 of them, from databases to queues to warehouses.
          </p>
        </div>

        <div
          data-animate
          data-animate-delay="1"
          className="mt-10 flex flex-wrap items-center justify-center gap-2.5"
        >
          {sources.map((source) => (
            <a
              key={source.name}
              href={source.href}
              className="plugin-chip group inline-flex items-center gap-2 px-3.5 py-2 rounded-lg bg-white dark:bg-gray-900/40"
            >
              <Icon
                icon={source.icon}
                className={`w-5 h-5 ${source.name === "Kafka" ? "kafka-icon" : ""}`}
              />
              <span className="text-sm font-medium text-gray-700 dark:text-gray-300 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors">
                {source.name}
              </span>
            </a>
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
            View all plugins
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
