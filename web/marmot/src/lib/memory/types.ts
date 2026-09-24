export type MemoryEntityType = 'asset' | 'data_product';

export type MemorySort = 'changed' | 'created';

export interface MemoryAuthor {
	type: string;
	id: string;
	name: string;
}

export interface Memory {
	id: string;
	entity_type: MemoryEntityType;
	entity_id: string;
	content: string;
	created_by: MemoryAuthor;
	session_id?: string;
	updated_by: MemoryAuthor;
	updated_session_id?: string;
	created_at: string;
	updated_at: string;
	score?: number;
}

export interface MemoryList {
	memories: Memory[];
	total: number;
}

export interface MemorySearchResult {
	memories: Memory[];
}
