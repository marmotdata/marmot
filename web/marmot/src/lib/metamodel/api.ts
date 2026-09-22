import { fetchApi } from '$lib/api';
import type { Asset } from '$lib/assets/types';
import type { MetamodelSchema, MetamodelValidationError } from './types';
import { assetETag, parseIfMatchETag } from './values';

export class MetamodelHttpError extends Error {
	status: number;
	body: unknown;

	constructor(status: number, body: unknown, message: string) {
		super(message);
		this.status = status;
		this.body = body;
	}
}

let cached: Promise<MetamodelSchema> | null = null;

// The server reads its profile once at startup, so one request serves the whole session.
export function fetchMetamodel(): Promise<MetamodelSchema> {
	cached ??= loadMetamodel().catch((err) => {
		cached = null;
		throw err;
	});
	return cached;
}

async function loadMetamodel(): Promise<MetamodelSchema> {
	const response = await fetchApi('/metamodel');
	if (!response.ok) {
		throw new MetamodelHttpError(
			response.status,
			await safeJson(response),
			'Failed to load metamodel'
		);
	}
	return response.json();
}

export async function patchAssetFields(
	id: string,
	version: number,
	fields: Record<string, unknown>
): Promise<Asset> {
	const response = await fetchApi(`/assets/${id}`, {
		method: 'PATCH',
		headers: { 'If-Match': assetETag(version) },
		body: JSON.stringify({ fields })
	});
	if (!response.ok) {
		const body = await safeJson(response);
		throw new MetamodelHttpError(response.status, body, 'Failed to patch governed fields');
	}
	const asset = (await response.json()) as Asset;
	const fromHeader = parseIfMatchETag(response.headers.get('ETag'));
	if (fromHeader != null) asset.version = fromHeader;
	return asset;
}

export function isValidationError(body: unknown): body is MetamodelValidationError {
	return (
		!!body && typeof body === 'object' && Array.isArray((body as MetamodelValidationError).fields)
	);
}

async function safeJson(response: Response): Promise<unknown> {
	try {
		return await response.json();
	} catch {
		return null;
	}
}
