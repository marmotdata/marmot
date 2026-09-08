The Athena plugin discovers data catalogs, databases, tables, views, workgroups and saved queries from Amazon Athena.

Athena keeps no catalog of its own: the tables it queries live in the Glue Data Catalog. Databases, tables and views are therefore filed under the `Glue` provider with the same names the Glue plugin uses, so an Athena run and a Glue run land on one asset. Workgroups, saved queries and data catalogs are Athena's own objects and are filed under `Athena`.

Databases and tables are read through the Athena metadata API, which also serves federated catalogs. When that API does not answer for a Glue-backed catalog, the plugin reads the Glue Data Catalog directly instead. Set `metadata_api` to `athena` or `glue` to pin the choice.

Query history is not read, so this plugin produces no usage statistics or query-derived table-to-table lineage.

## Required Permissions

<Collapsible
  title="IAM Policy"
  icon="mdi:shield-check"
  policyJson={{
    Version: "2012-10-17",
    Statement: [
      {
        Effect: "Allow",
        Action: [
          "athena:ListDataCatalogs",
          "athena:GetDataCatalog",
          "athena:ListDatabases",
          "athena:ListTableMetadata",
          "athena:ListWorkGroups",
          "athena:GetWorkGroup",
          "athena:ListNamedQueries",
          "athena:BatchGetNamedQuery",
          "athena:GetNamedQuery",
          "athena:ListTagsForResource",
          "glue:GetDatabases",
          "glue:GetTables",
          "sts:GetCallerIdentity"
        ],
        Resource: "*"
      }
    ]
  }}
  minimalPolicyJson={{
    Version: "2012-10-17",
    Statement: [
      {
        Effect: "Allow",
        Action: [
          "athena:ListDataCatalogs",
          "athena:ListDatabases",
          "athena:ListTableMetadata"
        ],
        Resource: "*"
      }
    ]
  }}
/>

`sts:GetCallerIdentity` is only needed with `tags_to_metadata`, because Athena does not return workgroup or catalog ARNs and the plugin builds them from the account id.

## AWS Configuration

See [AWS Configuration](./Shared%20Configuration/AWS%20Configuration.md) for the supported AWS configuration options.
