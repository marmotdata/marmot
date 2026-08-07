package release

// OCI artefact and media types for Marmot plugin releases. Kept in
// lockstep with the plugin registry, which recognises artefacts by
// these strings.
const (
	ArtifactTypePlugin     = "application/vnd.marmot.plugin.v1"
	ArtifactTypePluginInfo = "application/vnd.marmot.plugin.info.v1+json"

	MediaTypePluginGzip   = "application/vnd.marmot.plugin.v1+gzip"
	MediaTypeReadme       = "application/vnd.marmot.plugin.readme.v1+markdown"
	MediaTypeMetadata     = "application/vnd.marmot.plugin.metadata.v1+json"
	MediaTypeAssetSchemas = "application/vnd.marmot.plugin.asset-schemas.v1+json"

	titleAnnotation   = "org.opencontainers.image.title"
	sourceAnnotation  = "org.opencontainers.image.source"
	versionAnnotation = "org.opencontainers.image.version"
)
