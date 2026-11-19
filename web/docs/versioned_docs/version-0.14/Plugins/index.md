# Plugins

Plugins discover the assets in a system and catalog them in Marmot, along with their schemas, owners and lineage where the system has them. A [pipeline](../Populating/Pipelines.md) runs a plugin against a source on a schedule.

The [plugin registry](https://plugins.marmotdata.io) lists every plugin, with its configuration, the permissions it needs and examples.

A plugin isn't the only way to fill the catalog. [Terraform](../Populating/Terraform.md), [Pulumi](../Populating/Pulumi.md) and the [API](../Populating/API.md) describe assets you declare yourself, such as services and APIs no plugin can see.

## Next steps

- [Create a pipeline in the UI](../Populating/UI.md): point a plugin at a source.
- [Build a plugin](../Develop/creating-plugins.md): connect a system that has no plugin yet.
