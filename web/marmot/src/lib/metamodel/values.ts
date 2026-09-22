import type { MetamodelField } from './types';

export type Draft = string | string[] | null | undefined;

export type ParseResult = { ok: true; value: unknown } | { ok: false; code: string };

const SCALAR_ITEM_TYPES = ['string', 'integer', 'number', 'boolean', 'date', 'enum'];

export function isMetadataStorage(storage: string): boolean {
	return storage.startsWith('metadata.');
}

export function metadataPath(storage: string): string[] {
	return storage.slice('metadata.'.length).split('.').filter(Boolean);
}

export function readMetadataValue(
	metadata: Record<string, unknown> | undefined,
	storage: string
): unknown {
	if (!metadata || !isMetadataStorage(storage)) return undefined;
	let value: unknown = metadata;
	for (const part of metadataPath(storage)) {
		if (!isPlainObject(value)) return undefined;
		value = value[part];
	}
	return value;
}

/**
 * Profile fields stored in asset metadata: required first, then by section, order and id.
 * Sections follow the order the profile first mentions them; fields without one come last.
 */
export function governedFields(fields: MetamodelField[]): MetamodelField[] {
	const sections: string[] = [];
	for (const field of fields) {
		const section = field.presentation?.section;
		if (section && !sections.includes(section)) sections.push(section);
	}
	const rank = (field: MetamodelField) => {
		const index = sections.indexOf(field.presentation?.section ?? '');
		return index < 0 ? sections.length : index;
	};
	return fields
		.filter((field) => isMetadataStorage(field.storage))
		.sort(
			(a, b) =>
				Number(b.required) - Number(a.required) ||
				rank(a) - rank(b) ||
				(a.presentation?.order ?? 0) - (b.presentation?.order ?? 0) ||
				a.id.localeCompare(b.id)
		);
}

export function governedPaths(fields: MetamodelField[]): string[][] {
	return fields
		.filter((field) => isMetadataStorage(field.storage))
		.map((f) => metadataPath(f.storage));
}

/**
 * Copy of metadata without the governed leaves. A parent that only held governed leaves
 * disappears; free siblings and objects that were already empty stay.
 */
export function omitPaths(
	metadata: Record<string, unknown>,
	paths: string[][]
): Record<string, unknown> {
	if (paths.length === 0) return metadata;
	const out: Record<string, unknown> = {};
	for (const [key, child] of Object.entries(metadata)) {
		const below = paths.filter((path) => path[0] === key);
		if (below.length === 0) {
			out[key] = child;
		} else if (below.some((path) => path.length === 1)) {
			continue;
		} else if (!isPlainObject(child)) {
			out[key] = child;
		} else {
			const kept = omitPaths(
				child,
				below.map((path) => path.slice(1))
			);
			if (Object.keys(kept).length > 0 || Object.keys(child).length === 0) out[key] = kept;
		}
	}
	return out;
}

export function typeLabel(field: MetamodelField): string {
	if (field.presentation?.control === 'user') return 'user';
	return field.type === 'list' ? `list<${field.itemType ?? '?'}>` : field.type;
}

/** An Iconify name for a field's type, shared by the governed table and the blade summary. */
export function typeIcon(field: MetamodelField): string {
	if (field.presentation?.control === 'user') return 'material-symbols:person-outline-rounded';
	switch (field.type) {
		case 'integer':
		case 'number':
			return 'material-symbols:tag-rounded';
		case 'boolean':
			return 'material-symbols:toggle-on-outline-rounded';
		case 'date':
			return 'material-symbols:calendar-today-outline-rounded';
		case 'enum':
			return 'material-symbols:list-alt-outline-rounded';
		case 'list':
			return 'material-symbols:format-list-bulleted-rounded';
		default:
			return 'material-symbols:text-fields-rounded';
	}
}

/** Badge colour for a scalar value, matching the same convention as free-form metadata. */
export function valueClass(value: unknown): string {
	if (typeof value === 'boolean') {
		return value
			? 'bg-green-100 dark:bg-green-900/30 text-green-800 dark:text-green-200'
			: 'bg-red-100 dark:bg-red-900/30 text-red-800 dark:text-red-200';
	}
	if (typeof value === 'number') {
		return 'bg-blue-100 dark:bg-blue-900/30 text-blue-800 dark:text-blue-200';
	}
	return 'bg-gray-100 dark:bg-gray-700 text-gray-800 dark:text-gray-200';
}

/**
 * A profile section id as a heading: "data_quality" -> "Data quality". Sections are not
 * translated (the profile format has no sectionKey), so this only reformats the raw id; an
 * empty section is the caller's business, typically an i18n fallback like "Other".
 */
export function sectionLabel(section: string): string {
	return section
		.split(/[_-]+/)
		.filter(Boolean)
		.map((word, i) => (i === 0 ? word[0].toUpperCase() + word.slice(1) : word))
		.join(' ');
}

export function isUnset(value: unknown): boolean {
	return value === undefined || value === null;
}

/** Editable form of a stored value: text for scalars, a list of text for lists. */
export function draftFromValue(field: MetamodelField, value: unknown): Draft {
	if (field.type === 'list') {
		return Array.isArray(value) ? value.map((item) => String(item)) : [];
	}
	return isUnset(value) ? '' : String(value);
}

export function parseScalar(type: string, raw: string, values?: string[]): ParseResult {
	switch (type) {
		case 'integer':
			return /^-?\d+$/.test(raw.trim())
				? { ok: true, value: Number(raw) }
				: { ok: false, code: 'type' };
		case 'number': {
			const n = Number(raw);
			return raw.trim() !== '' && Number.isFinite(n)
				? { ok: true, value: n }
				: { ok: false, code: 'type' };
		}
		case 'boolean':
			return raw === 'true' || raw === 'false'
				? { ok: true, value: raw === 'true' }
				: { ok: false, code: 'type' };
		case 'date':
			return /^\d{4}-\d{2}-\d{2}$/.test(raw) && !Number.isNaN(Date.parse(raw))
				? { ok: true, value: raw }
				: { ok: false, code: 'date' };
		case 'enum':
			return values?.includes(raw) ? { ok: true, value: raw } : { ok: false, code: 'enum' };
		default:
			return { ok: true, value: raw };
	}
}

/** Mirrors the server constraints so the form can explain a problem before the round trip. */
export function checkConstraints(field: MetamodelField, value: unknown): string | null {
	const rules = field.validation ?? {};
	const check = (item: unknown, type: string): string | null => {
		if (typeof item === 'number') {
			if (rules.minimum != null && item < rules.minimum) return 'range';
			if (rules.maximum != null && item > rules.maximum) return 'range';
		}
		if (typeof item === 'string' && ['string', 'enum', 'date'].includes(type)) {
			const length = [...item].length;
			if (rules.minLength != null && length < rules.minLength) return 'length';
			if (rules.maxLength != null && length > rules.maxLength) return 'length';
		}
		return null;
	};
	if (field.type === 'list') {
		const items = Array.isArray(value) ? value : [];
		if (rules.minItems != null && items.length < rules.minItems) return 'items';
		if (rules.maxItems != null && items.length > rules.maxItems) return 'items';
		for (const item of items) {
			const code = check(item, field.itemType ?? 'string');
			if (code) return code;
		}
		return null;
	}
	return check(value, field.type);
}

/**
 * Turns the editor draft into the value to PATCH. Empty scalars become null only where the
 * profile allows clearing; the server stays the authority on everything else.
 */
export function toPayload(field: MetamodelField, draft: Draft): ParseResult {
	if (field.type === 'list') {
		const itemType = SCALAR_ITEM_TYPES.includes(field.itemType ?? '') ? field.itemType! : 'string';
		const items: unknown[] = [];
		for (const raw of Array.isArray(draft) ? draft : []) {
			const parsed = parseScalar(itemType, raw, field.values);
			if (!parsed.ok) return parsed;
			items.push(parsed.value);
		}
		const code = checkConstraints(field, items);
		return code ? { ok: false, code } : { ok: true, value: items };
	}
	const raw = typeof draft === 'string' ? draft : '';
	if (raw.trim() === '') {
		if (field.required) return { ok: false, code: 'required' };
		if (!field.nullable) return { ok: false, code: 'not_nullable' };
		return { ok: true, value: null };
	}
	const parsed = parseScalar(field.type, field.type === 'string' ? raw : raw.trim(), field.values);
	if (!parsed.ok) return parsed;
	const code = checkConstraints(field, parsed.value);
	return code ? { ok: false, code } : parsed;
}

export function sameValue(a: unknown, b: unknown): boolean {
	return JSON.stringify(a ?? null) === JSON.stringify(b ?? null);
}

export function parseIfMatchETag(header: string | null): number | null {
	if (!header) return null;
	const trimmed = header.trim();
	if (trimmed.length < 3 || trimmed[0] !== '"' || trimmed.at(-1) !== '"') return null;
	const version = Number(trimmed.slice(1, -1));
	return Number.isInteger(version) && version > 0 ? version : null;
}

export function assetETag(version: number): string {
	return `"${version}"`;
}

function isPlainObject(value: unknown): value is Record<string, unknown> {
	return typeof value === 'object' && value !== null && !Array.isArray(value);
}
