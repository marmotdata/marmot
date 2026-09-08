package unitycatalog

// Responses captured from unitycatalog/unitycatalog:latest (the OSS
// server), trimmed to the objects the tests need. Fields the plugin does
// not read are left in on purpose so the decoder is exercised against
// the real shape, nulls included.

const unityCatalogJSON = `{
  "name": "unity",
  "comment": "reference catalog using oss UC",
  "properties": {"description": "provides categorized schemas containing collections of tables"},
  "owner": null,
  "created_at": 1721241605334,
  "created_by": null,
  "updated_at": 1747235246384,
  "updated_by": null,
  "id": "f029b870-9468-4f10-badc-630b41e5690d",
  "storage_root": null,
  "storage_location": null
}`

const systemCatalogJSON = `{
  "name": "system",
  "comment": null,
  "properties": {},
  "owner": null,
  "created_at": 1721241605334,
  "updated_at": null,
  "id": "00000000-0000-0000-0000-000000000000"
}`

const numbersTableJSON = `{
  "name": "numbers",
  "catalog_name": "unity",
  "schema_name": "default",
  "table_type": "EXTERNAL",
  "data_source_format": "DELTA",
  "columns": [
    {"name": "as_int", "type_text": "int", "type_json": "{\"name\":\"as_int\",\"type\":\"integer\",\"nullable\":false,\"metadata\":{}}", "type_name": "INT", "type_precision": 0, "type_scale": 0, "type_interval_type": null, "position": 0, "comment": "Int column", "nullable": false, "partition_index": null},
    {"name": "as_double", "type_text": "double", "type_json": "{\"name\":\"as_double\",\"type\":\"double\",\"nullable\":false,\"metadata\":{}}", "type_name": "DOUBLE", "type_precision": 0, "type_scale": 0, "type_interval_type": null, "position": 1, "comment": "Double column", "nullable": false, "partition_index": null}
  ],
  "storage_location": "file:///home/unitycatalog/etc/data/external/unity/default/tables/numbers",
  "comment": "External table",
  "properties": {"key1": "value1", "key2": "value2"},
  "owner": null,
  "created_at": 1721241605617,
  "created_by": null,
  "updated_at": 1721241605617,
  "updated_by": null,
  "table_id": "32025924-be53-4d67-ac39-501a86046c01",
  "view_definition": null,
  "view_dependencies": null
}`

const userCountriesTableJSON = `{
  "name": "user_countries",
  "catalog_name": "unity",
  "schema_name": "default",
  "table_type": "EXTERNAL",
  "data_source_format": "DELTA",
  "columns": [
    {"name": "first_name", "type_text": "string", "type_name": "STRING", "position": 0, "comment": "string column", "nullable": false, "partition_index": null},
    {"name": "age", "type_text": "bigint", "type_name": "LONG", "position": 1, "comment": "long column", "nullable": false, "partition_index": null},
    {"name": "country", "type_text": "string", "type_name": "STRING", "position": 2, "comment": "partition column", "nullable": false, "partition_index": 0}
  ],
  "storage_location": "file:///home/unitycatalog/etc/data/external/unity/default/tables/user_countries",
  "comment": "Partitioned table",
  "properties": {},
  "owner": null,
  "created_at": 1721241605622,
  "updated_at": 1721241605622,
  "table_id": "26ed93b5-9a18-4726-8ae8-c89dfcfea069",
  "view_definition": null,
  "view_dependencies": null
}`

const customersTableJSON = `{
  "name": "customers",
  "catalog_name": "shop",
  "schema_name": "sales",
  "table_type": "EXTERNAL",
  "data_source_format": "DELTA",
  "columns": [
    {"name": "id", "type_text": "int", "type_name": "INT", "type_precision": null, "type_scale": null, "position": 0, "comment": "Customer id", "nullable": false, "partition_index": null},
    {"name": "email", "type_text": "string", "type_name": "STRING", "position": 1, "comment": null, "nullable": true, "partition_index": null}
  ],
  "storage_location": "s3://marmot-lake/customers",
  "comment": "Customer master data",
  "properties": {"owner_team": "crm"},
  "owner": null,
  "created_at": 1788835315903,
  "updated_at": 1788835315903,
  "table_id": "bfd1d184-c3f4-4f4c-bef2-264c5808fa15",
  "view_definition": null,
  "view_dependencies": null
}`

const ordersTableJSON = `{
  "name": "orders",
  "catalog_name": "shop",
  "schema_name": "sales",
  "table_type": "EXTERNAL",
  "data_source_format": "DELTA",
  "columns": [
    {"name": "id", "type_text": "int", "type_name": "INT", "position": 0, "comment": "Order id", "nullable": false, "partition_index": null},
    {"name": "customer_id", "type_text": "int", "type_name": "INT", "position": 1, "comment": null, "nullable": false, "partition_index": null},
    {"name": "amount", "type_text": "decimal(10,2)", "type_name": "DECIMAL", "type_precision": 10, "type_scale": 2, "position": 2, "comment": null, "nullable": true, "partition_index": null},
    {"name": "order_date", "type_text": "date", "type_name": "DATE", "position": 3, "comment": null, "nullable": false, "partition_index": 0}
  ],
  "storage_location": "s3://marmot-lake/orders",
  "comment": "One row per order",
  "properties": {},
  "owner": null,
  "created_at": 1788835316473,
  "updated_at": 1788835316473,
  "table_id": "afb97ba1-4aa6-4bae-a5d6-3b39f6b4740c",
  "view_definition": null,
  "view_dependencies": null
}`

const bigOrdersViewJSON = `{
  "name": "big_orders",
  "catalog_name": "shop",
  "schema_name": "sales",
  "table_type": "VIEW",
  "data_source_format": "DELTA",
  "columns": [
    {"name": "id", "type_text": "int", "type_name": "INT", "position": 0, "comment": null, "nullable": false, "partition_index": null}
  ],
  "storage_location": null,
  "comment": "Orders above 100",
  "properties": {},
  "owner": null,
  "created_at": 1788835321984,
  "updated_at": 1788835321984,
  "table_id": "9d5d056f-c413-47f7-a179-8d045dec908d",
  "view_definition": "SELECT o.id FROM shop.sales.orders o JOIN customers c ON o.customer_id = c.id WHERE o.amount > 100",
  "view_dependencies": {"dependencies": []}
}`

const txtFilesVolumeJSON = `{
  "catalog_name": "unity",
  "schema_name": "default",
  "name": "txt_files",
  "comment": null,
  "owner": null,
  "created_at": 1721241605627,
  "created_by": null,
  "updated_at": 1721241605627,
  "updated_by": null,
  "volume_id": "74695d77-d48b-4f8e-9894-54a3e110b1ae",
  "volume_type": "MANAGED",
  "storage_location": "file:///home/unitycatalog/etc/data/managed/unity/default/volumes/txt_files",
  "full_name": "unity.default.txt_files"
}`

const sumFunctionJSON = `{
  "name": "sum",
  "catalog_name": "unity",
  "schema_name": "default",
  "input_params": {
    "parameters": [
      {"name": "x", "type_text": "int", "type_json": "{\"name\":\"x\",\"type\":\"integer\",\"nullable\":false,\"metadata\":{}}", "type_name": "INT", "type_precision": null, "type_scale": null, "type_interval_type": null, "position": 0, "parameter_mode": "IN", "parameter_type": "PARAM", "parameter_default": null, "comment": null},
      {"name": "y", "type_text": "int", "type_name": "INT", "position": 1, "parameter_mode": "IN", "parameter_type": "PARAM"},
      {"name": "z", "type_text": "int", "type_name": "INT", "position": 2, "parameter_mode": "IN", "parameter_type": "PARAM"}
    ]
  },
  "data_type": "INT",
  "full_data_type": "INT",
  "return_params": null,
  "routine_body": "EXTERNAL",
  "routine_definition": "t = x + y + z\\nreturn t",
  "routine_dependencies": null,
  "parameter_style": "S",
  "is_deterministic": true,
  "sql_data_access": "NO_SQL",
  "is_null_call": false,
  "security_type": "DEFINER",
  "specific_name": "sum",
  "comment": "Adds two numbers.",
  "properties": null,
  "full_name": "unity.default.sum",
  "owner": null,
  "created_at": 1721234405630,
  "created_by": null,
  "updated_at": null,
  "updated_by": null,
  "function_id": "7ad91857-fc34-4417-9694-1738642f874c",
  "external_language": "python"
}`

const netAmountFunctionJSON = `{
  "name": "net_amount",
  "catalog_name": "shop",
  "schema_name": "sales",
  "input_params": {
    "parameters": [
      {"name": "gross", "type_text": "decimal(10,2)", "type_name": "DECIMAL", "type_precision": 10, "type_scale": 2, "position": 0, "parameter_mode": "IN", "parameter_type": "PARAM"},
      {"name": "rate", "type_text": "double", "type_name": "DOUBLE", "position": 1, "parameter_mode": "IN", "parameter_type": "PARAM"}
    ]
  },
  "data_type": "DECIMAL",
  "full_data_type": "decimal(10,2)",
  "return_params": null,
  "routine_body": "SQL",
  "routine_definition": "gross * (1 - rate)",
  "is_deterministic": true,
  "sql_data_access": "CONTAINS_SQL",
  "comment": "Gross minus tax",
  "full_name": "shop.sales.net_amount",
  "owner": null,
  "created_at": 1788835332102,
  "updated_at": 1788835332102,
  "function_id": "412752ab-af36-4840-b88b-47fbfce2935d",
  "external_language": null
}`

const churnModelJSON = `{
  "name": "churn",
  "catalog_name": "ml",
  "schema_name": "models",
  "storage_location": "file:///tmp/uc-ml/__unitystorage/catalogs/53bdc6a5-d727-4460-b703-f5ea5eafb527/models/a89ad57e-58e2-4c72-89fa-52f382eaeb0f",
  "full_name": "ml.models.churn",
  "comment": "Customer churn classifier",
  "owner": null,
  "created_at": 1788835354697,
  "created_by": null,
  "updated_at": 1788835354697,
  "updated_by": null,
  "id": "a89ad57e-58e2-4c72-89fa-52f382eaeb0f"
}`

const churnVersion1JSON = `{
  "model_name": "churn",
  "catalog_name": "ml",
  "schema_name": "models",
  "version": 1,
  "source": "file:///tmp/uc-ml/src/churn-1",
  "run_id": "run-1",
  "status": "PENDING_REGISTRATION",
  "storage_location": "file:///tmp/uc-ml/__unitystorage/catalogs/53bdc6a5-d727-4460-b703-f5ea5eafb527/models/a89ad57e-58e2-4c72-89fa-52f382eaeb0f/versions/f006c0aa-83b1-4430-947a-1ef49ea9032a",
  "comment": "first cut",
  "created_at": 1788835372128,
  "updated_at": 1788835372128,
  "id": "f006c0aa-83b1-4430-947a-1ef49ea9032a"
}`

const churnVersion2JSON = `{
  "model_name": "churn",
  "catalog_name": "ml",
  "schema_name": "models",
  "version": 2,
  "source": "file:///tmp/uc-ml/src/churn-2",
  "run_id": "run-2",
  "status": "READY",
  "comment": "second cut",
  "created_at": 1788835372266,
  "updated_at": 1788835372266,
  "id": "8ff649b2-109e-4088-9164-39f5fb4711eb"
}`

// paymentsTableJSON carries table_constraints in the shape the Databricks
// API documents. The OSS server drops constraints on create and never
// returns them, so this shape is only ever seen from Databricks.
const paymentsTableJSON = `{
  "name": "payments",
  "catalog_name": "shop",
  "schema_name": "sales",
  "table_type": "MANAGED",
  "data_source_format": "DELTA",
  "columns": [
    {"name": "id", "type_text": "int", "type_name": "INT", "position": 0, "nullable": false, "partition_index": null},
    {"name": "order_id", "type_text": "int", "type_name": "INT", "position": 1, "nullable": false, "partition_index": null}
  ],
  "storage_location": "gs://marmot-gcs-lake/payments",
  "comment": null,
  "properties": {},
  "owner": "alice@example.com",
  "created_at": 1788835354299,
  "updated_at": 1788835354299,
  "table_id": "e03e8439-3c51-4678-9d87-f5c73e9f73e6",
  "view_definition": null,
  "table_constraints": [
    {"primary_key_constraint": {"name": "pk_payments", "child_columns": ["id"]}},
    {"foreign_key_constraint": {"name": "fk_payments_order", "child_columns": ["order_id"], "parent_table": "shop.sales.orders", "parent_columns": ["id"]}}
  ]
}`
