import type { SearchBarField } from '@marmotdata/elements/search-bar.js';
import { providerNames } from '@marmotdata/icons';
import { fetchApi } from '#lib/api.ts';
import { typeNames } from '#lib/assetNames.ts';
import type { MetadataFieldSuggestion, MetadataValueSuggestion } from '#lib/assets/types.ts';
import { m } from '#lib/paraglide/messages.js';
import { getLocale } from '#lib/paraglide/runtime.js';

/** The search result kinds the backend knows, in the order the facets list them. */
export const RESULT_KINDS = ['asset', 'data_product', 'glossary', 'team'] as const;

/** A result type an edition adds to search beyond the catalogue's own, with how Discover, the search bar and the command palette show it. */
export interface SearchKind {
	/** The result type the search API returns, e.g. `task`. */
	value: string;
	/** One result, as a caption reads it: "Task". */
	label: () => string;
	/** Several, as a filter or a palette group reads it: "Tasks". */
	pluralLabel: () => string;
	icon: string;
}

const addedKinds: SearchKind[] = [];

/** Adds a result type to search. Call it once, when the app starts, before any search page renders. */
export function registerSearchKind(kind: SearchKind): void {
	if (!addedKinds.some((k) => k.value === kind.value)) addedKinds.push(kind);
}

/** The result types an edition added, in the order they were registered. */
export function addedSearchKinds(): readonly SearchKind[] {
	return addedKinds;
}

export function addedSearchKind(kind: string): SearchKind | undefined {
	return addedKinds.find((k) => k.value === kind);
}

/** Every result type search can return: the catalogue's own, then any an edition added. */
export function resultKinds(): string[] {
	return [...RESULT_KINDS, ...addedKinds.map((k) => k.value)];
}

export function kindLabel(kind: string): string {
	const labels: Record<string, () => string> = {
		asset: m.discover_kind_asset,
		data_product: m.discover_kind_data_product,
		glossary: m.discover_kind_glossary_term,
		team: m.discover_kind_team
	};
	return (labels[kind] ?? addedSearchKind(kind)?.label)?.() ?? kind;
}

/** A provider key ("postgresql") or stored name ("PostgreSQL") in its usual spelling. */
export function providerLabel(provider: string): string {
	return (providerNames as Record<string, string>)[provider.toLowerCase()] ?? provider;
}

/** An asset type ("KAFKA_TOPIC", "table") in everyday words. */
export function typeLabel(type: string): string {
	const known = typeNames[type.toLowerCase()];
	if (known) return known;
	const words = type.replace(/_/g, ' ').toLowerCase();
	return words.charAt(0).toUpperCase() + words.slice(1);
}

interface FieldValues {
	types?: string[];
	providers?: string[];
	tags?: string[];
}

/** The fields the backend query parser accepts (`@kind`, `@type`, `@provider`, `@name`, `@term`, `@tag`), with everyday labels; any `@metadata.*` path can be typed as well. */
export function catalogFields(known: FieldValues = {}): SearchBarField[] {
	return [
		{
			value: 'kind',
			label: m.discover_field_kind(),
			operators: ['='],
			values: resultKinds().map((k) => ({ value: k, label: kindLabel(k) }))
		},
		{
			value: 'type',
			label: m.common_type(),
			values: known.types?.map((t) => ({ value: t, label: typeLabel(t) }))
		},
		{
			value: 'provider',
			label: m.discover_field_provider(),
			values: known.providers?.map((p) => ({ value: p, label: providerLabel(p) }))
		},
		{ value: 'name', label: m.common_name() },
		{ value: 'term', label: m.discover_field_term(), kind: 'term' },
		{ value: 'tag', label: m.discover_field_tag(), values: known.tags }
	];
}

/** A large count in short form for captions, such as 1.2M; rows only need a sense of scale. */
export function compactNumber(n: number): string {
	try {
		return new Intl.NumberFormat(getLocale(), {
			notation: 'compact',
			maximumFractionDigits: 1
		}).format(n);
	} catch {
		return String(n);
	}
}

let metadataFields: Promise<string[]> | null = null;

/** Metadata paths for the query builder, from the same suggestion endpoint the old query input used; fetched once per page load. */
export function loadMetadataFields(prefix: string): Promise<string[]> {
	metadataFields ??= fetchApi('/assets/suggestions/metadata/fields')
		.then((res) => (res.ok ? (res.json() as Promise<MetadataFieldSuggestion[] | null>) : []))
		.then((data) => (data ?? []).map((f) => f.field))
		.catch(() => {
			metadataFields = null;
			return [];
		});
	const p = prefix.toLowerCase().replace(/^metadata\./, '');
	return metadataFields.then((fields) => fields.filter((f) => f.toLowerCase().includes(p)));
}

/** Value suggestions for a field: tags and glossary terms have their own endpoints, everything else goes to the metadata values endpoint, which also knows kind, type, provider and name. */
export async function loadFieldValues(field: string, prefix: string): Promise<string[]> {
	try {
		if (field === 'tag') {
			const res = await fetchApi(
				`/assets/suggestions/tags?${new URLSearchParams({ prefix, limit: '10' })}`
			);
			const data: unknown = res.ok ? await res.json() : [];
			return Array.isArray(data) ? data.filter((t): t is string => typeof t === 'string') : [];
		}
		if (field === 'term') {
			const res = await fetchApi(`/glossary/search?q=${encodeURIComponent(prefix)}`);
			const data: { terms?: { name: string }[] } = res.ok ? await res.json() : {};
			return (data.terms ?? []).map((t) => t.name).slice(0, 10);
		}
		const params = new URLSearchParams({
			field: field.replace(/^metadata\./, ''),
			prefix: prefix.trim(),
			limit: '10'
		});
		const res = await fetchApi(`/assets/suggestions/metadata/values?${params}`);
		const data: MetadataValueSuggestion[] | null = res.ok ? await res.json() : [];
		return (data ?? []).map((v) => String(v.value)).filter(Boolean);
	} catch {
		return [];
	}
}
