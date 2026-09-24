import { auth } from '$lib/stores/auth';
import { fetchApi } from '$lib/api';

export type SheetFormat = 'xlsx' | 'csv';
export type ImportMode = 'validate' | 'apply';
export type OnExisting = 'skip' | 'update';
export type ImportAction = 'create' | 'update' | 'skip' | 'error';

export interface ImportProblem {
	column?: string;
	code: string;
	message: string;
}

export interface ImportRow {
	line: number;
	name: string;
	action: ImportAction;
	errors?: ImportProblem[];
	warnings?: ImportProblem[];
}

export interface ImportResult {
	rows: ImportRow[];
	problems?: ImportProblem[];
	warnings?: ImportProblem[];
	summary: { create: number; update: number; skip: number; errors: number };
	applied: boolean;
}

export class ImportError extends Error {
	constructor(
		readonly status: number,
		message: string
	) {
		super(message);
	}
}

/** Downloads the import template, or the whole glossary in the same columns. */
export async function downloadSheet(
	what: 'template' | 'export',
	format: SheetFormat,
	locale: string
): Promise<void> {
	const path = what === 'template' ? '/glossary/import/template' : '/glossary/export';
	const response = await fetchApi(`${path}?format=${format}&locale=${encodeURIComponent(locale)}`);
	if (!response.ok) {
		const body = await response.json().catch(() => ({}));
		throw new ImportError(response.status, body.error ?? response.statusText);
	}
	const filename =
		/filename="([^"]+)"/.exec(response.headers.get('Content-Disposition') ?? '')?.[1] ??
		`glossary.${format}`;
	const url = URL.createObjectURL(await response.blob());
	const link = document.createElement('a');
	link.href = url;
	link.download = filename;
	link.click();
	URL.revokeObjectURL(url);
}

/**
 * Sends a file to validate or apply. A file with errors comes back as a
 * result either way: 422 on apply only says nothing was written.
 */
export async function importTerms(
	file: File,
	mode: ImportMode,
	onExisting: OnExisting
): Promise<ImportResult> {
	const form = new FormData();
	form.append('file', file, file.name);
	// fetchApi sets a JSON content type; the browser must set the multipart boundary.
	const headers: Record<string, string> = { 'X-Marmot-Client': 'web' };
	const token = auth.getToken();
	if (token) headers['Authorization'] = `Bearer ${token}`;
	const response = await fetch(`/api/v1/glossary/import?mode=${mode}&on_existing=${onExisting}`, {
		method: 'POST',
		body: form,
		headers
	});
	const body = await response.json().catch(() => ({}));
	if (response.ok || response.status === 422) return body as ImportResult;
	throw new ImportError(response.status, body.error ?? response.statusText);
}
