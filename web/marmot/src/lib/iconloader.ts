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
import BigQueryIcon from '~icons/logos/google-cloud';
import DuckDBIcon from '~icons/devicon/duckdb';
import DatabricksIcon from '~icons/simple-icons/databricks';
import ClickHouseIcon from '~icons/devicon/clickhouse';
import TrinoIcon from '~icons/simple-icons/trino';
import TeradataIcon from '~icons/simple-icons/teradata';
import OracleIcon from '~icons/devicon/oracle';
import SalesforceIcon from '~icons/devicon/salesforce';
import AthenaIcon from '~icons/logos/aws-athena';
import RedshiftIcon from '~icons/logos/aws-redshift';
import GlueIcon from '~icons/logos/aws-glue';
import AzureIcon from '~icons/logos/microsoft-azure';
import SingleStoreIcon from '~icons/logos/singlestore-icon';
import KafkaIcon from '~icons/devicon/apachekafka';
import NatsIcon from '~icons/devicon/nats';
import PulsarIcon from '~icons/devicon/pulsar';
import RabbitMQIcon from '~icons/devicon/rabbitmq';
import TableauIcon from '~icons/simple-icons/tableau';

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
import StreamOutline from '~icons/material-symbols/stream';
import InputOutline from '~icons/material-symbols/input';
import OutputOutline from '~icons/material-symbols/output';
import BookOutline from '~icons/material-symbols/book-2-outline';
import DnsOutline from '~icons/material-symbols/dns-outline';
import SyncAltOutline from '~icons/material-symbols/sync-alt';
import WebhookOutline from '~icons/material-symbols/webhook';
import WifiOutline from '~icons/material-symbols/wifi';
import RouterOutline from '~icons/material-symbols/router';
import CloudSyncOutline from '~icons/material-symbols/cloud-sync-outline';
import MemoryOutline from '~icons/material-symbols/memory';
import SendOutline from '~icons/material-symbols/send-outline';
import LinkOutline from '~icons/material-symbols/link';
import DashboardOutline from '~icons/material-symbols/dashboard-outline';
import StorageOutline from '~icons/material-symbols/storage';

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
	amqp: { default: RabbitMQIcon, displayName: 'AMQP' },
	rabbitmq: { default: RabbitMQIcon, displayName: 'RabbitMQ' },
	duckdb: { default: DuckDBIcon, displayName: 'DuckDB' },
	openlineage: {
		default: OpenLineage,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'OpenLineage'
	},
	databricks: { default: DatabricksIcon, class: 'text-[#FF3621]', displayName: 'Databricks' },
	clickhouse: { default: ClickHouseIcon, displayName: 'ClickHouse' },
	trino: { default: TrinoIcon, class: 'text-[#DD00A1]', displayName: 'Trino' },
	starburst: { default: TrinoIcon, class: 'text-[#DD00A1]', displayName: 'Starburst' },
	teradata: { default: TeradataIcon, class: 'text-[#F37440]', displayName: 'Teradata' },
	oracle: { default: OracleIcon, displayName: 'Oracle' },
	salesforce: { default: SalesforceIcon, displayName: 'Salesforce' },
	athena: { default: AthenaIcon, displayName: 'Athena' },
	redshift: { default: RedshiftIcon, displayName: 'Redshift' },
	'aws-glue': { default: GlueIcon, displayName: 'AWS Glue' },
	glue: { default: GlueIcon, displayName: 'AWS Glue' },
	'azure-synapse': { default: AzureIcon, displayName: 'Azure Synapse' },
	synapse: { default: AzureIcon, displayName: 'Azure Synapse' },
	'microsoft-fabric': { default: AzureIcon, displayName: 'Microsoft Fabric' },
	fabric: { default: AzureIcon, displayName: 'Microsoft Fabric' },
	singlestore: { default: SingleStoreIcon, displayName: 'SingleStore' },
	alloydb: { default: PostgresqlIcon, displayName: 'AlloyDB' },
	postgres: { default: PostgresqlIcon, displayName: 'Postgres' },
	azureblob: { default: AzureIcon, displayName: 'Azure Blob Storage' },
	'azure-blob': { default: AzureIcon, displayName: 'Azure Blob Storage' },
	gcs: { default: BigQueryIcon, displayName: 'Google Cloud Storage' },
	'google-cloud-storage': { default: BigQueryIcon, displayName: 'Google Cloud Storage' },
	materialize: {
		default: DatabaseOutlineIcon,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Materialize'
	},
	dremio: {
		default: DatabaseOutlineIcon,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Dremio'
	},
	netezza: {
		default: DatabaseOutlineIcon,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Netezza'
	},
	// AsyncAPI binding providers
	kafka: { default: KafkaIcon, displayName: 'Kafka' },
	mqtt: {
		default: WifiOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'MQTT'
	},
	nats: { default: NatsIcon, displayName: 'NATS' },
	pulsar: { default: PulsarIcon, displayName: 'Pulsar' },
	solace: {
		default: CloudSyncOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Solace'
	},
	ibmmq: {
		default: MemoryOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'IBM MQ'
	},
	jms: {
		default: MemoryOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'JMS'
	},
	websocket: {
		default: LinkOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'WebSocket'
	},
	anypointmq: {
		default: SendOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Anypoint MQ'
	},
	http: {
		default: WebhookOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'HTTP'
	},
	googlepubsub: { default: BigQueryIcon, displayName: 'Google Pub/Sub' },
	'google-pubsub': { default: BigQueryIcon, displayName: 'Google Pub/Sub' },
	tableau: { default: TableauIcon, class: 'text-[#E97627]', displayName: 'Tableau' }
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
	queue: {
		default: QueueListIcon,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Queue'
	},
	topic: {
		default: ChatBubbleIcon,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Topic'
	},
	service: {
		default: CodeBracketIcon,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Service'
	},
	database: {
		default: DatabaseOutlineIcon,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Database'
	},
	table: {
		default: TableOutlineIcon,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Table'
	},
	bucket: {
		default: HomeStorageOutlineIcon,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Bucket'
	},
	container: {
		default: HomeStorageOutlineIcon,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Container'
	},
	view: {
		default: ViewOutlineIcon,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'View'
	},
	exchange: {
		default: PartnerExchangeOutlineRounded,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Exchange'
	},
	dataset: {
		default: DatasetOutlineRounded,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Dataset'
	},
	collection: {
		default: BackupTableRounded,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Collection'
	},
	model: {
		default: ModelingOutlineRounded,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Model'
	},
	project: {
		default: ClinicalNotesOutlineRounded,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Project'
	},
	task: {
		default: TaskOutlineRounded,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Task'
	},
	dag: { default: Graph4, class: 'text-gray-900 dark:text-gray-100', displayName: 'DAG' },
	endpoint: {
		default: Endpoint,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Endpoint'
	},
	job: {
		default: FolderDataOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Job'
	},
	// Specialized dbt adapter asset types
	'materialized-view': {
		default: SyncAltOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Materialized View'
	},
	'dynamic-table': {
		default: SyncAltOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Dynamic Table'
	},
	'streaming-table': {
		default: StreamOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Streaming Table'
	},
	dictionary: {
		default: BookOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Dictionary'
	},
	'distributed-table': {
		default: DnsOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Distributed Table'
	},
	source: {
		default: InputOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Source'
	},
	sink: {
		default: OutputOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Sink'
	},
	'data-model-object': {
		default: DatasetOutlineRounded,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Data Model Object'
	},
	// AsyncAPI specific types
	subject: {
		default: RouterOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Subject'
	},
	fifoqueue: {
		default: QueueListIcon,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'FIFO Queue'
	},
	channel: {
		default: StreamOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Channel'
	},
	dashboard: {
		default: DashboardOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Dashboard'
	},
	datasource: {
		default: StorageOutline,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'DataSource'
	},
	pipeline: {
		default: Graph4,
		class: 'text-gray-900 dark:text-gray-100',
		displayName: 'Pipeline'
	}
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
			console.debug(`SVG not found for ${name}`);
		}

		try {
			const pngUrl = `/images/asset-logos/${formattedName}.png`;
			const response = await fetch(pngUrl);

			if (response.ok) {
				const contentType = response.headers.get('content-type');
				if (contentType && contentType.includes('image/png')) {
					return pngUrl;
				}
			}
		} catch (error) {
			console.warn(`Failed to load icon for ${name}`);
		}

		return '/images/marmot.svg';
	}
}
