import React from "react";
import { Icon } from "@iconify/react";

export default function DataSources(): JSX.Element {
  const sources = [
    {
      name: "PostgreSQL",
      href: "/docs/Plugins/PostgreSQL",
      icon: "devicon:postgresql",
    },
    { name: "MySQL", href: "/docs/Plugins/MySQL", icon: "devicon:mysql" },
    {
      name: "MongoDB",
      href: "/docs/Plugins/MongoDB",
      icon: "devicon:mongodb",
    },
    {
      name: "ClickHouse",
      href: "/docs/Plugins/ClickHouse",
      icon: "simple-icons:clickhouse",
    },
    {
      name: "BigQuery",
      href: "/docs/Plugins/BigQuery",
      icon: "devicon:googlecloud",
    },
    {
      name: "Kafka",
      href: "/docs/Plugins/Kafka",
      icon: "devicon:apachekafka",
    },
    {
      name: "Airflow",
      href: "/docs/Plugins/Airflow",
      icon: "logos:airflow-icon",
    },
    { name: "dbt", href: "/docs/Plugins/DBT", icon: "simple-icons:dbt" },
    { name: "S3", href: "/docs/Plugins/S3", icon: "logos:aws-s3" },
    { name: "SNS", href: "/docs/Plugins/SNS", icon: "logos:aws-sns" },
    { name: "SQS", href: "/docs/Plugins/SQS", icon: "logos:aws-sqs" },
    {
      name: "OpenAPI",
      href: "/docs/Plugins/OpenAPI",
      icon: "simple-icons:openapiinitiative",
    },
  ];

  return (
    <section className="py-24 px-4 sm:px-6 lg:px-8 bg-earthy-brown-50 dark:bg-gray-900">
      <div className="max-w-5xl mx-auto">
        <div data-animate className="text-center mb-14">
          <h2 className="text-3xl sm:text-4xl font-extrabold text-gray-900 dark:text-white mb-4 tracking-tight">
            Connect to your data sources
          </h2>
          <p className="text-lg text-gray-500 dark:text-gray-400">
            Growing ecosystem of plugins
          </p>
        </div>

        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4">
          {sources.map((source, index) => (
            <a
              key={source.name}
              href={source.href}
              data-animate
              data-animate-delay={((index % 4) + 1).toString()}
              className="group glass-card rounded-2xl p-6 flex flex-col items-center gap-3 text-center"
            >
              <div className="transform transition-all duration-300 group-hover:scale-110 group-hover:-translate-y-1">
                <Icon
                  icon={source.icon}
                  className={`w-12 h-12 ${source.name === "Kafka" ? "kafka-icon" : ""}`}
                />
              </div>
              <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-200 group-hover:text-earthy-terracotta-700 dark:group-hover:text-earthy-terracotta-400 transition-colors">
                {source.name}
              </h3>
            </a>
          ))}
        </div>

        <div data-animate className="text-center mt-10">
          <a
            href="/docs/Plugins/"
            className="inline-flex items-center gap-1 text-earthy-terracotta-700 dark:text-earthy-terracotta-400 hover:text-earthy-terracotta-800 dark:hover:text-earthy-terracotta-300 font-semibold transition-colors"
          >
            View all plugins
            <svg
              className="w-4 h-4"
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
        </div>

        <p
          data-animate
          className="text-center mt-6 text-sm text-gray-500 dark:text-gray-400"
        >
          Don't see your data source?{" "}
          <a
            href="https://github.com/marmotdata/marmot/issues/new"
            target="_blank"
            rel="noopener noreferrer"
            className="text-earthy-terracotta-600 dark:text-earthy-terracotta-400 hover:underline font-medium"
          >
            Open an issue
          </a>{" "}
          to request a plugin.
        </p>
      </div>
    </section>
  );
}
