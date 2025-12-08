import { fetchApi } from '../api';
import type {
	GlossaryTerm,
	CreateTermInput,
	UpdateTermInput,
	TermsListResponse,
	TermChildrenResponse,
	TermAncestorsResponse
} from './types';

export async function listTerms(
	offset: number = 0,
	limit: number = 20
): Promise<TermsListResponse> {
	const response = await fetchApi(`/glossary/list?offset=${offset}&limit=${limit}`);
	if (!response.ok) {
		throw new Error('Failed to list glossary terms');
	}
	return response.json();
}

export async function searchTerms(
	query: string = '',
	parentTermId: string | null = null,
	offset: number = 0,
	limit: number = 20
): Promise<TermsListResponse> {
	const params = new URLSearchParams({
		offset: offset.toString(),
		limit: limit.toString()
	});

	if (query) {
		params.append('q', query);
	}

	if (parentTermId !== null) {
		params.append('parent_term_id', parentTermId);
	}

	const response = await fetchApi(`/glossary/search?${params.toString()}`);
	if (!response.ok) {
		throw new Error('Failed to search glossary terms');
	}
	return response.json();
}

export async function getTerm(id: string): Promise<GlossaryTerm> {
	const response = await fetchApi(`/glossary/${id}`);
	if (!response.ok) {
		throw new Error('Failed to get glossary term');
	}
	return response.json();
}

export async function getTermByShortName(shortName: string): Promise<GlossaryTerm> {
	const response = await fetchApi(`/glossary/name/${shortName}`);
	if (!response.ok) {
		throw new Error('Failed to get glossary term');
	}
	return response.json();
}

export async function createTerm(input: CreateTermInput): Promise<GlossaryTerm> {
	const response = await fetchApi('/glossary/', {
		method: 'POST',
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'Failed to create glossary term');
	}
	return response.json();
}

export async function updateTerm(id: string, input: UpdateTermInput): Promise<GlossaryTerm> {
	const response = await fetchApi(`/glossary/${id}`, {
		method: 'PUT',
		body: JSON.stringify(input)
	});
	if (!response.ok) {
		const error = await response.json();
		throw new Error(error.error || 'Failed to update glossary term');
	}
	return response.json();
}

export async function deleteTerm(id: string): Promise<void> {
	const response = await fetchApi(`/glossary/${id}`, {
		method: 'DELETE'
	});
	if (!response.ok) {
		throw new Error('Failed to delete glossary term');
	}
}

export async function getChildren(id: string): Promise<TermChildrenResponse> {
	const response = await fetchApi(`/glossary/children/${id}`);
	if (!response.ok) {
		throw new Error('Failed to get term children');
	}
	return response.json();
}

export async function getAncestors(id: string): Promise<TermAncestorsResponse> {
	const response = await fetchApi(`/glossary/ancestors/${id}`);
	if (!response.ok) {
		throw new Error('Failed to get term ancestors');
	}
	return response.json();
}
