import { auth } from './stores/auth';
import { goto } from '$app/navigation';

interface FetchApiOptions extends RequestInit {
	skipAuth?: boolean;
	prefix?: string | null;
}

export async function fetchApi(endpoint: string, options: FetchApiOptions = {}) {
	const { skipAuth = false, prefix = '/api/v1', ...fetchOptions } = options;
	const token = auth.getToken();

	const headers: Record<string, string> = {
		'Content-Type': 'application/json',
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
		goto('/login');
		throw new Error('Unauthorized');
	}

	return response;
}

export interface AssetPreviewResponse {
	column_names: string[];
	rows: any[][];
	total_rows?: number;
}

export async function fetchAssetPreview(assetId: string): Promise<AssetPreviewResponse> {
	const response = await fetchApi(`/assets/preview/${assetId}`);
	if (!response.ok) {
		const errorData = await response.json();
		const error: any = new Error(errorData.error || 'Failed to fetch preview');
		error.status = response.status;
		throw error;
	}
	return await response.json();
}
