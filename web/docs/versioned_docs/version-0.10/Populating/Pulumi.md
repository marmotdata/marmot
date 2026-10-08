---
sidebar_position: 3
---

Using Pulumi with the Marmot Terraform provider provides a powerful "Data Catalog as Code" approach, allowing you to define, version control, and automate your data catalog infrastructure, or integrate with your existing infrastructure pipelines, all with the added benefits of your preferred programming language.

## Getting Started

### Setting Up the Provider

First, add the Marmot Terraform provider to your Pulumi project:

```bash
$ pulumi package add terraform-provider marmotdata/marmot
```

Follow the instructions provided to link the generated SDK into your project.

### Using the Provider

After adding the Marmot provider, you can use it in your Pulumi program:

#### Go Example

```go
package main

import (
	"github.com/pulumi/pulumi-terraform-provider/sdks/go/marmot/v3/marmot"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v3/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {
		// Get configuration
		conf := config.New(ctx, "")
		apiKey := conf.RequireSecret("marmotApiKey")

		// Configure the Marmot provider
		provider, err := marmot.NewProvider(ctx, "marmot-provider", &marmot.ProviderArgs{
			Host:   pulumi.String("http://localhost:8080"),
			ApiKey: apiKey,
		})
		if err != nil {
			return err
		}

		// Create a Marmot asset
		databaseAsset, err := marmot.NewAsset(ctx, "customer-database", &marmot.AssetArgs{
			Name:        pulumi.String("customer-database"),
			Type:        pulumi.String("Database"),
			Description: pulumi.String("PostgreSQL database for customer data"),
			Services:    pulumi.StringArray{pulumi.String("PostgreSQL")},
			Tags:        pulumi.StringArray{pulumi.String("database"), pulumi.String("customer")},
			Metadata: pulumi.StringMap{
				"owner":   pulumi.String("data-team"),
				"version": pulumi.String("13.4"),
			},
		}, pulumi.Provider(provider))

		if err != nil {
			return err
		}

		// Export the asset ID and MRN
		ctx.Export("databaseAssetId", databaseAsset.ResourceId)
		ctx.Export("databaseAssetMrn", databaseAsset.Mrn)

		return nil
	})
}
```

#### TypeScript Example

```typescript
import * as pulumi from "@pulumi/pulumi";
import * as marmot from "@pulumi/terraform-provider-marmot";

const config = new pulumi.Config();
const apiKey = config.requireSecret("marmotApiKey");

// Configure the Marmot provider
const provider = new marmot.Provider("marmot-provider", {
  host: "http://localhost:8080",
  apiKey: apiKey,
});

// Create a Marmot asset
const databaseAsset = new marmot.Asset(
  "customer-database",
  {
    name: "customer-database",
    type: "Database",
    description: "PostgreSQL database for customer data",
    services: ["PostgreSQL"],
    tags: ["database", "customer"],
    metadata: {
      owner: "data-team",
      version: "13.4",
    },
  },
  { provider },
);

// Export the asset ID and MRN
export const databaseAssetId = databaseAsset.resourceId;
export const databaseAssetMrn = databaseAsset.mrn;
```

### Core Resources

The Marmot provider offers these primary resources:

#### `marmot.Asset`

Define data assets in your catalog. Refer to the [Terraform provider documentation](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs/resources/asset) for all available configuration options.

#### `marmot.Lineage`

Establish data lineage relationships between assets. Refer to the [Terraform provider documentation](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs/resources/lineage) for all available configuration options.

## Learn More

- [Pulumi Documentation](https://www.pulumi.com/docs/)
- [Pulumi "Any Terraform Provider" Documentation](https://www.pulumi.com/registry/packages/terraform-provider/)
- [Marmot Terraform Provider Documentation](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs)
- [Terraform Provider Repository for Marmot](https://github.com/marmotdata/terraform-provider-marmot)
