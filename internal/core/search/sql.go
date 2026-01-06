package search

// SQL column expressions for building search result metadata JSON objects.
// These are used across multiple search methods (full-text, exact match, trigram).

const assetMetadataColumns = `jsonb_build_object(
	'id', id,
	'name', name,
	'mrn', mrn,
	'type', type,
	'providers', providers,
	'environments', environments,
	'external_links', external_links,
	'description', description,
	'user_description', user_description,
	'metadata', metadata,
	'schema', schema,
	'sources', sources,
	'tags', tags,
	'created_at', created_at,
	'created_by', created_by,
	'updated_at', updated_at,
	'last_sync_at', last_sync_at,
	'query', query,
	'query_language', query_language,
	'is_stub', is_stub
) as metadata`

const assetURLExpr = `'/discover/' || type || '/' || providers[1] || '/' || SUBSTRING(mrn FROM 'mrn://[^/]+/[^/]+/(.+)') as url`

const glossaryMetadataColumns = `jsonb_build_object(
	'id', id,
	'name', name,
	'definition', definition,
	'description', description,
	'parent_term_id', parent_term_id,
	'metadata', metadata,
	'tags', tags,
	'created_at', created_at,
	'updated_at', updated_at
) as metadata`

const glossaryURLExpr = `'/glossary/' || id::text as url`

const teamMetadataColumns = `jsonb_build_object(
	'id', id,
	'name', name,
	'description', description,
	'metadata', metadata,
	'tags', tags,
	'created_via_sso', created_via_sso,
	'sso_provider', sso_provider,
	'created_by', created_by,
	'created_at', created_at,
	'updated_at', updated_at
) as metadata`

const teamURLExpr = `'/teams/' || id as url`

const dataProductMetadataColumns = `jsonb_build_object(
	'id', dp.id,
	'name', dp.name,
	'description', dp.description,
	'icon_url', CASE WHEN pi.id IS NOT NULL THEN '/api/v1/products/images/' || dp.id::text || '/icon' ELSE NULL END,
	'metadata', dp.metadata,
	'tags', dp.tags,
	'created_by', dp.created_by,
	'created_at', dp.created_at,
	'updated_at', dp.updated_at
) as metadata`

const dataProductURLExpr = `'/products/' || dp.id::text as url`
