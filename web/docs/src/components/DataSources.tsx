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
    { name: "MongoDB", href: "/docs/Plugins/MongoDB", icon: "devicon:mongodb" },
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
    { name: "Kafka", href: "/docs/Plugins/Kafka", icon: "devicon:apachekafka" },
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
    <section className="py-20 px-4 sm:px-6 lg:px-8 bg-white dark:bg-gray-800">
      <div className="max-w-6xl mx-auto">
        <div className="text-center mb-12">
          <h2 className="text-3xl sm:text-4xl font-bold text-gray-900 dark:text-white mb-4">
            Connect to your data sources
          </h2>
          <p className="text-lg text-gray-600 dark:text-gray-300">
            Growing ecosystem of plugins
          </p>
        </div>

        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          {sources.map((source) => (
            <a
              key={source.name}
              href={source.href}
              className="group p-6 bg-gray-50 dark:bg-gray-900 rounded-lg border border-gray-200 dark:border-gray-700 hover:border-earthy-terracotta-500 dark:hover:border-earthy-terracotta-600 hover:shadow-lg transition-all text-center"
            >
              <div className="flex justify-center mb-3 transform group-hover:scale-110 transition-transform">
                <Icon
                  icon={source.icon}
                  className={`w-12 h-12 ${source.name === "Kafka" ? "kafka-icon" : ""}`}
                />
              </div>
              <h3 className="text-base font-bold text-gray-900 dark:text-white">
                {source.name}
              </h3>
            </a>
          ))}
        </div>

        <div className="text-center mt-8">
          <a
            href="/docs/Plugins/"
            className="text-earthy-terracotta-700 dark:text-earthy-terracotta-500 hover:text-earthy-terracotta-800 dark:hover:text-earthy-terracotta-400 font-medium"
          >
            and more →
          </a>
        </div>
      </div>
    </section>
  );
}
