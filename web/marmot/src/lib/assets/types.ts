export interface AssetSource {
  name: string;
  last_sync_at: string;
  properties: Record<string, any>;
  priority: number;
}

export interface Environment {
  name: string;
  path: string;
  metadata: Record<string, any>;
}

export interface Asset {
  id: string;
  name: string;
  mrn: string;
  type: string;
  providers: string[];
  description?: string;
  tags: string[];
  created_at: string;
  updated_at: string;
  created_by: string;
  metadata: Record<string, any>;
  schema: Record<string, any>;
  parent_mrn?: string;
  last_sync_at?: string;
  environments?: Record<string, Environment>;
  sources: AssetSource[];
}

export interface AssetsResponse {
  assets: Asset[];
  total: number;
  limit: number;
  offset: number;
  filters: {
    types: { [key: string]: number };
    providers: { [key: string]: number };
    tags: { [key: string]: number };
  };
}

export interface AssetSummaryResponse {
  types: { [key: string]: number };
  providers: { [key: string]: number };
  tags: { [key: string]: number };
}

export interface MetadataFieldSuggestion {
  field: string;
  type: string;
  example: any;
  count: number;
}

export interface MetadataValueSuggestion {
  value: string;
  count: number;
  example?: Asset;
}

export interface QueryToken {
  text: string;
  type: 'field' | 'operator' | 'value' | 'boolean' | 'text';
  color: string;
}

export interface Filters {
  types: string[];
  providers: string[];
  tags: string[];
  updatedAfter?: string;
}
