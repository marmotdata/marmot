// Checks a whole document before it leaves its code view, for switching to a visual editor or to the other format. The schema checker is fetched with the editor, so this costs nothing extra once an editor has opened.

import { m } from '$lib/paraglide/messages';
import { parseDocument, type DocumentMode } from './format';
import type { Schema } from './codemirror/schema';

export type CheckResult = { ok: true; value: unknown } | { ok: false; message: string };

export async function checkDocument(
	text: string,
	mode: DocumentMode,
	schema: Schema | null
): Promise<CheckResult> {
	const parsed = await parseDocument(text, mode);
	if (!parsed.ok) {
		return {
			ok: false,
			message: parsed.line
				? m.ui_document_syntax_error_at({ line: parsed.line, message: parsed.message })
				: m.ui_document_syntax_error({ message: parsed.message })
		};
	}
	if (schema) {
		const { validateAgainst } = await import('./codemirror/schema');
		const problems = validateAgainst(schema, parsed.value);
		if (problems.length > 0) {
			return { ok: false, message: m.ui_document_schema_errors({ count: problems.length }) };
		}
	}
	return { ok: true, value: parsed.value };
}

/** A copy of a schema without some top-level properties, for an editor that writes only part of what the API accepts. */
export function omitProperties(schema: Schema, names: string[]): Schema {
	const properties = { ...(schema.properties ?? {}) };
	for (const name of names) delete properties[name];
	return { ...schema, properties };
}
