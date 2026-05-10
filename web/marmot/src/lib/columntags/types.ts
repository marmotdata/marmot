export interface ColumnTag {
	id: string;
	asset_id: string;
	column_path: string;
	tag_id: string;
	created_at: string;
	updated_at: string;
}

export type ColumnTagMap = Record<string, ColumnTag[]>;
