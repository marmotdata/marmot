// Documentation page types
export type EntityType = 'asset' | 'data_product';

export interface Page {
	id: string;
	entity_type: EntityType;
	entity_id: string;
	parent_id: string | null;
	position: number;
	title: string;
	emoji: string | null;
	content: string | null;
	created_by: string | null;
	created_at: string;
	updated_at: string;
	children?: Page[];
	image_count?: number;
}

export interface ImageMeta {
	id: string;
	page_id: string;
	filename: string;
	content_type: string;
	size_bytes: number;
	url: string;
	created_at: string;
}

export interface StorageStats {
	entity_type: EntityType;
	entity_id: string;
	used_bytes: number;
	max_bytes: number;
	image_count: number;
	page_count: number;
	used_percent: number;
}

export interface PageTree {
	pages: Page[];
	total_pages: number;
	stats: StorageStats;
}

export interface CreatePageInput {
	parent_id?: string | null;
	title: string;
	emoji?: string | null;
	content?: string | null;
}

export interface UpdatePageInput {
	title?: string;
	emoji?: string | null;
	content?: string;
}

export interface MovePageInput {
	parent_id?: string | null;
	position: number;
}

export interface UploadImageInput {
	filename: string;
	content_type: string;
	data: string; // Base64 encoded
}
