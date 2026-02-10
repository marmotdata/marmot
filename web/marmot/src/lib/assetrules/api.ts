import { fetchApi } from '../api';
import type {
	AssetRule,
	AssetRulesListResponse,
	CreateAssetRuleInput,
	UpdateAssetRuleInput,
	RulePreviewResponse,
	RuleAssetsResponse
} from './types';

export async function listAssetRules(
	offset: number = 0,
	limit: number = 50
): Promise<AssetRulesListResponse> {
	const response = await fetchApi(`/asset-rules/list?offset=${offset}&limit=${limit}`);
	if (!response.ok) {
		throw new Error('Failed to list asset rules');
	}
	return response.json();
}

export async function searchAssetRules(
	query: string = '',
	offset: number = 0,
	limit: number = 50
): Promise<AssetRulesListResponse> {
	const params = new URLSearchParams({
		offset: offset.toString(),
		limit: limit.toString()
	});

	if (query) {
		params.append('query', query);
	}

	const response = await fetchApi(`/asset-rules/search?${params.toString()}`);
	if (!response.ok) {
		throw new Error('Failed to search asset rules');
	}
	return response.json();
}

export async function getAssetRule(id: string): Promise<AssetRule> {
	const response = await fetchApi(`/asset-rules/${id}`);
	if (!response.ok) {
		throw new Error('Failed to get asset rule');
	}
	return response.json();
}

export async function createAssetRule(input: CreateAssetRuleInput): Promise<AssetRule> {
	const response = await fetchApi('/asset-rules/', {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'Failed to create asset rule');
	}
	return response.json();
}

export async function updateAssetRule(id: string, input: UpdateAssetRuleInput): Promise<AssetRule> {
	const response = await fetchApi(`/asset-rules/${id}`, {
		method: 'PUT',
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'Failed to update asset rule');
	}
	return response.json();
}

export async function deleteAssetRule(id: string): Promise<void> {
	const response = await fetchApi(`/asset-rules/${id}`, {
		method: 'DELETE'
	});
	if (!response.ok) {
		throw new Error('Failed to delete asset rule');
	}
}

export async function previewAssetRule(input: {
	rule_type: string;
	query_expression?: string;
	metadata_field?: string;
	pattern_type?: string;
	pattern_value?: string;
	limit?: number;
}): Promise<RulePreviewResponse> {
	const response = await fetchApi('/asset-rules/preview', {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'Failed to preview asset rule');
	}
	return response.json();
}

export async function getAssetRuleAssets(
	id: string,
	offset: number = 0,
	limit: number = 50
): Promise<RuleAssetsResponse> {
	const response = await fetchApi(`/asset-rules/assets/${id}?offset=${offset}&limit=${limit}`);
	if (!response.ok) {
		throw new Error('Failed to get asset rule assets');
	}
	return response.json();
}
