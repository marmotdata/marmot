import React, { useEffect, useState } from "react";
import Icon from "./Icon";

// Import all icons
import IconAsyncApi from "~icons/logos/async-api-icon";
import IconKafka from "~icons/logos/kafka-icon";
import IconAwsSns from "~icons/logos/aws-sns";
import IconAwsSqs from "~icons/logos/aws-sqs";
import IconDbt from "~icons/logos/dbt-icon";
import IconAwsKinesis from "~icons/logos/aws-kinesis";
import IconNats from "~icons/logos/nats-icon";
import IconMongodb from "~icons/logos/mongodb-icon";
import IconPostgresql from "~icons/logos/postgresql";
import IconOpenApi from "~icons/logos/openapi-icon";
import IconAirflow from "~icons/logos/airflow-icon";
import IconRedis from "~icons/logos/redis";
import IconDynamoDB from "~icons/logos/aws-dynamodb";
import IconElasticsearch from "~icons/logos/elasticsearch";
import IconMySql from "~icons/logos/mysql-icon";
import IconS3 from "~icons/logos/aws-s3";
import IconSpark from "~icons/logos/apache-spark";
import IconApache from "~icons/logos/apache";
import IconSnowflake from "~icons/logos/snowflake-icon";

interface IconsScrollProps {
  className?: string;
}

const IconsScroll: React.FC<IconsScrollProps> = ({ className = "" }) => {
  const [isDarkMode, setIsDarkMode] = useState(
    typeof document !== "undefined" &&
      document.documentElement.getAttribute("data-theme") === "dark",
  );

  useEffect(() => {
    if (typeof document === "undefined") return;

    const observer = new MutationObserver((mutations) => {
      mutations.forEach((mutation) => {
        if (mutation.attributeName === "data-theme") {
          const newTheme = document.documentElement.getAttribute("data-theme");
          setIsDarkMode(newTheme === "dark");
        }
      });
    });

    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ["data-theme"],
    });

    return () => observer.disconnect();
  }, []);

  // All service icons with their components
  const serviceIcons = [
    { name: "Async API", component: IconAsyncApi },
    { name: "Open API", component: IconOpenApi },
    { name: "Kafka", component: IconKafka },
    { name: "AWS SNS", component: IconAwsSns },
    { name: "AWS SQS", component: IconAwsSqs },
    { name: "dbt", component: IconDbt },
    { name: "AWS Kinesis", component: IconAwsKinesis },
    { name: "NATS", component: IconNats },
    { name: "MongoDB", component: IconMongodb },
    { name: "PostgreSQL", component: IconPostgresql },
    { name: "Airflow", component: IconAirflow },
    { name: "Redis", component: IconRedis },
    { name: "DynamoDB", component: IconDynamoDB },
    { name: "Elasticsearch", component: IconElasticsearch },
    { name: "MySQL", component: IconMySql },
    { name: "AWS S3", component: IconS3 },
    { name: "Apache Spark", component: IconSpark },
    { name: "Snowflake", component: IconSnowflake },
    { name: "Apache", component: IconApache },
  ];

  return (
    <div
      className={`w-full py-12 bg-earthy-brown-50 dark:bg-gray-900 ${className}`}
    >
      <div className="scroll-container">
        <div className="scroll-content animate-scroll-left">
          {/* Duplicate the array multiple times to ensure smooth infinite scrolling */}
          {[...serviceIcons, ...serviceIcons, ...serviceIcons].map(
            (service, index) => (
              <div
                key={`service-${index}`}
                className="flex items-center justify-center mx-6 grayscale opacity-50 transition-opacity hover:opacity-80"
                style={{ minWidth: "250px" }}
              >
                <Icon
                  type={service.component}
                  size="lg"
                  className={`${isDarkMode ? "text-white" : "text-gray-800"}`}
                />
                <span className="ml-3 text-sm font-medium text-gray-700 dark:text-gray-300">
                  {service.name}
                </span>
              </div>
            ),
          )}
        </div>
      </div>
    </div>
  );
};

export default IconsScroll;
