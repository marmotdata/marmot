import { fetchApi } from '../api';
import type {
	DataProduct,
	DataProductsListResponse,
	CreateDataProductInput,
	UpdateDataProductInput,
	Rule,
	RuleInput,
	RulePreviewResponse,
	ResolvedAssetsResponse,
	AssetsResponse
} from './types';

export async function listDataProducts(
	offset: number = 0,
	limit: number = 50
): Promise<DataProductsListResponse> {
	const response = await fetchApi(`/products/list?offset=${offset}&limit=${limit}`);
	if (!response.ok) {
		throw new Error('Failed to list data products');
	}
	return response.json();
}

export async function searchDataProducts(
	query: string = '',
	offset: number = 0,
	limit: number = 50
): Promise<DataProductsListResponse> {
	const params = new URLSearchParams({
		offset: offset.toString(),
		limit: limit.toString()
	});

	if (query) {
		params.append('q', query);
	}

	const response = await fetchApi(`/products/search?${params.toString()}`);
	if (!response.ok) {
		throw new Error('Failed to search data products');
	}
	return response.json();
}

export async function getDataProduct(id: string): Promise<DataProduct> {
	const response = await fetchApi(`/products/${id}`);
	if (!response.ok) {
		throw new Error('Failed to get data product');
	}
	return response.json();
}

export async function createDataProduct(input: CreateDataProductInput): Promise<DataProduct> {
	const response = await fetchApi('/products/', {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'Failed to create data product');
	}
	return response.json();
}

export async function updateDataProduct(
	id: string,
	input: UpdateDataProductInput
): Promise<DataProduct> {
	const response = await fetchApi(`/products/${id}`, {
		method: 'PUT',
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'Failed to update data product');
	}
	return response.json();
}

export async function deleteDataProduct(id: string): Promise<void> {
	const response = await fetchApi(`/products/${id}`, {
		method: 'DELETE'
	});
	if (!response.ok) {
		throw new Error('Failed to delete data product');
	}
}

export async function getDataProductAssets(
	id: string,
	offset: number = 0,
	limit: number = 50
): Promise<AssetsResponse> {
	const response = await fetchApi(
		`/products/assets/${id}?offset=${offset}&limit=${limit}`
	);
	if (!response.ok) {
		throw new Error('Failed to get data product assets');
	}
	return response.json();
}

export async function addDataProductAssets(id: string, assetIds: string[]): Promise<void> {
	const response = await fetchApi(`/products/assets/${id}`, {
		method: 'POST',
		body: JSON.stringify({ asset_ids: assetIds })
	});
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'Failed to add assets to data product');
	}
}

export async function removeDataProductAsset(id: string, assetId: string): Promise<void> {
	const response = await fetchApi(`/products/assets/${id}/${assetId}`, {
		method: 'DELETE'
	});
	if (!response.ok) {
		throw new Error('Failed to remove asset from data product');
	}
}

export async function getDataProductRules(id: string): Promise<Rule[]> {
	const response = await fetchApi(`/products/rules/${id}`);
	if (!response.ok) {
		throw new Error('Failed to get data product rules');
	}
	const data = await response.json();
	return data.rules || [];
}

export async function createRule(id: string, input: RuleInput): Promise<Rule> {
	const response = await fetchApi(`/products/rules/${id}`, {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'Failed to create rule');
	}
	return response.json();
}

export async function updateRule(id: string, ruleId: string, input: RuleInput): Promise<Rule> {
	const response = await fetchApi(`/products/rules/${id}/${ruleId}`, {
		method: 'PUT',
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'Failed to update rule');
	}
	return response.json();
}

export async function deleteRule(id: string, ruleId: string): Promise<void> {
	const response = await fetchApi(`/products/rules/${id}/${ruleId}`, {
		method: 'DELETE'
	});
	if (!response.ok) {
		throw new Error('Failed to delete rule');
	}
}

export async function previewRule(input: RuleInput): Promise<RulePreviewResponse> {
	const response = await fetchApi('/products/rule-preview', {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'Failed to preview rule');
	}
	return response.json();
}

export async function getResolvedAssets(
	id: string,
	offset: number = 0,
	limit: number = 50
): Promise<ResolvedAssetsResponse> {
	const response = await fetchApi(
		`/products/resolved-assets/${id}?offset=${offset}&limit=${limit}`
	);
	if (!response.ok) {
		throw new Error('Failed to get resolved assets');
	}
	return response.json();
}
