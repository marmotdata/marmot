import type { ComponentType, SvelteComponent } from 'svelte';

import AsyncApiIcon from '~icons/logos/async-api-icon';
import OpenApiIcon from '~icons/logos/openapi-icon';
import DbtIcon from '~icons/logos/dbt-icon';
import AirflowIcon from '~icons/logos/airflow-icon';
import RedisIcon from '~icons/logos/redis';
import PostgresqlIcon from '~icons/logos/postgresql';
import DynamoDBIcon from '~icons/logos/aws-dynamodb';
import KinesisIcon from '~icons/logos/aws-kinesis';
import SnsIcon from '~icons/logos/aws-sns';
import SqsIcon from '~icons/logos/aws-sqs';
import ElasticsearchIcon from '~icons/logos/elasticsearch';
import MySqlIcon from '~icons/logos/mysql-icon';
import MongoDBIcon from '~icons/logos/mongodb-icon';
import S3Icon from '~icons/logos/aws-s3';
import SparkIcon from '~icons/logos/apache-spark';
import SnowflakeIcon from '~icons/logos/snowflake-icon';
import KubernetesIcon from '~icons/logos/kubernetes';
import AMQPIcon from '~icons/logos/rabbitmq-icon';
import BigQueryIcon from '~icons/logos/google-cloud';

import QueueListIcon from '~icons/heroicons/queue-list';
import ChatBubbleIcon from '~icons/heroicons/chat-bubble-left-ellipsis';
import CodeBracketIcon from '~icons/heroicons/code-bracket-16-solid';
import DatabaseOutlineIcon from '~icons/material-symbols/database-outline';
import TableOutlineIcon from '~icons/material-symbols/table-outline';
import ViewOutlineIcon from '~icons/material-symbols/view-list-outline';
import HomeStorageOutlineIcon from '~icons/material-symbols/home-storage-outline';
import PartnerExchangeOutlineRounded from '~icons/material-symbols/partner-exchange-outline-rounded';
import DatasetOutlineRounded from '~icons/material-symbols/dataset-outline-rounded';
import BackupTableRounded from '~icons/material-symbols/backup-table-rounded';

export type IconResult = string | { component: ComponentType<SvelteComponent>; class?: string };

const iconMap: Record<
  string,
  {
    default: ComponentType<SvelteComponent>;
    dark?: ComponentType<SvelteComponent>;
    class?: string;
  }
> = {
  asyncapi: { default: AsyncApiIcon },
  openapi: { default: OpenApiIcon },
  dbt: { default: DbtIcon },
  airflow: { default: AirflowIcon },
  redis: { default: RedisIcon },
  postgresql: { default: PostgresqlIcon },
  dynamodb: { default: DynamoDBIcon },
  kinesis: { default: KinesisIcon },
  sns: { default: SnsIcon },
  sqs: { default: SqsIcon },
  elasticsearch: { default: ElasticsearchIcon },
  mysql: { default: MySqlIcon },
  mongodb: { default: MongoDBIcon },
  s3: { default: S3Icon },
  spark: { default: SparkIcon },
  snowflake: { default: SnowflakeIcon },
  kubernetes: { default: KubernetesIcon },
  bigquery: { default: BigQueryIcon },
  amqp: { default: AMQPIcon },
  rabbitmq: { default: AMQPIcon },
  queue: { default: QueueListIcon, class: 'text-gray-900 dark:text-gray-100' },
  topic: { default: ChatBubbleIcon, class: 'text-gray-900 dark:text-gray-100' },
  service: { default: CodeBracketIcon, class: 'text-gray-900 dark:text-gray-100' },
  database: { default: DatabaseOutlineIcon, class: 'text-gray-900 dark:text-gray-100' },
  table: { default: TableOutlineIcon, class: 'text-gray-900 dark:text-gray-100' },
  bucket: { default: HomeStorageOutlineIcon, class: 'text-gray-900 dark:text-gray-100' },
  view: { default: ViewOutlineIcon, class: 'text-gray-900 dark:text-gray-100' },
  exchange: { default: PartnerExchangeOutlineRounded, class: 'text-gray-900 dark:text-gray-100' },
  dataset: { default: DatasetOutlineRounded, class: 'text-gray-900 dark:text-gray-100' },
  collection: { default: BackupTableRounded, class: 'text-gray-900 dark:text-gray-100' }
};

export class IconLoader {
  private static instance: IconLoader;
  private cache: Map<string, Promise<IconResult>>;
  private loadingIcons: Set<string>;

  private constructor() {
    this.cache = new Map();
    this.loadingIcons = new Set();
  }

  static getInstance(): IconLoader {
    if (!IconLoader.instance) {
      IconLoader.instance = new IconLoader();
    }
    return IconLoader.instance;
  }

  async loadIcon(name: string, isDark: boolean): Promise<IconResult> {
    const cacheKey = `${name}-${isDark}`;

    if (this.cache.has(cacheKey)) {
      return this.cache.get(cacheKey)!;
    }

    if (this.loadingIcons.has(cacheKey)) {
      return new Promise((resolve) => {
        const checkCache = () => {
          if (this.cache.has(cacheKey)) {
            resolve(this.cache.get(cacheKey)!);
          } else {
            setTimeout(checkCache, 50);
          }
        };
        checkCache();
      });
    }

    this.loadingIcons.add(cacheKey);

    try {
      const iconPromise = this.fetchIcon(name, isDark);
      this.cache.set(cacheKey, iconPromise);
      await iconPromise;
      return iconPromise;
    } finally {
      this.loadingIcons.delete(cacheKey);
    }
  }

  private async fetchIcon(name: string, isDark: boolean): Promise<IconResult> {
    const formattedName = name.toLowerCase().replace(/_/g, '-');

    if (iconMap[formattedName]) {
      if (isDark && iconMap[formattedName].dark) {
        return {
          component: iconMap[formattedName].dark!,
          class: iconMap[formattedName].class
        };
      }
      return {
        component: iconMap[formattedName].default,
        class: iconMap[formattedName].class
      };
    }

    if (isDark) {
      try {
        const darkUrl = `/images/asset-logos/dark-${formattedName}.svg`;
        const response = await fetch(darkUrl);
        if (response.ok) return darkUrl;
      } catch (error) {
        console.debug(`Dark variant not found for ${name}`);
      }
    }

    try {
      const regularUrl = `/images/asset-logos/${formattedName}.svg`;
      const response = await fetch(regularUrl);
      if (response.ok) return regularUrl;
    } catch (error) {
      console.warn(`Failed to load icon for ${name}`);
    }

    return '/images/marmot.svg';
  }
}
