// The API answers a refused document with {error, fields: [{path, message}]}; this keeps the fields so the editor can mark them.

import type { DocumentProblem } from './types';

export class DocumentError extends Error {
	readonly status: number;
	readonly fields: DocumentProblem[];

	constructor(message: string, status: number, fields: DocumentProblem[] = []) {
		super(message);
		this.name = 'DocumentError';
		this.status = status;
		this.fields = fields;
	}
}

/** Builds a DocumentError from a failed response, falling back to the given message. */
export async function documentErrorFrom(
	response: Response,
	fallback: string
): Promise<DocumentError> {
	const body = await response.json().catch(() => null);
	const fields = Array.isArray(body?.fields)
		? (body.fields as DocumentProblem[]).filter((field) => typeof field?.message === 'string')
		: [];
	return new DocumentError(body?.error ?? fallback, response.status, fields);
}
