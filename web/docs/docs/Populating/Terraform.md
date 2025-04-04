---
sidebar_position: 2
---

Using Terraform with Marmot provides a powerful "Data Catalog as Code" approach, allowing you to define, version control, and automate your data catalog infrastructure, or, integrate with yoru existing infrastructure pipelines.

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
  host    = "http://localhost:8080"  # Marmot API endpoint
  api_key = var.marmot_api_key       # API key for authentication
}
```

## Resources

The Marmot provider offers these primary resources:

- `marmot_asset` - Define data assets in your catalog. [See complete documentation and examples here.](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs/resources/asset)
- `marmot_lineage` Establish data lineage relationships between assets. [See complete documentation and examples here](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs/resources/lineage).

## Learn More

- [Marmot Terraform Provider Documentation](https://registry.terraform.io/providers/marmotdata/marmot/latest/docs)
- [Full Terraform example](https://github.com/marmotdata/terraform-provider-marmot/tree/main/examples/full)
