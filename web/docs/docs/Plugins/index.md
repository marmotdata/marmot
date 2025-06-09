# Plugins

Plugins automatically discover and catalog your data assets in Marmot. They connect to external systems, extract metadata and lineage, and create asset entries with minimal effort.

Marmot isn't limited to plugin-based ingestion. You can also use:

### Infrastructure as Code

- [Terraform Provider](/docs/populating/terraform) - Manage Marmot assets as Terraform resources
- [Pulumi Package](/docs/populating/pulumi) - Integrate Marmot with Pulumi infrastructure definitions

These approaches enable version-controlled asset definitions and integration with existing infrastructure workflows.

### API

The [Marmot API](/docs/populating/api) lets you programmatically create, update, and manage assets.

## Available Plugins

- [AsyncAPI](plugins/asyncapi) - Discover services, topics, and queues from AsyncAPI specifications
- [BigQuery](plugins/bigquery) - Catalog datasets, tables, and views from Google BigQuery projects
- [Kafka](plugins/kafka) - Catalog topics from Apache Kafka clusters with Schema Registry integration
- [MongoDB](plugins/mongodb) - Discover databases and collections from MongoDB instances
- [MySQL](plugins/mysql) - Discover databases and tables from MySQL instances
- [PostgreSQL](plugins/postgresql) - Discover tables, views, and relationships from PostgreSQL databases
- [SNS](plugins/sns) - Catalog topics from Amazon SNS
- [SQS](plugins/sqs) - Discover queues from Amazon SQS

Don't see your data source? Learn how to [create your own plugin](/docs/Develop/Creating%20a%20CLI%20Plugin).
