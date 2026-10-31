import { auth } from './stores/auth';
import { goto } from '$app/navigation';
import { error } from '@sveltejs/kit';
import { resolve } from '$app/paths';
import { m } from '$lib/paraglide/messages';

interface FetchApiOptions extends RequestInit {
	skipAuth?: boolean;
	prefix?: string | null;
}

export async function fetchApi(endpoint: string, options: FetchApiOptions = {}) {
	const { skipAuth = false, prefix = '/api/v1', ...fetchOptions } = options;
	const token = auth.getToken();

	const headers: Record<string, string> = {
		'Content-Type': 'application/json',
		'X-Marmot-Client': 'web',
		...(fetchOptions.headers as Record<string, string>)
	};

	// Only add Authorization header if we have a token and skipAuth is false
	if (token && !skipAuth) {
		headers['Authorization'] = `Bearer ${token}`;
	}

	const url = prefix !== null ? `${prefix}${endpoint}` : endpoint;
	const response = await fetch(url, {
		...fetchOptions,
		headers
	});

	if (response.status === 401 && !skipAuth) {
		auth.clearToken();
		goto(resolve('/login'));
		throw new Error('Unauthorized');
	}

	return response;
}

/**
 * Turns a failed API response into the error page, carrying the server's request reference so a 500 can be quoted to an admin. For load functions; components show failures inline instead.
 */
export async function throwApiError(response: Response): Promise<never> {
	let message = m.toast_request_failed({ status: response.status });
	let requestId = response.headers.get('X-Request-Id') ?? undefined;
	try {
		const data = await response.json();
		if (typeof data.error === 'string') message = data.error;
		if (typeof data.request_id === 'string') requestId = data.request_id;
	} catch {
		// non-JSON body; keep the default message
	}
	error(response.status, { message, requestId });
}

export interface AssetPreviewResponse {
	column_names: string[];
	rows: unknown[][];
	total_rows?: number;
}

interface ApiError extends Error {
	status?: number;
}

export async function fetchAssetPreview(assetId: string): Promise<AssetPreviewResponse> {
	const response = await fetchApi(`/assets/preview/${assetId}`);
	if (!response.ok) {
		const errorData = await response.json();
		const error: ApiError = new Error(errorData.error || m.ui_preview_fetch_error());
		error.status = response.status;
		throw error;
	}
	return await response.json();
}
