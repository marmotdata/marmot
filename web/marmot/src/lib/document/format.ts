// Reading and writing editable documents as JSON or YAML. js-yaml is imported on first use, so a page that never opens a code view never downloads it.

import { m } from '$lib/paraglide/messages';

export type DocumentMode = 'json' | 'yaml';

export type ParseResult =
	{ ok: true; value: unknown } | { ok: false; message: string; line?: number; offset?: number };

async function yaml() {
	return import('js-yaml');
}

/** The line, counted from 1, of a character offset. */
function lineOf(text: string, offset: number): number {
	return text.slice(0, offset).split('\n').length;
}

export async function parseDocument(text: string, mode: DocumentMode): Promise<ParseResult> {
	if (!text.trim()) return { ok: false, message: m.ui_document_empty() };
	if (mode === 'json') {
		try {
			return { ok: true, value: JSON.parse(text) };
		} catch (error) {
			const message = error instanceof Error ? error.message : String(error);
			const position = /position (\d+)/.exec(message);
			const offset = position ? Number(position[1]) : undefined;
			return {
				ok: false,
				message,
				offset,
				line: offset === undefined ? undefined : lineOf(text, offset)
			};
		}
	}
	const { load, CORE_SCHEMA } = await yaml();
	try {
		// The core schema keeps timestamps as strings, which is what the API expects of expires_at.
		return { ok: true, value: load(text, { schema: CORE_SCHEMA }) };
	} catch (error) {
		const mark = (error as { mark?: { line: number; position: number } }).mark;
		const reason = (error as { reason?: string }).reason;
		return {
			ok: false,
			message: reason ?? (error instanceof Error ? error.message : String(error)),
			line: mark ? mark.line + 1 : undefined,
			offset: mark?.position
		};
	}
}

export async function stringifyDocument(value: unknown, mode: DocumentMode): Promise<string> {
	if (mode === 'json') return `${JSON.stringify(value, null, 2)}\n`;
	const { dump } = await yaml();
	// The default schema quotes strings that would read back as another type, such as timestamps.
	return dump(value, { lineWidth: -1, noRefs: true });
}

/** Saves text as a file in the browser. */
export function downloadText(text: string, filename: string, type: string) {
	const url = URL.createObjectURL(new Blob([text], { type }));
	const link = document.createElement('a');
	link.href = url;
	link.download = filename;
	link.click();
	URL.revokeObjectURL(url);
}

export function mimeType(mode: DocumentMode): string {
	return mode === 'json' ? 'application/json' : 'application/yaml';
}

/** The views a form with a code editor offers: its own visual editor and the document as JSON or YAML. */
export type DocumentView = 'visual' | DocumentMode;

export function documentViewOptions(): { id: DocumentView; label: string; icon: string }[] {
	return [
		{ id: 'visual', label: m.ui_document_view_visual(), icon: 'material-symbols:tune' },
		{ id: 'json', label: m.ui_document_view_json(), icon: 'material-symbols:data-object' },
		{ id: 'yaml', label: m.ui_document_view_yaml(), icon: 'material-symbols:code' }
	];
}

/** Copies text to the clipboard, answering whether it worked. */
export async function copyText(text: string): Promise<boolean> {
	try {
		await navigator.clipboard.writeText(text);
		return true;
	} catch {
		return false;
	}
}
