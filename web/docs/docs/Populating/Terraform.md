---
sidebar_position: 2
toc_max_heading_level: 4
---

Using Terraform, you can manage Marmot as code, declaring your Marmot resources and automating them alongside the rest of your infrastructure.

## Getting Started

### Provider Configuration

To use the Marmot Terraform provider, add it to your Terraform configuration:

```hcl
terraform {
  required_providers {
    marmot = {
      source = "marmotdata/marmot"
    }
  }
}

provider "marmot" {
  host    = "http://localhost:8080" # or the MARMOT_HOST environment variable
  api_key = var.marmot_api_key      # or the MARMOT_API_KEY environment variable
}
```

### Authentication

The provider authenticates with a Marmot API key, set through the `api_key`
attribute or the `MARMOT_API_KEY` environment variable. A bearer `token` (or `MARMOT_TOKEN`) is also
supported, and when no credential is provided the provider falls back to the Marmot
CLI credentials from `marmot login`.

To keep the secret entirely out of state, inject it using a Terraform [ephemeral resource](https://developer.hashicorp.com/terraform/language/resources/ephemeral) (Terraform >= 1.10). 

For example, with Google Secret Manager:

```hcl
ephemeral "google_secret_manager_secret_version" "marmot_api_key" {
  secret  = "marmot-api-key"
  version = "latest"
}

provider "marmot" {
  host    = "https://your-marmot-host.com"
  api_key = ephemeral.google_secret_manager_secret_version.marmot_api_key.secret_data
}
```

The same pattern works with any provider that exposes secrets as an ephemeral resource, such as AWS Secrets Manager or HashiCorp Vault.

## Resources

The Marmot provider offers these primary resources:

### Assets

Register the datasets, services, and other resources in your platform as assets:

```hcl
resource "marmot_asset" "customer_orders" {
  name     = "customer-orders"
  type     = "Database"
  services = ["PostgreSQL"]
  tags     = ["orders", "customer", "customer-orders"]
}
```

Reference a resource's own attributes instead of hardcoding names and IDs. The asset updates in the same `terraform apply` as the resource it describes, so its metadata never drifts from what's actually deployed.

#### Google Cloud

Register a BigQuery table alongside its definition:

```hcl
resource "google_bigquery_dataset" "analytics" {
  dataset_id = "analytics"
  location   = "US"
}

resource "google_bigquery_table" "orders" {
  dataset_id = google_bigquery_dataset.analytics.dataset_id
  table_id   = "orders"
}

resource "marmot_asset" "orders" {
  name     = google_bigquery_table.orders.table_id
  type     = "Table"
  services = ["BigQuery"]
  tags     = ["orders", "analytics"]

  metadata = {
    project  = google_bigquery_table.orders.project
    dataset  = google_bigquery_dataset.analytics.dataset_id
    location = google_bigquery_dataset.analytics.location
  }
}
```

#### AWS

Register a DynamoDB table the same way:

```hcl
resource "aws_dynamodb_table" "orders" {
  name         = "orders"
  billing_mode = "PAY_PER_REQUEST"
  hash_key     = "order_id"

  attribute {
    name = "order_id"
    type = "S"
  }
}

resource "marmot_asset" "orders" {
  name     = aws_dynamodb_table.orders.name
  type     = "Table"
  services = ["DynamoDB"]
  tags     = ["orders", "analytics"]

  metadata = {
    arn          = aws_dynamodb_table.orders.arn
    hash_key     = aws_dynamodb_table.orders.hash_key
    billing_mode = aws_dynamodb_table.orders.billing_mode
  }
}
```

#### Azure

Register an Azure Table Storage table the same way:

```hcl
resource "azurerm_storage_account" "analytics" {
  name                     = "analyticsdata"
  resource_group_name      = "analytics"
  location                 = "East US"
  account_tier             = "Standard"
  account_replication_type = "LRS"
}

resource "azurerm_storage_table" "orders" {
  name                 = "orders"
  storage_account_name = azurerm_storage_account.analytics.name
}

resource "marmot_asset" "orders" {
  name     = azurerm_storage_table.orders.name
  type     = "Table"
  services = ["Azure Table Storage"]
  tags     = ["orders", "analytics"]

  metadata = {
    storage_account = azurerm_storage_account.analytics.name
    location        = azurerm_storage_account.analytics.location
  }
}
```

See the [`marmot_asset` documentation](https://registry.terraform.io/providers/marmotdata/marmot/0.4.0/docs/resources/asset) for all available configuration options.

### Lineage

Describe how data flows between assets to build a lineage graph:

```hcl
resource "marmot_asset" "order_processor" {
  name     = "order-processor"
  type     = "Service"
  services = ["Kubernetes"]
}

resource "marmot_lineage" "orders_to_processor" {
  source = marmot_asset.customer_orders.mrn
  target = marmot_asset.order_processor.mrn
}
```

See the [`marmot_lineage` documentation](https://registry.terraform.io/providers/marmotdata/marmot/0.4.0/docs/resources/lineage) for all available configuration options.

### Glossary Terms

Define shared business terminology and organize it hierarchically:

```hcl
resource "marmot_glossary_term" "active_customer" {
  name       = "Active Customer"
  definition = "A customer with at least one order in the last 90 days."
  metadata = {
    domain = "sales"
  }
}
```

See the [`marmot_glossary_term` documentation](https://registry.terraform.io/providers/marmotdata/marmot/0.4.0/docs/resources/glossary_term) for all available configuration options.

### Teams

Manage the teams that own catalog entities:

```hcl
resource "marmot_team" "analytics" {
  name        = "analytics"
  description = "Owns the reporting datasets"
}
```

See the [`marmot_team` documentation](https://registry.terraform.io/providers/marmotdata/marmot/0.4.0/docs/resources/team) for all available configuration options.

### Users

Manage user accounts. The password is set through the write-only `password_wo` attribute (Terraform >= 1.11), so it never lands in state:

```hcl
ephemeral "random_password" "alice" {
  length = 24
}

resource "marmot_user" "alice" {
  name                = "Alice Nguyen"
  username            = "alice"
  password_wo         = ephemeral.random_password.alice.result
  password_wo_version = "1"
  role_names          = ["admin"]
}
```

Change `password_wo_version` to push a new password on a later apply.

See the [`marmot_user` documentation](https://registry.terraform.io/providers/marmotdata/marmot/0.4.0/docs/resources/user) for all available configuration options.

### Data Products

Group related assets into a data product. Teams and users own it through `owner_team_ids` and `owner_user_ids`:

```hcl
resource "marmot_data_product" "orders" {
  name        = "orders"
  description = "Order events and the tables derived from them"
  tags        = ["orders"]

  owner_team_ids = [marmot_team.analytics.id]
}
```

Assets join a data product directly with `marmot_data_product_asset`:

```hcl
resource "marmot_data_product_asset" "orders_customer" {
  data_product_id = marmot_data_product.orders.id
  asset_id        = marmot_asset.customer_orders.id
}
```

Or dynamically with `marmot_data_product_rule`, which matches assets by a search query or a metadata pattern:

```hcl
resource "marmot_data_product_rule" "order_datasets" {
  data_product_id = marmot_data_product.orders.id

  name             = "order-datasets"
  type             = "query"
  query_expression = "tag:orders"
}
```

See the [`marmot_data_product`](https://registry.terraform.io/providers/marmotdata/marmot/0.4.0/docs/resources/data_product), [`marmot_data_product_asset`](https://registry.terraform.io/providers/marmotdata/marmot/0.4.0/docs/resources/data_product_asset), and [`marmot_data_product_rule`](https://registry.terraform.io/providers/marmotdata/marmot/0.4.0/docs/resources/data_product_rule) documentation for all available configuration options.

## Learn More

- Full documentation for [the Marmot provider on the Terraform Registry](https://registry.terraform.io/providers/marmotdata/marmot/0.4.0/docs)
- A [full example](https://github.com/marmotdata/terraform-provider-marmot/tree/main/examples/full) in the provider repository
