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
import ModelingOutlineRounded from '~icons/material-symbols/modeling-outline-rounded';
import ClinicalNotesOutlineRounded from '~icons/material-symbols/clinical-notes-outline-rounded';
import TaskOutlineRounded from '~icons/material-symbols/task-outline-rounded';
import Graph4 from '~icons/material-symbols/graph-4';
import Endpoint from '~icons/material-symbols/api-rounded';
import OpenLineage from '~icons/material-symbols/flowchart-outline-sharp';
import FolderDataOutline from '~icons/material-symbols/folder-data-outline';

export type IconResult = string | { component: ComponentType<SvelteComponent>; class?: string };

// Provider icons with display names
export const providerIconMap: Record<
  string,
  {
    default: ComponentType<SvelteComponent>;
    dark?: ComponentType<SvelteComponent>;
    class?: string;
    displayName: string;
  }
> = {
  asyncapi: { default: AsyncApiIcon, displayName: 'AsyncAPI' },
  openapi: { default: OpenApiIcon, displayName: 'OpenAPI' },
  dbt: { default: DbtIcon, displayName: 'dbt' },
  airflow: { default: AirflowIcon, displayName: 'Airflow' },
  redis: { default: RedisIcon, displayName: 'Redis' },
  postgresql: { default: PostgresqlIcon, displayName: 'PostgreSQL' },
  dynamodb: { default: DynamoDBIcon, displayName: 'DynamoDB' },
  kinesis: { default: KinesisIcon, displayName: 'Kinesis' },
  sns: { default: SnsIcon, displayName: 'SNS' },
  sqs: { default: SqsIcon, displayName: 'SQS' },
  elasticsearch: { default: ElasticsearchIcon, displayName: 'Elasticsearch' },
  mysql: { default: MySqlIcon, displayName: 'MySQL' },
  mongodb: { default: MongoDBIcon, displayName: 'MongoDB' },
  s3: { default: S3Icon, displayName: 'S3' },
  spark: { default: SparkIcon, displayName: 'Spark' },
  snowflake: { default: SnowflakeIcon, displayName: 'Snowflake' },
  kubernetes: { default: KubernetesIcon, displayName: 'Kubernetes' },
  bigquery: { default: BigQueryIcon, displayName: 'BigQuery' },
  amqp: { default: AMQPIcon, displayName: 'AMQP' },
  rabbitmq: { default: AMQPIcon, displayName: 'RabbitMQ' },
  openlineage: { default: OpenLineage, class: 'text-gray-900 dark:text-gray-100', displayName: 'OpenLineage' }
};

// Type icons with display names
export const typeIconMap: Record<
  string,
  {
    default: ComponentType<SvelteComponent>;
    dark?: ComponentType<SvelteComponent>;
    class?: string;
    displayName: string;
  }
> = {
  queue: { default: QueueListIcon, class: 'text-gray-900 dark:text-gray-100', displayName: 'Queue' },
  topic: { default: ChatBubbleIcon, class: 'text-gray-900 dark:text-gray-100', displayName: 'Topic' },
  service: { default: CodeBracketIcon, class: 'text-gray-900 dark:text-gray-100', displayName: 'Service' },
  database: { default: DatabaseOutlineIcon, class: 'text-gray-900 dark:text-gray-100', displayName: 'Database' },
  table: { default: TableOutlineIcon, class: 'text-gray-900 dark:text-gray-100', displayName: 'Table' },
  bucket: { default: HomeStorageOutlineIcon, class: 'text-gray-900 dark:text-gray-100', displayName: 'Bucket' },
  view: { default: ViewOutlineIcon, class: 'text-gray-900 dark:text-gray-100', displayName: 'View' },
  exchange: { default: PartnerExchangeOutlineRounded, class: 'text-gray-900 dark:text-gray-100', displayName: 'Exchange' },
  dataset: { default: DatasetOutlineRounded, class: 'text-gray-900 dark:text-gray-100', displayName: 'Dataset' },
  collection: { default: BackupTableRounded, class: 'text-gray-900 dark:text-gray-100', displayName: 'Collection' },
  model: { default: ModelingOutlineRounded, class: 'text-gray-900 dark:text-gray-100', displayName: 'Model' },
  project: { default: ClinicalNotesOutlineRounded, class: 'text-gray-900 dark:text-gray-100', displayName: 'Project' },
  task: { default: TaskOutlineRounded, class: 'text-gray-900 dark:text-gray-100', displayName: 'Task' },
  dag: { default: Graph4, class: 'text-gray-900 dark:text-gray-100', displayName: 'DAG' },
  endpoint: { default: Endpoint, class: 'text-gray-900 dark:text-gray-100', displayName: 'Endpoint' },
  job: { default: FolderDataOutline, class: 'text-gray-900 dark:text-gray-100', displayName: 'Job' }
};

// Combined map for backward compatibility
const iconMap: Record<
  string,
  {
    default: ComponentType<SvelteComponent>;
    dark?: ComponentType<SvelteComponent>;
    class?: string;
  }
> = {
  ...providerIconMap,
  ...typeIconMap
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

    // Check if response is actually an SVG
    const isValidSvg = async (response: Response): Promise<boolean> => {
      if (!response.ok) return false;

      const contentType = response.headers.get('content-type');
      if (
        contentType &&
        !contentType.includes('image/svg+xml') &&
        !contentType.includes('text/html')
      ) {
        return false;
      }

      const text = await response.text();
      return text.trim().startsWith('<svg') || text.includes('<svg');
    };

    if (isDark) {
      try {
        const darkUrl = `/images/asset-logos/dark-${formattedName}.svg`;
        const response = await fetch(darkUrl);
        const responseClone = response.clone();

        if (await isValidSvg(responseClone)) {
          return darkUrl;
        }
      } catch (error) {
        console.debug(`Dark variant not found for ${name}`);
      }
    }

    try {
      const regularUrl = `/images/asset-logos/${formattedName}.svg`;
      const response = await fetch(regularUrl);
      const responseClone = response.clone();

      if (await isValidSvg(responseClone)) {
        return regularUrl;
      }
    } catch (error) {
      console.warn(`Failed to load icon for ${name}`);
    }

    return '/images/marmot.svg';
  }
}
